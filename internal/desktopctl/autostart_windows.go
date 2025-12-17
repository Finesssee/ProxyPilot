//go:build windows

package desktopctl

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const windowsRunKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

func IsWindowsRunAutostartEnabled(appName string) (enabled bool, command string, err error) {
	appName = strings.TrimSpace(appName)
	if appName == "" {
		return false, "", fmt.Errorf("app name is required")
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, windowsRunKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, "", err
	}
	defer k.Close()
	s, _, err := k.GetStringValue(appName)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, "", nil
		}
		return false, "", err
	}
	return strings.TrimSpace(s) != "", s, nil
}

func EnableWindowsRunAutostart(appName string, command string) error {
	appName = strings.TrimSpace(appName)
	command = strings.TrimSpace(command)
	if appName == "" {
		return fmt.Errorf("app name is required")
	}
	if command == "" {
		return fmt.Errorf("autostart command is required")
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, windowsRunKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(appName, command)
}

func DisableWindowsRunAutostart(appName string) error {
	appName = strings.TrimSpace(appName)
	if appName == "" {
		return fmt.Errorf("app name is required")
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, windowsRunKeyPath, registry.SET_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	defer k.Close()
	if err := k.DeleteValue(appName); err != nil && err != registry.ErrNotExist {
		return err
	}
	return nil
}
