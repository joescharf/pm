package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/joescharf/pm/internal/git"
	"github.com/joescharf/pm/internal/output"
	"github.com/joescharf/pm/internal/refresh"
	"github.com/joescharf/pm/internal/store"
)

// Package-level shared dependencies, initialized in cobra.OnInitialize.
var (
	ui      *output.UI
	dataStore store.Store

	verbose bool
	dryRun  bool
)

var rootCmd = &cobra.Command{
	Use:   "pm",
	Short: "Project Manager - track projects, issues, and AI agents",
	Long: `pm manages multiple AI-based app development projects.
It tracks projects, issues, agent sessions, and provides a dashboard
for managing parallel development across multiple repos.`,
	SilenceUsage:      true,
	SilenceErrors:     true,
	DisableAutoGenTag: true,
}

// Execute is the main entry point called from main.go.
func Execute(version, commit, date string) {
	buildVersion = version
	buildCommit = commit
	buildDate = date

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig, initDeps)

	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return rootRun(cmd)
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "Show what would happen without making changes")
	rootCmd.PersistentFlags().String("config", "", "Config file (default ~/.config/pm/config.yaml)")
}

func initConfig() {
	// If --config is explicitly set, use that file
	if cfgFile, _ := rootCmd.PersistentFlags().GetString("config"); cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot find home directory: %v\n", err)
			os.Exit(1)
		}

		configDir := filepath.Join(home, ".config", "pm")
		viper.AddConfigPath(configDir)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.SetEnvPrefix("PM")
	viper.AutomaticEnv()

	// Defaults via viper.SetDefault()
	home, _ := os.UserHomeDir()
	defaultConfigDir := filepath.Join(home, ".config", "pm")

	viper.SetDefault("state_dir", defaultConfigDir)
	viper.SetDefault("db_path", filepath.Join(defaultConfigDir, "pm.db"))
	viper.SetDefault("github.default_org", "")
	viper.SetDefault("agent.model", "opus")
	viper.SetDefault("agent.auto_launch", false)
	viper.SetDefault("anthropic.api_key", "")
	viper.SetDefault("anthropic.model", "claude-haiku-4-5-20251001")
	viper.SetDefault("review.max_attempts", 3)
	viper.SetDefault("review.allowed_tools", "Read Write Edit Glob Grep Bash(git:*) Bash(make:*) Bash(go:*) mcp__pm__*")

	// Read config file if it exists (optional)
	_ = viper.ReadInConfig()
}

func initDeps() {
	ui = output.New()
	ui.Verbose = verbose
	ui.DryRun = dryRun

	// Initialize store lazily — only when commands actually need it.
	// This allows config/version commands to run without a db.
}

// rootRun handles `pm` with no subcommand: detect project from cwd, refresh, and show.
func rootRun(cmd *cobra.Command) error {
	s, err := getStore()
	if err != nil {
		return cmd.Help()
	}

	ctx := context.Background()
	p, err := resolveProjectFromCwd(ctx, s)
	if err != nil {
		return cmd.Help()
	}

	// Best-effort refresh
	gc := git.NewClient()
	ghc := git.NewGitHubClient()
	_, _ = refresh.Project(ctx, s, p, gc, ghc)

	return projectShowRun(p.Name)
}

// getStore returns the shared store, initializing it on first call.
func getStore() (store.Store, error) {
	if dataStore != nil {
		return dataStore, nil
	}

	dbPath := viper.GetString("db_path")
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := s.Migrate(rootCmd.Context()); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	dataStore = s
	return dataStore, nil
}
