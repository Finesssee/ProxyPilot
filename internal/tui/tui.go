package tui

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

// ═══════════════════════════════════════════════════════════════════════════════
// PREMIUM TUI - Proper lipgloss borders, high contrast, btop-style
// ═══════════════════════════════════════════════════════════════════════════════

type Screen int

const (
	ScreenDashboard Screen = iota
	ScreenServer
	ScreenProviders
	ScreenAgents
	ScreenUsage
	ScreenLogs
	ScreenMappings
	ScreenIntegrations
)

type MenuItem struct {
	Key    string
	Title  string
	Icon   string
	Screen Screen
}

type Model struct {
	screen        Screen
	cursor        int
	menu          []MenuItem
	width         int
	height        int
	quitting      bool
	ready         bool
	serverOn      bool
	cfg           *config.Config
	host          string
	port          int
	spinner       spinner.Model
	loginModel    LoginModel
	switchModel   SwitchModel
	mappingsModel MappingsModel
	usageModel    UsageModel
	logsModel     LogsModel
	tick          int
	spark         []int
}

type TickMsg time.Time

func doTick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func NewModel(cfg *config.Config, host string, port int) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(Accent)

	spark := make([]int, 40)
	for i := range spark {
		spark[i] = rand.Intn(8)
	}

	m := Model{
		screen: ScreenDashboard,
		cursor: 0,
		menu: []MenuItem{
			{Key: "1", Title: "Dashboard", Screen: ScreenDashboard},
			{Key: "2", Title: "Server", Screen: ScreenServer},
			{Key: "3", Title: "Providers", Screen: ScreenProviders},
			{Key: "4", Title: "Agents", Screen: ScreenAgents},
			{Key: "5", Title: "Usage", Screen: ScreenUsage},
			{Key: "6", Title: "Logs", Screen: ScreenLogs},
			{Key: "7", Title: "Mappings", Screen: ScreenMappings},
			{Key: "8", Title: "Setup", Screen: ScreenIntegrations},
		},
		width:         120,
		height:        40,
		cfg:           cfg,
		host:          host,
		port:          port,
		spinner:       s,
		loginModel:    NewLoginModel(cfg),
		switchModel:   NewSwitchModel(cfg),
		mappingsModel: NewMappingsModel(cfg),
		usageModel:    NewUsageModel(cfg),
		logsModel:     NewLogsModel(),
		spark:         spark,
	}

	status := CheckServerRunning(host, port)
	m.serverOn = status.Running

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, doTick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case TickMsg:
		m.tick++
		if m.tick%2 == 0 {
			m.spark = append(m.spark[1:], rand.Intn(8))
		}
		return m, doTick()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "1", "2", "3", "4", "5", "6", "7", "8":
			idx := int(msg.String()[0] - '1')
			if idx >= 0 && idx < len(m.menu) {
				m.cursor = idx
				m.screen = m.menu[idx].Screen
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.screen = m.menu[m.cursor].Screen
			}
		case "down", "j":
			if m.cursor < len(m.menu)-1 {
				m.cursor++
				m.screen = m.menu[m.cursor].Screen
			}
		case "esc":
			m.screen = ScreenDashboard
			m.cursor = 0
		case "r":
			status := CheckServerRunning(m.host, m.port)
			m.serverOn = status.Running
		}
	}

	// Forward to submodels
	switch m.screen {
	case ScreenProviders:
		updated, cmd := m.loginModel.Update(msg)
		if lm, ok := updated.(LoginModel); ok {
			m.loginModel = lm
		}
		cmds = append(cmds, cmd)
	case ScreenAgents:
		updated, cmd := m.switchModel.Update(msg)
		if sm, ok := updated.(SwitchModel); ok {
			m.switchModel = sm
		}
		cmds = append(cmds, cmd)
	case ScreenMappings:
		m.mappingsModel, _ = m.mappingsModel.Update(msg)
	case ScreenUsage:
		m.usageModel, _ = m.usageModel.Update(msg)
	case ScreenLogs:
		m.logsModel, _ = m.logsModel.Update(msg)
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// ═══════════════════════════════════════════════════════════════════════════════
// VIEW - Main layout with REAL lipgloss borders
// ═══════════════════════════════════════════════════════════════════════════════

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if !m.ready {
		return "\n  Loading..."
	}

	// Border (2) + Padding (2) = 4 chars overhead per bordered panel
	const panelOverhead = 4

	// Calculate dimensions - responsive sidebar
	// sidebarWidth is the CONTENT width (lipgloss adds border+padding on top)
	sidebarWidth := 20 // Renders as 20 + 4 = 24 total
	if m.width < 80 {
		sidebarWidth = 14 // Renders as 14 + 4 = 18 total
	}

	// contentWidth = total width - sidebar rendered - 1 space - content panel overhead
	// sidebar rendered = sidebarWidth + panelOverhead
	// We need: (sidebarWidth + panelOverhead) + 1 + (contentWidth + panelOverhead) <= m.width
	// So: contentWidth = m.width - sidebarWidth - panelOverhead - 1 - panelOverhead
	//                  = m.width - sidebarWidth - 9
	contentWidth := m.width - sidebarWidth - 9
	if contentWidth < 30 {
		contentWidth = 30
	}
	mainHeight := m.height - 4

	// Render components
	header := m.renderHeader()
	sidebar := m.renderSidebar(sidebarWidth, mainHeight-2)
	content := m.renderContentView(contentWidth, mainHeight-2)
	footer := m.renderFooter()

	// Join sidebar and content
	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, " ", content)

	return lipgloss.JoinVertical(lipgloss.Left, header, main, footer)
}

// ═══════════════════════════════════════════════════════════════════════════════
// HEADER - Status bar with logo, sparkline, and server status
// ═══════════════════════════════════════════════════════════════════════════════

func (m Model) renderHeader() string {
	// Logo
	logo := lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true).
		Render("◆ PROXYPILOT")

	version := lipgloss.NewStyle().
		Foreground(TextMuted).
		Render(" v0.2")

	// Status indicator
	var status string
	if m.serverOn {
		pulse := []string{"●", "◉", "●", "○"}[m.tick%4]
		status = lipgloss.NewStyle().Foreground(Green).Bold(true).Render(pulse + " ONLINE")
	} else {
		status = lipgloss.NewStyle().Foreground(Red).Bold(true).Render("○ OFFLINE")
	}

	// Sparkline with block characters
	sparkChars := []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	var spark strings.Builder
	for _, v := range m.spark {
		var color lipgloss.Color
		switch {
		case v < 2:
			color = Blue
		case v < 4:
			color = Accent
		case v < 6:
			color = Green
		default:
			color = Yellow
		}
		spark.WriteString(lipgloss.NewStyle().Foreground(color).Render(sparkChars[v]))
	}

	// Compose header
	leftSide := logo + version
	rightSide := spark.String() + "  " + status

	gap := m.width - lipgloss.Width(leftSide) - lipgloss.Width(rightSide) - 4
	if gap < 0 {
		gap = 0
	}

	content := "  " + leftSide + strings.Repeat(" ", gap) + rightSide + "  "

	// Header bar with background and bottom border
	return lipgloss.NewStyle().
		Background(BgSurface).
		Foreground(Text).
		Width(m.width).
		Render(content) + "\n"
}

// ═══════════════════════════════════════════════════════════════════════════════
// SIDEBAR - Navigation with REAL rounded border
// ═══════════════════════════════════════════════════════════════════════════════

func (m Model) renderSidebar(width, height int) string {
	var items []string

	for i, item := range m.menu {
		isSelected := i == m.cursor

		key := lipgloss.NewStyle().
			Foreground(TextMuted).
			Render(fmt.Sprintf("[%s]", item.Key))

		var row string
		if isSelected {
			// Selected: cyan cursor bar + highlighted background
			cursor := lipgloss.NewStyle().
				Foreground(Accent).
				Bold(true).
				Render("▌")
			title := lipgloss.NewStyle().
				Background(BgSelected).
				Foreground(Accent).
				Bold(true).
				Width(width - 7).
				Render(item.Title)
			row = cursor + " " + key + " " + title
		} else {
			title := lipgloss.NewStyle().
				Foreground(TextDim).
				Width(width - 7).
				Render(item.Title)
			row = "  " + key + " " + title
		}
		items = append(items, row)
	}

	// Pad to fill height
	for len(items) < height-2 {
		items = append(items, "")
	}

	content := strings.Join(items, "\n")

	// REAL lipgloss border - rounded, visible color
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Border).
		Width(width).
		Height(height).
		Padding(0, 1).
		Render(content)
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONTENT - Main area with ACCENT border
// ═══════════════════════════════════════════════════════════════════════════════

func (m Model) renderContentView(width, height int) string {
	var content string

	innerWidth := width - 4
	innerHeight := height - 2

	switch m.screen {
	case ScreenDashboard:
		content = m.renderDashboard(innerWidth, innerHeight)
	case ScreenServer:
		content = m.renderServerView(innerWidth, innerHeight)
	case ScreenProviders:
		content = m.loginModel.ViewWithSize(innerWidth, innerHeight)
	case ScreenAgents:
		content = m.switchModel.ViewWithSize(innerWidth, innerHeight)
	case ScreenMappings:
		content = m.mappingsModel.ViewWithSize(innerWidth, innerHeight)
	case ScreenUsage:
		content = m.usageModel.ViewWithSize(innerWidth, innerHeight)
	case ScreenLogs:
		content = m.logsModel.ViewWithSize(innerWidth, innerHeight)
	case ScreenIntegrations:
		content = m.renderSetupView(innerWidth, innerHeight)
	default:
		content = "Select an option from the menu"
	}

	// Content panel with ACCENT colored border
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Accent).
		Width(width).
		Height(height).
		Padding(0, 1).
		Render(content)
}

// ═══════════════════════════════════════════════════════════════════════════════
// DASHBOARD - Multi-card layout with bordered sections
// ═══════════════════════════════════════════════════════════════════════════════

func (m Model) renderDashboard(width, height int) string {
	var b strings.Builder

	// Card style with dim border
	cardBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderDim).
		Padding(0, 1)

	titleStyle := lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true)

	labelStyle := lipgloss.NewStyle().Foreground(TextMuted)
	valueStyle := lipgloss.NewStyle().Foreground(Accent).Bold(true)

	// Determine layout mode based on width
	narrowMode := width < 50
	var cardWidth int
	if narrowMode {
		// Single column: cards use full width
		cardWidth = width - 2
	} else {
		// Two column: cards share width
		cardWidth = (width - 3) / 2
	}

	// ─── SERVER STATUS ───
	var statusIcon, statusText string
	if m.serverOn {
		statusIcon = lipgloss.NewStyle().Foreground(Green).Render("●")
		statusText = lipgloss.NewStyle().Foreground(Green).Bold(true).Render("Running")
	} else {
		statusIcon = lipgloss.NewStyle().Foreground(Red).Render("○")
		statusText = lipgloss.NewStyle().Foreground(Red).Bold(true).Render("Stopped")
	}

	serverContent := titleStyle.Render("Server") + "\n\n" +
		fmt.Sprintf(" %s %s\n", statusIcon, statusText) +
		fmt.Sprintf(" %s %s", labelStyle.Render("Endpoint:"), valueStyle.Render(fmt.Sprintf("%s:%d", m.host, m.port)))
	serverCard := cardBorder.Width(cardWidth).Render(serverContent)

	// ─── STATISTICS ───
	agentCount := 0
	for _, a := range m.switchModel.agents {
		if a.Available {
			agentCount++
		}
	}
	providerCount := 0
	if m.cfg != nil {
		if len(m.cfg.ClaudeKey) > 0 {
			providerCount++
		}
		if len(m.cfg.GeminiKey) > 0 {
			providerCount++
		}
		if len(m.cfg.CodexKey) > 0 {
			providerCount++
		}
	}

	statsContent := titleStyle.Render("Statistics") + "\n\n" +
		fmt.Sprintf(" %s %s\n", labelStyle.Render("Agents:"), valueStyle.Render(fmt.Sprintf("%d", agentCount))) +
		fmt.Sprintf(" %s %s", labelStyle.Render("Providers:"), valueStyle.Render(fmt.Sprintf("%d", providerCount)))
	statsCard := cardBorder.Width(cardWidth).Render(statsContent)

	// Top row: layout depends on terminal width
	if narrowMode {
		// Narrow: stack cards vertically
		b.WriteString(serverCard)
		b.WriteString("\n")
		b.WriteString(statsCard)
	} else {
		// Normal: two cards side by side
		topRow := lipgloss.JoinHorizontal(lipgloss.Top, serverCard, " ", statsCard)
		b.WriteString(topRow)
	}
	b.WriteString("\n\n")

	// ─── ACTIVITY GRAPH ───
	graphTitle := titleStyle.Render("Activity") + "\n"
	var graphLines []string
	graphWidth := width - 4 // Responsive graph width
	for row := 7; row >= 0; row-- {
		var line strings.Builder
		line.WriteString(" ")
		sparkLen := min(graphWidth, len(m.spark))
		for i := 0; i < sparkLen; i++ {
			v := m.spark[i]
			if v >= row {
				line.WriteString(lipgloss.NewStyle().Foreground(Green).Render("█"))
			} else {
				line.WriteString(lipgloss.NewStyle().Foreground(BorderDim).Render("░"))
			}
		}
		graphLines = append(graphLines, line.String())
	}
	graphContent := graphTitle + strings.Join(graphLines, "\n")
	activityCard := cardBorder.Width(width - 2).Render(graphContent)
	b.WriteString(activityCard)
	b.WriteString("\n\n")

	// ─── PROVIDERS ───
	providers := []struct {
		name   string
		active bool
		color  lipgloss.Color
	}{
		{"Claude", m.cfg != nil && len(m.cfg.ClaudeKey) > 0, ClaudeBrand},
		{"Gemini", m.cfg != nil && len(m.cfg.GeminiKey) > 0, GeminiBrand},
		{"OpenAI", m.cfg != nil && len(m.cfg.CodexKey) > 0, CodexBrand},
	}

	providerLines := titleStyle.Render("Providers") + "\n"
	for _, p := range providers {
		icon := lipgloss.NewStyle().Foreground(p.color).Render("●")
		name := lipgloss.NewStyle().Foreground(TextDim).Width(10).Render(p.name)
		var status string
		if p.active {
			status = lipgloss.NewStyle().Foreground(Green).Render("Ready")
		} else {
			status = lipgloss.NewStyle().Foreground(TextMuted).Render("--")
		}
		providerLines += fmt.Sprintf("\n %s %s %s", icon, name, status)
	}
	providersCard := cardBorder.Width(width - 2).Render(providerLines)
	b.WriteString(providersCard)

	return b.String()
}

// ═══════════════════════════════════════════════════════════════════════════════
// SERVER VIEW
// ═══════════════════════════════════════════════════════════════════════════════

func (m Model) renderServerView(width, height int) string {
	var b strings.Builder

	// Responsive card style
	cardBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderDim).
		Padding(0, 1)

	titleStyle := lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true)

	labelStyle := lipgloss.NewStyle().Foreground(TextMuted)
	valueStyle := lipgloss.NewStyle().Foreground(Accent).Bold(true)

	// Calculate responsive widths
	cardWidth := width - 2
	if cardWidth < 20 {
		cardWidth = 20
	}

	// For narrow terminals, use single column layout
	// For wider terminals, use two-column layout
	useWideLayout := width >= 60

	// ─── STATUS CARD ───
	var statusIcon, statusText string
	if m.serverOn {
		pulse := []string{"●", "◉", "●", "○"}[m.tick%4]
		statusIcon = lipgloss.NewStyle().Foreground(Green).Render(pulse)
		statusText = lipgloss.NewStyle().Foreground(Green).Bold(true).Render("Running")
	} else {
		statusIcon = lipgloss.NewStyle().Foreground(Red).Render("○")
		statusText = lipgloss.NewStyle().Foreground(Red).Bold(true).Render("Stopped")
	}

	statusContent := titleStyle.Render("Status") + "\n\n" +
		fmt.Sprintf(" %s %s", statusIcon, statusText)

	// ─── ENDPOINT CARD ───
	endpoint := fmt.Sprintf("http://%s:%d", m.host, m.port)
	// Truncate endpoint for narrow terminals
	maxEndpointLen := cardWidth - 14
	if maxEndpointLen < 10 {
		maxEndpointLen = 10
	}
	if len(endpoint) > maxEndpointLen && !useWideLayout {
		endpoint = endpoint[:maxEndpointLen-3] + "..."
	}

	endpointContent := titleStyle.Render("Endpoint") + "\n\n" +
		fmt.Sprintf(" %s", valueStyle.Render(endpoint))

	if useWideLayout {
		// Two-column layout for wider terminals
		halfWidth := (width - 3) / 2
		if halfWidth < 20 {
			halfWidth = 20
		}
		statusCard := cardBorder.Width(halfWidth).Render(statusContent)
		endpointCard := cardBorder.Width(halfWidth).Render(endpointContent)
		topRow := lipgloss.JoinHorizontal(lipgloss.Top, statusCard, " ", endpointCard)
		b.WriteString(topRow)
	} else {
		// Single column layout for narrow terminals
		statusCard := cardBorder.Width(cardWidth).Render(statusContent)
		endpointCard := cardBorder.Width(cardWidth).Render(endpointContent)
		b.WriteString(statusCard)
		b.WriteString("\n")
		b.WriteString(endpointCard)
	}
	b.WriteString("\n\n")

	// ─── SERVER DETAILS CARD ───
	detailsContent := titleStyle.Render("Details") + "\n"

	// Calculate label width for alignment - adapts to available space
	labelWidth := 10
	if width < 50 {
		labelWidth = 8
	}

	labels := []string{"Protocol:", "Version:", "Host:", "Port:"}
	values := []string{
		"HTTP/1.1, SSE",
		"v0.2.0",
		m.host,
		fmt.Sprintf("%d", m.port),
	}

	for i, label := range labels {
		labelText := labelStyle.Width(labelWidth).Render(label)
		valueText := valueStyle.Render(values[i])
		detailsContent += fmt.Sprintf("\n %s %s", labelText, valueText)
	}

	detailsCard := cardBorder.Width(cardWidth).Render(detailsContent)
	b.WriteString(detailsCard)
	b.WriteString("\n\n")

	// ─── HELP HINT ───
	hintStyle := lipgloss.NewStyle().Foreground(TextMuted)
	keyStyle := lipgloss.NewStyle().Foreground(Accent).Bold(true)

	hint := fmt.Sprintf(" %s %s", keyStyle.Render("r"), hintStyle.Render("refresh status"))
	b.WriteString(hint)

	return b.String()
}

// ═══════════════════════════════════════════════════════════════════════════════
// SETUP VIEW
// ═══════════════════════════════════════════════════════════════════════════════

func (m Model) renderSetupView(width, height int) string {
	var b strings.Builder

	title := lipgloss.NewStyle().Foreground(Tertiary).Bold(true).Render("Setup")
	b.WriteString(title + "\n\n")

	items := []struct {
		key  string
		name string
		desc string
	}{
		{"1", "Quick Setup", "Guided configuration wizard"},
		{"2", "Provider Login", "Authenticate with AI providers"},
		{"3", "Agent Config", "Configure agent routing modes"},
		{"4", "Edit Config", "Open config.yaml in editor"},
	}

	// Card style that adapts to available width
	cardBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderDim).
		Padding(0, 1)

	// Calculate card width based on available space
	cardWidth := width - 2
	if cardWidth < 20 {
		cardWidth = 20
	}

	for _, item := range items {
		key := lipgloss.NewStyle().Foreground(Accent).Bold(true).Render(fmt.Sprintf("[%s]", item.key))
		name := lipgloss.NewStyle().Foreground(Text).Bold(true).Render(item.name)
		desc := lipgloss.NewStyle().Foreground(TextMuted).Render(item.desc)

		// Build card content
		cardContent := fmt.Sprintf("%s %s\n%s", key, name, desc)
		card := cardBorder.Width(cardWidth).Render(cardContent)
		b.WriteString(card)
		b.WriteString("\n")
	}

	return b.String()
}

// ═══════════════════════════════════════════════════════════════════════════════
// FOOTER - Keybindings
// ═══════════════════════════════════════════════════════════════════════════════

func (m Model) renderFooter() string {
	keyStyle := lipgloss.NewStyle().Foreground(Accent).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(TextMuted)
	sep := lipgloss.NewStyle().Foreground(Border).Render(" │ ")

	help := keyStyle.Render("↑↓") + descStyle.Render(" nav") + sep +
		keyStyle.Render("1-8") + descStyle.Render(" jump") + sep +
		keyStyle.Render("⏎") + descStyle.Render(" select") + sep +
		keyStyle.Render("r") + descStyle.Render(" refresh") + sep +
		keyStyle.Render("q") + descStyle.Render(" quit")

	timeStr := lipgloss.NewStyle().Foreground(TextMuted).Render(time.Now().Format("15:04:05"))

	gap := m.width - lipgloss.Width(help) - lipgloss.Width(timeStr) - 4
	if gap < 0 {
		gap = 0
	}

	content := "  " + help + strings.Repeat(" ", gap) + timeStr + "  "

	return "\n" + lipgloss.NewStyle().
		Background(BgSurface).
		Width(m.width).
		Render(content)
}

// ═══════════════════════════════════════════════════════════════════════════════
// RUN - Entry point
// ═══════════════════════════════════════════════════════════════════════════════

func Run() error {
	cfg, err := config.LoadConfigOptional("config.yaml", true)
	if err != nil || cfg == nil {
		cfg = &config.Config{}
	}

	host := "127.0.0.1"
	port := 8317

	if cfg.Host != "" {
		host = cfg.Host
	}
	if cfg.Port != 0 {
		port = cfg.Port
	}

	m := NewModel(cfg, host, port)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
