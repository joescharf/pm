package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var configForce bool

// configDirFunc returns the config directory path, replaceable in tests.
var configDirFunc = defaultConfigDir

func defaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "pm"), nil
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or manage configuration",
	Long: `Show or manage pm configuration.

Running bare 'pm config' is the same as 'pm config show'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return configShowRun()
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create config file with commented defaults",
	RunE: func(cmd *cobra.Command, args []string) error {
		return configInitRun()
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show effective configuration with sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		return configShowRun()
	},
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open config file in $EDITOR",
	RunE: func(cmd *cobra.Command, args []string) error {
		return configEditRun()
	},
}

func init() {
	configInitCmd.Flags().BoolVar(&configForce, "force", false, "Overwrite existing config file")
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configEditCmd)
	rootCmd.AddCommand(configCmd)
}

// configTemplate is the template for generating config.yaml with comments.
const configTemplate = `# pm configuration
# See: pm config show (for effective values and sources)

# State/data directory (default: ~/.config/pm)
# state_dir: {{ .StateDir }}

# SQLite database path (default: ~/.config/pm/pm.db)
# db_path: {{ .DBPath }}

# GitHub
github:
  # Default GitHub organization for project lookups
  default_org: "{{ .GitHubDefaultOrg }}"

# Agent settings
agent:
  # Claude model to use (default: "opus")
  model: "{{ .AgentModel }}"

  # Auto-launch Claude agent when creating worktrees (default: false)
  auto_launch: {{ .AgentAutoLaunch }}
`

type configTemplateData struct {
	StateDir        string
	DBPath          string
	GitHubDefaultOrg string
	AgentModel      string
	AgentAutoLaunch bool
}

func configFilePath() (string, error) {
	dir, err := configDirFunc()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func configInitRun() error {
	cfgPath, err := configFilePath()
	if err != nil {
		return err
	}

	// Check if file already exists
	if _, err := os.Stat(cfgPath); err == nil {
		if !configForce {
			return fmt.Errorf("config file already exists: %s (use --force to overwrite)", cfgPath)
		}
		ui.Warning("Overwriting existing config file")
	}

	// Build template data from current viper values
	data := configTemplateData{
		StateDir:        viper.GetString("state_dir"),
		DBPath:          viper.GetString("db_path"),
		GitHubDefaultOrg: viper.GetString("github.default_org"),
		AgentModel:      viper.GetString("agent.model"),
		AgentAutoLaunch: viper.GetBool("agent.auto_launch"),
	}

	tmpl, err := template.New("config").Parse(configTemplate)
	if err != nil {
		return fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("template execute error: %w", err)
	}

	if dryRun {
		ui.DryRunMsg("Would create config file: %s", cfgPath)
		fmt.Fprintln(ui.Out)
		fmt.Fprint(ui.Out, buf.String())
		return nil
	}

	// Create config directory
	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(cfgPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	ui.Success("Config file created: %s", cfgPath)
	fmt.Fprintln(ui.Out)
	fmt.Fprint(ui.Out, buf.String())
	return nil
}

// configKeyInfo describes a config key for display purposes.
type configKeyInfo struct {
	Key    string
	EnvVar string
}

var configKeys = []configKeyInfo{
	{Key: "state_dir", EnvVar: "PM_STATE_DIR"},
	{Key: "db_path", EnvVar: "PM_DB_PATH"},
	{Key: "github.default_org", EnvVar: "PM_GITHUB_DEFAULT_ORG"},
	{Key: "agent.model", EnvVar: "PM_AGENT_MODEL"},
	{Key: "agent.auto_launch", EnvVar: "PM_AGENT_AUTO_LAUNCH"},
}

func configShowRun() error {
	cfgPath, err := configFilePath()
	if err != nil {
		return err
	}

	// Check if config file exists
	if _, err := os.Stat(cfgPath); err == nil {
		ui.Info("Config file: %s", cfgPath)
	} else {
		ui.Info("Config file: (none)")
	}
	fmt.Fprintln(ui.Out)

	// Read config file values to determine file source
	fileValues := readConfigFileValues(cfgPath)

	for _, k := range configKeys {
		val := viper.Get(k.Key)
		source := detectSource(k.Key, k.EnvVar, fileValues)
		fmt.Fprintf(ui.Out, "  %-22s %v  %s\n", k.Key, val, source)
	}

	return nil
}

// readConfigFileValues reads the raw YAML file and returns a flat map of keys present in it.
func readConfigFileValues(path string) map[string]bool {
	result := make(map[string]bool)

	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}

	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return result
	}

	// Flatten nested keys with dot notation
	flattenKeys("", parsed, result)
	return result
}

// flattenKeys recursively flattens a nested map to dot-notation keys.
func flattenKeys(prefix string, m map[string]any, result map[string]bool) {
	for key, val := range m {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		if nested, ok := val.(map[string]any); ok {
			flattenKeys(fullKey, nested, result)
		} else {
			result[fullKey] = true
		}
	}
}

// detectSource determines where a config value is coming from.
func detectSource(key, envVar string, fileValues map[string]bool) string {
	if _, ok := os.LookupEnv(envVar); ok {
		return fmt.Sprintf("(env: %s)", envVar)
	}
	if fileValues[key] {
		return "(file)"
	}
	return "(default)"
}

func configEditRun() error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		return fmt.Errorf("$EDITOR is not set — set it to your preferred editor (e.g. export EDITOR=vim)")
	}

	cfgPath, err := configFilePath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s (run 'pm config init' first)", cfgPath)
	}

	if dryRun {
		ui.DryRunMsg("Would open %s in %s", cfgPath, editor)
		return nil
	}

	editCmd := exec.Command(editor, cfgPath)
	editCmd.Stdin = os.Stdin
	editCmd.Stdout = os.Stdout
	editCmd.Stderr = os.Stderr
	return editCmd.Run()
}
