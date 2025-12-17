//go:build !windows

package desktopctl

import (
	"os"
	"syscall"
)

func isProcessAlive(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}
	// Signal 0 checks existence on Unix-y platforms.
	if err := p.Signal(syscall.Signal(0)); err != nil {
		return false, nil
	}
	return true, nil
}
