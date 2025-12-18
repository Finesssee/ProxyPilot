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
		// New preferred engine name (ProxyPilot branding).
		preferred := filepath.Join(repoRoot, "bin", "proxypilot-engine.exe")
		if _, err := os.Stat(preferred); err == nil {
			return preferred
		}
		// Back-compat fallbacks.
		latest := filepath.Join(repoRoot, "bin", "cliproxyapi-latest.exe")
		if _, err := os.Stat(latest); err == nil {
			return latest
		}
		return filepath.Join(repoRoot, "bin", "cliproxyapi.exe")
	}
	// New preferred engine name (ProxyPilot branding).
	preferred := filepath.Join(repoRoot, "bin", "proxypilot-engine")
	if _, err := os.Stat(preferred); err == nil {
		return preferred
	}
	// Back-compat fallbacks.
	latest := filepath.Join(repoRoot, "bin", "cliproxyapi-latest")
	if _, err := os.Stat(latest); err == nil {
		return latest
	}
	return filepath.Join(repoRoot, "bin", "cliproxyapi")
}
