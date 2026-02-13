package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/joescharf/pm/internal/api"
	"github.com/joescharf/pm/internal/git"
	pmcp "github.com/joescharf/pm/internal/mcp"
	"github.com/joescharf/pm/internal/refresh"
	embedui "github.com/joescharf/pm/internal/ui"
	"github.com/joescharf/pm/internal/wt"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the web UI and API server",
	Long:  "Start an HTTP server serving the REST API, embedded web UI, and MCP server.\nBy default it listens on port 8080 (API/UI) and 8081 (MCP).",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// Refresh all projects in the background
		go func() {
			if _, err := refresh.All(context.Background(), s, gc, ghc); err != nil {
				ui.Warning("Background refresh: %v", err)
			}
		}()

		// Create API server
		apiServer := api.NewServer(s, gc, ghc, wtc)

		// Create UI handler
		uiHandler, err := embedui.Handler()
		if err != nil {
			return fmt.Errorf("failed to initialize UI handler: %w", err)
		}

		// Mount API routes and UI
		mux := http.NewServeMux()
		mux.Handle("/api/", apiServer.Router())
		mux.Handle("/", uiHandler)

		addr := fmt.Sprintf(":%d", port)
		url := fmt.Sprintf("http://localhost%s", addr)
		ui.Info("Serving API at %s/api/v1/", url)
		ui.Info("Serving UI at %s", url)

		// Start MCP StreamableHTTP server concurrently
		if mcpEnabled {
			mcpSrv := pmcp.NewServer(s, gc, ghc, wtc)
			httpMCP := server.NewStreamableHTTPServer(mcpSrv.MCPServer())
			mcpAddr := fmt.Sprintf(":%d", mcpPort)
			mcpURL := fmt.Sprintf("http://localhost%s/mcp", mcpAddr)
			ui.Info("Serving MCP at %s", mcpURL)

			go func() {
				if err := httpMCP.Start(mcpAddr); err != nil {
					ui.Warning("MCP server error: %v", err)
				}
			}()
		}

		// Open browser
		openBrowser(url)

		return http.ListenAndServe(addr, mux)
	},
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

	serveCmd.Flags().IntP("port", "p", 8080, "port to listen on")
	serveCmd.Flags().Bool("mcp", true, "enable MCP StreamableHTTP server")
	serveCmd.Flags().Int("mcp-port", 8081, "MCP server port")

	viper.SetDefault("port", 8080)
	viper.SetDefault("mcp", true)
	viper.SetDefault("mcp_port", 8081)

	_ = viper.BindPFlag("port", serveCmd.Flags().Lookup("port"))
	_ = viper.BindPFlag("mcp", serveCmd.Flags().Lookup("mcp"))
	_ = viper.BindPFlag("mcp_port", serveCmd.Flags().Lookup("mcp-port"))
}
