package cmd

import (
	"fmt"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/joescharf/pm/internal/api"
	"github.com/joescharf/pm/internal/git"
	embedui "github.com/joescharf/pm/internal/ui"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the web UI and API server",
	Long:  "Start an HTTP server serving the REST API and embedded web UI.\nBy default it listens on port 8080. Use --port to change it.",
	RunE: func(cmd *cobra.Command, args []string) error {
		port := viper.GetInt("port")

		s, err := getStore()
		if err != nil {
			return err
		}

		gc := git.NewClient()
		ghc := git.NewGitHubClient()

		// Create API server
		apiServer := api.NewServer(s, gc, ghc)

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
	viper.SetDefault("port", 8080)
	_ = viper.BindPFlag("port", serveCmd.Flags().Lookup("port"))
}
