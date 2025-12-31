package tui

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/embedded"
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
	serverLoading bool
	serverMessage string
	serverMsgErr  bool
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

// serverStartResultMsg is sent when the server start operation completes
type serverStartResultMsg struct {
	success bool
	err     error
}

// serverStopResultMsg is sent when the server stop operation completes
type serverStopResultMsg struct {
	success bool
	err     error
}

func doTick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// startServerCmd returns a command that starts the embedded server
func (m Model) startServerCmd() tea.Cmd {
	return func() tea.Msg {
		configPath := "config.yaml"
		if existing := embedded.GlobalServer().ConfigPath(); existing != "" {
			configPath = existing
		}
		err := embedded.StartGlobal(configPath, "")
		return serverStartResultMsg{
			success: err == nil,
			err:     err,
		}
	}
}

// stopServerCmd returns a command that stops the embedded server
func (m Model) stopServerCmd() tea.Cmd {
	return func() tea.Msg {
		err := embedded.StopGlobal()
		return serverStopResultMsg{
			success: err == nil,
			err:     err,
		}
	}
}

func NewModel(cfg *config.Config, host string, port int) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(Accent)

	spark := make([]int, 200) // Wide enough for any terminal
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

	case serverStartResultMsg:
		m.serverLoading = false
		if msg.success {
			m.serverOn = true
			m.serverMessage = "Server started successfully"
			m.serverMsgErr = false
		} else {
			errMsg := "unknown error"
			if msg.err != nil {
				errMsg = msg.err.Error()
			}
			m.serverMessage = "Failed to start: " + errMsg
			m.serverMsgErr = true
		}
		// Refresh status after action
		status := CheckServerRunning(m.host, m.port)
		m.serverOn = status.Running
		return m, nil

	case serverStopResultMsg:
		m.serverLoading = false
		if msg.success {
			m.serverOn = false
			m.serverMessage = "Server stopped successfully"
			m.serverMsgErr = false
		} else {
			errMsg := "unknown error"
			if msg.err != nil {
				errMsg = msg.err.Error()
			}
			m.serverMessage = "Failed to stop: " + errMsg
			m.serverMsgErr = true
		}
		// Refresh status after action
		status := CheckServerRunning(m.host, m.port)
		m.serverOn = status.Running
		return m, nil

	case tea.WindowSizeMsg:
		// Some TTYs (e.g., ttyd) may report 0x0 during init; ignore those.
		if msg.Width > 0 && msg.Height > 0 {
			m.width = safeTermWidth(msg.Width)
			m.height = msg.Height
			m.ready = true
		}
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
				m.serverMessage = "" // Clear message when switching screens
			}
		case "up", "k":
			// Only handle sidebar navigation for screens without internal navigation
			if m.screen != ScreenMappings && m.screen != ScreenLogs {
				if m.cursor > 0 {
					m.cursor--
					m.screen = m.menu[m.cursor].Screen
				}
			}
		case "down", "j":
			// Only handle sidebar navigation for screens without internal navigation
			if m.screen != ScreenMappings && m.screen != ScreenLogs {
				if m.cursor < len(m.menu)-1 {
					m.cursor++
					m.screen = m.menu[m.cursor].Screen
				}
			}
		case "esc":
			m.screen = ScreenDashboard
			m.cursor = 0
		case "r":
			status := CheckServerRunning(m.host, m.port)
			m.serverOn = status.Running
			m.serverMessage = ""
		case "s":
			// Start server (only on Server screen)
			if m.screen == ScreenServer && !m.serverLoading {
				if m.serverOn {
					m.serverMessage = "Server is already running"
					m.serverMsgErr = false
				} else {
					m.serverLoading = true
					m.serverMessage = ""
					return m, m.startServerCmd()
				}
			}
		case "x":
			// Stop server (only on Server screen)
			if m.screen == ScreenServer && !m.serverLoading {
				if !m.serverOn {
					m.serverMessage = "Server is not running"
					m.serverMsgErr = false
				} else {
					m.serverLoading = true
					m.serverMessage = ""
					return m, m.stopServerCmd()
				}
			}
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
	// CRITICAL FIX: Prevent panic on startup when dimensions are 0
	if m.width == 0 || m.height == 0 {
		return "\n  Initializing..."
	}

    // 1. Calculate strictly enforced dimensions (never exceed actual size)
    totalWidth := m.width
    if totalWidth < 40 || m.height < 8 {
            msg := lipgloss.NewStyle().Foreground(TextMuted).Render("Terminal too small")
            return lipgloss.NewStyle().Width(m.width).Height(m.height).Render(msg)
    }

    // Render header/footer first to get accurate heights.
    header := m.renderHeader()
    footer := m.renderFooter()
    headerHeight := lipgloss.Height(header)
    footerHeight := lipgloss.Height(footer)
    mainHeight := m.height - headerHeight - footerHeight
    if mainHeight < 3 {
            msg := lipgloss.NewStyle().Foreground(TextMuted).Render("Terminal too small")
            return lipgloss.NewStyle().Width(m.width).Height(m.height).Render(msg)
    }

    // Sidebar width (clamped so content always fits).
    const minContentWidth = 24
    const minSidebarWidth = 12
    sidebarWidth := 20
    if totalWidth < 80 {
            sidebarWidth = 16
    }
    maxSidebarWidth := totalWidth - minContentWidth - 1
    if maxSidebarWidth < minSidebarWidth {
            msg := lipgloss.NewStyle().Foreground(TextMuted).Render("Terminal too small")
            return lipgloss.NewStyle().Width(m.width).Height(m.height).Render(msg)
    }
    if sidebarWidth > maxSidebarWidth {
            sidebarWidth = maxSidebarWidth
    }
    if sidebarWidth < minSidebarWidth {
            sidebarWidth = minSidebarWidth
    }
    contentWidth := totalWidth - sidebarWidth - 1
    if contentWidth < minContentWidth {
            msg := lipgloss.NewStyle().Foreground(TextMuted).Render("Terminal too small")
            return lipgloss.NewStyle().Width(m.width).Height(m.height).Render(msg)
    }

    // 2. Render components with explicit constraints
    sidebar := m.renderSidebar(sidebarWidth, mainHeight)
    content := m.renderContentView(contentWidth, mainHeight)

	// 3. Compose layout
	// Use JoinHorizontal with a full-height gap to avoid stale artifacts.
	gap := lipgloss.NewStyle().Width(1).Height(mainHeight).Render(" ")
	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, gap, content)

	// 4. Vertical stack
	view := lipgloss.JoinVertical(lipgloss.Left, header, main, footer)
	view = clampLines(view, m.height)
	// Ensure the full terminal area is repainted to avoid stale artifacts.
    rendered := lipgloss.NewStyle().
            Width(m.width).
            Height(m.height).
            Render(view)
    return rendered
}

// ═══════════════════════════════════════════════════════════════════════════════
// HEADER - Status bar with logo, sparkline, and server status
// ═══════════════════════════════════════════════════════════════════════════════

func (m Model) renderHeader() string {
	// Status Text
	var statusText string
	if m.serverOn {
		statusText = lipgloss.NewStyle().Foreground(Green).Bold(true).Render("ONLINE")
	} else {
		statusText = lipgloss.NewStyle().Foreground(Red).Bold(true).Render("OFFLINE")
	}

	// Title
	title := lipgloss.NewStyle().Foreground(Accent).Bold(true).Render("PROXYPILOT")

	// Sparkline - simplified to avoid wrapping issues
	sparkChars := []string{" ", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	if useASCIISymbols() {
		sparkChars = []string{" ", ".", ":", "-", "=", "+", "*", "#"}
	}
	var spark strings.Builder

	// Calculate safe width for sparkline
	// Total - Title - Status - Spacers/Borders
	safeWidth := m.width - lipgloss.Width(title) - lipgloss.Width(statusText) - 10
	if safeWidth < 10 {
		safeWidth = 10
	}

	// Clamp data to safe width
	start := 0
	if len(m.spark) > safeWidth {
		start = len(m.spark) - safeWidth
	}
	for _, v := range m.spark[start:] {
		spark.WriteString(lipgloss.NewStyle().Foreground(AccentDim).Render(sparkChars[v]))
	}

	// Layout: [Title] [Sparkline......] [Status]
	// Using SpaceBetween logic manually
	left := title
	right := statusText
	center := spark.String()

	// Render a single line with distinct sections
	headerLine := lipgloss.JoinHorizontal(lipgloss.Center,
		left,
		"  ",
		center,
		"  ",
		right,
	)
	if maxContent := m.width - 2; maxContent > 0 {
		headerLine = clampWidth(headerLine, maxContent)
	}

	// Add a bottom border using proper Lipgloss border
	// This ensures it takes up exactly 1 line of border + content
	return lipgloss.NewStyle().
		Width(m.width).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(BorderDim).
		Padding(0, 1).
		Render(headerLine)
}

// ═══════════════════════════════════════════════════════════════════════════════
// SIDEBAR - Navigation with REAL rounded border
// ═══════════════════════════════════════════════════════════════════════════════

func (m Model) renderSidebar(width, height int) string {
	var lines []string

	// Calculate inner width for card to avoid overflow into the gap.
	// Sidebar content width is (width - 4); selected card adds 4 chars (border+padding).
	cardContentWidth := width - 8
	if cardContentWidth < 8 {
		cardContentWidth = 8
	}

	for i, item := range m.menu {
		isSelected := i == m.cursor

		// Get icon based on selection state
		icon := GetNavIcon(item.Title, isSelected)

		// Get superscript key hint
		superKey := SuperScriptMap[item.Key]
		if superKey == "" {
			superKey = item.Key
		}
		if useASCIISymbols() {
			icon = GetNavIconASCII(item.Title, isSelected)
			superKey = item.Key
		}

		var row string
		if isSelected {
			// Selected: card-style with rounded border
			iconStyled := lipgloss.NewStyle().
				Foreground(Accent).
				Bold(true).
				Render(icon)
			titleStyled := lipgloss.NewStyle().
				Foreground(Accent).
				Bold(true).
				Render(item.Title)
			superStyled := lipgloss.NewStyle().
				Foreground(AccentDim).
				Render(superKey)

			// Build card content
			cardContent := iconStyled + " " + titleStyled + " " + superStyled

			// Render as card with rounded border
			row = NavCardSelected.
				Width(cardContentWidth).
				Render(cardContent)
		} else {
			// Unselected: icon + title + superscript key
			iconStyled := lipgloss.NewStyle().
				Foreground(TextDim).
				Render(icon)
			titleStyled := lipgloss.NewStyle().
				Foreground(TextDim).
				Render(item.Title)
			superStyled := lipgloss.NewStyle().
				Foreground(TextMuted).
				Render(superKey)

			row = "  " + iconStyled + " " + titleStyled + " " + superStyled
		}
	for _, line := range strings.Split(row, "\n") {
		lines = append(lines, line)
	}
	}

	// Pad to fill height (account for selected item card taking 3 lines)
	for len(lines) < height-2 {
		lines = append(lines, "")
	}
	if len(lines) > height-2 {
		lines = lines[:height-2]
	}

	content := strings.Join(lines, "\n")
	// Clamp to available content size to avoid terminal scrollback artifacts.
	content = clampLines(content, height-2)
	content = clampWidth(content, width-4)

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
	// Clamp to available content size to avoid terminal scrollback artifacts.
	content = clampLines(content, innerHeight)
	content = clampWidth(content, innerWidth)

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
	onGlyph := "●"
	offGlyph := "○"
	if useASCIISymbols() {
		onGlyph = "o"
		offGlyph = "o"
	}
	if m.serverOn {
		statusIcon = lipgloss.NewStyle().Foreground(Green).Render(onGlyph)
		statusText = lipgloss.NewStyle().Foreground(Green).Bold(true).Render("Running")
	} else {
		statusIcon = lipgloss.NewStyle().Foreground(Red).Render(offGlyph)
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
	// Graph width must fit inside card (account for card padding)
	graphWidth := width - 6 // Card border + padding = 4, plus extra margin
	if graphWidth < 1 {
		graphWidth = 1
	}
	if graphWidth > len(m.spark) {
		graphWidth = len(m.spark)
	}

	fillChar := "█"
	emptyChar := "░"
	emptyStyle := lipgloss.NewStyle().Foreground(BorderDim)
	if useASCIISymbols() {
		fillChar = "#"
		emptyChar = "."
		emptyStyle = lipgloss.NewStyle().Foreground(TextMuted)
	}
	for row := 7; row >= 0; row-- {
		var line strings.Builder
		for i := 0; i < graphWidth; i++ {
			v := m.spark[i]
			if v >= row {
				line.WriteString(lipgloss.NewStyle().Foreground(Green).Render(fillChar))
			} else {
				line.WriteString(emptyStyle.Render(emptyChar))
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
		iconGlyph := "●"
		if useASCIISymbols() {
			iconGlyph = "o"
		}
		icon := lipgloss.NewStyle().Foreground(p.color).Render(iconGlyph)
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
	pulseFrames := []string{"●", "◉", "●", "○"}
	if useASCIISymbols() {
		pulseFrames = []string{"o", "O", "o", "."}
	}
	if m.serverOn {
		pulse := pulseFrames[m.tick%len(pulseFrames)]
		statusIcon = lipgloss.NewStyle().Foreground(Green).Render(pulse)
		statusText = lipgloss.NewStyle().Foreground(Green).Bold(true).Render("Running")
	} else {
		statusIcon = lipgloss.NewStyle().Foreground(Red).Render(pulseFrames[len(pulseFrames)-1])
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

	// ─── LOADING / MESSAGE ───
	if m.serverLoading {
		loadingStyle := lipgloss.NewStyle().Foreground(Accent)
		b.WriteString(" " + m.spinner.View() + " " + loadingStyle.Render("Processing..."))
		b.WriteString("\n\n")
	} else if m.serverMessage != "" {
		var msgStyle lipgloss.Style
		if m.serverMsgErr {
			msgStyle = lipgloss.NewStyle().Foreground(Red)
		} else {
			msgStyle = lipgloss.NewStyle().Foreground(Green)
		}
		b.WriteString(" " + msgStyle.Render(m.serverMessage))
		b.WriteString("\n\n")
	}

	// ─── HELP HINT ───
	hintStyle := lipgloss.NewStyle().Foreground(TextMuted)
	keyStyle := lipgloss.NewStyle().Foreground(Accent).Bold(true)

	hint := fmt.Sprintf(" %s %s  %s %s  %s %s",
		keyStyle.Render("r"), hintStyle.Render("refresh"),
		keyStyle.Render("s"), hintStyle.Render("start"),
		keyStyle.Render("x"), hintStyle.Render("stop"))
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
// FOOTER - Keybindings with dotted divider and keyboard pills
// ═══════════════════════════════════════════════════════════════════════════════

func (m Model) renderFooter() string {
	// Dotted separator line at the top
	divider := RenderDottedDivider(m.width)

	descStyle := lipgloss.NewStyle().Foreground(TextMuted)

	// Build help text with keyboard pills
	navKey := "↑↓"
	enterKey := "⏎"
	if useASCIISymbols() {
		navKey = "UD"
		enterKey = "enter"
	}
	help := " " + RenderKeyboardPill(navKey) + descStyle.Render(" nav") + "  " +
		RenderKeyboardPill("1-8") + descStyle.Render(" jump") + "  " +
		RenderKeyboardPill(enterKey) + descStyle.Render(" select") + "  " +
		RenderKeyboardPill("r") + descStyle.Render(" refresh") + "  " +
		RenderKeyboardPill("q") + descStyle.Render(" quit")

	// Time with blinking colon effect
	now := time.Now()
	var colonChar string
	if m.tick%2 == 0 {
		colonChar = ":"
	} else {
		colonChar = " "
	}
	timeStr := lipgloss.NewStyle().Foreground(TextMuted).Render(
		fmt.Sprintf("%02d%s%02d%s%02d", now.Hour(), colonChar, now.Minute(), colonChar, now.Second()),
	)

	gap := m.width - lipgloss.Width(help) - lipgloss.Width(timeStr) - 4
	if gap < 0 {
		gap = 0
	}

	content := " " + help + strings.Repeat(" ", gap) + timeStr + "  "
	content = clampWidth(content, m.width)

	footerBar := lipgloss.NewStyle().
		Background(BgSurface).
		Width(m.width).
		Render(content)

	return divider + "\n" + footerBar
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
	if cols := envInt("COLUMNS"); cols > 0 {
		m.width = safeTermWidth(cols)
		m.ready = true
	}
	if rows := envInt("LINES"); rows > 0 {
		m.height = rows
		m.ready = true
	}

	opts := []tea.ProgramOption{}
	if !envBool("PROXYPILOT_TUI_NO_ALT_SCREEN") {
		opts = append(opts, tea.WithAltScreen())
	}
	p := tea.NewProgram(m, opts...)

	_, err = p.Run()
	return err
}

func clampLines(text string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Join(lines, "\n")
}

func clampWidth(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		rendered := lipgloss.NewStyle().Width(maxWidth).Render(line)
		if idx := strings.IndexByte(rendered, '\n'); idx >= 0 {
			rendered = rendered[:idx]
		}
		lines[i] = rendered
	}
	return strings.Join(lines, "\n")
}

func envBool(name string) bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	return val == "1" || val == "true" || val == "yes" || val == "on"
}

func envInt(name string) int {
	val := strings.TrimSpace(os.Getenv(name))
	if val == "" {
		return 0
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func safeTermWidth(width int) int {
	if width <= 0 {
		return width
	}
	margin := 1
	if runtime.GOOS == "windows" || os.Getenv("WT_SESSION") != "" {
		margin = 2
	}
	if envMargin := envInt("PROXYPILOT_TUI_MARGIN"); envMargin > 0 {
		margin = envMargin
	}
	if width > margin {
		return width - margin
	}
	return width
}

func useASCIISymbols() bool {
	if envBool("PROXYPILOT_TUI_UNICODE") {
		return false
	}
	return envBool("PROXYPILOT_TUI_ASCII")
}
