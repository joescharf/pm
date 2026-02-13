package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joescharf/pm/internal/daemon"
)

func TestPidFile_Path(t *testing.T) {
	dir := testEnv(t)

	pf := pidFile()
	expected := filepath.Join(dir, "pm-serve.pid")
	assert.Equal(t, expected, pf.Path)
}

func TestServeLogPath(t *testing.T) {
	dir := testEnv(t)

	logPath := serveLogPath()
	expected := filepath.Join(dir, "pm-serve.log")
	assert.Equal(t, expected, logPath)
}

func TestServeStatusRun_NotRunning(t *testing.T) {
	testEnv(t)

	// No PID file exists, so status should show "not running" without error.
	err := serveStatusRun()
	assert.NoError(t, err)
}

func TestServeStopRun_NotRunning(t *testing.T) {
	testEnv(t)

	// No PID file exists, so stop should return an error.
	err := serveStopRun()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestServeStartRun_AlreadyRunning(t *testing.T) {
	dir := testEnv(t)

	// Write a PID file for the current process (which is alive).
	pf := daemon.NewPIDFile(filepath.Join(dir, "pm-serve.pid"))
	require.NoError(t, pf.Write())
	t.Cleanup(func() { _ = os.Remove(pf.Path) })

	err := serveStartRun()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}
