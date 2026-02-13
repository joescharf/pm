package daemon

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// PIDFile manages a PID file for daemon process tracking.
type PIDFile struct {
	Path string
}

// NewPIDFile creates a PIDFile manager for the given path.
func NewPIDFile(path string) *PIDFile {
	return &PIDFile{Path: path}
}

// Write writes the current process's PID to the file.
func (p *PIDFile) Write() error {
	return p.WritePID(os.Getpid())
}

// WritePID writes the given PID to the file.
func (p *PIDFile) WritePID(pid int) error {
	return os.WriteFile(p.Path, []byte(strconv.Itoa(pid)+"\n"), 0o644)
}

// Read reads the PID from the file.
func (p *PIDFile) Read() (int, error) {
	data, err := os.ReadFile(p.Path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID file content: %w", err)
	}
	return pid, nil
}

// Remove deletes the PID file.
func (p *PIDFile) Remove() error {
	return os.Remove(p.Path)
}
