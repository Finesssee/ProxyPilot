// Package agents provides unified agent configuration management for ProxyPilot.
// It tracks all changes made to CLI agent configurations and provides precise
// enable/disable functionality without destructive backup/restore operations.
package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StateVersion is the current version of the state file format
const StateVersion = 1

// State represents the overall state of ProxyPilot's agent configurations
type State struct {
	Version int                   `json:"version"`
	Agents  map[string]AgentState `json:"agents"`
}

// AgentState represents the configuration state for a single agent
type AgentState struct {
	Enabled    bool      `json:"enabled"`
	ConfigPath string    `json:"config_path"`
	Changes    []Change  `json:"changes"`
	EnabledAt  time.Time `json:"enabled_at,omitempty"`
	DisabledAt time.Time `json:"disabled_at,omitempty"`
}

// Change represents a single configuration change made to an agent
type Change struct {
	Path     string `json:"path"`           // JSON path or config key (e.g., "env.ANTHROPIC_BASE_URL")
	Original any    `json:"original"`       // Original value (nil if key didn't exist)
	Applied  any    `json:"applied"`        // Value applied by ProxyPilot
	Type     string `json:"type,omitempty"` // Type of change: "json", "toml", "env", "shell"
	File     string `json:"file,omitempty"` // Specific file if different from ConfigPath
}

// StateManager handles loading, saving, and manipulating ProxyPilot state
type StateManager struct {
	mu        sync.RWMutex
	state     *State
	statePath string
	baseDir   string
}

// NewStateManager creates a new StateManager instance
func NewStateManager() (*StateManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	baseDir := filepath.Join(homeDir, ".proxypilot")
	statePath := filepath.Join(baseDir, "state.json")

	sm := &StateManager{
		statePath: statePath,
		baseDir:   baseDir,
	}

	if err := sm.ensureBaseDir(); err != nil {
		return nil, err
	}

	if err := sm.load(); err != nil {
		return nil, err
	}

	return sm, nil
}

// ensureBaseDir creates the base directory if it doesn't exist
func (sm *StateManager) ensureBaseDir() error {
	agentsDir := filepath.Join(sm.baseDir, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create agents directory: %w", err)
	}
	return nil
}

// load reads the state from disk, or creates a new state if none exists
func (sm *StateManager) load() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	data, err := os.ReadFile(sm.statePath)
	if os.IsNotExist(err) {
		sm.state = &State{
			Version: StateVersion,
			Agents:  make(map[string]AgentState),
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to parse state file: %w", err)
	}

	if state.Agents == nil {
		state.Agents = make(map[string]AgentState)
	}

	sm.state = &state
	return nil
}

// Save persists the current state to disk
func (sm *StateManager) Save() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.saveUnlocked()
}

func (sm *StateManager) saveUnlocked() error {
	data, err := json.MarshalIndent(sm.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(sm.statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// GetAgentState returns the state for a specific agent
func (sm *StateManager) GetAgentState(agentID string) (AgentState, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	state, exists := sm.state.Agents[agentID]
	return state, exists
}

// IsAgentEnabled returns whether an agent is currently enabled
func (sm *StateManager) IsAgentEnabled(agentID string) bool {
	state, exists := sm.GetAgentState(agentID)
	return exists && state.Enabled
}

// RecordEnable records that an agent has been enabled with the given changes
func (sm *StateManager) RecordEnable(agentID string, configPath string, changes []Change) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.state.Agents[agentID] = AgentState{
		Enabled:    true,
		ConfigPath: configPath,
		Changes:    changes,
		EnabledAt:  time.Now(),
	}

	return sm.saveUnlocked()
}

// RecordDisable records that an agent has been disabled
func (sm *StateManager) RecordDisable(agentID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if state, exists := sm.state.Agents[agentID]; exists {
		state.Enabled = false
		state.DisabledAt = time.Now()
		sm.state.Agents[agentID] = state
	}

	return sm.saveUnlocked()
}

// GetChanges returns the list of changes for an agent
func (sm *StateManager) GetChanges(agentID string) []Change {
	state, exists := sm.GetAgentState(agentID)
	if !exists {
		return nil
	}
	return state.Changes
}

// GetAllAgentStates returns all agent states
func (sm *StateManager) GetAllAgentStates() map[string]AgentState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]AgentState, len(sm.state.Agents))
	for k, v := range sm.state.Agents {
		result[k] = v
	}
	return result
}

// GetBaseDir returns the base directory for ProxyPilot config
func (sm *StateManager) GetBaseDir() string {
	return sm.baseDir
}

// GetAgentsDir returns the directory for agent-specific configs
func (sm *StateManager) GetAgentsDir() string {
	return filepath.Join(sm.baseDir, "agents")
}

// ClearAgent removes all state for an agent (used after successful disable)
func (sm *StateManager) ClearAgent(agentID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.state.Agents, agentID)
	return sm.saveUnlocked()
}
