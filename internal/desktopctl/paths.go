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
	return filepath.Join(base, "CLIProxyAPI", "ui-state.json")
}
