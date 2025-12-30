package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/cmd"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

// ═══════════════════════════════════════════════════════════════════════════════
// SWITCH SCREEN - Cyberpunk Neon Agent Configuration Panel
// ═══════════════════════════════════════════════════════════════════════════════

// AgentItem represents a single agent in the switch list
type AgentItem struct {
	ID          string
	DisplayName string
	Mode        cmd.SwitchMode
	Available   bool
	Message     string // For errors or special messages
}

// SwitchModel is the bubbletea model for the agent switch screen
type SwitchModel struct {
	agents   []AgentItem
	cursor   int
	config   *config.Config
	width    int
	height   int
	quitting bool
	message  string // Status message after toggle
}

// SwitchKeyMap defines key bindings for the switch screen
type SwitchKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Toggle key.Binding
	Quit   key.Binding
}

// ═══════════════════════════════════════════════════════════════════════════════
// AGENT TYPE ICONS - Violet accented symbols
// ═══════════════════════════════════════════════════════════════════════════════

var agentIcons = map[string]string{
	"claude":   "C",
	"gemini":   "G",
	"codex":    "X",
	"opencode": "O",
	"droid":    "D",
	"cursor":   "R",
	"kilo":     "K",
	"roocode":  "P",
}

// ═══════════════════════════════════════════════════════════════════════════════
// SWITCH SCREEN STYLES - Using colors from styles.go
// ═══════════════════════════════════════════════════════════════════════════════

var (
	// Header with electric glow
	switchHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(Accent).
				Background(BgDark).
				Padding(0, 1).
				MarginBottom(1)

	// Agent card - unselected
	agentCardStyle = lipgloss.NewStyle().
			Border(RoundedBorder).
			BorderForeground(BorderDim).
			Padding(0, 2).
			Width(52).
			MarginBottom(0)

	// Agent card - selected with glowing accent border
	agentCardSelectedStyle = lipgloss.NewStyle().
				Border(RoundedBorder).
				BorderForeground(Accent).
				Padding(0, 2).
				Width(52).
				MarginBottom(0)

	// Agent name - normal
	agentNameStyle = lipgloss.NewStyle().
			Foreground(Text).
			Bold(true)

	// Agent name - selected (glowing accent)
	agentNameSelectedStyle = lipgloss.NewStyle().
				Foreground(AccentBright).
				Bold(true)

	// Agent icon container with tertiary accent
	agentIconStyle = lipgloss.NewStyle().
			Foreground(BgDark).
			Background(Tertiary).
			Bold(true).
			Padding(0, 1)

	// Mode badge - PROXY (Green)
	modeBadgeProxy = lipgloss.NewStyle().
			Foreground(BgDark).
			Background(Green).
			Bold(true).
			Padding(0, 1)

	// Mode badge - DIRECT (Yellow/Amber)
	modeBadgeDirect = lipgloss.NewStyle().
			Foreground(BgDark).
			Background(Yellow).
			Bold(true).
			Padding(0, 1)

	// Mode badge - unavailable
	modeBadgeUnavailable = lipgloss.NewStyle().
				Foreground(TextMuted).
				Background(BgPanel).
				Padding(0, 1)

	// Availability status - online
	statusOnlineStyle = lipgloss.NewStyle().
				Foreground(Green).
				Bold(true)

	// Availability status - offline
	statusOfflineStyle = lipgloss.NewStyle().
				Foreground(Red).
				Bold(true)

	// Toggle indicator
	toggleIndicatorStyle = lipgloss.NewStyle().
				Foreground(Secondary).
				Bold(true)

	// Success message
	successMessageStyle = lipgloss.NewStyle().
				Foreground(Green).
				Bold(true).
				Padding(0, 1)

	// Error message
	errorMessageStyle = lipgloss.NewStyle().
				Foreground(Red).
				Bold(true).
				Padding(0, 1)

	// Separator line
	separatorStyle = lipgloss.NewStyle().
			Foreground(Tertiary)

	// Help section container
	helpContainerStyle = lipgloss.NewStyle().
				Border(RoundedBorder).
				BorderForeground(BorderDim).
				BorderTop(true).
				BorderBottom(false).
				BorderLeft(false).
				BorderRight(false).
				Padding(1, 0).
				MarginTop(1)

	// Cursor arrow
	cursorArrowStyle = lipgloss.NewStyle().
				Foreground(Secondary).
				Bold(true)
)

// ═══════════════════════════════════════════════════════════════════════════════
// KEY BINDINGS
// ═══════════════════════════════════════════════════════════════════════════════

// DefaultSwitchKeyMap returns the default key bindings
func DefaultSwitchKeyMap() SwitchKeyMap {
	return SwitchKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("up/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("down/j", "down"),
		),
		Toggle: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter", "toggle mode"),
		),
		Quit: key.NewBinding(
			key.WithKeys("esc", "q"),
			key.WithHelp("esc/q", "back"),
		),
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// MODEL INITIALIZATION
// ═══════════════════════════════════════════════════════════════════════════════

// NewSwitchModel creates a new switch model
func NewSwitchModel(cfg *config.Config) SwitchModel {
	m := SwitchModel{
		config: cfg,
		width:  80,
		height: 24,
	}
	m.refreshAgents()
	return m
}

// refreshAgents reloads agent status from the cmd package
func (m *SwitchModel) refreshAgents() {
	agentIDs := []string{"claude", "gemini", "codex", "opencode", "droid", "cursor", "kilo", "roocode"}
	m.agents = make([]AgentItem, 0, len(agentIDs))

	for _, id := range agentIDs {
		result := cmd.GetSwitchStatus(id)
		item := AgentItem{
			ID:          id,
			DisplayName: result.Agent,
			Mode:        result.Mode,
			Available:   result.Success,
			Message:     result.Message,
		}
		m.agents = append(m.agents, item)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// TEA MODEL IMPLEMENTATION
// ═══════════════════════════════════════════════════════════════════════════════

// Init implements tea.Model
func (m SwitchModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m SwitchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keys := DefaultSwitchKeyMap()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
			m.message = ""
			return m, nil

		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.agents)-1 {
				m.cursor++
			}
			m.message = ""
			return m, nil

		case key.Matches(msg, keys.Toggle):
			m.toggleCurrent()
			return m, nil
		}
	}

	return m, nil
}

// toggleCurrent toggles the mode of the currently selected agent
func (m *SwitchModel) toggleCurrent() {
	if m.cursor >= len(m.agents) {
		return
	}

	agent := m.agents[m.cursor]

	// Determine target mode (toggle)
	var targetMode cmd.SwitchMode
	if agent.Mode == cmd.ModeProxy {
		targetMode = cmd.ModeNative
	} else {
		targetMode = cmd.ModeProxy
	}

	// Perform the switch
	var result cmd.SwitchResult
	if targetMode == cmd.ModeProxy {
		result = cmd.SwitchToProxy(m.config, agent.ID)
	} else {
		result = cmd.SwitchToNative(agent.ID)
	}

	// Update the agent state
	if result.Success {
		m.agents[m.cursor].Mode = result.Mode
		m.agents[m.cursor].Available = true
		m.message = fmt.Sprintf("%s switched to %s", result.Agent, strings.ToUpper(string(result.Mode)))
	} else {
		m.message = fmt.Sprintf("Error: %s", result.Message)
	}

	// Refresh status to ensure accurate display
	m.refreshAgents()
}

// ═══════════════════════════════════════════════════════════════════════════════
// VIEW RENDERING - Premium Cyberpunk Interface
// ═══════════════════════════════════════════════════════════════════════════════

// View implements tea.Model - Returns content only (no outer borders)
func (m SwitchModel) View() string {
	return m.ViewWithSize(m.width, m.height)
}

// ViewWithSize renders with explicit dimensions for responsive layout
func (m SwitchModel) ViewWithSize(width, height int) string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Section title (compact)
	title := RenderSectionTitle("Agent Configuration")
	subtitle := lipgloss.NewStyle().
		Foreground(TextMuted).
		Render(" - Toggle routing modes")
	b.WriteString(title + subtitle + "\n\n")

	// Calculate visible rows based on height
	maxRows := height - 5
	if maxRows < 4 {
		maxRows = 4
	}
	if maxRows > len(m.agents) {
		maxRows = len(m.agents)
	}

	// Agent list - responsive rows
	for i := 0; i < maxRows; i++ {
		if i >= len(m.agents) {
			break
		}
		agent := m.agents[i]
		isSelected := i == m.cursor
		row := m.renderAgentRowWithWidth(agent, isSelected, width)
		b.WriteString(row + "\n")
	}

	// Status message
	if m.message != "" {
		b.WriteString("\n")
		if strings.HasPrefix(m.message, "Error:") {
			b.WriteString(lipgloss.NewStyle().Foreground(Red).Render(IconCross + " " + m.message))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(Green).Render(IconCheck + " " + m.message))
		}
	}

	return b.String()
}

// renderAgentRowWithWidth creates a row that adapts to available width
func (m SwitchModel) renderAgentRowWithWidth(agent AgentItem, isSelected bool, width int) string {
	// Cursor indicator
	var cursor string
	if isSelected {
		cursor = lipgloss.NewStyle().Foreground(Accent).Bold(true).Render(IconChevron + " ")
	} else {
		cursor = "  "
	}

	// Agent icon
	icon := agentIcons[agent.ID]
	if icon == "" {
		icon = "?"
	}
	iconRendered := lipgloss.NewStyle().
		Foreground(BgDark).
		Background(Tertiary).
		Bold(true).
		Padding(0, 1).
		Render(icon)

	// Calculate responsive widths
	nameWidth := 14
	if width > 60 {
		nameWidth = 18
	}

	// Agent name
	var nameStyle lipgloss.Style
	if isSelected {
		nameStyle = lipgloss.NewStyle().Foreground(AccentBright).Bold(true).Width(nameWidth)
	} else {
		nameStyle = lipgloss.NewStyle().Foreground(Text).Width(nameWidth)
	}
	name := nameStyle.Render(agent.DisplayName)

	// Status badge (only show on wider terminals)
	var status string
	if width > 50 {
		if agent.Available {
			status = lipgloss.NewStyle().Foreground(Green).Bold(true).Render("◉ READY")
		} else {
			status = lipgloss.NewStyle().Foreground(Red).Bold(true).Render("✗ N/A  ")
		}
		status = lipgloss.NewStyle().Width(10).Render(status)
	}

	// Mode badge
	modeBadge := m.renderModeBadge(agent)

	// Compose row
	row := cursor + iconRendered + " " + name
	if status != "" {
		row += " " + status
	}
	row += " " + modeBadge

	// Selection highlight
	if isSelected {
		row = lipgloss.NewStyle().Background(BgSelected).Render(row)
	}

	return row
}

// renderAgentRow creates a simple row for the agent list (no card borders)
func (m SwitchModel) renderAgentRow(agent AgentItem, isSelected bool) string {
	// Cursor indicator
	var cursor string
	if isSelected {
		cursor = lipgloss.NewStyle().Foreground(Accent).Bold(true).Render(IconChevron + " ")
	} else {
		cursor = "  "
	}

	// Agent icon
	icon := agentIcons[agent.ID]
	if icon == "" {
		icon = "?"
	}
	iconRendered := lipgloss.NewStyle().
		Foreground(BgDark).
		Background(Tertiary).
		Bold(true).
		Padding(0, 1).
		Render(icon)

	// Agent name
	var nameStyle lipgloss.Style
	if isSelected {
		nameStyle = lipgloss.NewStyle().Foreground(AccentBright).Bold(true).Width(16)
	} else {
		nameStyle = lipgloss.NewStyle().Foreground(Text).Width(16)
	}
	name := nameStyle.Render(agent.DisplayName)

	// Status badge
	var status string
	if agent.Available {
		status = lipgloss.NewStyle().Foreground(Green).Bold(true).Render("◉ READY")
	} else {
		status = lipgloss.NewStyle().Foreground(Red).Bold(true).Render("✗ N/A  ")
	}
	statusPadded := lipgloss.NewStyle().Width(10).Render(status)

	// Mode badge
	modeBadge := m.renderModeBadge(agent)

	// Compose row
	row := fmt.Sprintf("%s%s %s %s %s", cursor, iconRendered, name, statusPadded, modeBadge)

	// Selection highlight
	if isSelected {
		row = lipgloss.NewStyle().Background(BgSelected).Render(row)
	}

	return row
}

// renderAgentCard creates a premium styled agent card (legacy, kept for standalone mode)
func (m SwitchModel) renderAgentCard(agent AgentItem, isSelected bool) string {
	// Get the icon for this agent type
	icon := agentIcons[agent.ID]
	if icon == "" {
		icon = "?"
	}

	// Render icon with tertiary accent
	iconRendered := agentIconStyle.Render(icon)

	// Render agent name
	var nameRendered string
	if isSelected {
		nameRendered = agentNameSelectedStyle.Render(agent.DisplayName)
	} else {
		nameRendered = agentNameStyle.Render(agent.DisplayName)
	}

	// Pad name for alignment
	namePadded := lipgloss.NewStyle().Width(18).Render(nameRendered)

	// Render availability badge
	var availBadge string
	if agent.Available {
		availBadge = lipgloss.NewStyle().Foreground(Green).Bold(true).Render("◉ READY")
	} else {
		availBadge = lipgloss.NewStyle().Foreground(Red).Bold(true).Render("✗ N/A")
	}
	availPadded := lipgloss.NewStyle().Width(12).Render(availBadge)

	// Render mode badge
	modeBadge := m.renderModeBadge(agent)

	// Build the card content
	cardContent := fmt.Sprintf("%s  %s  %s  %s", iconRendered, namePadded, availPadded, modeBadge)

	// Wrap in card container
	var cardStyle lipgloss.Style
	if isSelected {
		cardStyle = agentCardSelectedStyle
	} else {
		cardStyle = agentCardStyle
	}

	// Add cursor indicator
	var cursor string
	if isSelected {
		cursor = cursorArrowStyle.Render(IconChevron + " ")
	} else {
		cursor = "  "
	}

	return cursor + cardStyle.Render(cardContent)
}

// renderModeBadge returns a styled mode badge for an agent
func (m SwitchModel) renderModeBadge(agent AgentItem) string {
	if !agent.Available {
		// Agent not found or has special status - use ErrorBadgeFilled
		return ErrorBadgeFilled.Render("✗ N/A")
	}

	switch agent.Mode {
	case cmd.ModeProxy:
		// Proxy mode - Green filled badge with filled circle
		return SuccessBadgeFilled.Render("◉ PROXY")
	case cmd.ModeNative:
		// Direct/Native mode - Yellow/Warning filled badge with hollow circle
		return WarningBadgeFilled.Render("◯ DIRECT")
	default:
		return ErrorBadgeFilled.Render("✗ N/A")
	}
}

// renderNeonHelp renders the help footer with cyberpunk neon styling
func (m SwitchModel) renderNeonHelp() string {
	// Build help items using styles from styles.go
	items := []string{
		HelpKeyStyle.Copy().Background(BgPanel).Padding(0, 1).Render("Enter") + " " + HelpDescStyle.Render("Toggle Mode"),
		HelpKeyStyle.Copy().Background(BgPanel).Padding(0, 1).Render("j/k") + " " + HelpDescStyle.Render("Navigate"),
		HelpKeyStyle.Copy().Background(BgPanel).Padding(0, 1).Render("Esc") + " " + HelpDescStyle.Render("Back"),
	}

	// Separator
	sepLine := separatorStyle.Render(strings.Repeat("─", 54))

	// Join items with spacing
	helpLine := strings.Join(items, "    ")

	return sepLine + "\n" + "  " + helpLine
}

// ═══════════════════════════════════════════════════════════════════════════════
// ENTRY POINT
// ═══════════════════════════════════════════════════════════════════════════════

// RunSwitchScreen runs the switch screen as a standalone TUI program
func RunSwitchScreen(cfg *config.Config) error {
	model := NewSwitchModel(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
