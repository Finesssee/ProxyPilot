package desktopctl

import "time"

type Status struct {
	Running    bool      `json:"running"`
	Managed    bool      `json:"managed"`
	PID        int       `json:"pid,omitempty"`
	Port       int       `json:"port,omitempty"`
	BaseURL    string    `json:"base_url,omitempty"`
	ConfigPath string    `json:"config_path,omitempty"`
	ExePath    string    `json:"exe_path,omitempty"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	LastError  string    `json:"last_error,omitempty"`
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
