//go:build !windows

package desktopctl

func IsWindowsRunAutostartEnabled(appName string) (bool, string, error) {
	return false, "", nil
}

func EnableWindowsRunAutostart(appName string, command string) error {
	return nil
}

func DisableWindowsRunAutostart(appName string) error {
	return nil
}
