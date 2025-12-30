package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ═══════════════════════════════════════════════════════════════════════════════
// INTEGRATION CARD ICONS - Brand-appropriate terminal icons
// ═══════════════════════════════════════════════════════════════════════════════

var integrationIcons = map[string]string{
	"Claude Code":    ">>> ",
	"Codex CLI":      "[*] ",
	"Gemini CLI":     "<*> ",
	"Factory Droid":  "{#} ",
	"Cursor IDE":     "|>| ",
	"Continue":       "... ",
}

// ═══════════════════════════════════════════════════════════════════════════════
// INTEGRATION CARD STYLES - Cyberpunk neon aesthetic
// ═══════════════════════════════════════════════════════════════════════════════

var (
	// Card container styles
	integrationCardStyle = lipgloss.NewStyle().
		Border(SoftBorder).
		BorderForeground(BorderDim).
		Padding(0, 2).
		MarginBottom(0)

	integrationCardSelectedStyle = lipgloss.NewStyle().
		Border(CyberBorder).
		BorderForeground(Cyan).
		Padding(0, 2).
		MarginBottom(0)

	integrationCardExpandedStyle = lipgloss.NewStyle().
		Border(NeonBorder).
		BorderForeground(Magenta).
		Padding(1, 2).
		MarginBottom(0)

	// Setup box styles
	setupBoxStyle = lipgloss.NewStyle().
		Border(SoftBorder).
		BorderForeground(Violet).
		Background(DarkSurface).
		Padding(1, 2).
		MarginLeft(2).
		MarginTop(1)

	// Code block style
	codeBlockStyle = lipgloss.NewStyle().
		Background(Surface).
		Foreground(NeonGreen).
		Padding(0, 1)

	// Config path style
	configPathStyle = lipgloss.NewStyle().
		Foreground(TextDim).
		Italic(true)

	// Section header inside setup box
	setupHeaderStyle = lipgloss.NewStyle().
		Foreground(Cyan).
		Bold(true)

	// Step number style
	stepNumberStyle = lipgloss.NewStyle().
		Foreground(Magenta).
		Bold(true)

	// Instruction text style
	instructionStyle = lipgloss.NewStyle().
		Foreground(TextMuted)
)

// ═══════════════════════════════════════════════════════════════════════════════
// STATUS BADGE STYLES - Setup status indicators
// ═══════════════════════════════════════════════════════════════════════════════

var (
	statusProxyBadge = lipgloss.NewStyle().
		Foreground(DeepBlack).
		Background(NeonGreen).
		Bold(true).
		Padding(0, 1)

	statusNativeBadge = lipgloss.NewStyle().
		Foreground(DeepBlack).
		Background(Amber).
		Bold(true).
		Padding(0, 1)

	statusManualBadge = lipgloss.NewStyle().
		Foreground(DeepBlack).
		Background(ElecBlue).
		Bold(true).
		Padding(0, 1)

	statusNotSetBadge = lipgloss.NewStyle().
		Foreground(TextMuted).
		Background(Surface).
		Padding(0, 1)
)

// Integration represents a tool integration
type Integration struct {
	Name        string
	Description string
	ConfigPath  string
	Status      string
	Setup       []string
}

// IntegrationsModel represents the integrations/setup screen
type IntegrationsModel struct {
	width        int
	height       int
	cursor       int
	integrations []Integration
	expanded     int // Which integration is expanded (-1 for none)
}

// NewIntegrationsModel creates a new integrations model
func NewIntegrationsModel() IntegrationsModel {
	integrations := []Integration{
		{
			Name:        "Claude Code",
			Description: "Anthropic's official CLI for Claude",
			ConfigPath:  "~/.claude/settings.json",
			Status:      detectAgentMode(expandPath("~/.claude/settings.json")),
			Setup: []string{
				`Add to ~/.claude/settings.json:`,
				`{`,
				`  "env": {`,
				`    "ANTHROPIC_BASE_URL": "http://127.0.0.1:8317",`,
				`    "ANTHROPIC_AUTH_TOKEN": "your-key"`,
				`  }`,
				`}`,
			},
		},
		{
			Name:        "Codex CLI",
			Description: "OpenAI's coding assistant",
			ConfigPath:  "~/.codex/config.toml",
			Status:      detectAgentMode(expandPath("~/.codex/config.toml")),
			Setup: []string{
				`Add to ~/.codex/config.toml:`,
				``,
				`[openai]`,
				`api_base_url = "http://127.0.0.1:8317"`,
			},
		},
		{
			Name:        "Gemini CLI",
			Description: "Google's Gemini CLI tool",
			ConfigPath:  "~/.gemini/settings.json",
			Status:      detectAgentMode(expandPath("~/.gemini/settings.json")),
			Setup: []string{
				`Add to ~/.gemini/settings.json:`,
				`{`,
				`  "api_base_url": "http://127.0.0.1:8317"`,
				`}`,
			},
		},
		{
			Name:        "Factory Droid",
			Description: "AI-powered development assistant",
			ConfigPath:  "~/.factory/config.json",
			Status:      detectAgentMode(expandPath("~/.factory/config.json")),
			Setup: []string{
				`Add to ~/.factory/config.json:`,
				`{`,
				`  "customModels": [{`,
				`    "name": "ProxyPilot",`,
				`    "baseUrl": "http://127.0.0.1:8317"`,
				`  }]`,
				`}`,
			},
		},
		{
			Name:        "Cursor IDE",
			Description: "AI-first code editor",
			ConfigPath:  "Settings > Models > OpenAI API Base URL",
			Status:      "manual",
			Setup: []string{
				`In Cursor Settings:`,
				``,
				`1. Open Settings (Cmd/Ctrl + ,)`,
				`2. Search for "OpenAI Base URL"`,
				`3. Set to: http://127.0.0.1:8317`,
				`4. Enable "Override OpenAI Base URL"`,
			},
		},
		{
			Name:        "Continue",
			Description: "Open-source AI code assistant",
			ConfigPath:  "~/.continue/config.json",
			Status:      detectAgentMode(expandPath("~/.continue/config.json")),
			Setup: []string{
				`Add to ~/.continue/config.json:`,
				`{`,
				`  "models": [{`,
				`    "title": "ProxyPilot",`,
				`    "provider": "openai",`,
				`    "model": "gpt-4",`,
				`    "apiBase": "http://127.0.0.1:8317"`,
				`  }]`,
				`}`,
			},
		},
	}

	return IntegrationsModel{
		width:        80,
		height:       24,
		integrations: integrations,
		expanded:     -1,
	}
}

// Init initializes the integrations model
func (m IntegrationsModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the integrations model
func (m IntegrationsModel) Update(msg tea.Msg) (IntegrationsModel, tea.Cmd) {
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
			if m.cursor < len(m.integrations)-1 {
				m.cursor++
			}
		case "enter", " ":
			if m.expanded == m.cursor {
				m.expanded = -1 // Collapse
			} else {
				m.expanded = m.cursor // Expand
			}
		}
	}

	return m, nil
}

// View renders the integrations screen
func (m IntegrationsModel) View() string {
	var b strings.Builder

	// Cyber header with decorative elements
	headerLine := lipgloss.NewStyle().Foreground(Violet).Render("━━━") +
		lipgloss.NewStyle().Foreground(Cyan).Bold(true).Render(" INTEGRATIONS ") +
		lipgloss.NewStyle().Foreground(Violet).Render("━━━")
	b.WriteString(headerLine)
	b.WriteString("\n\n")

	// Info text with neon styling
	infoIcon := lipgloss.NewStyle().Foreground(ElecBlue).Render(IconInfo)
	b.WriteString(infoIcon + " " + lipgloss.NewStyle().Foreground(TextMuted).Render("Configure your AI coding tools to route through ProxyPilot"))
	b.WriteString("\n")
	hintIcon := lipgloss.NewStyle().Foreground(Violet).Render(IconChevron)
	b.WriteString(hintIcon + " " + lipgloss.NewStyle().Foreground(TextDim).Render("Press Enter to expand setup instructions"))
	b.WriteString("\n\n")

	// Status legend
	b.WriteString(m.renderStatusLegend())
	b.WriteString("\n\n")

	// Integration list
	for i, integration := range m.integrations {
		isExpanded := m.expanded == i
		isSelected := m.cursor == i

		b.WriteString(m.renderIntegrationCard(integration, isSelected, isExpanded))

		if i < len(m.integrations)-1 {
			b.WriteString("\n")
		}
	}

	// Help footer
	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	return b.String()
}

// renderStatusLegend renders the status indicator legend
func (m IntegrationsModel) renderStatusLegend() string {
	proxyIndicator := lipgloss.NewStyle().Foreground(NeonGreen).Render(IconOnline) + " " +
		lipgloss.NewStyle().Foreground(TextDim).Render("Proxy Active")

	nativeIndicator := lipgloss.NewStyle().Foreground(Amber).Render(IconPending) + " " +
		lipgloss.NewStyle().Foreground(TextDim).Render("Native Mode")

	manualIndicator := lipgloss.NewStyle().Foreground(ElecBlue).Render(IconInfo) + " " +
		lipgloss.NewStyle().Foreground(TextDim).Render("Manual Setup")

	notSetIndicator := lipgloss.NewStyle().Foreground(TextDim).Render(IconOffline) + " " +
		lipgloss.NewStyle().Foreground(TextDim).Render("Not Configured")

	return lipgloss.JoinHorizontal(lipgloss.Top,
		proxyIndicator+"   ",
		nativeIndicator+"   ",
		manualIndicator+"   ",
		notSetIndicator,
	)
}

// renderIntegrationCard renders a single integration as a styled card
func (m IntegrationsModel) renderIntegrationCard(integration Integration, selected, expanded bool) string {
	cardWidth := min(m.width-4, 70)

	// Get the brand icon
	icon := integrationIcons[integration.Name]
	if icon == "" {
		icon = "[?] "
	}

	// Status badge
	var statusBadge string
	var statusIcon string
	switch integration.Status {
	case "proxy":
		statusBadge = statusProxyBadge.Render(" PROXY ")
		statusIcon = lipgloss.NewStyle().Foreground(NeonGreen).Render(IconOnline)
	case "native":
		statusBadge = statusNativeBadge.Render(" NATIVE ")
		statusIcon = lipgloss.NewStyle().Foreground(Amber).Render(IconPending)
	case "manual":
		statusBadge = statusManualBadge.Render(" MANUAL ")
		statusIcon = lipgloss.NewStyle().Foreground(ElecBlue).Render(IconInfo)
	default:
		statusBadge = statusNotSetBadge.Render(" NOT SET ")
		statusIcon = lipgloss.NewStyle().Foreground(TextDim).Render(IconOffline)
	}

	// Build card content
	var content strings.Builder

	// Header line with icon, name, and status
	var nameStyle lipgloss.Style
	var iconStyle lipgloss.Style
	if selected {
		nameStyle = lipgloss.NewStyle().Foreground(Cyan).Bold(true)
		iconStyle = lipgloss.NewStyle().Foreground(Magenta).Bold(true)
	} else {
		nameStyle = lipgloss.NewStyle().Foreground(TextBright)
		iconStyle = lipgloss.NewStyle().Foreground(Violet)
	}

	expandIcon := lipgloss.NewStyle().Foreground(Violet).Render("  ")
	if expanded {
		expandIcon = lipgloss.NewStyle().Foreground(Magenta).Bold(true).Render(IconArrowDown + " ")
	} else if selected {
		expandIcon = lipgloss.NewStyle().Foreground(Cyan).Render(IconChevron + " ")
	}

	headerLine := fmt.Sprintf("%s%s%s  %s  %s",
		expandIcon,
		iconStyle.Render(icon),
		nameStyle.Render(integration.Name),
		statusIcon,
		statusBadge,
	)
	content.WriteString(headerLine)

	// Description line
	content.WriteString("\n")
	descStyle := lipgloss.NewStyle().Foreground(TextMuted)
	content.WriteString("     " + descStyle.Render(integration.Description))

	// Expanded content
	if expanded {
		content.WriteString("\n\n")

		// Config path with styling
		configLabel := lipgloss.NewStyle().Foreground(Violet).Render("Config: ")
		configValue := configPathStyle.Render(integration.ConfigPath)
		content.WriteString("     " + configLabel + configValue)

		content.WriteString("\n")

		// Setup instructions box
		setupContent := m.renderSetupInstructions(integration.Setup)
		content.WriteString(setupContent)
	}

	// Apply card style based on state
	var cardStyle lipgloss.Style
	if expanded {
		cardStyle = integrationCardExpandedStyle.Width(cardWidth)
	} else if selected {
		cardStyle = integrationCardSelectedStyle.Width(cardWidth)
	} else {
		cardStyle = integrationCardStyle.Width(cardWidth)
	}

	return cardStyle.Render(content.String())
}

// renderSetupInstructions renders the setup instructions with cyber styling
func (m IntegrationsModel) renderSetupInstructions(setup []string) string {
	boxWidth := min(m.width-14, 58)

	var content strings.Builder

	// Header with neon styling
	headerDecor := lipgloss.NewStyle().Foreground(Cyan).Render("//")
	headerText := setupHeaderStyle.Render(" SETUP INSTRUCTIONS ")
	content.WriteString(headerDecor + headerText + headerDecor)
	content.WriteString("\n")

	// Divider line
	divider := lipgloss.NewStyle().Foreground(BorderDim).Render(strings.Repeat("─", boxWidth-4))
	content.WriteString(divider)
	content.WriteString("\n\n")

	// Process each line
	inCodeBlock := false
	for _, line := range setup {
		if line == "" {
			content.WriteString("\n")
			continue
		}

		// Check if it's a step instruction (starts with number)
		if len(line) > 2 && line[0] >= '1' && line[0] <= '9' && line[1] == '.' {
			stepNum := stepNumberStyle.Render(string(line[0:2]))
			stepText := instructionStyle.Render(line[2:])
			content.WriteString(stepNum + stepText)
			content.WriteString("\n")
			continue
		}

		// Check if it's the header instruction
		if strings.HasPrefix(line, "Add to") || strings.HasPrefix(line, "In ") {
			content.WriteString(instructionStyle.Render(line))
			content.WriteString("\n\n")
			inCodeBlock = true
			continue
		}

		// Code-like content (JSON, TOML, etc.)
		if inCodeBlock || strings.HasPrefix(line, "{") || strings.HasPrefix(line, "}") ||
			strings.HasPrefix(line, "[") || strings.HasPrefix(line, "]") ||
			strings.HasPrefix(line, `"`) || strings.Contains(line, "=") ||
			strings.HasPrefix(line, "  ") {

			// Style the code line
			styledLine := m.styleCodeLine(line)
			content.WriteString(styledLine)
			content.WriteString("\n")
		} else {
			content.WriteString(instructionStyle.Render(line))
			content.WriteString("\n")
		}
	}

	return setupBoxStyle.Width(boxWidth).Render(content.String())
}

// styleCodeLine applies syntax highlighting to a code line
func (m IntegrationsModel) styleCodeLine(line string) string {
	// Bracket and brace styling
	if line == "{" || line == "}" || line == "[" || line == "]" ||
		strings.HasSuffix(line, "{") || strings.HasSuffix(line, "},") ||
		strings.HasSuffix(line, "}") || strings.HasSuffix(line, "],") ||
		line == "  }]" || line == "  }]," {
		return lipgloss.NewStyle().Foreground(Violet).Render(line)
	}

	// Key-value pairs
	if strings.Contains(line, ":") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			// Style the key (usually in quotes)
			keyStyle := lipgloss.NewStyle().Foreground(Cyan)
			valueStyle := lipgloss.NewStyle().Foreground(NeonGreen)
			colonStyle := lipgloss.NewStyle().Foreground(TextMuted)

			return keyStyle.Render(parts[0]) + colonStyle.Render(":") + valueStyle.Render(parts[1])
		}
	}

	// TOML sections
	if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
		return lipgloss.NewStyle().Foreground(Magenta).Bold(true).Render(line)
	}

	// TOML key-value
	if strings.Contains(line, " = ") {
		parts := strings.SplitN(line, " = ", 2)
		if len(parts) == 2 {
			keyStyle := lipgloss.NewStyle().Foreground(Cyan)
			valueStyle := lipgloss.NewStyle().Foreground(NeonGreen)
			eqStyle := lipgloss.NewStyle().Foreground(TextMuted)

			return keyStyle.Render(parts[0]) + eqStyle.Render(" = ") + valueStyle.Render(parts[1])
		}
	}

	// Default code styling
	return lipgloss.NewStyle().Foreground(NeonGreen).Render(line)
}

// renderHelp renders the help footer with cyber styling
func (m IntegrationsModel) renderHelp() string {
	divider := lipgloss.NewStyle().Foreground(BorderDim).Render(strings.Repeat("─", min(m.width-4, 70)))

	helpItems := []struct {
		key  string
		desc string
	}{
		{"  /  ", "Navigate"},
		{"Enter", "Expand/Collapse"},
		{"Esc", "Back"},
	}

	var helpParts []string
	for _, item := range helpItems {
		key := lipgloss.NewStyle().
			Foreground(Cyan).
			Background(Surface).
			Bold(true).
			Padding(0, 1).
			Render(item.key)
		desc := lipgloss.NewStyle().Foreground(TextDim).Render(" " + item.desc)
		helpParts = append(helpParts, key+desc)
	}

	helpLine := strings.Join(helpParts, "   ")

	return "\n" + divider + "\n" + helpLine
}
