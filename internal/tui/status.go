// Package tui provides terminal user interface components for ProxyPilot.
// It uses the bubbletea framework to build interactive terminal applications.
package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/agents"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
)

// StatusRefreshInterval defines how often the status screen auto-refreshes.
const StatusRefreshInterval = 3 * time.Second

// StatusModel represents the state of the status display screen.
type StatusModel struct {
	port              int
	host              string
	serverRunning     bool
	providerCount     int
	agentCount        int
	requestsToday     int64
	totalRequests     int64
	successCount      int64
	failureCount      int64
	quitting          bool
	lastRefresh       time.Time
	cfg               *config.Config
	stateManager      *agents.StateManager
	width             int
	height            int
	autoRefreshTicker *time.Ticker
}

// tickMsg signals that the auto-refresh timer has fired.
type tickMsg time.Time

// StatusOption configures the StatusModel.
type StatusOption func(*StatusModel)

// WithConfig sets the configuration for the status model.
func WithConfig(cfg *config.Config) StatusOption {
	return func(m *StatusModel) {
		m.cfg = cfg
		if cfg != nil {
			m.port = cfg.Port
			m.host = cfg.Host
		}
	}
}

// WithPort sets the server port for the status model.
func WithPort(port int) StatusOption {
	return func(m *StatusModel) {
		m.port = port
	}
}

// WithHost sets the server host for the status model.
func WithHost(host string) StatusOption {
	return func(m *StatusModel) {
		m.host = host
	}
}

// WithServerRunning sets whether the server is running.
func WithServerRunning(running bool) StatusOption {
	return func(m *StatusModel) {
		m.serverRunning = running
	}
}

// NewStatusModel creates a new StatusModel with the given options.
func NewStatusModel(opts ...StatusOption) StatusModel {
	m := StatusModel{
		port:          8317,
		serverRunning: true,
		lastRefresh:   time.Now(),
	}

	for _, opt := range opts {
		opt(&m)
	}

	m.refresh()
	return m
}

// refresh updates the model with current statistics.
func (m *StatusModel) refresh() {
	m.lastRefresh = time.Now()

	// Count authenticated providers from config and auth files
	m.providerCount = m.countProviders()

	// Count configured agents
	m.agentCount = m.countAgents()

	// Get request statistics
	stats := usage.GetRequestStatistics()
	if stats != nil {
		snapshot := stats.Snapshot()
		m.totalRequests = snapshot.TotalRequests
		m.successCount = snapshot.SuccessCount
		m.failureCount = snapshot.FailureCount

		// Get today's request count
		today := time.Now().Format("2006-01-02")
		if count, ok := snapshot.RequestsByDay[today]; ok {
			m.requestsToday = count
		} else {
			m.requestsToday = 0
		}
	}
}

// countProviders returns the number of authenticated providers.
func (m *StatusModel) countProviders() int {
	count := 0

	if m.cfg != nil {
		// Count auth files
		if m.cfg.AuthDir != "" {
			count += util.CountAuthFiles(m.cfg.AuthDir)
		}

		// Count API key configurations
		count += len(m.cfg.GeminiKey)
		count += len(m.cfg.ClaudeKey)
		count += len(m.cfg.CodexKey)
		count += len(m.cfg.KiroKey)
		count += len(m.cfg.VertexCompatAPIKey)

		// Count OpenAI compatibility providers
		for _, compat := range m.cfg.OpenAICompatibility {
			count += len(compat.APIKeyEntries)
		}
	}

	return count
}

// countAgents returns the number of agents configured in proxy mode.
func (m *StatusModel) countAgents() int {
	if m.stateManager == nil {
		var err error
		m.stateManager, err = agents.NewStateManager()
		if err != nil {
			return 0
		}
	}

	states := m.stateManager.GetAllAgentStates()
	enabledCount := 0
	for _, state := range states {
		if state.Enabled {
			enabledCount++
		}
	}
	return enabledCount
}

// Init initializes the StatusModel and starts the auto-refresh ticker.
func (m StatusModel) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
	)
}

// tickCmd creates a command that fires after the refresh interval.
func tickCmd() tea.Cmd {
	return tea.Tick(StatusRefreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update handles messages and updates the model accordingly.
func (m StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "r":
			m.refresh()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		m.refresh()
		return m, tickCmd()
	}

	return m, nil
}

// View renders the status screen.
func (m StatusModel) View() string {
	if m.quitting {
		return ""
	}

	// Define styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	dividerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	labelStyle := lipgloss.NewStyle().
		Width(12).
		Foreground(lipgloss.Color("252"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("42"))

	runningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("42"))

	stoppedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		MarginTop(1)

	// Build the view
	var s string

	// Title
	s += titleStyle.Render("ProxyPilot Status") + "\n"

	// Divider
	s += dividerStyle.Render("─────────────────") + "\n"

	// Server status
	serverLabel := labelStyle.Render("Server:")
	serverAddr := m.formatServerAddress()
	var serverValue string
	if m.serverRunning {
		serverValue = runningStyle.Render("Running on " + serverAddr)
	} else {
		serverValue = stoppedStyle.Render("Stopped")
	}
	s += serverLabel + serverValue + "\n"

	// Provider count
	providerLabel := labelStyle.Render("Providers:")
	providerValue := valueStyle.Render(fmt.Sprintf("%d authenticated", m.providerCount))
	s += providerLabel + providerValue + "\n"

	// Agent count
	agentLabel := labelStyle.Render("Agents:")
	agentValue := valueStyle.Render(fmt.Sprintf("%d in proxy mode", m.agentCount))
	s += agentLabel + agentValue + "\n"

	// Request stats
	requestLabel := labelStyle.Render("Requests:")
	requestValue := valueStyle.Render(fmt.Sprintf("%d today", m.requestsToday))
	s += requestLabel + requestValue + "\n"

	// Additional stats if available
	if m.totalRequests > 0 {
		totalLabel := labelStyle.Render("Total:")
		totalValue := valueStyle.Render(fmt.Sprintf("%d (%d ok, %d failed)", m.totalRequests, m.successCount, m.failureCount))
		s += totalLabel + totalValue + "\n"
	}

	// Footer with controls
	s += footerStyle.Render("[r] Refresh  [Esc] Back")

	return s
}

// formatServerAddress returns the formatted server address string.
func (m StatusModel) formatServerAddress() string {
	host := m.host
	if host == "" {
		host = ""
	}
	return fmt.Sprintf("%s:%d", host, m.port)
}

// RunStatusScreen runs the status screen as a bubbletea program.
// It blocks until the user exits.
func RunStatusScreen(opts ...StatusOption) error {
	model := NewStatusModel(opts...)
	p := tea.NewProgram(model)
	_, err := p.Run()
	return err
}

// StatusData holds the current status information for external consumption.
type StatusData struct {
	Port          int    `json:"port"`
	Host          string `json:"host"`
	ServerRunning bool   `json:"server_running"`
	ProviderCount int    `json:"provider_count"`
	AgentCount    int    `json:"agent_count"`
	RequestsToday int64  `json:"requests_today"`
	TotalRequests int64  `json:"total_requests"`
	SuccessCount  int64  `json:"success_count"`
	FailureCount  int64  `json:"failure_count"`
}

// GetStatusData returns the current status data without running the TUI.
// Useful for non-interactive contexts.
func GetStatusData(cfg *config.Config) StatusData {
	m := NewStatusModel(WithConfig(cfg), WithServerRunning(true))
	return StatusData{
		Port:          m.port,
		Host:          m.host,
		ServerRunning: m.serverRunning,
		ProviderCount: m.providerCount,
		AgentCount:    m.agentCount,
		RequestsToday: m.requestsToday,
		TotalRequests: m.totalRequests,
		SuccessCount:  m.successCount,
		FailureCount:  m.failureCount,
	}
}
