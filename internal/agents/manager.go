package agents

import (
	"fmt"
	"sync"
)

// AgentInfo contains information about a CLI agent
type AgentInfo struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Description      string            `json:"description,omitempty"`
	Detected         bool              `json:"detected"`
	Configured       bool              `json:"configured"`
	CanAutoConfigure bool              `json:"can_auto_configure"`
	ConfigPath       string            `json:"config_path,omitempty"`
	Instructions     string            `json:"instructions,omitempty"`
	EnvVars          map[string]string `json:"env_vars,omitempty"`
}

// ConfigResult represents the result of a configuration operation
type ConfigResult struct {
	Success      bool   `json:"success"`
	AgentID      string `json:"agent_id"`
	Message      string `json:"message,omitempty"`
	Error        string `json:"error,omitempty"`
	ConfigPath   string `json:"config_path,omitempty"`
	Instructions string `json:"instructions,omitempty"`
}

// AgentHandler defines the interface for agent-specific configuration handlers
type AgentHandler interface {
	// ID returns the unique identifier for this agent
	ID() string

	// Name returns the display name for this agent
	Name() string

	// Detect checks if the agent is installed on the system
	Detect() (bool, error)

	// IsConfigured checks if the agent is already configured for ProxyPilot
	IsConfigured(proxyURL string) (bool, error)

	// Enable configures the agent to use ProxyPilot and returns the changes made
	Enable(proxyURL string) ([]Change, error)

	// Disable removes ProxyPilot configuration from the agent using recorded changes
	Disable(changes []Change) error

	// GetConfigPath returns the path to the agent's configuration file
	GetConfigPath() string

	// CanAutoConfigure returns whether this agent supports automatic configuration
	CanAutoConfigure() bool

	// GetInstructions returns manual configuration instructions
	GetInstructions(proxyURL string) string
}

// Manager manages all CLI agent configurations
type Manager struct {
	mu       sync.RWMutex
	state    *StateManager
	handlers map[string]AgentHandler
	proxyURL string
}

// NewManager creates a new agent configuration manager
func NewManager(proxyPort int) (*Manager, error) {
	state, err := NewStateManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create state manager: %w", err)
	}

	proxyURL := fmt.Sprintf("http://127.0.0.1:%d", proxyPort)

	m := &Manager{
		state:    state,
		handlers: make(map[string]AgentHandler),
		proxyURL: proxyURL,
	}

	// Register all known agent handlers
	m.registerDefaultHandlers()

	return m, nil
}

// registerDefaultHandlers registers all built-in agent handlers
func (m *Manager) registerDefaultHandlers() {
	handlers := []AgentHandler{
		NewClaudeCodeHandler(),
		NewCodexHandler(),
		NewGeminiCLIHandler(),
		NewCursorHandler(),
		NewOpenCodeHandler(),
		NewFactoryDroidHandler(),
		NewAmazonQCLIHandler(),
	}

	for _, h := range handlers {
		m.handlers[h.ID()] = h
	}
}

// RegisterHandler adds a custom agent handler
func (m *Manager) RegisterHandler(handler AgentHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[handler.ID()] = handler
}

// GetHandler returns a handler by ID
func (m *Manager) GetHandler(agentID string) (AgentHandler, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, ok := m.handlers[agentID]
	return h, ok
}

// DetectAll detects all agents and returns their info
func (m *Manager) DetectAll() ([]AgentInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var agents []AgentInfo

	for _, handler := range m.handlers {
		info := AgentInfo{
			ID:               handler.ID(),
			Name:             handler.Name(),
			CanAutoConfigure: handler.CanAutoConfigure(),
			ConfigPath:       handler.GetConfigPath(),
			Instructions:     handler.GetInstructions(m.proxyURL),
		}

		detected, err := handler.Detect()
		if err != nil {
			// Log error but continue with other agents
			info.Detected = false
		} else {
			info.Detected = detected
		}

		if detected {
			configured, err := handler.IsConfigured(m.proxyURL)
			if err == nil {
				info.Configured = configured
			}
		}

		// Check if we have state for this agent
		if state, exists := m.state.GetAgentState(handler.ID()); exists {
			info.Configured = state.Enabled
		}

		agents = append(agents, info)
	}

	return agents, nil
}

// GetAgentInfo returns info for a specific agent
func (m *Manager) GetAgentInfo(agentID string) (AgentInfo, error) {
	handler, ok := m.GetHandler(agentID)
	if !ok {
		return AgentInfo{}, fmt.Errorf("unknown agent: %s", agentID)
	}

	info := AgentInfo{
		ID:               handler.ID(),
		Name:             handler.Name(),
		CanAutoConfigure: handler.CanAutoConfigure(),
		ConfigPath:       handler.GetConfigPath(),
		Instructions:     handler.GetInstructions(m.proxyURL),
	}

	detected, _ := handler.Detect()
	info.Detected = detected

	if detected {
		configured, _ := handler.IsConfigured(m.proxyURL)
		info.Configured = configured
	}

	if state, exists := m.state.GetAgentState(agentID); exists {
		info.Configured = state.Enabled
	}

	return info, nil
}

// Enable enables ProxyPilot for an agent
func (m *Manager) Enable(agentID string) ConfigResult {
	handler, ok := m.GetHandler(agentID)
	if !ok {
		return ConfigResult{
			Success: false,
			AgentID: agentID,
			Error:   fmt.Sprintf("unknown agent: %s", agentID),
		}
	}

	if !handler.CanAutoConfigure() {
		return ConfigResult{
			Success:      false,
			AgentID:      agentID,
			Error:        "agent does not support automatic configuration",
			Instructions: handler.GetInstructions(m.proxyURL),
		}
	}

	// Check if already enabled
	if m.state.IsAgentEnabled(agentID) {
		return ConfigResult{
			Success:    true,
			AgentID:    agentID,
			Message:    "agent already configured",
			ConfigPath: handler.GetConfigPath(),
		}
	}

	// Enable the agent
	changes, err := handler.Enable(m.proxyURL)
	if err != nil {
		return ConfigResult{
			Success: false,
			AgentID: agentID,
			Error:   fmt.Sprintf("failed to enable: %v", err),
		}
	}

	// Record the state
	if err := m.state.RecordEnable(agentID, handler.GetConfigPath(), changes); err != nil {
		return ConfigResult{
			Success: false,
			AgentID: agentID,
			Error:   fmt.Sprintf("failed to save state: %v", err),
		}
	}

	return ConfigResult{
		Success:    true,
		AgentID:    agentID,
		Message:    fmt.Sprintf("%s configured successfully", handler.Name()),
		ConfigPath: handler.GetConfigPath(),
	}
}

// Disable disables ProxyPilot for an agent
func (m *Manager) Disable(agentID string) ConfigResult {
	handler, ok := m.GetHandler(agentID)
	if !ok {
		return ConfigResult{
			Success: false,
			AgentID: agentID,
			Error:   fmt.Sprintf("unknown agent: %s", agentID),
		}
	}

	// Get recorded changes
	changes := m.state.GetChanges(agentID)
	if len(changes) == 0 {
		// No recorded changes, try to detect and remove anyway
		if !m.state.IsAgentEnabled(agentID) {
			return ConfigResult{
				Success: true,
				AgentID: agentID,
				Message: "agent was not configured",
			}
		}
	}

	// Disable the agent
	if err := handler.Disable(changes); err != nil {
		return ConfigResult{
			Success: false,
			AgentID: agentID,
			Error:   fmt.Sprintf("failed to disable: %v", err),
		}
	}

	// Clear the state
	if err := m.state.ClearAgent(agentID); err != nil {
		return ConfigResult{
			Success: false,
			AgentID: agentID,
			Error:   fmt.Sprintf("failed to clear state: %v", err),
		}
	}

	return ConfigResult{
		Success: true,
		AgentID: agentID,
		Message: fmt.Sprintf("%s configuration removed", handler.Name()),
	}
}

// EnableAll enables ProxyPilot for all detected agents that support auto-configuration
func (m *Manager) EnableAll() []ConfigResult {
	var results []ConfigResult

	for agentID, handler := range m.handlers {
		if !handler.CanAutoConfigure() {
			continue
		}

		detected, _ := handler.Detect()
		if !detected {
			continue
		}

		result := m.Enable(agentID)
		results = append(results, result)
	}

	return results
}

// DisableAll disables ProxyPilot for all configured agents
func (m *Manager) DisableAll() []ConfigResult {
	var results []ConfigResult

	states := m.state.GetAllAgentStates()
	for agentID, state := range states {
		if !state.Enabled {
			continue
		}

		result := m.Disable(agentID)
		results = append(results, result)
	}

	return results
}

// GetState returns the state manager for direct access
func (m *Manager) GetState() *StateManager {
	return m.state
}

// SetProxyURL updates the proxy URL
func (m *Manager) SetProxyURL(port int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.proxyURL = fmt.Sprintf("http://127.0.0.1:%d", port)
}
