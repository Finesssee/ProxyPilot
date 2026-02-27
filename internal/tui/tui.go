// Package tui provides an interactive terminal user interface for ProxyPilot.
// It uses the Bubble Tea framework (charmbracelet/bubbletea) for a modern TUI experience.
package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ═══════════════════════════════════════════════════════════════════════════════
// FLIGHT DECK THEME
// Industrial control room aesthetic with high contrast and dense information
// ═══════════════════════════════════════════════════════════════════════════════

var (
	// Core colors - Warm industrial palette
	colorOrange      = lipgloss.Color("#FF6B35") // Primary accent
	colorOrangeLight = lipgloss.Color("#FF8C5A")
	colorOrangeDim   = lipgloss.Color("#CC5429")
	colorAmber       = lipgloss.Color("#FFB347") // Warning
	colorGold        = lipgloss.Color("#FFD700")

	// Status colors
	colorGreen    = lipgloss.Color("#39FF14") // Neon green
	colorGreenDim = lipgloss.Color("#228B22")
	colorRed      = lipgloss.Color("#FF3131")
	colorRedDim   = lipgloss.Color("#8B0000")
	colorCyan     = lipgloss.Color("#00FFFF")

	// Neutral tones - dark industrial
	colorBlack    = lipgloss.Color("#0A0A0A")
	colorCharcoal = lipgloss.Color("#1A1A1A")
	colorGraphite = lipgloss.Color("#2D2D2D")
	colorSteel    = lipgloss.Color("#4A4A4A")
	colorSilver   = lipgloss.Color("#888888")
	colorLight    = lipgloss.Color("#CCCCCC")
	colorWhite    = lipgloss.Color("#F5F5F5")

	// Box drawing characters
	boxTL = "┏"
	boxTR = "┓"
	boxBL = "┗"
	boxBR = "┛"
	boxH  = "━"
	boxV  = "┃"
	boxT  = "┳"
	boxB  = "┻"

	// Styled components
	titleStyle = lipgloss.NewStyle().
			Foreground(colorOrange).
			Bold(true)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorSilver)

	// Panel with double borders
	panelBorder = lipgloss.Border{
		Top:         "━",
		Bottom:      "━",
		Left:        "┃",
		Right:       "┃",
		TopLeft:     "┏",
		TopRight:    "┓",
		BottomLeft:  "┗",
		BottomRight: "┛",
	}

	panelStyle = lipgloss.NewStyle().
			Border(panelBorder).
			BorderForeground(colorOrangeDim).
			Padding(1, 2)

	panelTitleStyle = lipgloss.NewStyle().
			Foreground(colorOrange).
			Bold(true).
			Background(colorCharcoal).
			Padding(0, 1)

	// Tab styles
	tabStyle = lipgloss.NewStyle().
			Foreground(colorSilver).
			Padding(0, 3)

	tabActiveStyle = lipgloss.NewStyle().
			Foreground(colorOrange).
			Bold(true).
			Padding(0, 3).
			Background(colorGraphite)

	// Status indicators
	statusOnline = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	statusOffline = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	statusWarning = lipgloss.NewStyle().
			Foreground(colorAmber).
			Bold(true)

	// Data styles
	labelStyle = lipgloss.NewStyle().
			Foreground(colorSilver)

	valueStyle = lipgloss.NewStyle().
			Foreground(colorWhite).
			Bold(true)

	accentStyle = lipgloss.NewStyle().
			Foreground(colorOrange).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(colorSteel)

	// Help bar
	helpStyle = lipgloss.NewStyle().
			Foreground(colorSteel)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorOrange).
			Bold(true)

	// Menu styles
	menuItemStyle = lipgloss.NewStyle().
			Foreground(colorLight).
			Padding(0, 2)

	menuSelectedStyle = lipgloss.NewStyle().
				Foreground(colorOrange).
				Background(colorGraphite).
				Bold(true).
				Padding(0, 2)

	// Progress bar segments
	barFull  = "█"
	barThree = "▓"
	barHalf  = "▒"
	barEmpty = "░"

	// Status icons
	iconPower    = "⏻"
	iconOnline   = "●"
	iconOffline  = "○"
	iconCooldown = "◐"
	iconDisabled = "✕"
	iconArrow    = "▸"
	iconCheck    = "✓"
	iconCross    = "✗"
	iconGauge    = "◎"
	iconChart    = "▤"
	iconUser     = "◉"
	iconLog      = "▥"
)

// Tab represents the different views in the TUI.
type Tab int

const (
	TabStatus Tab = iota
	TabAccounts
	TabRateLimits
	TabUsage
	TabLogs
)

func (t Tab) String() string {
	return [...]string{"STATUS", "ACCOUNTS", "RATE LIMITS", "USAGE", "LOGS"}[t]
}

func (t Tab) Icon() string {
	return [...]string{iconPower, iconUser, iconGauge, iconChart, iconLog}[t]
}

// Provider represents an OAuth provider.
type Provider struct {
	ID   string
	Name string
}

var providers = []Provider{
	{ID: "anthropic", Name: "Claude (Anthropic)"},
	{ID: "codex", Name: "Codex (OpenAI)"},
	{ID: "gemini", Name: "Gemini (Google)"},
	{ID: "antigravity", Name: "Antigravity"},
	{ID: "qwen", Name: "Qwen"},
	{ID: "kiro", Name: "Kiro"},
}

// Model is the main Bubble Tea model for the TUI.
type Model struct {
	ctx    context.Context
	cancel context.CancelFunc
	client *Client

	// Current tab
	currentTab Tab
	tabs       []Tab

	// Data
	status     ProxyStatus
	accounts   []AccountInfo
	rateLimits RateLimitSummary
	usage      UsageStats
	logs       []string

	// UI components
	spinner       spinner.Model
	accountsTable table.Model

	// Login flow state
	showLoginMenu   bool
	loginMenuIndex  int
	loginInProgress bool
	loginProvider   string
	loginState      string
	loginMessage    string

	// State
	loading     bool
	lastRefresh time.Time
	err         error
	width       int
	height      int
	quitting    bool
}

// ProxyStatus represents the current proxy state.
type ProxyStatus struct {
	Running  bool
	Port     int
	Accounts int
	Models   int
	Uptime   string
}

// AccountInfo represents a single account.
type AccountInfo struct {
	ID       string
	Provider string
	Email    string
	Status   string
	Expires  string
	Usage    string
}

// RateLimitSummary represents rate limit overview.
type RateLimitSummary struct {
	Total        int
	Available    int
	CoolingDown  int
	Disabled     int
	NextRecovery string
}

// UsageStats represents usage statistics.
type UsageStats struct {
	TotalRequests     int64
	TotalInputTokens  int64
	TotalOutputTokens int64
	EstimatedCost     float64
	TopModels         []ModelUsage
}

// ModelUsage represents per-model usage.
type ModelUsage struct {
	Model    string
	Requests int64
	Tokens   int64
}

// Message types
type tickMsg time.Time
type statusMsg ProxyStatus
type accountsMsg []AccountInfo
type rateLimitsMsg RateLimitSummary
type usageMsg UsageStats
type logsMsg []string
type errMsg error

// Login flow messages
type authURLMsg struct {
	URL   string
	State string
}
type authStatusMsg struct {
	Status  string
	Message string
}
type authCompleteMsg struct{}
type authErrorMsg struct{ error }

// NewModel creates a new TUI model.
func NewModel(proxyURL, managementKey string) Model {
	ctx, cancel := context.WithCancel(context.Background())

	// Industrial spinner
	s := spinner.New()
	s.Spinner = spinner.Spinner{
		Frames: []string{"◜", "◠", "◝", "◞", "◡", "◟"},
		FPS:    time.Millisecond * 100,
	}
	s.Style = lipgloss.NewStyle().Foreground(colorOrange)

	// Create API client
	client := NewClient(proxyURL, managementKey)

	// Create accounts table
	columns := []table.Column{
		{Title: "PROVIDER", Width: 14},
		{Title: "ACCOUNT", Width: 32},
		{Title: "STATUS", Width: 10},
		{Title: "EXPIRES", Width: 14},
		{Title: "TODAY", Width: 16},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(12),
	)

	// Industrial table styling
	ts := table.DefaultStyles()
	ts.Header = lipgloss.NewStyle().
		Foreground(colorOrange).
		Bold(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(colorOrangeDim).
		BorderBottom(true).
		Padding(0, 1)
	ts.Selected = lipgloss.NewStyle().
		Foreground(colorOrange).
		Background(colorGraphite).
		Bold(true)
	ts.Cell = lipgloss.NewStyle().
		Foreground(colorLight).
		Padding(0, 1)
	t.SetStyles(ts)

	return Model{
		ctx:           ctx,
		cancel:        cancel,
		client:        client,
		tabs:          []Tab{TabStatus, TabAccounts, TabRateLimits, TabUsage, TabLogs},
		currentTab:    TabStatus,
		spinner:       s,
		accountsTable: t,
		loading:       true,
		width:         100,
		height:        30,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.tick(),
		m.fetchStatus(),
	)
}

// tick returns a command that triggers a refresh.
func (m Model) tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle login menu navigation first
		if m.showLoginMenu {
			switch msg.String() {
			case "q", "esc":
				m.showLoginMenu = false
				return m, nil
			case "up", "k":
				if m.loginMenuIndex > 0 {
					m.loginMenuIndex--
				}
				return m, nil
			case "down", "j":
				if m.loginMenuIndex < len(providers)-1 {
					m.loginMenuIndex++
				}
				return m, nil
			case "enter":
				provider := providers[m.loginMenuIndex]
				m.showLoginMenu = false
				m.loginInProgress = true
				m.loginProvider = provider.Name
				m.loginMessage = "Opening browser..."
				return m, m.startLogin(provider.ID)
			}
			return m, nil
		}

		// Normal key handling
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			m.cancel()
			return m, tea.Quit
		case "a":
			m.showLoginMenu = true
			m.loginMenuIndex = 0
			return m, nil
		case "tab", "right", "l":
			m.currentTab = (m.currentTab + 1) % Tab(len(m.tabs))
			return m, m.fetchForTab()
		case "shift+tab", "left", "h":
			m.currentTab = (m.currentTab - 1 + Tab(len(m.tabs))) % Tab(len(m.tabs))
			return m, m.fetchForTab()
		case "r":
			m.loading = true
			return m, m.fetchForTab()
		case "1":
			m.currentTab = TabStatus
			return m, m.fetchStatus()
		case "2":
			m.currentTab = TabAccounts
			return m, m.fetchAccounts()
		case "3":
			m.currentTab = TabRateLimits
			return m, m.fetchRateLimits()
		case "4":
			m.currentTab = TabUsage
			return m, m.fetchUsage()
		case "5":
			m.currentTab = TabLogs
			return m, m.fetchLogs()
		}

		// Handle table navigation when on accounts tab
		if m.currentTab == TabAccounts {
			var tableCmd tea.Cmd
			m.accountsTable, tableCmd = m.accountsTable.Update(msg)
			cmds = append(cmds, tableCmd)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.accountsTable.SetWidth(msg.Width - 8)
		m.accountsTable.SetHeight(msg.Height - 16)

	case tickMsg:
		m.lastRefresh = time.Time(msg)
		cmds = append(cmds, m.tick(), m.fetchForTab())
		if m.loginInProgress && m.loginState != "" {
			cmds = append(cmds, m.pollAuthStatus())
		}

	case spinner.TickMsg:
		var spinnerCmd tea.Cmd
		m.spinner, spinnerCmd = m.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)

	case statusMsg:
		m.status = ProxyStatus(msg)
		m.loading = false
		m.err = nil

	case accountsMsg:
		m.accounts = msg
		m.loading = false
		m.err = nil
		rows := make([]table.Row, len(msg))
		for i, acc := range msg {
			rows[i] = table.Row{acc.Provider, acc.Email, acc.Status, acc.Expires, acc.Usage}
		}
		m.accountsTable.SetRows(rows)

	case rateLimitsMsg:
		m.rateLimits = RateLimitSummary(msg)
		m.loading = false
		m.err = nil

	case usageMsg:
		m.usage = UsageStats(msg)
		m.loading = false
		m.err = nil

	case logsMsg:
		m.logs = msg
		m.loading = false
		m.err = nil

	case errMsg:
		m.err = msg
		m.loading = false

	case authURLMsg:
		m.loginState = msg.State
		m.loginMessage = "Waiting for browser auth..."
		cmds = append(cmds, m.pollAuthStatus())

	case authStatusMsg:
		if msg.Status == "complete" {
			m.loginInProgress = false
			m.loginMessage = ""
			m.loginState = ""
			cmds = append(cmds, m.fetchAccounts())
		} else if msg.Status == "error" {
			m.loginInProgress = false
			m.loginMessage = "Auth failed: " + msg.Message
		} else {
			m.loginMessage = msg.Message
		}

	case authCompleteMsg:
		m.loginInProgress = false
		m.loginMessage = "✓ Login successful!"
		m.loginState = ""
		cmds = append(cmds, m.fetchAccounts())

	case authErrorMsg:
		m.loginInProgress = false
		m.loginMessage = "✕ Error: " + msg.Error()
	}

	return m, tea.Batch(cmds...)
}

// View renders the TUI.
func (m Model) View() string {
	if m.quitting {
		return m.renderShutdown()
	}

	var b strings.Builder

	// Header bar
	b.WriteString(m.renderHeader())

	// Login menu overlay
	if m.showLoginMenu {
		b.WriteString(m.renderLoginMenu())
		return b.String()
	}

	// Login progress banner
	if m.loginInProgress || m.loginMessage != "" {
		b.WriteString(m.renderLoginBanner())
	}

	// Tab bar
	b.WriteString(m.renderTabBar())
	b.WriteString("\n")

	// Content area
	if m.loading {
		b.WriteString(m.renderLoading())
	} else if m.err != nil {
		if errors.Is(m.err, ErrEngineNotRunning) {
			b.WriteString(m.renderOffline())
		} else {
			b.WriteString(m.renderError())
		}
	} else {
		switch m.currentTab {
		case TabStatus:
			b.WriteString(m.renderStatus())
		case TabAccounts:
			b.WriteString(m.renderAccounts())
		case TabRateLimits:
			b.WriteString(m.renderRateLimits())
		case TabUsage:
			b.WriteString(m.renderUsage())
		case TabLogs:
			b.WriteString(m.renderLogs())
		}
	}

	// Footer
	b.WriteString(m.renderFooter())

	return b.String()
}

// ═══════════════════════════════════════════════════════════════════════════════
// RENDER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

func (m Model) renderHeader() string {
	width := m.width
	if width < 80 {
		width = 80
	}

	// Top border with centered title
	titleText := "▓▓ PROXYPILOT ▓▓"
	subtitleText := "FLIGHT DECK v0.2"

	// Calculate centering
	totalTitleLen := len(titleText) + 4 + len(subtitleText) + 4
	leftPad := (width - totalTitleLen - 4) / 2
	rightPad := width - totalTitleLen - leftPad - 4
	if leftPad < 3 {
		leftPad = 3
	}
	if rightPad < 0 {
		rightPad = 0
	}

	topLine := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxTL)
	topLine += lipgloss.NewStyle().Foreground(colorOrangeDim).Render(strings.Repeat(boxH, leftPad))
	topLine += lipgloss.NewStyle().Foreground(colorOrangeDim).Render("╢ ")
	topLine += titleStyle.Render(titleText)
	topLine += lipgloss.NewStyle().Foreground(colorOrangeDim).Render(" ╟")
	topLine += lipgloss.NewStyle().Foreground(colorOrangeDim).Render(strings.Repeat(boxH, 3))
	topLine += lipgloss.NewStyle().Foreground(colorSteel).Render("┤ ")
	topLine += subtitleStyle.Render(subtitleText)
	topLine += lipgloss.NewStyle().Foreground(colorSteel).Render(" ├")
	topLine += lipgloss.NewStyle().Foreground(colorOrangeDim).Render(strings.Repeat(boxH, rightPad))
	topLine += lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxTR)

	return "\n" + topLine + "\n"
}

func (m Model) renderTabBar() string {
	width := m.width
	if width < 80 {
		width = 80
	}

	var tabs []string
	for i, tab := range m.tabs {
		var content string
		icon := tab.Icon()
		if Tab(i) == m.currentTab {
			// Active tab with glow effect
			content = lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render(
				"【" + icon + " " + tab.String() + "】",
			)
		} else {
			content = lipgloss.NewStyle().Foreground(colorSilver).Render(
				" " + icon + " " + tab.String() + " ",
			)
		}
		tabs = append(tabs, content)
	}

	// Build tab bar
	bar := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV + " ")
	bar += strings.Join(tabs, lipgloss.NewStyle().Foreground(colorSteel).Render("│"))

	// Fill remaining space with decorative scanline pattern
	barWidth := lipgloss.Width(bar)
	remaining := width - barWidth - 2
	if remaining > 0 {
		// Use visible dashed pattern
		pattern := strings.Repeat("╌", remaining-1)
		bar += lipgloss.NewStyle().Foreground(colorSteel).Render(pattern)
	}
	bar += lipgloss.NewStyle().Foreground(colorOrangeDim).Render(" " + boxV)

	// Heavy separator line
	sepLine := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(
		"┣" + strings.Repeat("━", width-2) + "┫",
	)

	return bar + "\n" + sepLine + "\n"
}

func (m Model) renderLoginMenu() string {
	var b strings.Builder

	// Panel
	header := panelTitleStyle.Render("  " + iconUser + " SELECT PROVIDER  ")
	divider := lipgloss.NewStyle().Foreground(colorSteel).Render(strings.Repeat("─", 44))

	b.WriteString("\n" + lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV) + "\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV) + "  " + header + "\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV) + "  " + divider + "\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV) + "\n")

	for i, p := range providers {
		prefix := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV)
		cursor := "   "
		style := menuItemStyle
		if i == m.loginMenuIndex {
			cursor = " " + accentStyle.Render(iconArrow) + " "
			style = menuSelectedStyle
		}
		b.WriteString(prefix + cursor + style.Render(p.Name) + "\n")
	}

	b.WriteString(lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV) + "\n")

	help := helpKeyStyle.Render("↑/↓") + helpStyle.Render(" navigate  ") +
		helpKeyStyle.Render("↵") + helpStyle.Render(" select  ") +
		helpKeyStyle.Render("esc") + helpStyle.Render(" cancel")
	b.WriteString(lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV) + "  " + help + "\n")

	return b.String()
}

func (m Model) renderLoginBanner() string {
	prefix := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV)
	var content string
	if m.loginInProgress {
		content = statusWarning.Render(" " + m.spinner.View() + " " + m.loginProvider + ": " + m.loginMessage)
	} else if strings.Contains(m.loginMessage, "✓") {
		content = statusOnline.Render(" " + m.loginMessage)
	} else {
		content = statusOffline.Render(" " + m.loginMessage)
	}
	return prefix + content + "\n"
}

func (m Model) renderLoading() string {
	width := m.width
	if width < 80 {
		width = 80
	}
	prefix := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV)
	suffix := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV)

	content := accentStyle.Render(" " + m.spinner.View() + " LOADING...")
	padLen := width - lipgloss.Width(prefix) - lipgloss.Width(content) - lipgloss.Width(suffix) - 1
	if padLen < 0 {
		padLen = 0
	}

	return prefix + content + strings.Repeat(" ", padLen) + suffix + "\n"
}

func (m Model) renderError() string {
	width := m.width
	if width < 80 {
		width = 80
	}
	prefix := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV)
	suffix := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV)

	errText := statusOffline.Render(" " + iconCross + " " + m.err.Error())
	padLen := width - lipgloss.Width(prefix) - lipgloss.Width(errText) - lipgloss.Width(suffix) - 1
	if padLen < 0 {
		padLen = 0
	}

	return prefix + errText + strings.Repeat(" ", padLen) + suffix + "\n"
}

func (m Model) renderOffline() string {
	width := m.width
	if width < 80 {
		width = 80
	}
	prefix := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV)
	suffix := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV)

	var b strings.Builder

	// Helper to pad line to full width
	padLine := func(content string) string {
		contentWidth := lipgloss.Width(content)
		padLen := width - lipgloss.Width(prefix) - contentWidth - lipgloss.Width(suffix)
		if padLen < 0 {
			padLen = 0
		}
		return prefix + content + strings.Repeat(" ", padLen) + suffix + "\n"
	}

	b.WriteString(padLine(""))
	b.WriteString(padLine(""))

	// Centered offline box
	boxContent := []string{
		statusOffline.Render("  ╔════════════════════════════════════════════╗"),
		statusOffline.Render("  ║") + "                                            " + statusOffline.Render("║"),
		statusOffline.Render("  ║") + statusOffline.Render("      ◯ ENGINE OFFLINE                     ") + statusOffline.Render("║"),
		statusOffline.Render("  ║") + dimStyle.Render("      Start ProxyPilot to continue         ") + statusOffline.Render("║"),
		statusOffline.Render("  ║") + dimStyle.Render("                                            ") + statusOffline.Render("║"),
		statusOffline.Render("  ║") + dimStyle.Render("      Run: ") + accentStyle.Render("proxypilot") + dimStyle.Render("                       ") + statusOffline.Render("║"),
		statusOffline.Render("  ║") + "                                            " + statusOffline.Render("║"),
		statusOffline.Render("  ╚════════════════════════════════════════════╝"),
	}

	for _, line := range boxContent {
		b.WriteString(padLine(line))
	}

	b.WriteString(padLine(""))

	return b.String()
}

func (m Model) renderStatus() string {
	width := m.width
	if width < 80 {
		width = 80
	}
	prefix := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV)
	suffix := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV)

	var b strings.Builder

	// Helper to pad line to full width
	padLine := func(content string) string {
		contentWidth := lipgloss.Width(content)
		padLen := width - lipgloss.Width(prefix) - contentWidth - lipgloss.Width(suffix)
		if padLen < 0 {
			padLen = 0
		}
		return prefix + content + strings.Repeat(" ", padLen) + suffix + "\n"
	}

	// Helper for scanline filler
	scanLine := func() string {
		inner := width - 2
		// Simple uniform pattern - subtle dots at fixed intervals
		pattern := strings.Repeat(" ", inner)
		return prefix + lipgloss.NewStyle().Foreground(colorCharcoal).Render(pattern) + suffix + "\n"
	}

	b.WriteString(padLine(""))

	// Status indicator - big and bold
	var statusIcon, statusText string
	var statusStyle lipgloss.Style
	if m.status.Running {
		statusIcon = "●"
		statusText = "SYSTEM ONLINE"
		statusStyle = statusOnline
	} else {
		statusIcon = "○"
		statusText = "SYSTEM OFFLINE"
		statusStyle = statusOffline
	}

	// Create dynamic separator that fills width
	sepWidth := width - 30
	if sepWidth < 20 {
		sepWidth = 20
	}
	statusHeader := "  " + statusStyle.Render(statusIcon+" "+statusText) + "  " +
		dimStyle.Render(strings.Repeat("━", sepWidth))
	b.WriteString(padLine(statusHeader))
	b.WriteString(scanLine())

	// Calculate column widths for 3-column layout
	innerWidth := width - 8
	colWidth := innerWidth / 3
	if colWidth < 20 {
		colWidth = 20
	}

	// Create stat boxes with DOUBLE borders
	makeStatBox := func(icon, label, value string, accent lipgloss.Style) string {
		boxWidth := colWidth - 4
		if boxWidth < 16 {
			boxWidth = 16
		}

		// Double-line borders for industrial look
		topBorder := "╔" + strings.Repeat("═", boxWidth) + "╗"
		botBorder := "╚" + strings.Repeat("═", boxWidth) + "╝"

		// Icon and label line
		iconLabel := accent.Render(icon) + " " + labelStyle.Render(label)
		iconPad := boxWidth - lipgloss.Width(iconLabel) - 1
		if iconPad < 0 {
			iconPad = 0
		}
		iconLine := "║ " + iconLabel + strings.Repeat(" ", iconPad) + "║"

		// Separator
		sepLine := "╟" + strings.Repeat("─", boxWidth) + "╢"

		// Value line (centered, larger)
		valueStyled := accent.Bold(true).Render(value)
		valuePad := (boxWidth - lipgloss.Width(valueStyled)) / 2
		if valuePad < 0 {
			valuePad = 0
		}
		valueRightPad := boxWidth - lipgloss.Width(valueStyled) - valuePad
		if valueRightPad < 0 {
			valueRightPad = 0
		}
		valueLine := "║" + strings.Repeat(" ", valuePad) + valueStyled + strings.Repeat(" ", valueRightPad) + "║"

		return lipgloss.NewStyle().Foreground(colorOrangeDim).Render(topBorder) + "\n" +
			lipgloss.NewStyle().Foreground(colorOrangeDim).Render(iconLine) + "\n" +
			lipgloss.NewStyle().Foreground(colorSteel).Render(sepLine) + "\n" +
			lipgloss.NewStyle().Foreground(colorOrangeDim).Render(valueLine) + "\n" +
			lipgloss.NewStyle().Foreground(colorOrangeDim).Render(botBorder)
	}

	// Create 3 stat boxes
	box1 := makeStatBox("◈", "PORT", fmt.Sprintf("%d", m.status.Port), accentStyle)
	box2 := makeStatBox("◉", "ACCOUNTS", fmt.Sprintf("%d", m.status.Accounts), statusOnline)
	box3 := makeStatBox("◎", "MODELS", fmt.Sprintf("%d", m.status.Models), valueStyle)

	// Split boxes into lines and render side by side
	box1Lines := strings.Split(box1, "\n")
	box2Lines := strings.Split(box2, "\n")
	box3Lines := strings.Split(box3, "\n")

	for i := 0; i < len(box1Lines); i++ {
		line := "  " + box1Lines[i] + "  " + box2Lines[i] + "  " + box3Lines[i]
		b.WriteString(padLine(line))
	}

	b.WriteString(scanLine())

	// Quick info section with better framing
	infoBoxWidth := width - 10
	if infoBoxWidth > 70 {
		infoBoxWidth = 70
	}
	infoTop := "  ╭─ Quick Info " + strings.Repeat("─", infoBoxWidth-14) + "╮"
	b.WriteString(padLine(lipgloss.NewStyle().Foreground(colorSteel).Render(infoTop)))

	endpoint := "  │  " + labelStyle.Render("API Endpoint: ") +
		accentStyle.Render(fmt.Sprintf("http://127.0.0.1:%d/v1", m.status.Port))
	endPad := infoBoxWidth - lipgloss.Width(endpoint) + 5
	if endPad < 0 {
		endPad = 0
	}
	b.WriteString(padLine(lipgloss.NewStyle().Foreground(colorSteel).Render("  │") +
		labelStyle.Render("  API Endpoint: ") +
		accentStyle.Render(fmt.Sprintf("http://127.0.0.1:%d/v1", m.status.Port))))

	b.WriteString(padLine(lipgloss.NewStyle().Foreground(colorSteel).Render("  │") +
		labelStyle.Render("  Compatible:   ") +
		valueStyle.Render("OpenAI • Anthropic • Gemini")))

	infoBot := "  ╰" + strings.Repeat("─", infoBoxWidth) + "╯"
	b.WriteString(padLine(lipgloss.NewStyle().Foreground(colorSteel).Render(infoBot)))

	b.WriteString(scanLine())

	// Quick actions with better visibility
	actionsLine := "  " + dimStyle.Render("▸ Press ") + helpKeyStyle.Render("a") + dimStyle.Render(" add account   ") +
		helpKeyStyle.Render("2") + dimStyle.Render(" accounts   ") +
		helpKeyStyle.Render("3") + dimStyle.Render(" rate limits   ") +
		helpKeyStyle.Render("4") + dimStyle.Render(" usage   ") +
		helpKeyStyle.Render("5") + dimStyle.Render(" logs")
	b.WriteString(padLine(actionsLine))

	// Fill remaining vertical space with scanlines
	contentLines := strings.Count(b.String(), "\n")
	availableLines := m.height - 8 // header + tabs + footer
	remainingLines := availableLines - contentLines
	for i := 0; i < remainingLines && i < 10; i++ {
		b.WriteString(scanLine())
	}

	return b.String()
}

func (m Model) renderAccounts() string {
	width := m.width
	if width < 80 {
		width = 80
	}
	prefix := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV)
	suffix := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV)

	var b strings.Builder

	padLine := func(content string) string {
		contentWidth := lipgloss.Width(content)
		padLen := width - lipgloss.Width(prefix) - contentWidth - lipgloss.Width(suffix)
		if padLen < 0 {
			padLen = 0
		}
		return prefix + content + strings.Repeat(" ", padLen) + suffix + "\n"
	}

	b.WriteString(padLine(""))

	if len(m.accounts) == 0 {
		b.WriteString(padLine(dimStyle.Render("  No accounts configured")))
		b.WriteString(padLine(""))
		b.WriteString(padLine(labelStyle.Render("  Press ") + helpKeyStyle.Render("a") + labelStyle.Render(" to add your first account")))
	} else {
		// Summary header
		summaryLine := "  " + accentStyle.Render(fmt.Sprintf("%d", len(m.accounts))) + labelStyle.Render(" accounts loaded") +
			dimStyle.Render("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		b.WriteString(padLine(summaryLine))
		b.WriteString(padLine(""))

		// Render table with borders
		tableStr := m.accountsTable.View()
		for _, line := range strings.Split(tableStr, "\n") {
			b.WriteString(padLine("  " + line))
		}
	}

	b.WriteString(padLine(""))

	return b.String()
}

func (m Model) renderRateLimits() string {
	width := m.width
	if width < 80 {
		width = 80
	}
	prefix := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV)
	suffix := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV)

	var b strings.Builder

	padLine := func(content string) string {
		contentWidth := lipgloss.Width(content)
		padLen := width - lipgloss.Width(prefix) - contentWidth - lipgloss.Width(suffix)
		if padLen < 0 {
			padLen = 0
		}
		return prefix + content + strings.Repeat(" ", padLen) + suffix + "\n"
	}

	scanLine := func() string {
		inner := width - 2
		// Simple uniform pattern - subtle dots at fixed intervals
		pattern := strings.Repeat(" ", inner)
		return prefix + lipgloss.NewStyle().Foreground(colorCharcoal).Render(pattern) + suffix + "\n"
	}

	b.WriteString(padLine(""))

	total := m.rateLimits.Total
	avail := m.rateLimits.Available
	cooling := m.rateLimits.CoolingDown
	disabled := m.rateLimits.Disabled

	// Calculate percentage
	var availPct float64
	if total > 0 {
		availPct = float64(avail) / float64(total) * 100
	}

	// Header with percentage - dynamic width
	sepWidth := width - 50
	if sepWidth < 10 {
		sepWidth = 10
	}
	headerLine := "  " + panelTitleStyle.Render(" "+iconGauge+" CREDENTIAL AVAILABILITY ") + "  " +
		accentStyle.Render(fmt.Sprintf("%.0f%%", availPct)) +
		dimStyle.Render("  "+strings.Repeat("━", sepWidth))
	b.WriteString(padLine(headerLine))
	b.WriteString(scanLine())

	// Visual gauge bar - full width
	barWidth := width - 12
	if barWidth > 90 {
		barWidth = 90
	}
	if barWidth < 40 {
		barWidth = 40
	}

	filledWidth := int(float64(barWidth) * float64(avail) / float64(max(total, 1)))
	coolingWidth := int(float64(barWidth) * float64(cooling) / float64(max(total, 1)))
	disabledWidth := int(float64(barWidth) * float64(disabled) / float64(max(total, 1)))
	emptyWidth := barWidth - filledWidth - coolingWidth - disabledWidth
	if emptyWidth < 0 {
		emptyWidth = 0
	}

	bar := statusOnline.Render(strings.Repeat(barFull, filledWidth))
	bar += statusWarning.Render(strings.Repeat(barThree, coolingWidth))
	bar += statusOffline.Render(strings.Repeat(barFull, disabledWidth))
	bar += dimStyle.Render(strings.Repeat(barEmpty, emptyWidth))

	// Double-line gauge box
	b.WriteString(padLine("  " + lipgloss.NewStyle().Foreground(colorOrangeDim).Render("╔"+strings.Repeat("═", barWidth+2)+"╗")))
	b.WriteString(padLine("  " + lipgloss.NewStyle().Foreground(colorOrangeDim).Render("║") + " " + bar + " " + lipgloss.NewStyle().Foreground(colorOrangeDim).Render("║")))
	b.WriteString(padLine("  " + lipgloss.NewStyle().Foreground(colorOrangeDim).Render("╚"+strings.Repeat("═", barWidth+2)+"╝")))
	b.WriteString(scanLine())

	// Stats in styled boxes
	makeStatPill := func(icon, label string, value int, style lipgloss.Style) string {
		return style.Render(icon) + " " + style.Render(label) + " " +
			lipgloss.NewStyle().Foreground(colorWhite).Bold(true).Render(fmt.Sprintf("%d", value))
	}

	statsLine := "  " +
		makeStatPill(iconOnline, "AVAILABLE", avail, statusOnline) + "    " +
		makeStatPill(iconCooldown, "COOLING", cooling, statusWarning) + "    " +
		makeStatPill(iconDisabled, "DISABLED", disabled, statusOffline) + "    " +
		labelStyle.Render("TOTAL ") + accentStyle.Bold(true).Render(fmt.Sprintf("%d", total))
	b.WriteString(padLine(statsLine))

	b.WriteString(scanLine())

	// Recovery info with box
	if m.rateLimits.NextRecovery != "" {
		b.WriteString(padLine(lipgloss.NewStyle().Foreground(colorSteel).Render("  ╭─ Recovery ─╮")))
		b.WriteString(padLine(lipgloss.NewStyle().Foreground(colorSteel).Render("  │") +
			dimStyle.Render(" ⏱ Next: ") + accentStyle.Render(m.rateLimits.NextRecovery)))
		b.WriteString(padLine(lipgloss.NewStyle().Foreground(colorSteel).Render("  ╰" + strings.Repeat("─", 20) + "╯")))
	}

	// Fill remaining space
	contentLines := strings.Count(b.String(), "\n")
	availableLines := m.height - 8
	remainingLines := availableLines - contentLines
	for i := 0; i < remainingLines && i < 8; i++ {
		b.WriteString(scanLine())
	}

	return b.String()
}

func (m Model) renderUsage() string {
	width := m.width
	if width < 80 {
		width = 80
	}
	prefix := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV)
	suffix := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV)

	var b strings.Builder

	padLine := func(content string) string {
		contentWidth := lipgloss.Width(content)
		padLen := width - lipgloss.Width(prefix) - contentWidth - lipgloss.Width(suffix)
		if padLen < 0 {
			padLen = 0
		}
		return prefix + content + strings.Repeat(" ", padLen) + suffix + "\n"
	}

	scanLine := func() string {
		inner := width - 2
		// Simple uniform pattern - subtle dots at fixed intervals
		pattern := strings.Repeat(" ", inner)
		return prefix + lipgloss.NewStyle().Foreground(colorCharcoal).Render(pattern) + suffix + "\n"
	}

	b.WriteString(padLine(""))

	// Header - dynamic width
	sepWidth := width - 35
	if sepWidth < 10 {
		sepWidth = 10
	}
	headerLine := "  " + panelTitleStyle.Render(" "+iconChart+" USAGE STATISTICS ") +
		dimStyle.Render("  "+strings.Repeat("━", sepWidth))
	b.WriteString(padLine(headerLine))
	b.WriteString(scanLine())

	// Format numbers
	formatNum := func(n int64) string {
		if n >= 1000000 {
			return fmt.Sprintf("%.1fM", float64(n)/1000000)
		} else if n >= 1000 {
			return fmt.Sprintf("%.1fK", float64(n)/1000)
		}
		return fmt.Sprintf("%d", n)
	}

	// Stats in boxed layout
	colWidth := (width - 12) / 4
	if colWidth < 18 {
		colWidth = 18
	}

	makeUsageBox := func(label, value string, style lipgloss.Style) string {
		boxW := colWidth - 2
		if boxW < 14 {
			boxW = 14
		}
		top := "╔" + strings.Repeat("═", boxW) + "╗"
		bot := "╚" + strings.Repeat("═", boxW) + "╝"

		labelPad := boxW - lipgloss.Width(label) - 1
		if labelPad < 0 {
			labelPad = 0
		}
		labelLine := "║ " + labelStyle.Render(label) + strings.Repeat(" ", labelPad) + "║"

		valuePad := (boxW - lipgloss.Width(value)) / 2
		if valuePad < 0 {
			valuePad = 0
		}
		valueRightPad := boxW - lipgloss.Width(value) - valuePad
		if valueRightPad < 0 {
			valueRightPad = 0
		}
		valueLine := "║" + strings.Repeat(" ", valuePad) + style.Bold(true).Render(value) + strings.Repeat(" ", valueRightPad) + "║"

		return lipgloss.NewStyle().Foreground(colorOrangeDim).Render(top) + "\n" +
			lipgloss.NewStyle().Foreground(colorOrangeDim).Render(labelLine) + "\n" +
			lipgloss.NewStyle().Foreground(colorOrangeDim).Render(valueLine) + "\n" +
			lipgloss.NewStyle().Foreground(colorOrangeDim).Render(bot)
	}

	box1 := makeUsageBox("REQUESTS", formatNum(m.usage.TotalRequests), accentStyle)
	box2 := makeUsageBox("INPUT", formatNum(m.usage.TotalInputTokens), valueStyle)
	box3 := makeUsageBox("OUTPUT", formatNum(m.usage.TotalOutputTokens), valueStyle)
	box4 := makeUsageBox("COST", fmt.Sprintf("$%.2f", m.usage.EstimatedCost), statusOnline)

	// Render boxes side by side
	box1Lines := strings.Split(box1, "\n")
	box2Lines := strings.Split(box2, "\n")
	box3Lines := strings.Split(box3, "\n")
	box4Lines := strings.Split(box4, "\n")

	for i := 0; i < len(box1Lines); i++ {
		line := " " + box1Lines[i] + " " + box2Lines[i] + " " + box3Lines[i] + " " + box4Lines[i]
		b.WriteString(padLine(line))
	}

	b.WriteString(scanLine())

	// Top models section with better framing
	if len(m.usage.TopModels) > 0 {
		modelsHeader := "  ╭─ Top Models " + strings.Repeat("─", 58) + "╮"
		b.WriteString(padLine(lipgloss.NewStyle().Foreground(colorSteel).Render(modelsHeader)))

		for i, model := range m.usage.TopModels {
			if i >= 5 {
				break
			}
			rank := accentStyle.Render(fmt.Sprintf("  │ %d.", i+1))
			name := valueStyle.Render(truncate(model.Model, 35))
			stats := dimStyle.Render(fmt.Sprintf("%s req  %s tok", formatNum(model.Requests), formatNum(model.Tokens)))
			b.WriteString(padLine(rank + " " + name + "  " + stats))
		}
		b.WriteString(padLine(lipgloss.NewStyle().Foreground(colorSteel).Render("  ╰" + strings.Repeat("─", 72) + "╯")))
	}

	// Fill remaining space
	contentLines := strings.Count(b.String(), "\n")
	availableLines := m.height - 8
	remainingLines := availableLines - contentLines
	for i := 0; i < remainingLines && i < 6; i++ {
		b.WriteString(scanLine())
	}

	return b.String()
}

func (m Model) renderLogs() string {
	width := m.width
	if width < 80 {
		width = 80
	}
	prefix := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV)
	suffix := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(boxV)

	var b strings.Builder

	padLine := func(content string) string {
		contentWidth := lipgloss.Width(content)
		padLen := width - lipgloss.Width(prefix) - contentWidth - lipgloss.Width(suffix)
		if padLen < 0 {
			padLen = 0
		}
		return prefix + content + strings.Repeat(" ", padLen) + suffix + "\n"
	}

	b.WriteString(padLine(""))

	if len(m.logs) == 0 {
		b.WriteString(padLine(dimStyle.Render("  No recent logs")))
		b.WriteString(padLine(""))
		return b.String()
	}

	// Header - dynamic width
	sepWidth := width - 45
	if sepWidth < 10 {
		sepWidth = 10
	}
	headerLine := "  " + panelTitleStyle.Render(" "+iconLog+" RECENT LOGS ") + "  " +
		dimStyle.Render(fmt.Sprintf("%d entries", len(m.logs))) +
		dimStyle.Render("  "+strings.Repeat("━", sepWidth))
	b.WriteString(padLine(headerLine))
	b.WriteString(padLine(""))

	// Calculate visible lines
	maxLines := m.height - 14
	if maxLines < 5 {
		maxLines = 5
	}
	if maxLines > 20 {
		maxLines = 20
	}
	start := len(m.logs) - maxLines
	if start < 0 {
		start = 0
	}

	for _, log := range m.logs[start:] {
		var styled string
		switch {
		case strings.Contains(log, "ERROR") || strings.Contains(log, "error"):
			styled = statusOffline.Render("  " + log)
		case strings.Contains(log, "WARN") || strings.Contains(log, "warn"):
			styled = statusWarning.Render("  " + log)
		case strings.Contains(log, "DEBUG") || strings.Contains(log, "debug"):
			styled = dimStyle.Render("  " + log)
		default:
			styled = labelStyle.Render("  " + log)
		}
		// Truncate if too long
		maxLogWidth := width - 8
		if lipgloss.Width(styled) > maxLogWidth {
			styled = truncate(styled, maxLogWidth)
		}
		b.WriteString(padLine(styled))
	}

	b.WriteString(padLine(""))

	return b.String()
}

func (m Model) renderFooter() string {
	width := m.width
	if width < 80 {
		width = 80
	}

	// Bottom border
	botLine := lipgloss.NewStyle().Foreground(colorOrangeDim).Render(
		boxBL + strings.Repeat(boxH, width-2) + boxBR,
	)

	// Help bar with better styling
	keys := []struct {
		key  string
		desc string
	}{
		{"←/→", "tabs"},
		{"1-5", "jump"},
		{"a", "add"},
		{"r", "refresh"},
		{"q", "quit"},
	}

	var parts []string
	for _, k := range keys {
		parts = append(parts, helpKeyStyle.Render(k.key)+helpStyle.Render(" "+k.desc))
	}
	helpLine := " " + strings.Join(parts, dimStyle.Render(" │ "))

	return "\n" + botLine + "\n" + helpLine + "\n"
}

func (m Model) renderShutdown() string {
	width := m.width
	if width < 60 {
		width = 60
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorOrangeDim).Render(
		"  ┏"+strings.Repeat("━", 30)+"┓",
	) + "\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorOrangeDim).Render("  ┃") +
		accentStyle.Render(" PROXYPILOT SHUTDOWN ") +
		lipgloss.NewStyle().Foreground(colorOrangeDim).Render("       ┃") + "\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorOrangeDim).Render(
		"  ┗"+strings.Repeat("━", 30)+"┛",
	) + "\n")
	b.WriteString("\n")

	return b.String()
}

// Helper functions
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// fetchForTab fetches data for the current tab.
func (m Model) fetchForTab() tea.Cmd {
	switch m.currentTab {
	case TabStatus:
		return m.fetchStatus()
	case TabAccounts:
		return m.fetchAccounts()
	case TabRateLimits:
		return m.fetchRateLimits()
	case TabUsage:
		return m.fetchUsage()
	case TabLogs:
		return m.fetchLogs()
	}
	return nil
}

// Fetch commands
func (m Model) fetchStatus() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		status, err := client.FetchStatus()
		if err != nil {
			return errMsg(err)
		}
		return statusMsg(status)
	}
}

func (m Model) fetchAccounts() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		accounts, err := client.FetchAccounts()
		if err != nil {
			return errMsg(err)
		}
		return accountsMsg(accounts)
	}
}

func (m Model) fetchRateLimits() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		limits, err := client.FetchRateLimits()
		if err != nil {
			return errMsg(err)
		}
		return rateLimitsMsg(limits)
	}
}

func (m Model) fetchUsage() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		usage, err := client.FetchUsage()
		if err != nil {
			return errMsg(err)
		}
		return usageMsg(usage)
	}
}

func (m Model) fetchLogs() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		logs, err := client.FetchLogs()
		if err != nil {
			return errMsg(err)
		}
		return logsMsg(logs)
	}
}

// Login flow commands
func (m Model) startLogin(providerID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		url, state, err := client.StartAuth(providerID)
		if err != nil {
			return authErrorMsg{err}
		}
		if err := openBrowser(url); err != nil {
			return authErrorMsg{fmt.Errorf("failed to open browser: %w", err)}
		}
		return authURLMsg{URL: url, State: state}
	}
}

func (m Model) pollAuthStatus() tea.Cmd {
	client := m.client
	state := m.loginState
	return func() tea.Msg {
		status, message, err := client.GetAuthStatus(state)
		if err != nil {
			return authErrorMsg{err}
		}
		return authStatusMsg{Status: status, Message: message}
	}
}
