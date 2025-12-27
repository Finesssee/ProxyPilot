package desktopctl

import "time"

type Status struct {
	Running         bool      `json:"running"`
	Version         string    `json:"version,omitempty"`
	Managed         bool      `json:"managed"`
	AutoStartProxy  bool      `json:"auto_start_proxy,omitempty"`
	PID             int       `json:"pid,omitempty"`
	Port            int       `json:"port,omitempty"`
	ThinkingPort    int       `json:"thinking_port,omitempty"`
	ThinkingRunning bool      `json:"thinking_running,omitempty"`
	BaseURL         string    `json:"base_url,omitempty"`
	ConfigPath      string    `json:"config_path,omitempty"`
	ExePath         string    `json:"exe_path,omitempty"`
	StartedAt       time.Time `json:"started_at,omitempty"`
	LastError       string    `json:"last_error,omitempty"`
}

type StartOptions struct {
	RepoRoot   string
	ConfigPath string
	ExePath    string
	LogDir     string
}

type StopOptions struct {
	PID int
}
