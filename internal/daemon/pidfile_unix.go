//go:build !windows

package daemon

import (
	"fmt"
	"syscall"
)

// IsRunning checks if the PID file exists and the process is alive.
// Returns the PID and whether the process is running.
func (p *PIDFile) IsRunning() (int, bool) {
	pid, err := p.Read()
	if err != nil {
		return 0, false
	}
	// Signal 0 tests if the process exists without sending a signal.
	err = syscall.Kill(pid, 0)
	return pid, err == nil
}

// Signal sends the given signal to the process in the PID file.
func (p *PIDFile) Signal(sig syscall.Signal) error {
	pid, err := p.Read()
	if err != nil {
		return fmt.Errorf("read PID file: %w", err)
	}
	return syscall.Kill(pid, sig)
}
