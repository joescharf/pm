package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/joescharf/pm/internal/git"
	pmcp "github.com/joescharf/pm/internal/mcp"
	"github.com/joescharf/pm/internal/wt"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP server for Claude Code integration",
	Long: `Start an MCP (Model Context Protocol) server on stdio.

This allows Claude Code to query pm natively for project status,
issues, and health scores. Configure in Claude Code with:

  pm mcp install

Or manually add to ~/.claude.json:

  {
    "mcpServers": {
      "pm": { "command": "/path/to/pm", "args": ["mcp"] }
    }
  }

Available tools: pm_list_projects, pm_project_status, pm_list_issues,
pm_create_issue, pm_update_issue, pm_launch_agent, pm_health_score`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return mcpServeRun()
	},
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP stdio server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return mcpServeRun()
	},
}

var mcpInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install pm as an MCP server in Claude Code",
	Long:  "Write the MCP server configuration to ~/.claude.json so Claude Code can use pm tools.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return mcpInstallRun()
	},
}

var mcpStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check MCP server installation status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return mcpStatusRun()
	},
}

func init() {
	mcpCmd.AddCommand(mcpServeCmd)
	mcpCmd.AddCommand(mcpInstallCmd)
	mcpCmd.AddCommand(mcpStatusCmd)
	rootCmd.AddCommand(mcpCmd)
}

func mcpServeRun() error {
	s, err := getStore()
	if err != nil {
		return err
	}

	gc := git.NewClient()
	ghc := git.NewGitHubClient()
	wtc := wt.NewClient()

	srv := pmcp.NewServer(s, gc, ghc, wtc)
	return srv.ServeStdio(context.Background())
}

func mcpInstallRun() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	claudeJSON := filepath.Join(home, ".claude.json")

	// Get the full path to the current executable
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	// Read existing config or start fresh
	config := make(map[string]any)
	if data, err := os.ReadFile(claudeJSON); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("parse %s: %w", claudeJSON, err)
		}
	}

	// Merge mcpServers.pm entry
	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		servers = make(map[string]any)
	}
	servers["pm"] = map[string]any{
		"command": exePath,
		"args":    []string{"mcp"},
	}
	config["mcpServers"] = servers

	// Write back
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if dryRun {
		ui.DryRunMsg("Would write to %s:\n%s", claudeJSON, string(data))
		return nil
	}

	if err := os.WriteFile(claudeJSON, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", claudeJSON, err)
	}

	ui.Success("Installed pm MCP server in %s", claudeJSON)
	ui.Info("  Command: %s mcp", exePath)
	ui.Info("  Restart Claude Code to pick up the change.")
	return nil
}

func mcpStatusRun() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	// Check ~/.claude.json
	claudeJSON := filepath.Join(home, ".claude.json")
	checkMCPConfig("~/.claude.json", claudeJSON)

	// Check .mcp.json in cwd
	cwd, _ := os.Getwd()
	if cwd != "" {
		mcpJSON := filepath.Join(cwd, ".mcp.json")
		checkMCPConfig(".mcp.json (cwd)", mcpJSON)
	}

	return nil
}

func checkMCPConfig(label, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		ui.Info("%s: not found", label)
		return
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		ui.Warning("%s: invalid JSON", label)
		return
	}

	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		ui.Info("%s: no mcpServers configured", label)
		return
	}

	pm, ok := servers["pm"]
	if !ok {
		ui.Info("%s: pm not configured (other servers present)", label)
		return
	}

	pmConfig, ok := pm.(map[string]any)
	if !ok {
		ui.Warning("%s: pm entry has unexpected format", label)
		return
	}

	cmd, _ := pmConfig["command"].(string)
	ui.Success("%s: pm configured (command: %s)", label, cmd)
}
