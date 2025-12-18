package desktopctl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func resolveConfigPath(repoRoot, configPath string) (string, error) {
	if strings.TrimSpace(configPath) != "" {
		return configPath, nil
	}
	if strings.TrimSpace(repoRoot) != "" {
		return filepath.Join(repoRoot, "config.yaml"), nil
	}
	return "", fmt.Errorf("config path is required")
}

func resolveExePath(repoRoot, exePath string) string {
	if strings.TrimSpace(exePath) != "" {
		return exePath
	}
	if strings.TrimSpace(repoRoot) == "" {
		return ""
	}
	return pickDefaultExePath(repoRoot)
}

func loadPort(configPath string) (int, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return 0, err
	}
	if cfg.Port == 0 {
		return 8318, nil
	}
	return cfg.Port, nil
}

func authDir(configPath string) (string, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return "", err
	}
	dir := strings.TrimSpace(cfg.AuthDir)
	if dir == "" {
		return "", fmt.Errorf("auth-dir not configured")
	}
	if strings.HasPrefix(dir, "~/") || dir == "~" {
		home, _ := os.UserHomeDir()
		if home != "" {
			if dir == "~" {
				dir = home
			} else {
				dir = filepath.Join(home, strings.TrimPrefix(dir, "~/"))
			}
		}
	}
	return dir, nil
}

// AuthDirFor returns the resolved auth directory for the given config file.
func AuthDirFor(configPath string) (string, error) {
	return authDir(configPath)
}
