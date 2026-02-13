package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/joescharf/pm/internal/api"
	"github.com/joescharf/pm/internal/daemon"
	"github.com/joescharf/pm/internal/git"
	pmcp "github.com/joescharf/pm/internal/mcp"
	"github.com/joescharf/pm/internal/refresh"
	embedui "github.com/joescharf/pm/internal/ui"
	"github.com/joescharf/pm/internal/wt"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the web UI and API server",
	Long:  "Start an HTTP server serving the REST API, embedded web UI, and MCP server.\nBy default it listens on port 8080 (API/UI) and 8081 (MCP).\n\nUse subcommands (start, stop, restart, status) for background management.",
	RunE: func(cmd *cobra.Command, args []string) error {
		isDaemon := viper.GetBool("daemon")
		return serveRun(cmd.Context(), isDaemon)
	},
}

var serveStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the server in the background",
	RunE: func(cmd *cobra.Command, args []string) error {
		return serveStartRun()
	},
}

var serveStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the background server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return serveStopRun()
	},
}

var serveRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the background server",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Stop (ignore error if not running).
		_ = serveStopRun()
		return serveStartRun()
	},
}

var serveStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show background server status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return serveStatusRun()
	},
}

// pidFile returns the PIDFile manager for the serve daemon.
func pidFile() *daemon.PIDFile {
	dir, err := configDirFunc()
	if err != nil {
		dir = os.TempDir()
	}
	return daemon.NewPIDFile(filepath.Join(dir, "pm-serve.pid"))
}

// serveLogPath returns the path to the daemon log file.
func serveLogPath() string {
	dir, err := configDirFunc()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "pm-serve.log")
}

// serveDaemonEnv is set by the parent process when spawning the daemon child.
// This distinguishes "user ran --daemon" (needs to fork) from "I am the child" (run server).
const serveDaemonEnv = "_PM_SERVE_DAEMON"

// serveRun runs the HTTP server, optionally in daemon mode.
func serveRun(ctx context.Context, isDaemon bool) error {
	// --daemon was passed: if we are NOT the child process, fork one and exit.
	if isDaemon && os.Getenv(serveDaemonEnv) != "1" {
		return serveStartRun()
	}

	isChild := os.Getenv(serveDaemonEnv) == "1"

	port := viper.GetInt("port")
	mcpEnabled := viper.GetBool("mcp")
	mcpPort := viper.GetInt("mcp_port")

	s, err := getStore()
	if err != nil {
		return err
	}

	gc := git.NewClient()
	ghc := git.NewGitHubClient()
	wtc := wt.NewClient()

	// Refresh all projects in the background.
	go func() {
		if _, rerr := refresh.All(context.Background(), s, gc, ghc); rerr != nil {
			ui.Warning("Background refresh: %v", rerr)
		}
	}()

	// Create API server.
	apiServer := api.NewServer(s, gc, ghc, wtc)

	// Create UI handler.
	uiHandler, err := embedui.Handler()
	if err != nil {
		return fmt.Errorf("failed to initialize UI handler: %w", err)
	}

	// Mount API routes and UI.
	mux := http.NewServeMux()
	mux.Handle("/api/", apiServer.Router())
	mux.Handle("/", uiHandler)

	addr := fmt.Sprintf(":%d", port)
	url := fmt.Sprintf("http://localhost%s", addr)
	ui.Info("Serving API at %s/api/v1/", url)
	ui.Info("Serving UI at %s", url)

	// Start MCP StreamableHTTP server concurrently.
	if mcpEnabled {
		mcpSrv := pmcp.NewServer(s, gc, ghc, wtc)
		httpMCP := server.NewStreamableHTTPServer(mcpSrv.MCPServer())
		mcpAddr := fmt.Sprintf(":%d", mcpPort)
		mcpURL := fmt.Sprintf("http://localhost%s/mcp", mcpAddr)
		ui.Info("Serving MCP at %s", mcpURL)

		go func() {
			if merr := httpMCP.Start(mcpAddr); merr != nil {
				ui.Warning("MCP server error: %v", merr)
			}
		}()
	}

	// Daemon child: write PID file, skip browser.
	// Foreground: open browser.
	if isChild {
		pf := pidFile()
		if err := pf.Write(); err != nil {
			return fmt.Errorf("write PID file: %w", err)
		}
		defer func() { _ = pf.Remove() }()
	} else {
		openBrowser(url)
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(ctx, shutdownSignals()...)
	defer stop()

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start server in a goroutine.
	errCh := make(chan error, 1)
	go func() {
		if serr := srv.ListenAndServe(); serr != nil && serr != http.ErrServerClosed {
			errCh <- serr
		}
		close(errCh)
	}()

	// Wait for shutdown signal or server error.
	select {
	case <-ctx.Done():
		ui.Info("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown: %w", err)
		}
		ui.Info("Server stopped.")
		return nil
	case err := <-errCh:
		return err
	}
}

// serveStartRun launches the server as a background daemon process.
func serveStartRun() error {
	pf := pidFile()
	if _, running := pf.IsRunning(); running {
		return fmt.Errorf("server is already running (PID file: %s)", pf.Path)
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}

	logPath := serveLogPath()
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	args := []string{"serve"}
	port := viper.GetInt("port")
	if port != 8080 {
		args = append(args, "--port", fmt.Sprintf("%d", port))
	}
	if !viper.GetBool("mcp") {
		args = append(args, "--mcp=false")
	}
	mcpPort := viper.GetInt("mcp_port")
	if mcpPort != 8081 {
		args = append(args, "--mcp-port", fmt.Sprintf("%d", mcpPort))
	}

	child := exec.Command(exePath, args...)
	child.Stdout = logFile
	child.Stderr = logFile
	child.Env = append(os.Environ(), serveDaemonEnv+"=1")
	setDaemonAttrs(child)

	if err := child.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("start daemon: %w", err)
	}
	_ = logFile.Close()

	ui.Success("Server started (PID %d)", child.Process.Pid)
	ui.Info("  Log: %s", logPath)
	ui.Info("  URL: http://localhost:%d", port)
	return nil
}

// serveStopRun stops the background daemon process.
func serveStopRun() error {
	pf := pidFile()
	pid, running := pf.IsRunning()
	if !running {
		return fmt.Errorf("server is not running")
	}

	// Send SIGTERM.
	if err := pf.Signal(sigTERM()); err != nil {
		return fmt.Errorf("send SIGTERM: %w", err)
	}

	// Wait up to 10 seconds for graceful shutdown.
	for i := 0; i < 100; i++ {
		time.Sleep(100 * time.Millisecond)
		if _, still := pf.IsRunning(); !still {
			ui.Success("Server stopped (was PID %d)", pid)
			_ = pf.Remove()
			return nil
		}
	}

	// Force kill.
	ui.Warning("Graceful shutdown timed out, force killing PID %d", pid)
	if err := pf.Signal(sigKILL()); err != nil {
		return fmt.Errorf("send SIGKILL: %w", err)
	}
	_ = pf.Remove()
	ui.Success("Server killed (PID %d)", pid)
	return nil
}

// serveStatusRun shows the status of the background daemon.
func serveStatusRun() error {
	pf := pidFile()
	pid, running := pf.IsRunning()
	if !running {
		ui.Info("Server is not running.")
		return nil
	}

	port := viper.GetInt("port")
	ui.Info("Server is running.")
	ui.Info("  PID:  %d", pid)
	ui.Info("  URL:  http://localhost:%d", port)
	ui.Info("  Log:  %s", serveLogPath())
	return nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	_ = cmd.Start()
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.PersistentFlags().IntP("port", "p", 8080, "port to listen on")
	serveCmd.PersistentFlags().Bool("mcp", true, "enable MCP StreamableHTTP server")
	serveCmd.PersistentFlags().Int("mcp-port", 8081, "MCP server port")

	serveCmd.Flags().BoolP("daemon", "d", false, "run server in the background")

	viper.SetDefault("port", 8080)
	viper.SetDefault("mcp", true)
	viper.SetDefault("mcp_port", 8081)
	viper.SetDefault("daemon", false)

	_ = viper.BindPFlag("port", serveCmd.PersistentFlags().Lookup("port"))
	_ = viper.BindPFlag("mcp", serveCmd.PersistentFlags().Lookup("mcp"))
	_ = viper.BindPFlag("mcp_port", serveCmd.PersistentFlags().Lookup("mcp-port"))
	_ = viper.BindPFlag("daemon", serveCmd.Flags().Lookup("daemon"))

	serveCmd.AddCommand(serveStartCmd)
	serveCmd.AddCommand(serveStopCmd)
	serveCmd.AddCommand(serveRestartCmd)
	serveCmd.AddCommand(serveStatusCmd)
}
