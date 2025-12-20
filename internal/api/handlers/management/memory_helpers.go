package management

import (
	"os"
	"path/filepath"
	"strconv"
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

func memoryExportMaxBytes() int64 {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_MEMORY_EXPORT_MAX_BYTES")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return 250 * 1024 * 1024
}
