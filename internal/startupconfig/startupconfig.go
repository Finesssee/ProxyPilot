package startupconfig

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/misc"
	log "github.com/sirupsen/logrus"
)

// Resolution describes the config path the server should use at startup.
type Resolution struct {
	ConfigPath   string
	TemplatePath string
	UsedDefault  bool
}

const embeddedConfigTemplate = `# Server host/interface to bind to. Default is empty ("") to bind all interfaces (IPv4 + IPv6).
# Use "127.0.0.1" or "localhost" to restrict access to local machine only.
host: ""

# Server port
port: 8317

# Authentication directory (supports ~ for home directory)
auth-dir: "~/.cli-proxy-api"

# API keys for authentication
api-keys:
  - "your-api-key-1"
  - "your-api-key-2"
  - "your-api-key-3"

# Enable debug logging
debug: false

# Proxy URL. Supports socks5/http/https protocols.
proxy-url: ""

# Gemini API keys
# gemini-api-key:
#   - api-key: "AIzaSy..."

# Codex API keys
# codex-api-key:
#   - api-key: "sk-..."

# Claude API keys
# claude-api-key:
#   - api-key: "sk-..."

# OpenAI compatible API keys
# openai-compatibility:
#   - name: "openai"
#     api-key: "sk-..."
#     base-url: "https://api.openai.com/v1"
`

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

// EnsureDefaultConfig bootstraps the implicit default config when the config
// file does not already exist. A colocated config.example.yaml is preferred;
// standalone binaries fall back to an embedded starter template.
func EnsureDefaultConfig(resolution Resolution) (bool, error) {
	if !resolution.UsedDefault || cleanPath(resolution.ConfigPath) == "" {
		return false, nil
	}
	if fileExists(resolution.ConfigPath) {
		return false, nil
	}
	if resolution.TemplatePath != "" && fileExists(resolution.TemplatePath) {
		if err := misc.CopyConfigTemplate(resolution.TemplatePath, resolution.ConfigPath); err != nil {
			return false, err
		}
		return true, nil
	}
	if err := writeConfigTemplate(resolution.ConfigPath, strings.NewReader(embeddedConfigTemplate)); err != nil {
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

func writeConfigTemplate(dst string, src io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if errClose := out.Close(); errClose != nil {
			log.WithError(errClose).Warn("failed to close destination config file")
		}
	}()

	if _, err = io.Copy(out, src); err != nil {
		return err
	}
	return out.Sync()
}
