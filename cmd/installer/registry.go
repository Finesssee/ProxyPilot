//go:build windows

package main

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	// Registry key paths.
	autostartKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	uninstallKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Uninstall\` + appName
	uninstallKeyBase   = `Software\Microsoft\Windows\CurrentVersion\Uninstall`

	// Application metadata.
	appPublisher = "ProxyPilot"
	appVersion   = "1.0.0"
)

// RegisterAutostart adds the application to the Windows autostart registry.
func RegisterAutostart(config *InstallConfig) error {
	if config.InstallDir == "" {
		return fmt.Errorf("install directory not specified")
	}

	exePath := filepath.Join(config.InstallDir, "ProxyPilot.exe")

	key, _, err := registry.CreateKey(registry.CURRENT_USER, autostartKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open autostart registry key: %w", err)
	}
	defer key.Close()

	// Add the application with quoted path to handle spaces.
	if err := key.SetStringValue(appName, `"`+exePath+`"`); err != nil {
		return fmt.Errorf("failed to set autostart value: %w", err)
	}

	return nil
}

// UnregisterAutostart removes the application from Windows autostart.
func UnregisterAutostart(config *InstallConfig) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, autostartKeyPath, registry.SET_VALUE)
	if err != nil {
		// Key doesn't exist or can't be opened; nothing to remove.
		return nil
	}
	defer key.Close()

	if err := key.DeleteValue(appName); err != nil {
		// Value doesn't exist; that's fine.
		if err == registry.ErrNotExist {
			return nil
		}
		return fmt.Errorf("failed to remove autostart value: %w", err)
	}

	return nil
}

// IsAutostartEnabled checks if the application is registered for autostart.
func IsAutostartEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, autostartKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	_, _, err = key.GetStringValue(appName)
	return err == nil
}

// RegisterUninstall registers the application in Add/Remove Programs.
func RegisterUninstall(config *InstallConfig) error {
	if config.InstallDir == "" {
		return fmt.Errorf("install directory not specified")
	}

	exePath := filepath.Join(config.InstallDir, "ProxyPilot.exe")
	iconPath := filepath.Join(config.InstallDir, "icon.ico")

	// Create the uninstall key.
	key, _, err := registry.CreateKey(registry.CURRENT_USER, uninstallKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to create uninstall registry key: %w", err)
	}
	defer key.Close()

	// Set the required values.
	values := map[string]string{
		"DisplayName":     appName,
		"DisplayVersion":  appVersion,
		"Publisher":       appPublisher,
		"InstallLocation": config.InstallDir,
		"DisplayIcon":     iconPath,
		"UninstallString": `"` + exePath + `" --uninstall`,
		"QuietUninstallString": `"` + exePath + `" --uninstall --quiet`,
	}

	for name, value := range values {
		if err := key.SetStringValue(name, value); err != nil {
			return fmt.Errorf("failed to set %s: %w", name, err)
		}
	}

	// Set NoModify and NoRepair flags.
	if err := key.SetDWordValue("NoModify", 1); err != nil {
		return fmt.Errorf("failed to set NoModify: %w", err)
	}
	if err := key.SetDWordValue("NoRepair", 1); err != nil {
		return fmt.Errorf("failed to set NoRepair: %w", err)
	}

	// Estimate the installed size in KB.
	estimatedSize := uint32(40 * 1024) // ~40 MB estimate.
	if err := key.SetDWordValue("EstimatedSize", estimatedSize); err != nil {
		// Non-fatal.
		_ = err
	}

	return nil
}

// UnregisterUninstall removes the application from Add/Remove Programs.
func UnregisterUninstall() error {
	// Delete the entire uninstall key.
	if err := registry.DeleteKey(registry.CURRENT_USER, uninstallKeyPath); err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return fmt.Errorf("failed to delete uninstall registry key: %w", err)
	}
	return nil
}

// GetInstallPathFromRegistry retrieves the installation path from the registry.
// Returns an empty string if not found.
func GetInstallPathFromRegistry() string {
	key, err := registry.OpenKey(registry.CURRENT_USER, uninstallKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()

	path, _, err := key.GetStringValue("InstallLocation")
	if err != nil {
		return ""
	}
	return path
}

// SetRegistryString is a helper to set a string value in the registry.
func SetRegistryString(keyPath, valueName, value string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue(valueName, value)
}

// DeleteRegistryValue is a helper to delete a registry value.
func DeleteRegistryValue(keyPath, valueName string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.DeleteValue(valueName)
}

// DeleteRegistryKey is a helper to delete a registry key.
func DeleteRegistryKey(keyPath string) error {
	return registry.DeleteKey(registry.CURRENT_USER, keyPath)
}
