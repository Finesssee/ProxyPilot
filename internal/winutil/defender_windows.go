//go:build windows

package winutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// DefenderExclusionStatus represents the current state of Windows Defender exclusions.
type DefenderExclusionStatus struct {
	ExeExcluded  bool
	DirExcluded  bool
	PortExcluded bool
	Error        error
}

// CheckDefenderExclusions checks if ProxyPilot paths are excluded from Windows Defender.
func CheckDefenderExclusions(exePath, dataDir string, port int) DefenderExclusionStatus {
	status := DefenderExclusionStatus{}

	// Check if we have admin rights (required to query exclusions)
	if !isAdmin() {
		return status
	}

	// Check executable exclusion
	output, err := runPowerShell("Get-MpPreference | Select-Object -ExpandProperty ExclusionPath")
	if err == nil {
		paths := strings.Split(output, "\n")
		for _, p := range paths {
			p = strings.TrimSpace(p)
			if strings.EqualFold(p, exePath) || strings.EqualFold(p, filepath.Dir(exePath)) {
				status.ExeExcluded = true
			}
			if strings.EqualFold(p, dataDir) {
				status.DirExcluded = true
			}
		}
	}

	// Check port exclusion (if applicable)
	// Note: Windows Defender doesn't directly exclude ports, but we can check process exclusions

	return status
}

// AddDefenderExclusions adds ProxyPilot to Windows Defender exclusions.
// Returns nil on success, error on failure.
// Requires admin privileges.
func AddDefenderExclusions(exePath, dataDir string) error {
	if !isAdmin() {
		return fmt.Errorf("administrator privileges required")
	}

	var errors []string

	// Add executable directory exclusion
	if exePath != "" {
		exeDir := filepath.Dir(exePath)
		cmd := fmt.Sprintf("Add-MpPreference -ExclusionPath '%s'", exeDir)
		if _, err := runPowerShell(cmd); err != nil {
			errors = append(errors, fmt.Sprintf("exe path: %v", err))
		}
	}

	// Add data directory exclusion
	if dataDir != "" {
		cmd := fmt.Sprintf("Add-MpPreference -ExclusionPath '%s'", dataDir)
		if _, err := runPowerShell(cmd); err != nil {
			errors = append(errors, fmt.Sprintf("data dir: %v", err))
		}
	}

	// Add process exclusion
	if exePath != "" {
		cmd := fmt.Sprintf("Add-MpPreference -ExclusionProcess '%s'", exePath)
		if _, err := runPowerShell(cmd); err != nil {
			errors = append(errors, fmt.Sprintf("process: %v", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to add exclusions: %s", strings.Join(errors, "; "))
	}

	return nil
}

// RemoveDefenderExclusions removes ProxyPilot from Windows Defender exclusions.
func RemoveDefenderExclusions(exePath, dataDir string) error {
	if !isAdmin() {
		return fmt.Errorf("administrator privileges required")
	}

	var errors []string

	if exePath != "" {
		exeDir := filepath.Dir(exePath)
		cmd := fmt.Sprintf("Remove-MpPreference -ExclusionPath '%s'", exeDir)
		if _, err := runPowerShell(cmd); err != nil {
			errors = append(errors, fmt.Sprintf("exe path: %v", err))
		}

		cmd = fmt.Sprintf("Remove-MpPreference -ExclusionProcess '%s'", exePath)
		if _, err := runPowerShell(cmd); err != nil {
			errors = append(errors, fmt.Sprintf("process: %v", err))
		}
	}

	if dataDir != "" {
		cmd := fmt.Sprintf("Remove-MpPreference -ExclusionPath '%s'", dataDir)
		if _, err := runPowerShell(cmd); err != nil {
			errors = append(errors, fmt.Sprintf("data dir: %v", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to remove exclusions: %s", strings.Join(errors, "; "))
	}

	return nil
}

// PromptDefenderExclusion shows a UAC elevation prompt to add Defender exclusions.
// This runs the current executable with elevated privileges and a special flag.
func PromptDefenderExclusion(dataDir string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Run ourselves elevated with a special flag
	verb := "runas"
	args := fmt.Sprintf("-add-defender-exclusion %q %q", exePath, dataDir)

	err = shellExecute(exePath, args, verb)
	if err != nil {
		return fmt.Errorf("failed to elevate: %w", err)
	}

	return nil
}

// HandleDefenderExclusionFlag handles the -add-defender-exclusion flag when run elevated.
// Returns true if the flag was handled.
func HandleDefenderExclusionFlag(args []string) bool {
	for i, arg := range args {
		if arg == "-add-defender-exclusion" {
			if i+2 < len(args) {
				exePath := args[i+1]
				dataDir := args[i+2]
				err := AddDefenderExclusions(exePath, dataDir)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to add Defender exclusions: %v\n", err)
					os.Exit(1)
				}
				fmt.Println("Windows Defender exclusions added successfully")
				os.Exit(0)
			}
			return true
		}
	}
	return false
}

// isAdmin checks if the current process has admin privileges.
func isAdmin() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	token := windows.Token(0)
	member, err := token.IsMember(sid)
	if err != nil {
		return false
	}
	return member
}

// shellExecute runs a program with the specified verb (e.g., "runas" for elevation).
func shellExecute(file, args, verb string) error {
	filePtr, _ := syscall.UTF16PtrFromString(file)
	argsPtr, _ := syscall.UTF16PtrFromString(args)
	verbPtr, _ := syscall.UTF16PtrFromString(verb)

	shell32 := syscall.NewLazyDLL("shell32.dll")
	shellExecuteW := shell32.NewProc("ShellExecuteW")

	ret, _, _ := shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verbPtr)),
		uintptr(unsafe.Pointer(filePtr)),
		uintptr(unsafe.Pointer(argsPtr)),
		0,
		1, // SW_SHOWNORMAL
	)

	if ret <= 32 {
		return fmt.Errorf("ShellExecuteW failed with code %d", ret)
	}

	return nil
}

// runPowerShell runs a PowerShell command and returns its output.
func runPowerShell(command string) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
