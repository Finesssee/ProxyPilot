//go:build windows

package desktopctl

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func isProcessAlive(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// If we can't open it, assume it's not ours / not running.
		return false, nil
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false, fmt.Errorf("GetExitCodeProcess: %w", err)
	}
	// STILL_ACTIVE == 259
	return code == 259, nil
}
