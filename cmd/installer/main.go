//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2"
)

const (
	windowWidth  = 500
	windowHeight = 400
	appName      = "ProxyPilot"
)

var (
	// assetServerURL holds the local HTTP server URL for serving embedded UI assets.
	assetServerURL string

	// webviewInstance is the global WebView2 instance for window control.
	webviewInstance webview2.WebView
)

func main() {
	// Lock this goroutine to an OS thread - required for Windows COM/GUI operations.
	runtime.LockOSThread()

	// Start embedded asset server for the installer UI.
	assetLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = messageBox("ProxyPilot Installer", "Failed to start asset server: "+err.Error())
		os.Exit(1)
	}
	assetServerURL = "http://" + assetLn.Addr().String()

	go func() {
		fsys, err := fs.Sub(embeddedUI, "ui")
		if err != nil {
			return
		}
		_ = http.Serve(assetLn, http.FileServer(http.FS(fsys)))
	}()

	// Create frameless WebView2 window with dark theme.
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "ProxyPilot Installer",
			Width:  uint(windowWidth),
			Height: uint(windowHeight),
			Center: true,
			IconId: 1, // Use embedded icon from resource.syso if present.
		},
	})
	if w == nil {
		_ = messageBox("ProxyPilot Installer", "Failed to initialize WebView2.\n\nPlease ensure WebView2 Runtime is installed.")
		os.Exit(1)
	}
	defer w.Destroy()

	webviewInstance = w

	// Bind Go functions to JavaScript.
	bindInstallerFunctions(w)

	// Navigate to the embedded UI.
	target := assetServerURL + "/index.html"
	w.Navigate(target)

	// Apply dark window chrome and frameless styling after navigation.
	w.Dispatch(func() {
		applyDarkWindowChrome(w)
	})

	w.Run()
}

// bindInstallerFunctions binds all Go functions to the JavaScript window.installer object.
func bindInstallerFunctions(w webview2.WebView) {
	// Create the installer namespace.
	w.Init(`window.installer = {};`)

	// Bind startInstall - starts the installation process.
	_ = w.Bind("installer_startInstall", func(optionsJSON string) (map[string]any, error) {
		var options struct {
			Desktop   bool `json:"desktop"`
			Autostart bool `json:"autostart"`
		}
		if err := json.Unmarshal([]byte(optionsJSON), &options); err != nil {
			options.Desktop = false
			options.Autostart = true
		}

		config := &InstallConfig{
			CreateDesktopShortcut: options.Desktop,
			EnableAutostart:       options.Autostart,
		}

		// Run installation in a goroutine to not block the UI.
		go func() {
			if err := runInstallation(w, config); err != nil {
				callSetProgress(w, -1, "Installation failed: "+err.Error())
			}
		}()

		return map[string]any{"started": true}, nil
	})

	// Bind launch - launches the installed application.
	_ = w.Bind("installer_launch", func() error {
		installDir := getInstallDir()
		exePath := filepath.Join(installDir, "ProxyPilot.exe")
		if _, err := os.Stat(exePath); err != nil {
			return fmt.Errorf("ProxyPilot.exe not found at %s", exePath)
		}
		cmd := exec.Command(exePath)
		cmd.Dir = installDir
		return cmd.Start()
	})

	// Bind close - closes the installer window.
	_ = w.Bind("installer_close", func() error {
		w.Dispatch(func() {
			w.Terminate()
		})
		return nil
	})

	// Bind minimize - minimizes the window.
	_ = w.Bind("installer_minimize", func() error {
		w.Dispatch(func() {
			minimizeWindow(w)
		})
		return nil
	})

	// Bind getInstallPath - returns the default installation path.
	_ = w.Bind("installer_getInstallPath", func() (string, error) {
		return getInstallDir(), nil
	})

	// Initialize the window.installer JavaScript API.
	w.Init(`
		window.installer.startInstall = function(options) {
			return new Promise((resolve, reject) => {
				installer_startInstall(JSON.stringify(options || {}))
					.then(resolve)
					.catch(reject);
			});
		};
		window.installer.launch = installer_launch;
		window.installer.close = installer_close;
		window.installer.minimize = installer_minimize;
		window.installer.getInstallPath = installer_getInstallPath;
	`)
}

// runInstallation performs the complete installation process with progress reporting.
func runInstallation(w webview2.WebView, config *InstallConfig) error {
	// Initialize install configuration with defaults.
	config.InstallDir = getInstallDir()

	// Step 1: Prepare installation directory (10%).
	callSetProgress(w, 5, "Preparing installation directory...")
	if err := os.MkdirAll(config.InstallDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}
	callSetProgress(w, 10, "Installation directory ready")

	// Step 2: Copy files (10% - 60%).
	callSetProgress(w, 15, "Copying application files...")
	if err := CopyFiles(config, func(progress int, status string) {
		// Map 0-100 to 15-60.
		mappedProgress := 15 + (progress * 45 / 100)
		callSetProgress(w, mappedProgress, status)
	}); err != nil {
		return fmt.Errorf("failed to copy files: %w", err)
	}
	callSetProgress(w, 60, "Files copied successfully")

	// Step 3: Create shortcuts (60% - 80%).
	callSetProgress(w, 65, "Creating shortcuts...")
	if err := CreateShortcuts(config); err != nil {
		// Non-fatal: log but continue.
		callSetProgress(w, 70, "Warning: Some shortcuts could not be created")
	} else {
		callSetProgress(w, 80, "Shortcuts created")
	}

	// Step 4: Register autostart if enabled (80% - 90%).
	if config.EnableAutostart {
		callSetProgress(w, 85, "Configuring autostart...")
		if err := RegisterAutostart(config); err != nil {
			// Non-fatal: log but continue.
			callSetProgress(w, 88, "Warning: Autostart could not be configured")
		} else {
			callSetProgress(w, 90, "Autostart configured")
		}
	} else {
		callSetProgress(w, 90, "Skipping autostart configuration")
	}

	// Step 5: Register uninstaller (90% - 100%).
	callSetProgress(w, 92, "Registering application...")
	if err := RegisterUninstall(config); err != nil {
		// Non-fatal: log but continue.
		callSetProgress(w, 95, "Warning: Uninstall registration incomplete")
	} else {
		callSetProgress(w, 98, "Application registered")
	}

	callSetProgress(w, 100, "Installation complete!")
	return nil
}

// callSetProgress calls window.setProgress in the WebView.
func callSetProgress(w webview2.WebView, percent int, statusText string) {
	// Escape the status text for JavaScript.
	escaped := strings.ReplaceAll(statusText, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "'", "\\'")
	escaped = strings.ReplaceAll(escaped, "\n", "\\n")

	js := fmt.Sprintf("if(window.setProgress){window.setProgress(%d,'%s');}", percent, escaped)
	w.Dispatch(func() {
		w.Eval(js)
	})
}

// getInstallDir returns the default installation directory.
func getInstallDir() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, _ := os.UserHomeDir()
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(localAppData, appName)
}

// applyDarkWindowChrome applies dark mode window chrome styling.
func applyDarkWindowChrome(w webview2.WebView) {
	hwnd := getWindowHandle(w)
	if hwnd == 0 {
		return
	}

	// Enable dark mode for the window frame.
	dwmapi := syscall.NewLazyDLL("dwmapi.dll")
	dwmSetWindowAttribute := dwmapi.NewProc("DwmSetWindowAttribute")

	// DWMWA_USE_IMMERSIVE_DARK_MODE = 20.
	const DWMWA_USE_IMMERSIVE_DARK_MODE = 20
	darkMode := int32(1)
	_, _, _ = dwmSetWindowAttribute.Call(
		hwnd,
		uintptr(DWMWA_USE_IMMERSIVE_DARK_MODE),
		uintptr(unsafe.Pointer(&darkMode)),
		unsafe.Sizeof(darkMode),
	)

	// DWMWA_CAPTION_COLOR = 35 (Windows 11).
	const DWMWA_CAPTION_COLOR = 35
	captionColor := uint32(0x001E1E1E) // Dark gray in BGR format.
	_, _, _ = dwmSetWindowAttribute.Call(
		hwnd,
		uintptr(DWMWA_CAPTION_COLOR),
		uintptr(unsafe.Pointer(&captionColor)),
		unsafe.Sizeof(captionColor),
	)
}

// getWindowHandle retrieves the HWND for the WebView window.
func getWindowHandle(w webview2.WebView) uintptr {
	// The go-webview2 library provides a Window() method that returns the HWND.
	return uintptr(w.Window())
}

// minimizeWindow minimizes the WebView window.
func minimizeWindow(w webview2.WebView) {
	hwnd := getWindowHandle(w)
	if hwnd == 0 {
		return
	}

	user32 := syscall.NewLazyDLL("user32.dll")
	showWindow := user32.NewProc("ShowWindow")
	const SW_MINIMIZE = 6
	_, _, _ = showWindow.Call(hwnd, uintptr(SW_MINIMIZE))
}

// messageBox displays a Windows message box.
func messageBox(title, text string) error {
	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("MessageBoxW")
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(text)
	// MB_OK | MB_ICONERROR.
	_, _, err := proc.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x00000000|0x00000010)
	if err == syscall.Errno(0) {
		return nil
	}
	return err
}
