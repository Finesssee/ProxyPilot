package desktopctl

import (
	"os"
	"path/filepath"
)

func defaultStatePath() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		if d, err := os.UserConfigDir(); err == nil && d != "" {
			base = d
		}
	}
	if base == "" {
		base = "."
	}

	newPath := filepath.Join(base, "ProxyPilot", "ui-state.json")
	oldPath := filepath.Join(base, "CLIProxyAPI", "ui-state.json")

	if _, err := os.Stat(newPath); err == nil {
		return newPath
	}
	if _, err := os.Stat(oldPath); err == nil {
		return oldPath
	}
	return newPath
}
