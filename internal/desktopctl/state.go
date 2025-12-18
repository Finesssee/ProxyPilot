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
	// OAuthPrivate controls whether OAuth login should be opened in a private window.
	OAuthPrivate bool `json:"oauth_private,omitempty"`
	// ManagementPassword is a locally-scoped management key used to unlock /v0/management
	// endpoints when ProxyPilot starts the engine. It is stored in the per-user state file.
	ManagementPassword string `json:"management_password,omitempty"`
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

// GetOAuthPrivate returns whether OAuth flows should be opened in a private window.
func GetOAuthPrivate() (bool, error) {
	s, err := loadState(defaultStatePath())
	if err != nil {
		return false, err
	}
	if s == nil {
		return false, nil
	}
	return s.OAuthPrivate, nil
}

// SetOAuthPrivate persists whether OAuth flows should be opened in a private window.
func SetOAuthPrivate(enabled bool) error {
	path := defaultStatePath()
	s, err := loadState(path)
	if err != nil {
		return err
	}
	if s == nil {
		s = &state{}
	}
	s.OAuthPrivate = enabled
	return saveState(path, s)
}

func getOrCreateManagementPassword() (string, error) {
	path := defaultStatePath()
	s, err := loadState(path)
	if err != nil {
		return "", err
	}
	if s == nil {
		s = &state{}
	}
	if s.ManagementPassword != "" {
		return s.ManagementPassword, nil
	}
	pw, err := randomPassword(32)
	if err != nil {
		return "", err
	}
	s.ManagementPassword = pw
	if err := saveState(path, s); err != nil {
		return "", err
	}
	return pw, nil
}

// GetManagementPassword returns the per-user management password used by ProxyPilot to unlock
// the engine Management API for localhost access.
func GetManagementPassword() (string, error) {
	return getOrCreateManagementPassword()
}
