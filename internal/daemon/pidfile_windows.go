//go:build windows

package daemon

import (
	"fmt"
	"os"
	"syscall"
)

// IsRunning checks if the PID file exists and the process is alive.
// On Windows, uses os.FindProcess + a zero signal equivalent.
func (p *PIDFile) IsRunning() (int, bool) {
	pid, err := p.Read()
	if err != nil {
		return 0, false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return pid, false
	}
	// On Windows, FindProcess always succeeds; test with Signal(0) equivalent.
	err = proc.Signal(syscall.Signal(0))
	return pid, err == nil
}

// Signal sends the given signal to the process in the PID file.
// On Windows, only SIGKILL (os.Kill) is reliably supported.
func (p *PIDFile) Signal(sig syscall.Signal) error {
	pid, err := p.Read()
	if err != nil {
		return fmt.Errorf("read PID file: %w", err)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}
	return proc.Signal(sig)
}
