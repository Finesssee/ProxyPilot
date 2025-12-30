package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
)

// ═══════════════════════════════════════════════════════════════════════════════
// USAGE MODEL - Cyberpunk Statistics Dashboard
// ═══════════════════════════════════════════════════════════════════════════════

// UsageModel represents the usage statistics screen
type UsageModel struct {
	width  int
	height int

	// Stats data
	totalRequests   int64
	successCount    int64
	failureCount    int64
	requestsToday   int64
	requestsByDay   map[string]int64
	tokensByModel   map[string]int64
	requestsByModel map[string]int64

	// UI state
	selectedTab int
	tabs        []string
	lastRefresh time.Time
	cfg         *config.Config
}

// usageRefreshMsg is sent when stats are refreshed
type usageRefreshMsg struct {
	totalRequests   int64
	successCount    int64
	failureCount    int64
	requestsToday   int64
	requestsByDay   map[string]int64
	tokensByModel   map[string]int64
	requestsByModel map[string]int64
}

// usageTickMsg triggers periodic refresh
type usageTickMsg time.Time

// NewUsageModel creates a new usage statistics model
func NewUsageModel(cfg *config.Config) UsageModel {
	return UsageModel{
		width:           80,
		height:          24,
		cfg:             cfg,
		tabs:            []string{"OVERVIEW", "MODELS", "HISTORY"},
		selectedTab:     0,
		lastRefresh:     time.Now(),
		requestsByDay:   make(map[string]int64),
		tokensByModel:   make(map[string]int64),
		requestsByModel: make(map[string]int64),
	}
}

// Init initializes the usage model
func (m UsageModel) Init() tea.Cmd {
	return tea.Batch(
		m.refreshStats,
		m.tickCmd(),
	)
}

// tickCmd returns a command for periodic refresh
func (m UsageModel) tickCmd() tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
		return usageTickMsg(t)
	})
}

// refreshStats fetches current statistics
func (m UsageModel) refreshStats() tea.Msg {
	stats := usage.GetRequestStatistics()
	if stats == nil {
		return usageRefreshMsg{}
	}

	snapshot := stats.Snapshot()
	today := time.Now().Format("2006-01-02")
	requestsToday := int64(0)
	if count, ok := snapshot.RequestsByDay[today]; ok {
		requestsToday = count
	}

	// Extract per-model stats from APIs
	tokensByModel := make(map[string]int64)
	requestsByModel := make(map[string]int64)

	for _, api := range snapshot.APIs {
		for modelName, modelStats := range api.Models {
			tokensByModel[modelName] += modelStats.TotalTokens
			requestsByModel[modelName] += modelStats.TotalRequests
		}
	}

	return usageRefreshMsg{
		totalRequests:   snapshot.TotalRequests,
		successCount:    snapshot.SuccessCount,
		failureCount:    snapshot.FailureCount,
		requestsToday:   requestsToday,
		requestsByDay:   snapshot.RequestsByDay,
		tokensByModel:   tokensByModel,
		requestsByModel: requestsByModel,
	}
}

// Update handles messages for the usage model
func (m UsageModel) Update(msg tea.Msg) (UsageModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "l", "right":
			m.selectedTab = (m.selectedTab + 1) % len(m.tabs)
		case "shift+tab", "h", "left":
			m.selectedTab = (m.selectedTab - 1 + len(m.tabs)) % len(m.tabs)
		case "r":
			return m, m.refreshStats
		}

	case usageTickMsg:
		return m, tea.Batch(m.refreshStats, m.tickCmd())

	case usageRefreshMsg:
		m.totalRequests = msg.totalRequests
		m.successCount = msg.successCount
		m.failureCount = msg.failureCount
		m.requestsToday = msg.requestsToday
		m.requestsByDay = msg.requestsByDay
		m.tokensByModel = msg.tokensByModel
		m.requestsByModel = msg.requestsByModel
		m.lastRefresh = time.Now()
	}

	return m, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// VIEW RENDERING - Returns content only (no outer borders)
// The main TUI wraps this in a bordered panel
// ═══════════════════════════════════════════════════════════════════════════════

// View renders the usage statistics screen content
func (m UsageModel) View() string {
	return m.ViewWithSize(m.width, m.height)
}

// ViewWithSize renders the usage statistics screen content with specified dimensions
func (m UsageModel) ViewWithSize(width, height int) string {
	var b strings.Builder

	// Section title with tabs
	title := lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true).
		Render("Usage Statistics")
	refreshTime := m.lastRefresh.Format("15:04:05")
	subtitle := lipgloss.NewStyle().
		Foreground(TextMuted).
		Render(fmt.Sprintf(" - Last sync: %s", refreshTime))
	b.WriteString(title + subtitle + "\n\n")

	// Tabs
	b.WriteString(m.renderTabs())
	b.WriteString("\n\n")

	// Content based on selected tab
	switch m.selectedTab {
	case 0:
		b.WriteString(m.renderOverview(width, height))
	case 1:
		b.WriteString(m.renderByModel(width, height))
	case 2:
		b.WriteString(m.renderHistory(width, height))
	}

	return b.String()
}

// renderTabs renders the tab bar
func (m UsageModel) renderTabs() string {
	var tabs []string

	for i, tab := range m.tabs {
		var style lipgloss.Style

		if i == m.selectedTab {
			// Active tab
			style = lipgloss.NewStyle().
				Bold(true).
				Foreground(BgDark).
				Background(Accent).
				Padding(0, 1)
		} else {
			// Inactive tab
			style = lipgloss.NewStyle().
				Foreground(TextMuted).
				Background(BgPanel).
				Padding(0, 1)
		}

		tabs = append(tabs, style.Render(tab))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

// ═══════════════════════════════════════════════════════════════════════════════
// OVERVIEW TAB - Neon Stat Cards
// ═══════════════════════════════════════════════════════════════════════════════

// renderOverview renders the overview statistics with neon cards
func (m UsageModel) renderOverview(width, height int) string {
	var b strings.Builder

	// Calculate success rate
	successRate := float64(0)
	if m.totalRequests > 0 {
		successRate = float64(m.successCount) / float64(m.totalRequests) * 100
	}

	// Threshold for single column layout (narrow terminals)
	const narrowThreshold = 55

	// Determine layout based on width
	useNarrowLayout := width < narrowThreshold

	var cardWidth int
	if useNarrowLayout {
		// Single column layout - use full width minus padding
		cardWidth = width - 6
		if cardWidth < 20 {
			cardWidth = 20 // Minimum card width
		}
		if cardWidth > 40 {
			cardWidth = 40 // Maximum card width for single column
		}
	} else {
		// Two column layout - account for 2 cards per row with spacing
		availableWidth := width - 4
		cardWidth = availableWidth / 2
		if cardWidth < 20 {
			cardWidth = 20 // Minimum card width
		}
		if cardWidth > 30 {
			cardWidth = 30 // Maximum card width
		}
	}

	// Create neon stat cards
	cards := []string{
		m.renderStatCard("TOTAL REQUESTS", formatLargeNumber(m.totalRequests), Cyan, "pulse", cardWidth),
		m.renderStatCard("TODAY", formatLargeNumber(m.requestsToday), ElecBlue, "bolt", cardWidth),
		m.renderStatCard("SUCCESS RATE", fmt.Sprintf("%.1f%%", successRate), NeonGreen, "check", cardWidth),
		m.renderStatCard("FAILED", formatLargeNumber(m.failureCount), func() lipgloss.Color {
			if m.failureCount > 0 {
				return HotCoral
			}
			return TextDim
		}(), "warn", cardWidth),
	}

	// Arrange cards based on layout
	if useNarrowLayout {
		// Single column layout - stack all cards vertically
		// Limit visible cards based on height (each card ~6 lines)
		maxCards := 4
		if height > 0 {
			// Reserve lines for header, tabs, breakdown section
			availableLines := height - 12
			if availableLines > 0 {
				possibleCards := availableLines / 6
				if possibleCards < maxCards && possibleCards > 0 {
					maxCards = possibleCards
				}
			}
		}
		for i := 0; i < len(cards) && i < maxCards; i++ {
			b.WriteString(cards[i])
			if i < len(cards)-1 && i < maxCards-1 {
				b.WriteString("\n")
			}
		}
	} else {
		// Two column layout - arrange cards in 2x2 grid
		row1 := lipgloss.JoinHorizontal(lipgloss.Top, cards[0], " ", cards[1])
		row2 := lipgloss.JoinHorizontal(lipgloss.Top, cards[2], " ", cards[3])

		b.WriteString(row1)
		b.WriteString("\n")
		b.WriteString(row2)
	}

	// Visual breakdown section
	// Calculate breakdown width based on available space
	breakdownWidth := width - 8 // Account for padding
	if breakdownWidth < 40 {
		breakdownWidth = 40 // Minimum width
	}
	if breakdownWidth > 60 {
		breakdownWidth = 60 // Maximum width
	}
	b.WriteString("\n\n")
	b.WriteString(m.renderBreakdown(breakdownWidth))

	return b.String()
}

// renderStatCard creates a neon-styled stat card
func (m UsageModel) renderStatCard(label, value string, color lipgloss.Color, icon string, cardWidth int) string {
	// Card dimensions
	width := cardWidth

	// Icon based on type
	var iconChar string
	switch icon {
	case "pulse":
		iconChar = IconPulse
	case "bolt":
		iconChar = IconBolt
	case "check":
		iconChar = IconCheck
	case "warn":
		iconChar = IconWarning
	default:
		iconChar = IconDiamond
	}

	// Build card content
	iconStyle := lipgloss.NewStyle().
		Foreground(color).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(TextMuted).
		Width(width - 4)

	valueStyle := lipgloss.NewStyle().
		Foreground(color).
		Bold(true).
		Width(width - 4)

	content := fmt.Sprintf("%s %s\n\n%s",
		iconStyle.Render(iconChar),
		labelStyle.Render(label),
		valueStyle.Render(value),
	)

	// Card with neon border
	cardStyle := lipgloss.NewStyle().
		Border(SoftBorder).
		BorderForeground(color).
		Padding(1, 2).
		Width(width)

	return cardStyle.Render(content)
}

// renderBreakdown shows success/failure breakdown with neon progress bar
func (m UsageModel) renderBreakdown(width int) string {
	var b strings.Builder

	// Section header
	sectionHeader := lipgloss.NewStyle().
		Bold(true).
		Foreground(Violet).
		Render("REQUEST BREAKDOWN")

	divider := lipgloss.NewStyle().
		Foreground(BorderDim).
		Render(strings.Repeat("─", width-20))

	b.WriteString(sectionHeader)
	b.WriteString("\n")
	b.WriteString(divider)
	b.WriteString("\n\n")

	if m.totalRequests > 0 {
		// Calculate proportions
		barWidth := width - 10 // Account for margins and borders
		successPct := float64(m.successCount) / float64(m.totalRequests)
		successBars := int(successPct * float64(barWidth))
		failBars := barWidth - successBars

		// Create gradient progress bar
		successBar := m.createGradientBar(successBars, []lipgloss.Color{NeonGreen, GreenMid, GreenDark})
		failBar := ""
		if failBars > 0 {
			failBar = lipgloss.NewStyle().Foreground(HotCoral).Render(strings.Repeat("█", failBars))
		}

		// Progress bar with glow effect
		barContainer := lipgloss.NewStyle().
			Foreground(BorderDim).
			Render("│")

		b.WriteString(fmt.Sprintf("  %s%s%s%s\n", barContainer, successBar, failBar, barContainer))

		// Legend with neon indicators
		successIndicator := lipgloss.NewStyle().Foreground(NeonGreen).Bold(true).Render("●")
		failIndicator := lipgloss.NewStyle().Foreground(HotCoral).Bold(true).Render("●")
		successLabel := lipgloss.NewStyle().Foreground(TextBright).Render(fmt.Sprintf("Success: %s", formatLargeNumber(m.successCount)))
		failLabel := lipgloss.NewStyle().Foreground(TextBright).Render(fmt.Sprintf("Failed: %s", formatLargeNumber(m.failureCount)))
		separator := lipgloss.NewStyle().Foreground(BorderDim).Render("  │  ")

		b.WriteString(fmt.Sprintf("\n  %s %s%s%s %s\n",
			successIndicator, successLabel,
			separator,
			failIndicator, failLabel,
		))
	} else {
		// Empty state with style
		emptyMsg := lipgloss.NewStyle().
			Foreground(TextDim).
			Italic(true).
			Render("  Awaiting incoming requests...")

		b.WriteString(emptyMsg)
	}

	return b.String()
}

// createGradientBar creates a gradient-colored progress bar
func (m UsageModel) createGradientBar(length int, colors []lipgloss.Color) string {
	if length <= 0 {
		return ""
	}

	var result strings.Builder
	for i := 0; i < length; i++ {
		colorIdx := i * len(colors) / length
		if colorIdx >= len(colors) {
			colorIdx = len(colors) - 1
		}
		result.WriteString(lipgloss.NewStyle().Foreground(colors[colorIdx]).Render("█"))
	}
	return result.String()
}

// ═══════════════════════════════════════════════════════════════════════════════
// MODELS TAB - Neon Data Table
// ═══════════════════════════════════════════════════════════════════════════════

// renderByModel renders statistics grouped by model with neon table
func (m UsageModel) renderByModel(width, height int) string {
	var b strings.Builder

	if len(m.requestsByModel) == 0 {
		return m.renderEmptyState("MODEL STATISTICS", "Statistics will appear as requests are processed.")
	}

	// Calculate responsive column widths based on available width
	// Reserve space for: bar indicator (variable), padding/separators (4)
	availableWidth := width - 4
	if availableWidth < 40 {
		availableWidth = 40 // Minimum usable width
	}

	// Calculate bar width based on terminal width
	barWidth := 8
	if width >= 100 {
		barWidth = 12
	} else if width < 60 {
		barWidth = 4
	}

	// Remaining width after bar for columns
	tableWidth := availableWidth - barWidth - 2

	// Allocate columns: model name gets ~50%, requests and tokens get ~25% each
	modelColWidth := tableWidth * 50 / 100
	countColWidth := tableWidth * 25 / 100
	tokensColWidth := tableWidth - modelColWidth - countColWidth

	// Ensure minimum column widths
	if modelColWidth < 15 {
		modelColWidth = 15
	}
	if countColWidth < 8 {
		countColWidth = 8
	}
	if tokensColWidth < 8 {
		tokensColWidth = 8
	}

	// Table header with neon accents
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Cyan).
		Width(modelColWidth)

	countHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Magenta).
		Width(countColWidth).
		Align(lipgloss.Right)

	tokensHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Violet).
		Width(tokensColWidth).
		Align(lipgloss.Right)

	// Header row
	b.WriteString(headerStyle.Render("MODEL"))
	b.WriteString(countHeaderStyle.Render("REQUESTS"))
	b.WriteString(tokensHeaderStyle.Render("TOKENS"))
	b.WriteString("\n")

	// Neon divider - adapt to total width
	dividerWidth := modelColWidth + countColWidth + tokensColWidth + barWidth + 2
	divider := lipgloss.NewStyle().
		Foreground(Violet).
		Render(strings.Repeat("━", dividerWidth))
	b.WriteString(divider)
	b.WriteString("\n")

	// Sort models by request count for consistent display
	type modelStat struct {
		name     string
		requests int64
		tokens   int64
	}
	var models []modelStat
	for model, count := range m.requestsByModel {
		models = append(models, modelStat{
			name:     model,
			requests: count,
			tokens:   m.tokensByModel[model],
		})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].requests > models[j].requests
	})

	// Model rows with alternating subtle highlights
	for i, model := range models {
		modelName := model.name
		// Truncate model name based on column width
		maxNameLen := modelColWidth - 3
		if maxNameLen < 5 {
			maxNameLen = 5
		}
		if len(modelName) > maxNameLen {
			modelName = modelName[:maxNameLen-3] + "..."
		}

		// Subtle row highlight for alternating rows
		rowBg := DeepBlack
		if i%2 == 1 {
			rowBg = DarkSurface
		}

		modelStyle := lipgloss.NewStyle().
			Foreground(TextBright).
			Background(rowBg).
			Width(modelColWidth)

		countStyle := lipgloss.NewStyle().
			Foreground(Cyan).
			Bold(true).
			Background(rowBg).
			Width(countColWidth).
			Align(lipgloss.Right)

		tokensStyle := lipgloss.NewStyle().
			Foreground(ElecBlue).
			Background(rowBg).
			Width(tokensColWidth).
			Align(lipgloss.Right)

		// Request count bar indicator
		maxRequests := models[0].requests
		barLen := 0
		if maxRequests > 0 {
			barLen = int(float64(model.requests) / float64(maxRequests) * float64(barWidth))
		}
		bar := lipgloss.NewStyle().Foreground(Cyan).Render(strings.Repeat("▪", barLen))
		emptyBar := lipgloss.NewStyle().Foreground(BorderDim).Render(strings.Repeat("▪", barWidth-barLen))

		b.WriteString(modelStyle.Render(modelName))
		b.WriteString(countStyle.Render(formatLargeNumber(model.requests)))
		b.WriteString(tokensStyle.Render(formatLargeNumber(model.tokens)))
		b.WriteString(" ")
		b.WriteString(bar)
		b.WriteString(emptyBar)
		b.WriteString("\n")
	}

	return b.String()
}

// ═══════════════════════════════════════════════════════════════════════════════
// HISTORY TAB - Neon Timeline
// ═══════════════════════════════════════════════════════════════════════════════

// renderHistory renders request history by day with neon visualization
func (m UsageModel) renderHistory(width, height int) string {
	var b strings.Builder

	if len(m.requestsByDay) == 0 {
		return m.renderEmptyState("REQUEST HISTORY", "History will appear as requests are processed.")
	}

	// Calculate responsive bar width
	// Reserve 25 chars for: indent(2) + date(12) + connector(3) + count(10) + spacing
	barWidth := width - 25
	if barWidth < 10 {
		barWidth = 10 // Minimum bar width
	}
	if barWidth > 50 {
		barWidth = 50 // Maximum bar width
	}

	// Section header
	sectionHeader := lipgloss.NewStyle().
		Bold(true).
		Foreground(Magenta).
		Render("ACTIVITY TIMELINE")

	subtitle := lipgloss.NewStyle().
		Foreground(TextDim).
		Render(" // Last 7 days")

	// Divider adapts to width
	dividerWidth := width - 4
	if dividerWidth < 30 {
		dividerWidth = 30
	}
	divider := lipgloss.NewStyle().
		Foreground(Violet).
		Render(strings.Repeat("━", dividerWidth))

	b.WriteString(sectionHeader)
	b.WriteString(subtitle)
	b.WriteString("\n")
	b.WriteString(divider)
	b.WriteString("\n\n")

	// Find max for scaling
	maxDaily := int64(0)
	for _, c := range m.requestsByDay {
		if c > maxDaily {
			maxDaily = c
		}
	}

	// Date and bar styling
	dateStyle := lipgloss.NewStyle().
		Foreground(TextMuted).
		Width(12)

	todayStyle := lipgloss.NewStyle().
		Foreground(Cyan).
		Bold(true).
		Width(12)

	// Get last 7 days
	today := time.Now()

	// Gradient colors for the bars
	barColors := []lipgloss.Color{Cyan, ElecBlue, Violet, Magenta}

	for i := 0; i < 7; i++ {
		date := today.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		count := m.requestsByDay[dateStr]

		// Format date display
		displayDate := date.Format("Jan 02")
		var formattedDate string
		if i == 0 {
			formattedDate = todayStyle.Render("Today")
		} else if i == 1 {
			formattedDate = dateStyle.Render("Yesterday")
		} else {
			formattedDate = dateStyle.Render(displayDate)
		}

		// Calculate bar length
		barLen := 0
		if count > 0 && maxDaily > 0 {
			barLen = int(float64(count) / float64(maxDaily) * float64(barWidth))
			if barLen < 1 && count > 0 {
				barLen = 1
			}
		}

		// Create gradient bar
		bar := m.createGradientBar(barLen, barColors)
		emptyBar := lipgloss.NewStyle().Foreground(BorderDim).Render(strings.Repeat("░", barWidth-barLen))

		// Count display
		countDisplay := lipgloss.NewStyle().
			Foreground(func() lipgloss.Color {
				if count > 0 {
					return TextBright
				}
				return TextDim
			}()).
			Width(10).
			Align(lipgloss.Right).
			Render(formatLargeNumber(count))

		// Timeline connector
		connector := lipgloss.NewStyle().Foreground(Violet).Render("│")

		b.WriteString(fmt.Sprintf("  %s %s %s%s %s\n",
			formattedDate,
			connector,
			bar,
			emptyBar,
			countDisplay,
		))
	}

	return b.String()
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER COMPONENTS
// ═══════════════════════════════════════════════════════════════════════════════

// renderEmptyState creates a styled empty state message
func (m UsageModel) renderEmptyState(title, message string) string {
	var b strings.Builder

	// Empty state container
	iconStyle := lipgloss.NewStyle().
		Foreground(TextDim).
		Render(IconPending)

	titleStyle := lipgloss.NewStyle().
		Foreground(TextMuted).
		Bold(true).
		Render(title)

	msgStyle := lipgloss.NewStyle().
		Foreground(TextDim).
		Italic(true).
		Render(message)

	b.WriteString(fmt.Sprintf("\n  %s  %s\n\n", iconStyle, titleStyle))
	b.WriteString(fmt.Sprintf("  %s\n", msgStyle))

	return b.String()
}

// renderHelp renders the help footer with cyber styling
func (m UsageModel) renderHelp() string {
	// Help bar with neon key hints
	divider := lipgloss.NewStyle().
		Foreground(BorderDim).
		Render(strings.Repeat("─", 56))

	keyStyle := lipgloss.NewStyle().
		Foreground(Violet).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(TextDim)

	separator := lipgloss.NewStyle().
		Foreground(BorderDim).
		Render("  │  ")

	help := fmt.Sprintf("%s %s%s%s %s%s%s %s",
		keyStyle.Render("[Tab/Arrow]"),
		descStyle.Render("Navigate"),
		separator,
		keyStyle.Render("[R]"),
		descStyle.Render("Refresh"),
		separator,
		keyStyle.Render("[Esc]"),
		descStyle.Render("Back"),
	)

	return fmt.Sprintf("\n%s\n%s", divider, help)
}

// formatLargeNumber formats numbers with K/M suffixes
func formatLargeNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
