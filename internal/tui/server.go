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
	"github.com/router-for-me/CLIProxyAPI/v6/internal/api/middleware"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/embedded"
)

// ═══════════════════════════════════════════════════════════════════════════════
// SERVER CONTROL PANEL - Cyberpunk Neon Edition
// ═══════════════════════════════════════════════════════════════════════════════

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

// ═══════════════════════════════════════════════════════════════════════════════
// CYBERPUNK STYLES - Neon-infused visual components
// ═══════════════════════════════════════════════════════════════════════════════

var (
	// Panel title with electric glow
	serverTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(Cyan).
				Background(DarkSurface).
				Padding(0, 2).
				MarginBottom(1)

	// Main control panel container
	controlPanelStyle = lipgloss.NewStyle().
				Border(CyberBorder).
				BorderForeground(Cyan).
				Background(DeepBlack).
				Padding(1, 3).
				MarginBottom(1)

	// Status panel with neon border
	statusPanelStyle = lipgloss.NewStyle().
				Border(SoftBorder).
				BorderForeground(Violet).
				Background(DarkSurface).
				Padding(1, 2).
				MarginBottom(1)

	// Glowing status indicator - RUNNING
	runningBadgeStyle = lipgloss.NewStyle().
				Foreground(DeepBlack).
				Background(NeonGreen).
				Bold(true).
				Padding(0, 2)

	// Glowing status indicator - STOPPED
	stoppedBadgeStyle = lipgloss.NewStyle().
				Foreground(TextBright).
				Background(HotCoral).
				Bold(true).
				Padding(0, 2)

	// Action buttons - Normal state
	actionButtonStyle = lipgloss.NewStyle().
				Foreground(TextMuted).
				Padding(0, 1)

	// Action buttons - Selected/Focused
	actionButtonSelectedStyle = lipgloss.NewStyle().
					Foreground(DeepBlack).
					Background(Cyan).
					Bold(true).
					Padding(0, 2)

	// Action buttons - Disabled
	actionButtonDisabledStyle = lipgloss.NewStyle().
					Foreground(TextDim).
					Background(Surface).
					Padding(0, 1)

	// Cursor arrow with magenta glow
	cursorActiveStyle = lipgloss.NewStyle().
				Foreground(Magenta).
				Bold(true)

	// Data labels
	labelStyle = lipgloss.NewStyle().
			Foreground(TextMuted).
			Width(14)

	// Data values with cyan highlight
	valueStyle = lipgloss.NewStyle().
			Foreground(Cyan).
			Bold(true)

	// Success message style
	successMsgStyle = lipgloss.NewStyle().
			Foreground(NeonGreen).
			Bold(true)

	// Error message style
	errorMsgStyle = lipgloss.NewStyle().
			Foreground(HotCoral).
			Bold(true)

	// Warning message style
	warningMsgStyle = lipgloss.NewStyle().
			Foreground(Amber).
			Bold(true)

	// Info/uptime style
	infoValueStyle = lipgloss.NewStyle().
			Foreground(ElecBlue)

	// Muted path/config style
	pathStyle = lipgloss.NewStyle().
			Foreground(TextDim).
			Italic(true)

	// Section header
	sectionHeaderStyle = lipgloss.NewStyle().
				Foreground(Violet).
				Bold(true).
				MarginTop(1).
				MarginBottom(1)

	// Help key highlight
	helpKeyNeonStyle = lipgloss.NewStyle().
				Foreground(Cyan).
				Background(Surface).
				Bold(true).
				Padding(0, 1)

	// Help description
	helpDescNeonStyle = lipgloss.NewStyle().
				Foreground(TextDim)

	// Spinner with cyan glow
	neonSpinnerStyle = lipgloss.NewStyle().
				Foreground(Cyan)

	// Decorative divider
	dividerStyle = lipgloss.NewStyle().
			Foreground(BorderDim)
)

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
	s.Spinner = spinner.Points
	s.Style = neonSpinnerStyle

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
			ConnectedCount: int(middleware.ActiveConnections.Count()),
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
			m.message = fmt.Sprintf("%s Server %s successfully", IconCheck, msg.action)
			m.msgStyle = successMsgStyle
		} else {
			errMsg := "unknown error"
			if msg.err != nil {
				errMsg = msg.err.Error()
			}
			m.message = fmt.Sprintf("%s Failed to %s server: %s", IconCross, msg.action, errMsg)
			m.msgStyle = errorMsgStyle
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
			m.message = fmt.Sprintf("%s Server is already running", IconWarning)
			m.msgStyle = warningMsgStyle
		}

	case ServerMenuOptionStop:
		if m.status.Running {
			m.loading = true
			m.message = ""
			return m, tea.Batch(m.spinner.Tick, m.stopServer())
		} else {
			m.message = fmt.Sprintf("%s Server is not running", IconWarning)
			m.msgStyle = warningMsgStyle
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

	// ═══════════════════════════════════════════════════════════════════════
	// HEADER - Cyberpunk title with neon glow
	// ═══════════════════════════════════════════════════════════════════════
	title := m.renderHeader()
	b.WriteString(title)
	b.WriteString("\n\n")

	// ═══════════════════════════════════════════════════════════════════════
	// STATUS PANEL - Server status with glowing indicators
	// ═══════════════════════════════════════════════════════════════════════
	statusPanel := m.renderStatusPanel()
	b.WriteString(statusPanel)
	b.WriteString("\n\n")

	// ═══════════════════════════════════════════════════════════════════════
	// ACTION BUTTONS - Neon-styled control options
	// ═══════════════════════════════════════════════════════════════════════
	actionsPanel := m.renderActionsPanel()
	b.WriteString(actionsPanel)

	// ═══════════════════════════════════════════════════════════════════════
	// FEEDBACK - Loading/Status messages
	// ═══════════════════════════════════════════════════════════════════════
	if m.loading {
		b.WriteString("\n\n")
		loadingText := fmt.Sprintf("  %s %s", m.spinner.View(), lipgloss.NewStyle().Foreground(Cyan).Render("Processing..."))
		b.WriteString(loadingText)
	}

	if m.message != "" && !m.loading {
		b.WriteString("\n\n")
		b.WriteString("  " + m.msgStyle.Render(m.message))
	}

	// ═══════════════════════════════════════════════════════════════════════
	// HELP BAR - Keyboard shortcuts
	// ═══════════════════════════════════════════════════════════════════════
	b.WriteString("\n\n")
	help := m.renderHelpBar()
	b.WriteString(help)

	return controlPanelStyle.Render(b.String())
}

// renderHeader renders the cyberpunk header with decorative elements
func (m ServerModel) renderHeader() string {
	// Create decorative line
	decorLine := lipgloss.NewStyle().Foreground(Violet).Render("━━━")

	// Main title with glow effect
	titleText := lipgloss.NewStyle().
		Foreground(Cyan).
		Bold(true).
		Render(" SERVER CONTROL ")

	// Accent decorations
	leftDecor := lipgloss.NewStyle().Foreground(Magenta).Bold(true).Render("[ ")
	rightDecor := lipgloss.NewStyle().Foreground(Magenta).Bold(true).Render(" ]")

	header := decorLine + leftDecor + titleText + rightDecor + decorLine

	// Subtitle
	subtitle := lipgloss.NewStyle().
		Foreground(TextDim).
		Render("ProxyPilot Control Panel")

	return header + "\n" + lipgloss.NewStyle().MarginLeft(4).Render(subtitle)
}

// renderStatusPanel renders the server status information panel with neon styling
func (m ServerModel) renderStatusPanel() string {
	var b strings.Builder

	// Section header with icon
	sectionIcon := lipgloss.NewStyle().Foreground(ElecBlue).Render(IconServer)
	sectionTitle := lipgloss.NewStyle().Foreground(Violet).Bold(true).Render(" STATUS MONITOR")
	b.WriteString(sectionIcon + sectionTitle)
	b.WriteString("\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("─", 40)))
	b.WriteString("\n\n")

	// Status indicator with glowing badge
	b.WriteString(labelStyle.Render("  Status"))
	b.WriteString(lipgloss.NewStyle().Foreground(TextDim).Render(": "))
	if m.status.Running {
		statusIcon := lipgloss.NewStyle().Foreground(NeonGreen).Render(IconOnline + " ")
		b.WriteString(statusIcon)
		b.WriteString(runningBadgeStyle.Render(" RUNNING "))
	} else {
		statusIcon := lipgloss.NewStyle().Foreground(HotCoral).Render(IconOffline + " ")
		b.WriteString(statusIcon)
		b.WriteString(stoppedBadgeStyle.Render(" STOPPED "))
	}
	b.WriteString("\n\n")

	// Port number with cyan highlight
	b.WriteString(labelStyle.Render("  Port"))
	b.WriteString(lipgloss.NewStyle().Foreground(TextDim).Render(": "))
	portValue := fmt.Sprintf(":%d", m.status.Port)
	b.WriteString(valueStyle.Render(portValue))
	b.WriteString("\n\n")

	// Endpoint display
	b.WriteString(labelStyle.Render("  Endpoint"))
	b.WriteString(lipgloss.NewStyle().Foreground(TextDim).Render(": "))
	endpoint := fmt.Sprintf("http://localhost%s", portValue)
	endpointStyle := lipgloss.NewStyle().Foreground(Cyan).Underline(true)
	b.WriteString(endpointStyle.Render(endpoint))
	b.WriteString("\n\n")

	// Connected clients
	b.WriteString(labelStyle.Render("  Clients"))
	b.WriteString(lipgloss.NewStyle().Foreground(TextDim).Render(": "))
	if m.status.ConnectedCount > 0 {
		clientStr := fmt.Sprintf("%d connected", m.status.ConnectedCount)
		b.WriteString(lipgloss.NewStyle().Foreground(NeonGreen).Bold(true).Render(clientStr))
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(TextMuted).Render("0 connected"))
	}
	b.WriteString("\n")

	// Uptime (if running)
	if m.status.Running && !m.status.StartedAt.IsZero() {
		uptime := time.Since(m.status.StartedAt)
		b.WriteString("\n")
		b.WriteString(labelStyle.Render("  Uptime"))
		b.WriteString(lipgloss.NewStyle().Foreground(TextDim).Render(": "))
		b.WriteString(infoValueStyle.Render(formatServerUptime(uptime)))
	}

	// Config path
	if m.status.ConfigPath != "" {
		b.WriteString("\n\n")
		b.WriteString(labelStyle.Render("  Config"))
		b.WriteString(lipgloss.NewStyle().Foreground(TextDim).Render(": "))
		configDisplay := m.status.ConfigPath
		if len(configDisplay) > 35 {
			configDisplay = "..." + configDisplay[len(configDisplay)-32:]
		}
		b.WriteString(pathStyle.Render(configDisplay))
	}

	return statusPanelStyle.Render(b.String())
}

// renderActionsPanel renders the action buttons with neon styling
func (m ServerModel) renderActionsPanel() string {
	var b strings.Builder

	// Section header
	sectionIcon := lipgloss.NewStyle().Foreground(Magenta).Render(IconTerminal)
	sectionTitle := lipgloss.NewStyle().Foreground(Violet).Bold(true).Render(" ACTIONS")
	b.WriteString(sectionIcon + sectionTitle)
	b.WriteString("\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("─", 40)))
	b.WriteString("\n\n")

	// Button icons
	buttonIcons := []string{IconBolt, IconSquare, IconArrowLeft}

	for i, opt := range m.options {
		isSelected := i == m.cursor
		isDisabled := false
		displayText := opt

		// Determine if option is disabled
		if i == int(ServerMenuOptionStart) && m.status.Running {
			isDisabled = true
			displayText = opt + " (running)"
		} else if i == int(ServerMenuOptionStop) && !m.status.Running {
			isDisabled = true
			displayText = opt + " (stopped)"
		}

		// Render cursor
		if isSelected {
			cursor := cursorActiveStyle.Render(IconChevron + " ")
			b.WriteString("  " + cursor)
		} else {
			b.WriteString("    ")
		}

		// Render button icon
		icon := buttonIcons[i]
		if isSelected && !isDisabled {
			iconStyle := lipgloss.NewStyle().Foreground(Cyan).Bold(true)
			b.WriteString(iconStyle.Render(icon) + " ")
		} else if isDisabled {
			iconStyle := lipgloss.NewStyle().Foreground(TextDim)
			b.WriteString(iconStyle.Render(icon) + " ")
		} else {
			iconStyle := lipgloss.NewStyle().Foreground(TextMuted)
			b.WriteString(iconStyle.Render(icon) + " ")
		}

		// Render button text
		var buttonStyle lipgloss.Style
		if isDisabled {
			buttonStyle = actionButtonDisabledStyle
		} else if isSelected {
			buttonStyle = actionButtonSelectedStyle
		} else {
			buttonStyle = actionButtonStyle
		}

		b.WriteString(buttonStyle.Render(displayText))
		b.WriteString("\n")
	}

	return b.String()
}

// renderHelpBar renders the help text with neon styling
func (m ServerModel) renderHelpBar() string {
	var parts []string

	// Navigation keys
	navKey := helpKeyNeonStyle.Render("^/v")
	navDesc := helpDescNeonStyle.Render(" navigate")
	parts = append(parts, navKey+navDesc)

	// Select key
	selKey := helpKeyNeonStyle.Render("Enter")
	selDesc := helpDescNeonStyle.Render(" select")
	parts = append(parts, selKey+selDesc)

	// Back key
	backKey := helpKeyNeonStyle.Render("Esc")
	backDesc := helpDescNeonStyle.Render(" back")
	parts = append(parts, backKey+backDesc)

	// Quit key
	quitKey := helpKeyNeonStyle.Render("q")
	quitDesc := helpDescNeonStyle.Render(" quit")
	parts = append(parts, quitKey+quitDesc)

	// Join with separator
	separator := lipgloss.NewStyle().Foreground(BorderDim).Render(" | ")
	helpLine := strings.Join(parts, separator)

	// Wrap in subtle container
	return lipgloss.NewStyle().
		Foreground(TextDim).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(BorderDim).
		PaddingTop(1).
		Render(helpLine)
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

// ═══════════════════════════════════════════════════════════════════════════════
// SERVER SCREEN - Standalone wrapper for the server control panel
// ═══════════════════════════════════════════════════════════════════════════════

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
		// Styled goodbye message
		goodbye := lipgloss.NewStyle().
			Foreground(Cyan).
			Bold(true).
			Render("Goodbye!")
		return goodbye + "\n"
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
