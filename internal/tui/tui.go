package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Screen represents the different screens/views in the TUI
type Screen int

const (
	ScreenMainMenu Screen = iota
	ScreenServer
	ScreenSwitch
	ScreenLogin
	ScreenStatus
)

// MenuItem represents a menu item in the main menu
type MenuItem struct {
	Title  string
	Screen Screen
}

// Model is the main TUI application state
type Model struct {
	currentScreen Screen
	cursor        int
	menuItems     []MenuItem
	width         int
	height        int
	quitting      bool
	message       string // For displaying status messages
}

// Styles defines the lipgloss styles for the TUI
type Styles struct {
	Title         lipgloss.Style
	Divider       lipgloss.Style
	MenuItem      lipgloss.Style
	SelectedItem  lipgloss.Style
	Help          lipgloss.Style
	StatusMessage lipgloss.Style
	Container     lipgloss.Style
}

// DefaultStyles returns the default styling for the TUI
func DefaultStyles() Styles {
	return Styles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99")).
			MarginBottom(0),
		Divider: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),
		MenuItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			PaddingLeft(2),
		SelectedItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true).
			PaddingLeft(0),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1),
		StatusMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			MarginTop(1),
		Container: lipgloss.NewStyle().
			Padding(1, 2),
	}
}

var styles = DefaultStyles()

// NewModel creates a new TUI model with default values
func NewModel() Model {
	return Model{
		currentScreen: ScreenMainMenu,
		cursor:        0,
		menuItems: []MenuItem{
			{Title: "Start Server", Screen: ScreenServer},
			{Title: "Switch Agent Config", Screen: ScreenSwitch},
			{Title: "Login to Provider", Screen: ScreenLogin},
			{Title: "Show Status", Screen: ScreenStatus},
		},
		width:    80,
		height:   24,
		quitting: false,
		message:  "",
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}
	return m, nil
}

// handleKeyPress handles keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.currentScreen {
	case ScreenMainMenu:
		return m.handleMainMenuKeys(msg)
	case ScreenServer:
		return m.handleServerKeys(msg)
	case ScreenSwitch:
		return m.handleSwitchKeys(msg)
	case ScreenLogin:
		return m.handleLoginKeys(msg)
	case ScreenStatus:
		return m.handleStatusKeys(msg)
	}
	return m, nil
}

// handleMainMenuKeys handles key presses on the main menu
func (m Model) handleMainMenuKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.menuItems) { // Allow cursor to reach Exit option
			m.cursor++
		}
	case "enter", " ":
		// Handle menu selection
		if m.cursor == len(m.menuItems) {
			// Exit option selected
			m.quitting = true
			return m, tea.Quit
		}
		if m.cursor >= 0 && m.cursor < len(m.menuItems) {
			selectedItem := m.menuItems[m.cursor]
			m.currentScreen = selectedItem.Screen
			m.message = ""
		}
	}
	return m, nil
}

// handleServerKeys handles key presses on the server screen
func (m Model) handleServerKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc", "b":
		m.currentScreen = ScreenMainMenu
		m.message = ""
	}
	return m, nil
}

// handleSwitchKeys handles key presses on the switch screen
func (m Model) handleSwitchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc", "b":
		m.currentScreen = ScreenMainMenu
		m.message = ""
	}
	return m, nil
}

// handleLoginKeys handles key presses on the login screen
func (m Model) handleLoginKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc", "b":
		m.currentScreen = ScreenMainMenu
		m.message = ""
	}
	return m, nil
}

// handleStatusKeys handles key presses on the status screen
func (m Model) handleStatusKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc", "b":
		m.currentScreen = ScreenMainMenu
		m.message = ""
	}
	return m, nil
}

// View implements tea.Model
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	var content string
	switch m.currentScreen {
	case ScreenMainMenu:
		content = m.viewMainMenu()
	case ScreenServer:
		content = m.viewServer()
	case ScreenSwitch:
		content = m.viewSwitch()
	case ScreenLogin:
		content = m.viewLogin()
	case ScreenStatus:
		content = m.viewStatus()
	}

	return styles.Container.Render(content)
}

// viewMainMenu renders the main menu
func (m Model) viewMainMenu() string {
	var b strings.Builder

	// Title
	b.WriteString(styles.Title.Render("ProxyPilot CLI"))
	b.WriteString("\n")

	// Divider
	b.WriteString(styles.Divider.Render(strings.Repeat("─", 17)))
	b.WriteString("\n")

	// Menu items
	for i, item := range m.menuItems {
		if i == m.cursor {
			b.WriteString(styles.SelectedItem.Render(fmt.Sprintf("> %s", item.Title)))
		} else {
			b.WriteString(styles.MenuItem.Render(item.Title))
		}
		b.WriteString("\n")
	}

	// Exit option (special handling)
	if m.cursor == len(m.menuItems) {
		b.WriteString(styles.SelectedItem.Render("> Exit"))
	} else {
		b.WriteString(styles.MenuItem.Render("Exit"))
	}
	b.WriteString("\n")

	// Help text
	b.WriteString(styles.Help.Render("↑/↓: navigate • enter: select • q: quit"))

	// Status message if any
	if m.message != "" {
		b.WriteString("\n")
		b.WriteString(styles.StatusMessage.Render(m.message))
	}

	return b.String()
}

// viewServer renders the server screen
func (m Model) viewServer() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Server"))
	b.WriteString("\n")
	b.WriteString(styles.Divider.Render(strings.Repeat("─", 17)))
	b.WriteString("\n\n")

	b.WriteString("Server management screen.\n")
	b.WriteString("Press 's' to start/stop the server.\n")

	b.WriteString("\n")
	b.WriteString(styles.Help.Render("esc/b: back • q: quit"))

	return b.String()
}

// viewSwitch renders the switch agent config screen
func (m Model) viewSwitch() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Switch Agent Config"))
	b.WriteString("\n")
	b.WriteString(styles.Divider.Render(strings.Repeat("─", 19)))
	b.WriteString("\n\n")

	b.WriteString("Switch between different agent configurations.\n")
	b.WriteString("Select an agent config to switch to.\n")

	b.WriteString("\n")
	b.WriteString(styles.Help.Render("esc/b: back • q: quit"))

	return b.String()
}

// viewLogin renders the login screen
func (m Model) viewLogin() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Login to Provider"))
	b.WriteString("\n")
	b.WriteString(styles.Divider.Render(strings.Repeat("─", 17)))
	b.WriteString("\n\n")

	b.WriteString("Login to different AI providers.\n")
	b.WriteString("Select a provider to authenticate.\n")

	b.WriteString("\n")
	b.WriteString(styles.Help.Render("esc/b: back • q: quit"))

	return b.String()
}

// viewStatus renders the status screen
func (m Model) viewStatus() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Status"))
	b.WriteString("\n")
	b.WriteString(styles.Divider.Render(strings.Repeat("─", 17)))
	b.WriteString("\n\n")

	b.WriteString("Current system status.\n")
	b.WriteString("Server: Not running\n")
	b.WriteString("Active Config: None\n")

	b.WriteString("\n")
	b.WriteString(styles.Help.Render("esc/b: back • q: quit"))

	return b.String()
}

// Run starts the TUI application
func Run() error {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// GetCurrentScreen returns the current screen
func (m Model) GetCurrentScreen() Screen {
	return m.currentScreen
}

// SetMessage sets a status message to display
func (m *Model) SetMessage(msg string) {
	m.message = msg
}
