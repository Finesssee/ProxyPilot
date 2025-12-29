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

// View implements tea.Model
func (m SwitchModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Title
	title := TitleStyle.Render("Switch Agent Configuration")
	b.WriteString(title)
	b.WriteString("\n")

	// Separator line
	separator := lipgloss.NewStyle().
		Foreground(BorderColor).
		Render(strings.Repeat("─", min(26, m.width-2)))
	b.WriteString(separator)
	b.WriteString("\n\n")

	// Agent list
	for i, agent := range m.agents {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		// Format the mode badge
		modeBadge := m.formatModeBadge(agent)

		// Build the line
		var line string
		nameStyle := MenuItemStyle
		if i == m.cursor {
			nameStyle = SelectedItemStyle.Copy().PaddingLeft(0)
		}

		// Pad the name to align badges
		paddedName := fmt.Sprintf("%-15s", agent.DisplayName)

		if i == m.cursor {
			cursorStr := CursorStyle.Render(cursor)
			nameStr := nameStyle.Render(paddedName)
			line = cursorStr + nameStr + " " + modeBadge
		} else {
			line = cursor + nameStyle.Render(paddedName) + " " + modeBadge
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	// Status message (if any)
	if m.message != "" {
		b.WriteString("\n")
		msgStyle := lipgloss.NewStyle().Foreground(InfoColor)
		if strings.HasPrefix(m.message, "Error:") {
			msgStyle = lipgloss.NewStyle().Foreground(ErrorColor)
		}
		b.WriteString(msgStyle.Render(m.message))
		b.WriteString("\n")
	}

	// Help footer
	b.WriteString("\n")
	helpText := m.renderHelp()
	b.WriteString(helpText)

	return b.String()
}

// formatModeBadge returns a styled mode badge for an agent
func (m SwitchModel) formatModeBadge(agent AgentItem) string {
	if !agent.Available {
		// Agent not found or has special status
		msg := agent.Message
		if msg == "" {
			msg = "NOT FOUND"
		}
		// Truncate long messages
		if len(msg) > 20 {
			msg = msg[:17] + "..."
		}
		return MutedBadge.Render(strings.ToUpper(msg))
	}

	switch agent.Mode {
	case cmd.ModeProxy:
		return SuccessBadge.Render("PROXY")
	case cmd.ModeNative:
		return WarningBadge.Render("NATIVE")
	default:
		return MutedBadge.Render("UNKNOWN")
	}
}

// renderHelp renders the help footer
func (m SwitchModel) renderHelp() string {
	keyStyle := HelpKeyStyle
	descStyle := HelpDescStyle

	help := []string{
		keyStyle.Render("[Enter]") + " " + descStyle.Render("Toggle Mode"),
		keyStyle.Render("[Esc]") + " " + descStyle.Render("Back"),
	}

	return HelpStyle.Render(strings.Join(help, "  "))
}

// RunSwitchScreen runs the switch screen as a standalone TUI program
func RunSwitchScreen(cfg *config.Config) error {
	model := NewSwitchModel(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
