package daemon

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPIDFile_WriteAndRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")
	pf := NewPIDFile(path)

	err := pf.WritePID(12345)
	require.NoError(t, err)

	pid, err := pf.Read()
	require.NoError(t, err)
	assert.Equal(t, 12345, pid)
}

func TestPIDFile_Write_CurrentPID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")
	pf := NewPIDFile(path)

	err := pf.Write()
	require.NoError(t, err)

	pid, err := pf.Read()
	require.NoError(t, err)
	assert.Equal(t, os.Getpid(), pid)
}

func TestPIDFile_Read_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.pid")
	pf := NewPIDFile(path)

	_, err := pf.Read()
	assert.Error(t, err)
}

func TestPIDFile_Read_InvalidContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.pid")
	require.NoError(t, os.WriteFile(path, []byte("not-a-number\n"), 0o644))

	pf := NewPIDFile(path)
	_, err := pf.Read()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid PID file content")
}

func TestPIDFile_Remove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")
	pf := NewPIDFile(path)

	require.NoError(t, pf.WritePID(1))

	err := pf.Remove()
	require.NoError(t, err)

	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestPIDFile_Remove_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.pid")
	pf := NewPIDFile(path)

	err := pf.Remove()
	assert.Error(t, err)
}

func TestPIDFile_IsRunning_CurrentProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")
	pf := NewPIDFile(path)

	require.NoError(t, pf.Write())

	pid, running := pf.IsRunning()
	assert.True(t, running)
	assert.Equal(t, os.Getpid(), pid)
}

func TestPIDFile_IsRunning_DeadProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")
	pf := NewPIDFile(path)

	// Use a very high PID that almost certainly doesn't exist.
	require.NoError(t, pf.WritePID(999999))

	pid, running := pf.IsRunning()
	// PID is read regardless.
	assert.Equal(t, 999999, pid)
	assert.False(t, running)
}

func TestPIDFile_IsRunning_NoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.pid")
	pf := NewPIDFile(path)

	pid, running := pf.IsRunning()
	assert.Equal(t, 0, pid)
	assert.False(t, running)
}

func TestPIDFile_Signal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")
	pf := NewPIDFile(path)

	require.NoError(t, pf.Write())

	// Signal 0 just checks if process exists, doesn't actually send a signal.
	err := pf.Signal(syscall.Signal(0))
	assert.NoError(t, err)
}

func TestPIDFile_Signal_NoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.pid")
	pf := NewPIDFile(path)

	err := pf.Signal(syscall.Signal(0))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read PID file")
}
