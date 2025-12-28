//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// GUID for IShellLinkW.
var (
	CLSID_ShellLink = windows.GUID{
		Data1: 0x00021401,
		Data2: 0x0000,
		Data3: 0x0000,
		Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
	}
	IID_IShellLinkW = windows.GUID{
		Data1: 0x000214F9,
		Data2: 0x0000,
		Data3: 0x0000,
		Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
	}
	IID_IPersistFile = windows.GUID{
		Data1: 0x0000010B,
		Data2: 0x0000,
		Data3: 0x0000,
		Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
	}
)

// IShellLinkW virtual table.
type IShellLinkWVtbl struct {
	QueryInterface   uintptr
	AddRef           uintptr
	Release          uintptr
	GetPath          uintptr
	GetIDList        uintptr
	SetIDList        uintptr
	GetDescription   uintptr
	SetDescription   uintptr
	GetWorkingDirectory uintptr
	SetWorkingDirectory uintptr
	GetArguments     uintptr
	SetArguments     uintptr
	GetHotkey        uintptr
	SetHotkey        uintptr
	GetShowCmd       uintptr
	SetShowCmd       uintptr
	GetIconLocation  uintptr
	SetIconLocation  uintptr
	SetRelativePath  uintptr
	Resolve          uintptr
	SetPath          uintptr
}

type IShellLinkW struct {
	vtbl *IShellLinkWVtbl
}

// IPersistFile virtual table.
type IPersistFileVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	GetClassID     uintptr
	IsDirty        uintptr
	Load           uintptr
	Save           uintptr
	SaveCompleted  uintptr
	GetCurFile     uintptr
}

type IPersistFile struct {
	vtbl *IPersistFileVtbl
}

var (
	ole32           = syscall.NewLazyDLL("ole32.dll")
	coInitializeEx  = ole32.NewProc("CoInitializeEx")
	coCreateInstance = ole32.NewProc("CoCreateInstance")
	coUninitialize  = ole32.NewProc("CoUninitialize")
)

const (
	COINIT_APARTMENTTHREADED = 0x2
	CLSCTX_INPROC_SERVER     = 0x1
)

// CreateShortcuts creates Start Menu and optionally Desktop shortcuts.
func CreateShortcuts(config *InstallConfig) error {
	if config.InstallDir == "" {
		return fmt.Errorf("install directory not specified")
	}

	exePath := filepath.Join(config.InstallDir, "ProxyPilot.exe")
	iconPath := filepath.Join(config.InstallDir, "icon.ico")

	// Create Start Menu shortcut.
	startMenuPath := getStartMenuPath()
	if startMenuPath != "" {
		shortcutPath := filepath.Join(startMenuPath, appName+".lnk")
		if err := createShortcut(shortcutPath, exePath, config.InstallDir, iconPath, "ProxyPilot - AI Proxy Router"); err != nil {
			return fmt.Errorf("failed to create Start Menu shortcut: %w", err)
		}
	}

	// Create Desktop shortcut if requested.
	if config.CreateDesktopShortcut {
		desktopPath := getDesktopPath()
		if desktopPath != "" {
			shortcutPath := filepath.Join(desktopPath, appName+".lnk")
			if err := createShortcut(shortcutPath, exePath, config.InstallDir, iconPath, "ProxyPilot - AI Proxy Router"); err != nil {
				return fmt.Errorf("failed to create Desktop shortcut: %w", err)
			}
		}
	}

	return nil
}

// createShortcut creates a Windows .lnk shortcut file using COM.
func createShortcut(shortcutPath, targetPath, workingDir, iconPath, description string) error {
	// Initialize COM.
	hr, _, _ := coInitializeEx.Call(0, uintptr(COINIT_APARTMENTTHREADED))
	if hr != 0 && hr != 1 { // S_OK or S_FALSE.
		return fmt.Errorf("CoInitializeEx failed: 0x%X", hr)
	}
	defer coUninitialize.Call()

	// Create IShellLink instance.
	var shellLink *IShellLinkW
	hr, _, _ = coCreateInstance.Call(
		uintptr(unsafe.Pointer(&CLSID_ShellLink)),
		0,
		uintptr(CLSCTX_INPROC_SERVER),
		uintptr(unsafe.Pointer(&IID_IShellLinkW)),
		uintptr(unsafe.Pointer(&shellLink)),
	)
	if hr != 0 {
		return fmt.Errorf("CoCreateInstance failed: 0x%X", hr)
	}

	// Set the target path.
	targetPathW, _ := syscall.UTF16PtrFromString(targetPath)
	hr, _, _ = syscall.SyscallN(shellLink.vtbl.SetPath, uintptr(unsafe.Pointer(shellLink)), uintptr(unsafe.Pointer(targetPathW)))
	if hr != 0 {
		return fmt.Errorf("SetPath failed: 0x%X", hr)
	}

	// Set the working directory.
	workingDirW, _ := syscall.UTF16PtrFromString(workingDir)
	hr, _, _ = syscall.SyscallN(shellLink.vtbl.SetWorkingDirectory, uintptr(unsafe.Pointer(shellLink)), uintptr(unsafe.Pointer(workingDirW)))
	if hr != 0 {
		return fmt.Errorf("SetWorkingDirectory failed: 0x%X", hr)
	}

	// Set the description.
	descriptionW, _ := syscall.UTF16PtrFromString(description)
	hr, _, _ = syscall.SyscallN(shellLink.vtbl.SetDescription, uintptr(unsafe.Pointer(shellLink)), uintptr(unsafe.Pointer(descriptionW)))
	if hr != 0 {
		return fmt.Errorf("SetDescription failed: 0x%X", hr)
	}

	// Set the icon.
	if iconPath != "" {
		iconPathW, _ := syscall.UTF16PtrFromString(iconPath)
		hr, _, _ = syscall.SyscallN(shellLink.vtbl.SetIconLocation, uintptr(unsafe.Pointer(shellLink)), uintptr(unsafe.Pointer(iconPathW)), 0)
		if hr != 0 {
			// Non-fatal: continue without icon.
			_ = hr
		}
	}

	// Query for IPersistFile interface.
	var persistFile *IPersistFile
	hr, _, _ = syscall.SyscallN(
		shellLink.vtbl.QueryInterface,
		uintptr(unsafe.Pointer(shellLink)),
		uintptr(unsafe.Pointer(&IID_IPersistFile)),
		uintptr(unsafe.Pointer(&persistFile)),
	)
	if hr != 0 {
		return fmt.Errorf("QueryInterface for IPersistFile failed: 0x%X", hr)
	}

	// Ensure the shortcut directory exists.
	shortcutDir := filepath.Dir(shortcutPath)
	if err := os.MkdirAll(shortcutDir, 0755); err != nil {
		return fmt.Errorf("failed to create shortcut directory: %w", err)
	}

	// Save the shortcut.
	shortcutPathW, _ := syscall.UTF16PtrFromString(shortcutPath)
	hr, _, _ = syscall.SyscallN(persistFile.vtbl.Save, uintptr(unsafe.Pointer(persistFile)), uintptr(unsafe.Pointer(shortcutPathW)), 1)
	if hr != 0 {
		return fmt.Errorf("Save failed: 0x%X", hr)
	}

	// Release interfaces.
	syscall.SyscallN(persistFile.vtbl.Release, uintptr(unsafe.Pointer(persistFile)))
	syscall.SyscallN(shellLink.vtbl.Release, uintptr(unsafe.Pointer(shellLink)))

	return nil
}

// getStartMenuPath returns the user's Start Menu Programs folder.
func getStartMenuPath() string {
	// CSIDL_PROGRAMS = 0x0002.
	path := getKnownFolderPath(0x0002)
	if path != "" {
		return path
	}

	// Fallback to constructing the path manually.
	appData := os.Getenv("APPDATA")
	if appData != "" {
		return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs")
	}
	return ""
}

// getDesktopPath returns the user's Desktop folder.
func getDesktopPath() string {
	// CSIDL_DESKTOP = 0x0000.
	path := getKnownFolderPath(0x0000)
	if path != "" {
		return path
	}

	// Fallback.
	home, _ := os.UserHomeDir()
	if home != "" {
		return filepath.Join(home, "Desktop")
	}
	return ""
}

// getKnownFolderPath retrieves a known folder path using SHGetFolderPath.
func getKnownFolderPath(csidl int) string {
	shell32 := syscall.NewLazyDLL("shell32.dll")
	shGetFolderPath := shell32.NewProc("SHGetFolderPathW")

	buf := make([]uint16, 260) // MAX_PATH.
	hr, _, _ := shGetFolderPath.Call(
		0,
		uintptr(csidl),
		0,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
	)
	if hr != 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
