// Package tui provides the terminal user interface for ProxyPilot.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/embedded"
)

// ServerMenuOption represents an option in the server control menu
type ServerMenuOption int

const (
	ServerMenuOptionStart ServerMenuOption = iota
	ServerMenuOptionStop
	ServerMenuOptionBack
)

// ServerControlStatus represents the current status of the server for the control screen
type ServerControlStatus struct {
	Running        bool
	Port           int
	ConnectedCount int
	StartedAt      time.Time
	ConfigPath     string
}

// serverControlStatusMsg is sent when server status is updated
type serverControlStatusMsg ServerControlStatus

// serverActionResultMsg is sent when a server action completes
type serverActionResultMsg struct {
	action  string
	success bool
	err     error
}

// serverTickMsg is sent on each tick for polling server status
type serverTickMsg time.Time

// ServerModel represents the server control screen state
type ServerModel struct {
	status     ServerControlStatus
	cursor     int
	options    []string
	spinner    spinner.Model
	loading    bool
	message    string
	msgStyle   lipgloss.Style
	width      int
	height     int
	keys       ServerKeyMap
	configPath string
	password   string
}

// ServerKeyMap defines key bindings for the server screen
type ServerKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Back   key.Binding
	Quit   key.Binding
}

// DefaultServerKeyMap returns the default server screen key bindings
func DefaultServerKeyMap() ServerKeyMap {
	return ServerKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("^/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("v/j", "down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "b"),
			key.WithHelp("esc/b", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// NewServerModel creates a new server control model
func NewServerModel(configPath, password string) ServerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = SpinnerStyle

	return ServerModel{
		status: ServerControlStatus{
			Running:        false,
			Port:           8317,
			ConnectedCount: 0,
		},
		cursor: 0,
		options: []string{
			"Start Server",
			"Stop Server",
			"Back to Menu",
		},
		spinner:    s,
		loading:    false,
		message:    "",
		msgStyle:   lipgloss.NewStyle(),
		width:      80,
		height:     24,
		keys:       DefaultServerKeyMap(),
		configPath: configPath,
		password:   password,
	}
}

// Init initializes the server model
func (m ServerModel) Init() tea.Cmd {
	return tea.Batch(
		m.pollServerStatus(),
		m.serverTickCmd(),
	)
}

// pollServerStatus returns a command that fetches the current server status
func (m ServerModel) pollServerStatus() tea.Cmd {
	return func() tea.Msg {
		server := embedded.GlobalServer()
		status := ServerControlStatus{
			Running:        server.IsRunning(),
			Port:           server.Port(),
			ConnectedCount: 0, // TODO: Implement actual connection tracking
			StartedAt:      server.StartedAt(),
			ConfigPath:     server.ConfigPath(),
		}
		if status.Port == 0 {
			status.Port = 8317 // Default port
		}
		return serverControlStatusMsg(status)
	}
}

// serverTickCmd returns a command that sends tick messages for polling
func (m ServerModel) serverTickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return serverTickMsg(t)
	})
}

// Update handles messages for the server model
func (m ServerModel) Update(msg tea.Msg) (ServerModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case serverTickMsg:
		// Poll server status and schedule next tick
		return m, tea.Batch(m.pollServerStatus(), m.serverTickCmd())

	case serverControlStatusMsg:
		m.status = ServerControlStatus(msg)
		// Update options based on server state
		if m.status.Running {
			m.options = []string{
				"Start Server",
				"Stop Server",
				"Back to Menu",
			}
		} else {
			m.options = []string{
				"Start Server",
				"Stop Server",
				"Back to Menu",
			}
		}

	case serverActionResultMsg:
		m.loading = false
		if msg.success {
			m.message = fmt.Sprintf("Server %s successfully", msg.action)
			m.msgStyle = lipgloss.NewStyle().Foreground(SuccessColor)
		} else {
			errMsg := "unknown error"
			if msg.err != nil {
				errMsg = msg.err.Error()
			}
			m.message = fmt.Sprintf("Failed to %s server: %s", msg.action, errMsg)
			m.msgStyle = lipgloss.NewStyle().Foreground(ErrorColor)
		}
		// Refresh status after action
		cmds = append(cmds, m.pollServerStatus())

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.KeyMsg:
		// Handle key presses
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Select):
			return m.handleSelection()
		}
	}

	return m, tea.Batch(cmds...)
}

// handleSelection handles menu option selection
func (m ServerModel) handleSelection() (ServerModel, tea.Cmd) {
	switch ServerMenuOption(m.cursor) {
	case ServerMenuOptionStart:
		if !m.status.Running {
			m.loading = true
			m.message = ""
			return m, tea.Batch(m.spinner.Tick, m.startServer())
		} else {
			m.message = "Server is already running"
			m.msgStyle = lipgloss.NewStyle().Foreground(WarningColor)
		}

	case ServerMenuOptionStop:
		if m.status.Running {
			m.loading = true
			m.message = ""
			return m, tea.Batch(m.spinner.Tick, m.stopServer())
		} else {
			m.message = "Server is not running"
			m.msgStyle = lipgloss.NewStyle().Foreground(WarningColor)
		}

	case ServerMenuOptionBack:
		// Return to main menu - handled by parent model
	}

	return m, nil
}

// startServer returns a command that starts the server
func (m ServerModel) startServer() tea.Cmd {
	return func() tea.Msg {
		configPath := m.configPath
		if configPath == "" {
			configPath = embedded.GlobalServer().ConfigPath()
		}
		if configPath == "" {
			configPath = "config.yaml"
		}

		err := embedded.StartGlobal(configPath, m.password)
		return serverActionResultMsg{
			action:  "started",
			success: err == nil,
			err:     err,
		}
	}
}

// stopServer returns a command that stops the server
func (m ServerModel) stopServer() tea.Cmd {
	return func() tea.Msg {
		err := embedded.StopGlobal()
		return serverActionResultMsg{
			action:  "stopped",
			success: err == nil,
			err:     err,
		}
	}
}

// IsBackSelected returns true if the back option is selected
func (m ServerModel) IsBackSelected() bool {
	return ServerMenuOption(m.cursor) == ServerMenuOptionBack
}

// View renders the server control screen
func (m ServerModel) View() string {
	var b strings.Builder

	// Title
	title := TitleStyle.Render("Server Control")
	b.WriteString(title)
	b.WriteString("\n")

	// Divider
	dividerWidth := lipgloss.Width(title)
	if dividerWidth < 20 {
		dividerWidth = 20
	}
	b.WriteString(Dim(strings.Repeat("-", dividerWidth)))
	b.WriteString("\n\n")

	// Status panel
	statusPanel := m.renderStatusPanel()
	b.WriteString(statusPanel)
	b.WriteString("\n\n")

	// Options menu
	b.WriteString(Bold("Options"))
	b.WriteString("\n")
	b.WriteString(Dim(strings.Repeat("-", 10)))
	b.WriteString("\n")

	for i, opt := range m.options {
		cursor := "  "
		style := MenuItemStyle

		if i == m.cursor {
			cursor = Primary("> ")
			style = SelectedItemStyle
		}

		// Disable start option if running, disable stop if not running
		optText := opt
		if i == int(ServerMenuOptionStart) && m.status.Running {
			style = DisabledItemStyle
			optText = opt + " (running)"
		} else if i == int(ServerMenuOptionStop) && !m.status.Running {
			style = DisabledItemStyle
			optText = opt + " (stopped)"
		}

		b.WriteString(cursor)
		b.WriteString(style.Render(optText))
		b.WriteString("\n")
	}

	// Loading indicator
	if m.loading {
		b.WriteString("\n")
		b.WriteString(m.spinner.View())
		b.WriteString(" Processing...")
	}

	// Status message
	if m.message != "" && !m.loading {
		b.WriteString("\n")
		b.WriteString(m.msgStyle.Render(m.message))
	}

	// Help text
	b.WriteString("\n\n")
	help := m.renderHelp()
	b.WriteString(HelpStyle.Render(help))

	return BoxStyle.Render(b.String())
}

// renderStatusPanel renders the server status information panel
func (m ServerModel) renderStatusPanel() string {
	var b strings.Builder

	// Status header
	b.WriteString(Bold("Server Status"))
	b.WriteString("\n")
	b.WriteString(Dim(strings.Repeat("-", 15)))
	b.WriteString("\n\n")

	// Running status with badge
	b.WriteString("  Status:      ")
	if m.status.Running {
		b.WriteString(SuccessBadge.Render(" RUNNING "))
	} else {
		b.WriteString(ErrorBadge.Render(" STOPPED "))
	}
	b.WriteString("\n")

	// Port number
	b.WriteString("  Port:        ")
	b.WriteString(Primary(fmt.Sprintf("%d", m.status.Port)))
	b.WriteString("\n")

	// Connected clients
	b.WriteString("  Clients:     ")
	clientsStr := fmt.Sprintf("%d connected", m.status.ConnectedCount)
	if m.status.ConnectedCount > 0 {
		b.WriteString(Success(clientsStr))
	} else {
		b.WriteString(Muted(clientsStr))
	}
	b.WriteString("\n")

	// Uptime (if running)
	if m.status.Running && !m.status.StartedAt.IsZero() {
		uptime := time.Since(m.status.StartedAt)
		b.WriteString("  Uptime:      ")
		b.WriteString(Info(formatServerUptime(uptime)))
		b.WriteString("\n")
	}

	// Config path
	if m.status.ConfigPath != "" {
		b.WriteString("  Config:      ")
		configDisplay := m.status.ConfigPath
		if len(configDisplay) > 40 {
			configDisplay = "..." + configDisplay[len(configDisplay)-37:]
		}
		b.WriteString(Muted(configDisplay))
		b.WriteString("\n")
	}

	return RoundedBorder.Render(b.String())
}

// renderHelp renders the help text
func (m ServerModel) renderHelp() string {
	parts := []string{
		HelpKeyStyle.Render("^/v") + HelpDescStyle.Render(": navigate"),
		HelpKeyStyle.Render("enter") + HelpDescStyle.Render(": select"),
		HelpKeyStyle.Render("esc/b") + HelpDescStyle.Render(": back"),
		HelpKeyStyle.Render("q") + HelpDescStyle.Render(": quit"),
	}
	return strings.Join(parts, "  ")
}

// formatServerUptime formats a duration in a human-readable way
func formatServerUptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours < 24 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	days := hours / 24
	hours = hours % 24
	return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
}

// ServerScreen wraps ServerModel to implement tea.Model for standalone use
type ServerScreen struct {
	model    ServerModel
	quitting bool
	goBack   bool
}

// NewServerScreen creates a new standalone server screen
func NewServerScreen(configPath, password string) ServerScreen {
	return ServerScreen{
		model:    NewServerModel(configPath, password),
		quitting: false,
		goBack:   false,
	}
}

// Init initializes the server screen
func (s ServerScreen) Init() tea.Cmd {
	return s.model.Init()
}

// Update handles messages for the server screen
func (s ServerScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, s.model.keys.Quit):
			s.quitting = true
			return s, tea.Quit
		case key.Matches(msg, s.model.keys.Back):
			s.goBack = true
			return s, tea.Quit
		case key.Matches(msg, s.model.keys.Select):
			if s.model.IsBackSelected() {
				s.goBack = true
				return s, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	s.model, cmd = s.model.Update(msg)
	return s, cmd
}

// View renders the server screen
func (s ServerScreen) View() string {
	if s.quitting {
		return "Goodbye!\n"
	}
	return s.model.View()
}

// ShouldGoBack returns true if user wants to go back to menu
func (s ServerScreen) ShouldGoBack() bool {
	return s.goBack
}

// RunServerScreen runs the server control screen as a standalone program
func RunServerScreen(configPath, password string) error {
	p := tea.NewProgram(NewServerScreen(configPath, password), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
