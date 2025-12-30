package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

// ═══════════════════════════════════════════════════════════════════════════════
// NEON TABLE STYLES - Cyberpunk-inspired data presentation
// ═══════════════════════════════════════════════════════════════════════════════

var (
	// Table header with electric cyan and violet underline
	neonTableHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(Cyan).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(Violet).
			PaddingBottom(0).
			MarginBottom(0)

	// Selected row with glowing cyan highlight
	selectedRowStyle = lipgloss.NewStyle().
				Background(BgSelected).
				Foreground(Cyan).
				Bold(true)

	// Normal row styling
	normalRowStyle = lipgloss.NewStyle().
			Foreground(TextBright)

	// Alias column - primary focus element
	aliasBaseStyle = lipgloss.NewStyle().
			Width(28)

	// Provider column
	providerBaseStyle = lipgloss.NewStyle().
				Width(14)

	// Model column
	modelBaseStyle = lipgloss.NewStyle().
			Width(36)

	// Cursor styles
	cursorActive = lipgloss.NewStyle().
			Foreground(Magenta).
			Bold(true)

	cursorInactive = lipgloss.NewStyle().
			Foreground(TextDim)

	// Scroll indicator style
	scrollIndicatorStyle = lipgloss.NewStyle().
				Foreground(Violet).
				Background(DarkSurface).
				Padding(0, 1)

	// Section divider with neon effect
	neonDivider = lipgloss.NewStyle().
			Foreground(Violet)
)

// ModelMapping represents a single model alias mapping
type ModelMapping struct {
	Alias    string
	Provider string
	Model    string
	Active   bool
}

// MappingsModel represents the model mappings screen
type MappingsModel struct {
	width    int
	height   int
	cursor   int
	mappings []ModelMapping
	cfg      *config.Config
}

// NewMappingsModel creates a new model mappings model
func NewMappingsModel(cfg *config.Config) MappingsModel {
	m := MappingsModel{
		width:  80,
		height: 24,
		cfg:    cfg,
	}
	m.loadMappings()
	return m
}

// loadMappings loads model mappings from config
func (m *MappingsModel) loadMappings() {
	m.mappings = []ModelMapping{}

	// Default mappings that ProxyPilot supports
	defaultMappings := []ModelMapping{
		// Claude models
		{Alias: "claude-3-opus", Provider: "Claude", Model: "claude-3-opus-20240229", Active: true},
		{Alias: "claude-3-sonnet", Provider: "Claude", Model: "claude-3-sonnet-20240229", Active: true},
		{Alias: "claude-3-haiku", Provider: "Claude", Model: "claude-3-haiku-20240307", Active: true},
		{Alias: "claude-opus-4-5-20251101", Provider: "Claude", Model: "claude-opus-4-5-20251101", Active: true},
		{Alias: "claude-sonnet-4-5-20251101", Provider: "Claude", Model: "claude-sonnet-4-5-20251101", Active: true},

		// GPT models
		{Alias: "gpt-4", Provider: "Codex", Model: "gpt-4", Active: true},
		{Alias: "gpt-4-turbo", Provider: "Codex", Model: "gpt-4-turbo", Active: true},
		{Alias: "gpt-4o", Provider: "Codex", Model: "gpt-4o", Active: true},
		{Alias: "o1", Provider: "Codex", Model: "o1", Active: true},
		{Alias: "o1-mini", Provider: "Codex", Model: "o1-mini", Active: true},

		// Gemini models
		{Alias: "gemini-pro", Provider: "Gemini", Model: "gemini-pro", Active: true},
		{Alias: "gemini-1.5-pro", Provider: "Gemini", Model: "gemini-1.5-pro", Active: true},
		{Alias: "gemini-1.5-flash", Provider: "Gemini", Model: "gemini-1.5-flash", Active: true},
		{Alias: "gemini-2.0-flash", Provider: "Gemini", Model: "gemini-2.0-flash-exp", Active: true},
	}

	m.mappings = defaultMappings

	// Note: Custom mappings can be added via config.yaml
	// The ModelMapping field would be added to config if needed
}

// Init initializes the mappings model
func (m MappingsModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the mappings model
func (m MappingsModel) Update(msg tea.Msg) (MappingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.mappings)-1 {
				m.cursor++
			}
		case "g":
			m.cursor = 0
		case "G":
			m.cursor = len(m.mappings) - 1
		}
	}

	return m, nil
}

// View renders the model mappings screen - Returns content only (no outer borders)
// The main TUI wraps this in a bordered panel
func (m MappingsModel) View() string {
	return m.ViewWithSize(m.width, m.height)
}

// ViewWithSize renders the model mappings screen with specified dimensions
// This allows for responsive layouts based on terminal size
func (m MappingsModel) ViewWithSize(width, height int) string {
	var b strings.Builder

	// Calculate responsive column widths based on available width
	aliasWidth, providerWidth, modelWidth := m.calculateColumnWidths(width)

	// Section title (compact)
	title := lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true).
		Render("Model Mappings")
	subtitle := lipgloss.NewStyle().
		Foreground(TextMuted).
		Render(" - Alias → Provider Model")
	b.WriteString(title + subtitle + "\n\n")

	// Table header with responsive widths
	headerAlias := lipgloss.NewStyle().Width(aliasWidth).Foreground(Accent).Bold(true).Render("  ALIAS")
	headerProvider := lipgloss.NewStyle().Width(providerWidth).Foreground(Accent).Bold(true).Render("PROVIDER")
	headerModel := lipgloss.NewStyle().Width(modelWidth).Foreground(Accent).Bold(true).Render("MODEL")
	b.WriteString(headerAlias + headerProvider + headerModel + "\n")

	// Divider width matches total table width
	dividerWidth := aliasWidth + providerWidth + modelWidth
	b.WriteString(lipgloss.NewStyle().Foreground(BorderDim).Render(strings.Repeat("─", dividerWidth)) + "\n")

	// Calculate max visible rows based on height
	// Reserve lines for: title (2), header (1), divider (1), scroll indicator (2)
	maxRows := height - 6
	if maxRows < 3 {
		maxRows = 3 // Minimum visible rows
	}

	visibleRows := min(maxRows, len(m.mappings))
	startIdx := 0
	if m.cursor >= visibleRows {
		startIdx = m.cursor - visibleRows + 1
	}

	for i := startIdx; i < min(startIdx+visibleRows, len(m.mappings)); i++ {
		mapping := m.mappings[i]
		isSelected := i == m.cursor
		row := m.renderMappingRowWithWidths(mapping, isSelected, aliasWidth, providerWidth, modelWidth)
		b.WriteString(row + "\n")
	}

	// Scroll indicator
	if len(m.mappings) > visibleRows {
		endIdx := min(startIdx+visibleRows, len(m.mappings))
		scrollText := fmt.Sprintf("%d-%d of %d", startIdx+1, endIdx, len(m.mappings))
		b.WriteString(lipgloss.NewStyle().Foreground(TextMuted).Render("\n" + scrollText))
	}

	return b.String()
}

// calculateColumnWidths returns responsive column widths based on terminal width
func (m MappingsModel) calculateColumnWidths(width int) (aliasWidth, providerWidth, modelWidth int) {
	// Narrow terminals (< 60)
	if width < 60 {
		aliasWidth = 16
		providerWidth = 10
		modelWidth = 20
		return
	}

	// Medium terminals (60-80)
	if width <= 80 {
		aliasWidth = 24
		providerWidth = 12
		modelWidth = 30
		return
	}

	// Wide terminals (> 80)
	aliasWidth = 28
	providerWidth = 14
	modelWidth = 36
	return
}

// renderMappingRow creates a simple row for the mappings list (uses default widths)
func (m MappingsModel) renderMappingRow(mapping ModelMapping, isSelected bool) string {
	return m.renderMappingRowWithWidths(mapping, isSelected, 24, 12, 30)
}

// renderMappingRowWithWidths creates a row with specified column widths
func (m MappingsModel) renderMappingRowWithWidths(mapping ModelMapping, isSelected bool, aliasWidth, providerWidth, modelWidth int) string {
	// Cursor indicator (takes 2 chars from alias width)
	var cursor string
	if isSelected {
		cursor = lipgloss.NewStyle().Foreground(Accent).Bold(true).Render(IconChevron + " ")
	} else {
		cursor = "  "
	}

	// Alias (subtract 2 for cursor)
	aliasContentWidth := aliasWidth - 2
	var aliasStyle lipgloss.Style
	if isSelected {
		aliasStyle = lipgloss.NewStyle().Foreground(AccentBright).Bold(true).Width(aliasContentWidth)
	} else {
		aliasStyle = lipgloss.NewStyle().Foreground(Text).Width(aliasContentWidth)
	}
	alias := aliasStyle.Render(mapping.Alias)

	// Provider badge
	providerBadge := m.renderProviderBadge(mapping.Provider)
	providerCell := lipgloss.NewStyle().Width(providerWidth).Render(providerBadge)

	// Model name - truncate based on available width
	modelName := mapping.Model
	maxModelLen := modelWidth - 2 // Leave some padding
	if len(modelName) > maxModelLen {
		modelName = modelName[:maxModelLen-3] + "..."
	}
	modelStyle := lipgloss.NewStyle().Foreground(TextMuted).Width(modelWidth)
	model := modelStyle.Render(modelName)

	// Compose row
	row := cursor + alias + providerCell + model

	// Selection highlight
	if isSelected {
		row = lipgloss.NewStyle().Background(BgSelected).Render(row)
	}

	return row
}

// renderProviderBadge creates a styled badge for each provider
func (m MappingsModel) renderProviderBadge(provider string) string {
	var badgeStyle lipgloss.Style
	var icon string

	switch provider {
	case "Claude":
		badgeStyle = lipgloss.NewStyle().
			Foreground(ClaudeBrand).
			Bold(true)
		icon = IconDiamond + " "
	case "Codex":
		badgeStyle = lipgloss.NewStyle().
			Foreground(CodexBrand).
			Bold(true)
		icon = IconCircle + " "
	case "Gemini":
		badgeStyle = lipgloss.NewStyle().
			Foreground(GeminiBrand).
			Bold(true)
		icon = IconStar + " "
	default:
		badgeStyle = lipgloss.NewStyle().
			Foreground(TextMuted)
		icon = IconSquare + " "
	}

	return badgeStyle.Render(icon + provider)
}

// renderHelp renders the help footer with neon styling
func (m MappingsModel) renderHelp() string {
	keyStyle := lipgloss.NewStyle().
		Foreground(Violet).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(TextDim)

	sepStyle := lipgloss.NewStyle().
		Foreground(TextDim).
		Render("  ")

	dividerStyle := lipgloss.NewStyle().
		Foreground(BorderDim)

	// Build help text
	help := dividerStyle.Render(strings.Repeat("─", 50)) + "\n"
	help += keyStyle.Render("["+IconArrowUp+"/"+IconArrowDown+"]") + " " + descStyle.Render("Navigate") + sepStyle
	help += keyStyle.Render("[j/k]") + " " + descStyle.Render("Navigate") + sepStyle
	help += keyStyle.Render("[g/G]") + " " + descStyle.Render("Top/Bottom") + sepStyle
	help += keyStyle.Render("[Esc]") + " " + descStyle.Render("Back")

	return lipgloss.NewStyle().MarginTop(0).Render(help)
}
