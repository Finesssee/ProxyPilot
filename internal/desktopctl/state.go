package desktopctl

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type state struct {
	PID        int       `json:"pid"`
	ConfigPath string    `json:"config_path"`
	ExePath    string    `json:"exe_path"`
	StartedAt  time.Time `json:"started_at"`
	// AutoStartProxy controls whether the tray app should start the proxy automatically on launch.
	AutoStartProxy bool `json:"auto_start_proxy,omitempty"`
}

func loadState(path string) (*state, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func saveState(path string, s *state) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func deleteState(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// GetAutoStartProxy returns whether the proxy should be started automatically by the tray app.
func GetAutoStartProxy() (bool, error) {
	s, err := loadState(defaultStatePath())
	if err != nil {
		return false, err
	}
	if s == nil {
		return false, nil
	}
	return s.AutoStartProxy, nil
}

// SetAutoStartProxy persists whether the proxy should be started automatically by the tray app.
func SetAutoStartProxy(enabled bool) error {
	path := defaultStatePath()
	s, err := loadState(path)
	if err != nil {
		return err
	}
	if s == nil {
		s = &state{}
	}
	s.AutoStartProxy = enabled
	return saveState(path, s)
}
