//go:build !windows

package desktopctl

import "os/exec"

// setSysProcAttr is a no-op on non-Windows platforms.
func setSysProcAttr(cmd *exec.Cmd) {
	// No special attributes needed on Unix-like systems
}
