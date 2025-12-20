package management

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
)

func memoryBaseDir() string {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_MEMORY_DIR")); v != "" {
		return v
	}
	if w := util.WritablePath(); w != "" {
		return filepath.Join(w, ".proxypilot", "memory")
	}
	return filepath.Join(".proxypilot", "memory")
}
