package integrations

import (
	"fmt"
	"os"
)

// IntegrationStatus represents the status of a tool integration.
type IntegrationStatus struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Installed   bool   `json:"installed"`
	Configured  bool   `json:"configured"`
	Description string `json:"description"`
}

// Detector is the interface for tool detection.
type Detector interface {
	Detect() (bool, error)
	IsConfigured(proxyURL string) (bool, error)
}

// Configurator is the interface for tool configuration.
type Configurator interface {
	Configure(proxyURL string) error
}

// ToolIntegration combines detection and configuration.
type ToolIntegration interface {
	Detector
	Configurator
	Meta() IntegrationStatus
}

// Manager handles all integrations.
type Manager struct {
	integrations map[string]ToolIntegration
	proxyURL     string
}

// NewManager creates a new integration manager.
func NewManager(proxyURL string) *Manager {
	return &Manager{
		integrations: make(map[string]ToolIntegration),
		proxyURL:     proxyURL,
	}
}

// Register adds an integration to the manager.
func (m *Manager) Register(i ToolIntegration) {
	m.integrations[i.Meta().ID] = i
}

// ListStatus returns the status of all registered integrations.
func (m *Manager) ListStatus() []IntegrationStatus {
	var statuses []IntegrationStatus
	for _, i := range m.integrations {
		meta := i.Meta()
		installed, _ := i.Detect()
		configured, _ := i.IsConfigured(m.proxyURL)
		
		meta.Installed = installed
		meta.Configured = configured
		statuses = append(statuses, meta)
	}
	return statuses
}

// Configure triggers configuration for a specific tool.
func (m *Manager) Configure(id string) error {
	i, ok := m.integrations[id]
	if !ok {
		return fmt.Errorf("integration not found: %s", id)
	}
	return i.Configure(m.proxyURL)
}

// Common helpers

func userHomeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	if h := os.Getenv("USERPROFILE"); h != "" { // Windows
		return h
	}
	return "."
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}
