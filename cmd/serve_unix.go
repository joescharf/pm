//go:build !windows

package cmd

import (
	"os"
	"os/exec"
	"syscall"
)

// setDaemonAttrs detaches the child process into its own session on Unix.
func setDaemonAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// shutdownSignals returns the OS signals to listen for graceful shutdown.
func shutdownSignals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM}
}

// sigTERM returns the termination signal for the platform.
func sigTERM() syscall.Signal { return syscall.SIGTERM }

// sigKILL returns the kill signal for the platform.
func sigKILL() syscall.Signal { return syscall.SIGKILL }
