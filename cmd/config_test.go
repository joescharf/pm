package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joescharf/pm/internal/output"
)

// testEnv sets up isolated config dir, viper, and output for testing.
func testEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Override configDirFunc for tests
	origFunc := configDirFunc
	configDirFunc = func() (string, error) { return dir, nil }
	t.Cleanup(func() { configDirFunc = origFunc })

	// Reset viper
	viper.Reset()
	viper.SetDefault("state_dir", dir)
	viper.SetDefault("db_path", filepath.Join(dir, "pm.db"))
	viper.SetDefault("github.default_org", "")
	viper.SetDefault("agent.model", "opus")
	viper.SetDefault("agent.auto_launch", false)

	// Initialize output
	ui = output.New()

	return dir
}

func TestConfigInit_CreatesFile(t *testing.T) {
	dir := testEnv(t)

	err := configInitRun()
	require.NoError(t, err)

	cfgPath := filepath.Join(dir, "config.yaml")
	_, err = os.Stat(cfgPath)
	assert.NoError(t, err, "config file should exist")

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "pm configuration")
	assert.Contains(t, string(data), "agent")
}

func TestConfigInit_RefusesOverwrite(t *testing.T) {
	dir := testEnv(t)

	// Create existing file
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("existing"), 0644))

	configForce = false
	err := configInitRun()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestConfigInit_ForceOverwrite(t *testing.T) {
	dir := testEnv(t)

	// Create existing file
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("existing"), 0644))

	configForce = true
	err := configInitRun()
	require.NoError(t, err)

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "pm configuration")
}

func TestConfigShow_NoFile(t *testing.T) {
	testEnv(t)

	err := configShowRun()
	assert.NoError(t, err)
}

func TestConfigShow_WithFile(t *testing.T) {
	testEnv(t)

	// Create config first
	require.NoError(t, configInitRun())

	err := configShowRun()
	assert.NoError(t, err)
}

func TestConfigEdit_NoEditor(t *testing.T) {
	testEnv(t)

	// Unset EDITOR and VISUAL
	origEditor := os.Getenv("EDITOR")
	origVisual := os.Getenv("VISUAL")
	_ = os.Unsetenv("EDITOR")
	_ = os.Unsetenv("VISUAL")
	t.Cleanup(func() {
		if origEditor != "" {
			_ = os.Setenv("EDITOR", origEditor)
		}
		if origVisual != "" {
			_ = os.Setenv("VISUAL", origVisual)
		}
	})

	err := configEditRun()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "$EDITOR is not set")
}

func TestConfigEdit_NoConfigFile(t *testing.T) {
	testEnv(t)

	_ = os.Setenv("EDITOR", "echo") // harmless command
	t.Cleanup(func() { _ = os.Unsetenv("EDITOR") })

	err := configEditRun()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDetectSource(t *testing.T) {
	fileValues := map[string]bool{"key_a": true}

	// From env
	os.Setenv("PM_TEST_KEY", "val")
	defer os.Unsetenv("PM_TEST_KEY")
	assert.Contains(t, detectSource("test_key", "PM_TEST_KEY", fileValues), "env")

	// From file
	assert.Contains(t, detectSource("key_a", "PM_KEY_A_NONEXISTENT", fileValues), "file")

	// Default
	assert.Contains(t, detectSource("key_b", "PM_KEY_B_NONEXISTENT", fileValues), "default")
}

func TestFlattenKeys(t *testing.T) {
	input := map[string]any{
		"top": "val",
		"nested": map[string]any{
			"a": "1",
			"b": "2",
		},
	}

	result := make(map[string]bool)
	flattenKeys("", input, result)

	assert.True(t, result["top"])
	assert.True(t, result["nested.a"])
	assert.True(t, result["nested.b"])
	assert.False(t, result["nested"])
}

func TestConfigInit_DryRun(t *testing.T) {
	dir := testEnv(t)
	dryRun = true
	ui.DryRun = true
	defer func() { dryRun = false }()

	err := configInitRun()
	require.NoError(t, err)

	// File should NOT have been created
	cfgPath := filepath.Join(dir, "config.yaml")
	_, err = os.Stat(cfgPath)
	assert.True(t, os.IsNotExist(err), "config file should not exist in dry-run mode")
}
