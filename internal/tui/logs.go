package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ═══════════════════════════════════════════════════════════════════════════════
// LOGS VIEWER - Cyberpunk Neon Edition
// A visually stunning log viewer with electric neon aesthetics
// ═══════════════════════════════════════════════════════════════════════════════

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
	Fields    map[string]interface{}
}

// LogsModel represents the logs viewer screen
type LogsModel struct {
	width    int
	height   int
	viewport viewport.Model
	logs     []LogEntry
	filter   string
	levels   []string
	selected int

	// Auto-scroll state
	autoScroll  bool
	lastRefresh time.Time

	// Animation state for pulsing effect
	pulseFrame int
}

// logsRefreshMsg is sent when logs are refreshed
type logsRefreshMsg struct {
	logs []LogEntry
}

// logsTickMsg triggers periodic refresh
type logsTickMsg time.Time

// logsPulseMsg triggers pulse animation
type logsPulseMsg time.Time

// ─────────────────────────────────────────────────────────────────────────────
// Log Level Badge Styles - Vibrant neon indicators
// ─────────────────────────────────────────────────────────────────────────────

var (
	// Error badge - Hot Coral with glow effect
	logErrorBadge = lipgloss.NewStyle().
			Foreground(DeepBlack).
			Background(HotCoral).
			Bold(true).
			Padding(0, 1)

	// Warning badge - Warm Amber
	logWarnBadge = lipgloss.NewStyle().
			Foreground(DeepBlack).
			Background(Amber).
			Bold(true).
			Padding(0, 1)

	// Info badge - Electric Blue
	logInfoBadge = lipgloss.NewStyle().
			Foreground(DeepBlack).
			Background(ElecBlue).
			Bold(true).
			Padding(0, 1)

	// Debug badge - Soft Violet
	logDebugBadge = lipgloss.NewStyle().
			Foreground(TextBright).
			Background(Violet).
			Bold(true).
			Padding(0, 1)

	// Default log badge - Muted
	logDefaultBadge = lipgloss.NewStyle().
			Foreground(TextMuted).
			Background(Surface).
			Padding(0, 1)

	// Timestamp style - Dim for subtle time display
	timestampStyle = lipgloss.NewStyle().
			Foreground(TextDim)

	// Log message style
	logMessageStyle = lipgloss.NewStyle().
			Foreground(TextBright)

	// Filter tab styles
	filterTabActive = lipgloss.NewStyle().
			Foreground(DeepBlack).
			Background(Cyan).
			Bold(true).
			Padding(0, 2).
			MarginRight(1)

	filterTabInactive = lipgloss.NewStyle().
				Foreground(TextMuted).
				Background(Surface).
				Padding(0, 2).
				MarginRight(1)

	// Status indicator styles
	autoScrollOnStyle = lipgloss.NewStyle().
				Foreground(NeonGreen).
				Bold(true)

	autoScrollOffStyle = lipgloss.NewStyle().
				Foreground(TextDim)

	// Pulse animation colors for auto-scroll indicator
	pulseColors = []lipgloss.Color{
		NeonGreen,
		GreenMid,
		GreenDark,
		GreenMid,
		NeonGreen,
		GreenMid,
	}

	// Log viewer title style
	logViewerTitle = lipgloss.NewStyle().
			Foreground(Cyan).
			Bold(true)

	// Section separator with neon accent
	sectionSeparator = lipgloss.NewStyle().
				Foreground(Violet)

	// Status bar container
	statusBarContainer = lipgloss.NewStyle().
				Foreground(TextMuted).
				Background(DarkSurface).
				Padding(0, 1)

	// Log count style
	logCountStyle = lipgloss.NewStyle().
			Foreground(Magenta).
			Bold(true)

	// Refresh timestamp style
	refreshTimeStyle = lipgloss.NewStyle().
				Foreground(TextDim)

	// Empty state style
	emptyStateStyle = lipgloss.NewStyle().
			Foreground(TextMuted).
			Italic(true)
)

// NewLogsModel creates a new logs viewer model
func NewLogsModel() LogsModel {
	vp := viewport.New(70, 15)
	vp.Style = lipgloss.NewStyle().
		Border(CyberBorder).
		BorderForeground(Cyan).
		Padding(0, 1)

	return LogsModel{
		width:       80,
		height:      24,
		viewport:    vp,
		logs:        []LogEntry{},
		levels:      []string{"all", "info", "warn", "error", "debug"},
		selected:    0,
		autoScroll:  true,
		lastRefresh: time.Now(),
		pulseFrame:  0,
	}
}

// Init initializes the logs model
func (m LogsModel) Init() tea.Cmd {
	return tea.Batch(
		m.refreshLogs,
		m.tickCmd(),
		m.pulseCmd(),
	)
}

// tickCmd returns a command for periodic refresh
func (m LogsModel) tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return logsTickMsg(t)
	})
}

// pulseCmd returns a command for pulse animation
func (m LogsModel) pulseCmd() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
		return logsPulseMsg(t)
	})
}

// refreshLogs fetches current logs
func (m LogsModel) refreshLogs() tea.Msg {
	// TODO: Implement log ring buffer in logging package
	// For now, show sample logs to demonstrate the UI
	logs := []LogEntry{
		{Timestamp: time.Now().Add(-10 * time.Minute), Level: "info", Message: "Initializing ProxyPilot server..."},
		{Timestamp: time.Now().Add(-9 * time.Minute), Level: "debug", Message: "Loading configuration from config.yaml"},
		{Timestamp: time.Now().Add(-8 * time.Minute), Level: "info", Message: "Server started on :8317"},
		{Timestamp: time.Now().Add(-7 * time.Minute), Level: "info", Message: "Claude provider initialized successfully"},
		{Timestamp: time.Now().Add(-6 * time.Minute), Level: "warn", Message: "Rate limit threshold approaching for claude-sonnet"},
		{Timestamp: time.Now().Add(-5 * time.Minute), Level: "info", Message: "Gemini provider initialized successfully"},
		{Timestamp: time.Now().Add(-4 * time.Minute), Level: "debug", Message: "Health check endpoint responding normally"},
		{Timestamp: time.Now().Add(-3 * time.Minute), Level: "info", Message: "Management API enabled on /api/v1"},
		{Timestamp: time.Now().Add(-2 * time.Minute), Level: "error", Message: "Connection refused to backup endpoint"},
		{Timestamp: time.Now().Add(-1 * time.Minute), Level: "warn", Message: "Retry attempt 1/3 for backup connection"},
		{Timestamp: time.Now(), Level: "info", Message: "Ready to accept connections"},
	}

	return logsRefreshMsg{logs: logs}
}

// Update handles messages for the logs model
func (m LogsModel) Update(msg tea.Msg) (LogsModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = max(m.width-10, 40)
		m.viewport.Height = max(m.height-18, 8)
		m.viewport.Style = lipgloss.NewStyle().
			Border(CyberBorder).
			BorderForeground(Cyan).
			Padding(0, 1).
			Width(m.viewport.Width)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.selected = (m.selected + 1) % len(m.levels)
			m.updateViewport()
		case "shift+tab":
			m.selected = (m.selected - 1 + len(m.levels)) % len(m.levels)
			m.updateViewport()
		case "a":
			m.autoScroll = !m.autoScroll
		case "r":
			return m, m.refreshLogs
		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
		}

	case logsPulseMsg:
		m.pulseFrame = (m.pulseFrame + 1) % len(pulseColors)
		cmds = append(cmds, m.pulseCmd())

	case logsTickMsg:
		cmds = append(cmds, m.tickCmd())
		if m.autoScroll {
			cmds = append(cmds, m.refreshLogs)
		}

	case logsRefreshMsg:
		m.logs = msg.logs
		m.lastRefresh = time.Now()
		m.updateViewport()
		if m.autoScroll {
			m.viewport.GotoBottom()
		}
	}

	// Handle viewport scrolling
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// updateViewport updates the viewport content based on current filter
func (m *LogsModel) updateViewport() {
	var content strings.Builder

	filterLevel := m.levels[m.selected]

	for _, log := range m.logs {
		// Apply filter
		if filterLevel != "all" && log.Level != filterLevel {
			continue
		}

		// Format log entry
		line := m.formatLogEntry(log, m.width)
		content.WriteString(line)
		content.WriteString("\n")
	}

	if content.Len() == 0 {
		content.WriteString(emptyStateStyle.Render("  No logs matching current filter"))
	}

	m.viewport.SetContent(content.String())
}

// formatLogEntry formats a single log entry with cyberpunk styling
func (m LogsModel) formatLogEntry(entry LogEntry, width int) string {
	// Timestamp with dim styling
	ts := entry.Timestamp.Format("15:04:05.000")
	formattedTs := timestampStyle.Render(ts)

	// Level badge with vibrant colors
	var levelBadge string
	switch entry.Level {
	case "error":
		levelBadge = logErrorBadge.Render("ERR")
	case "warn", "warning":
		levelBadge = logWarnBadge.Render("WRN")
	case "info":
		levelBadge = logInfoBadge.Render("INF")
	case "debug":
		levelBadge = logDebugBadge.Render("DBG")
	default:
		levelBadge = logDefaultBadge.Render("LOG")
	}

	// Message with appropriate styling based on level
	msg := entry.Message
	maxLen := width - 30 // Reserve space for timestamp and level badge
	if maxLen < 20 {
		maxLen = 20
	}
	if len(msg) > maxLen {
		msg = msg[:maxLen-3] + "..."
	}

	// Color message based on level for emphasis
	var formattedMsg string
	switch entry.Level {
	case "error":
		formattedMsg = lipgloss.NewStyle().Foreground(HotCoral).Render(msg)
	case "warn", "warning":
		formattedMsg = lipgloss.NewStyle().Foreground(Amber).Render(msg)
	default:
		formattedMsg = logMessageStyle.Render(msg)
	}

	// Build the line with proper spacing
	return fmt.Sprintf(" %s  %s  %s", formattedTs, levelBadge, formattedMsg)
}

// View renders the logs viewer screen - Returns content only (no outer borders)
// The main TUI wraps this in a bordered panel
func (m LogsModel) View() string {
	return m.ViewWithSize(m.width, m.height)
}

// ViewWithSize renders the logs viewer screen with specified dimensions
func (m LogsModel) ViewWithSize(width, height int) string {
	var b strings.Builder

	// Section title
	title := lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true).
		Render("Logs")
	subtitle := lipgloss.NewStyle().
		Foreground(TextMuted).
		Render(" - Real-time stream")
	b.WriteString(title + subtitle + "\n\n")

	// Filter tabs
	b.WriteString(m.renderFilterTabs())
	b.WriteString("\n\n")

	// Status bar
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n\n")

	// Log entries (rendered directly, not in viewport with its own border)
	b.WriteString(m.renderLogEntries(width, height))

	return b.String()
}

// renderLogEntries renders log entries directly without viewport borders
func (m LogsModel) renderLogEntries(width, height int) string {
	var content strings.Builder

	filterLevel := m.levels[m.selected]
	count := 0
	maxVisible := height - 8 // Reserve space for header, tabs, status bar, etc.
	if maxVisible < 5 {
		maxVisible = 5
	}

	// Collect filtered entries
	var filtered []LogEntry
	for _, log := range m.logs {
		if filterLevel != "all" && log.Level != filterLevel {
			continue
		}
		filtered = append(filtered, log)
	}

	// Show most recent entries
	start := 0
	if len(filtered) > maxVisible {
		start = len(filtered) - maxVisible
	}

	for i := start; i < len(filtered); i++ {
		line := m.formatLogEntry(filtered[i], width)
		content.WriteString(line + "\n")
		count++
	}

	if count == 0 {
		content.WriteString(lipgloss.NewStyle().Foreground(TextMuted).Italic(true).Render("No logs matching current filter"))
	}

	return content.String()
}

// renderFilterTabs renders the log level filter tabs
func (m LogsModel) renderFilterTabs() string {
	var tabs []string

	// Level labels
	levelLabels := map[string]string{
		"all":   "ALL",
		"info":  "INF",
		"warn":  "WRN",
		"error": "ERR",
		"debug": "DBG",
	}

	// Level-specific colors
	levelColors := map[string]lipgloss.Color{
		"all":   Accent,
		"info":  Blue,
		"warn":  Yellow,
		"error": Red,
		"debug": Secondary,
	}

	for i, level := range m.levels {
		label := levelLabels[level]

		if i == m.selected {
			style := lipgloss.NewStyle().
				Foreground(BgDark).
				Background(levelColors[level]).
				Bold(true).
				Padding(0, 1)
			tabs = append(tabs, style.Render(label))
		} else {
			style := lipgloss.NewStyle().
				Foreground(TextMuted).
				Background(BgPanel).
				Padding(0, 1)
			tabs = append(tabs, style.Render(label))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

// renderStatusBar renders the status bar
func (m LogsModel) renderStatusBar() string {
	// Log count
	filteredCount := 0
	filterLevel := m.levels[m.selected]
	for _, log := range m.logs {
		if filterLevel == "all" || log.Level == filterLevel {
			filteredCount++
		}
	}

	countText := lipgloss.NewStyle().Foreground(TextMuted).Render(fmt.Sprintf("Showing %d entries", filteredCount))

	// Auto-scroll indicator
	var scrollText string
	if m.autoScroll {
		scrollText = lipgloss.NewStyle().Foreground(Green).Render("● AUTO")
	} else {
		scrollText = lipgloss.NewStyle().Foreground(TextMuted).Render("○ PAUSED")
	}

	// Last refresh
	refreshText := lipgloss.NewStyle().Foreground(TextMuted).Render(fmt.Sprintf("Updated: %s", m.lastRefresh.Format("15:04:05")))

	sep := lipgloss.NewStyle().Foreground(BorderDim).Render(" │ ")
	return countText + sep + scrollText + sep + refreshText
}

// renderHelp renders the help footer with neon key hints
func (m LogsModel) renderHelp() string {
	helpItems := []struct {
		key  string
		desc string
	}{
		{"Tab", "Filter"},
		{"↑/↓", "Scroll"},
		{"g/G", "Top/Bottom"},
		{"a", "Auto-scroll"},
		{"r", "Refresh"},
		{"Esc", "Back"},
	}

	var parts []string
	for _, item := range helpItems {
		key := lipgloss.NewStyle().
			Foreground(Violet).
			Bold(true).
			Render("[" + item.key + "]")
		desc := lipgloss.NewStyle().
			Foreground(TextDim).
			Render(" " + item.desc)
		parts = append(parts, key+desc)
	}

	return strings.Join(parts, "  ")
}
