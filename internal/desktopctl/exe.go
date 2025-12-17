package desktopctl

import (
	"os"
	"path/filepath"
	"runtime"
)

func pickDefaultExePath(repoRoot string) string {
	if repoRoot == "" {
		return ""
	}
	if runtime.GOOS == "windows" {
		latest := filepath.Join(repoRoot, "bin", "cliproxyapi-latest.exe")
		if _, err := os.Stat(latest); err == nil {
			return latest
		}
		return filepath.Join(repoRoot, "bin", "cliproxyapi.exe")
	}
	latest := filepath.Join(repoRoot, "bin", "cliproxyapi-latest")
	if _, err := os.Stat(latest); err == nil {
		return latest
	}
	return filepath.Join(repoRoot, "bin", "cliproxyapi")
}
