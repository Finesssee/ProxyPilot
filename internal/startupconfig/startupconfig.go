package startupconfig

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/misc"
)

// Resolution describes the config path the server should use at startup.
type Resolution struct {
	ConfigPath   string
	TemplatePath string
	UsedDefault  bool
}

// ResolveConfigPath determines the startup config path.
// Explicit --config paths remain strict. Implicit defaults prefer a packaged
// config or template next to the executable before falling back to the
// current working directory.
func ResolveConfigPath(explicitConfigPath, workingDir, executablePath string) Resolution {
	if explicit := cleanPath(explicitConfigPath); explicit != "" {
		return Resolution{
			ConfigPath:  explicit,
			UsedDefault: false,
		}
	}

	workingDir = cleanPath(workingDir)
	executableRoot := executableConfigRoot(executablePath)
	candidates := uniqueNonEmpty(executableRoot, workingDir)

	for _, root := range candidates {
		resolution := resolutionForRoot(root)
		if fileExists(resolution.ConfigPath) || resolution.TemplatePath != "" {
			return resolution
		}
	}

	if workingDir != "" {
		return resolutionForRoot(workingDir)
	}
	if executableRoot != "" {
		return resolutionForRoot(executableRoot)
	}

	return Resolution{
		ConfigPath:  "config.yaml",
		UsedDefault: true,
	}
}

// EnsureDefaultConfig bootstraps the implicit default config from a colocated
// config.example.yaml when the config file does not already exist.
func EnsureDefaultConfig(resolution Resolution) (bool, error) {
	if !resolution.UsedDefault || cleanPath(resolution.ConfigPath) == "" {
		return false, nil
	}
	if fileExists(resolution.ConfigPath) {
		return false, nil
	}
	if resolution.TemplatePath == "" {
		return false, nil
	}
	if !fileExists(resolution.TemplatePath) {
		return false, nil
	}
	if err := misc.CopyConfigTemplate(resolution.TemplatePath, resolution.ConfigPath); err != nil {
		return false, err
	}
	return true, nil
}

func resolutionForRoot(root string) Resolution {
	templatePath := filepath.Join(root, "config.example.yaml")
	if !fileExists(templatePath) {
		templatePath = ""
	}
	return Resolution{
		ConfigPath:   filepath.Join(root, "config.yaml"),
		TemplatePath: templatePath,
		UsedDefault:  true,
	}
}

func executableConfigRoot(executablePath string) string {
	executablePath = cleanPath(executablePath)
	if executablePath == "" {
		return ""
	}
	root := filepath.Dir(executablePath)
	if strings.EqualFold(filepath.Base(root), "bin") {
		root = filepath.Dir(root)
	}
	return root
}

func cleanPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}

func uniqueNonEmpty(paths ...string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
