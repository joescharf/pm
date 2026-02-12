package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP stdio server for Claude Code integration",
	Long: `Start an MCP (Model Context Protocol) server on stdio.

This allows Claude Code to query pm natively for project status,
issues, and health scores. Configure in Claude Code with:

  {
    "mcpServers": {
      "pm": { "command": "pm", "args": ["mcp"] }
    }
  }

Available tools: pm_list_projects, pm_project_status, pm_list_issues,
pm_create_issue, pm_update_issue, pm_launch_agent, pm_health_score`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// MCP server implementation deferred to Phase 7.
		// Will use github.com/mark3labs/mcp-go when implemented.
		fmt.Fprintln(ui.Out, "MCP server not yet implemented. Coming soon.")
		fmt.Fprintln(ui.Out, "See: https://github.com/mark3labs/mcp-go")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
