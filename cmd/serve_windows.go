//go:build windows

package cmd

import (
	"os"
	"os/exec"
	"syscall"
)

// setDaemonAttrs is a no-op on Windows (no Setsid equivalent).
func setDaemonAttrs(_ *exec.Cmd) {}

// shutdownSignals returns the OS signals to listen for graceful shutdown.
func shutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

// sigTERM returns the termination signal for Windows (os.Kill).
func sigTERM() syscall.Signal { return syscall.SIGTERM }

// sigKILL returns the kill signal for Windows (os.Kill).
func sigKILL() syscall.Signal { return syscall.SIGKILL }
