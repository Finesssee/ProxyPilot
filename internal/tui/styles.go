package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ═══════════════════════════════════════════════════════════════════════════════
// PROXYPILOT THEME - Catppuccin Mocha
// Premium dark theme with HIGH CONTRAST visibility
// ═══════════════════════════════════════════════════════════════════════════════

var (
	// ─────────────────────────────────────────────────────────────────────────
	// CATPPUCCIN MOCHA - Premium dark theme with HIGH CONTRAST
	// ─────────────────────────────────────────────────────────────────────────

	// Backgrounds - 30+ hex levels apart for VISIBLE contrast
	BgDark      = lipgloss.Color("#1E1E2E") // Base - Catppuccin base
	BgPanel     = lipgloss.Color("#181825") // Mantle - darker panels
	BgSurface   = lipgloss.Color("#11111B") // Crust - darkest
	BgSelected  = lipgloss.Color("#313244") // Surface0 - selection
	BgHighlight = lipgloss.Color("#45475A") // Surface1 - hover

	// Borders - MUST BE VISIBLE (bright enough to see!)
	Border       = lipgloss.Color("#585B70") // Surface2 - VISIBLE!
	BorderDim    = lipgloss.Color("#45475A") // Surface1
	BorderBright = lipgloss.Color("#6C7086") // Overlay0
	BorderAccent = lipgloss.Color("#89B4FA") // Blue - focus accent

	// Text - clear hierarchy
	Text      = lipgloss.Color("#CDD6F4") // Text
	TextDim   = lipgloss.Color("#A6ADC8") // Subtext1
	TextMuted = lipgloss.Color("#6C7086") // Overlay0

	// Accents - VIBRANT
	Accent       = lipgloss.Color("#89B4FA") // Blue - primary
	AccentBright = lipgloss.Color("#B4BEFE") // Lavender - bright
	AccentDim    = lipgloss.Color("#74C7EC") // Sapphire - dim

	Secondary    = lipgloss.Color("#CBA6F7") // Mauve - secondary
	SecondaryDim = lipgloss.Color("#F5C2E7") // Pink

	Tertiary = lipgloss.Color("#94E2D5") // Teal

	// Status colors
	Green  = lipgloss.Color("#A6E3A1") // Green
	Red    = lipgloss.Color("#F38BA8") // Red
	Yellow = lipgloss.Color("#F9E2AF") // Yellow
	Blue   = lipgloss.Color("#89B4FA") // Blue

	// Neon compatibility aliases
	Cyan      = lipgloss.Color("#89DCEB") // Sky
	Magenta   = lipgloss.Color("#CBA6F7") // Mauve
	Violet    = lipgloss.Color("#B4BEFE") // Lavender
	NeonGreen = lipgloss.Color("#A6E3A1") // Green
	HotCoral  = lipgloss.Color("#F38BA8") // Red
	Amber     = lipgloss.Color("#FAB387") // Peach
	ElecBlue  = lipgloss.Color("#89B4FA") // Blue
	NeonPink  = lipgloss.Color("#F5C2E7") // Pink

	// Legacy mappings
	DeepBlack   = BgSurface
	DarkSurface = BgPanel
	Surface     = BgPanel
	BorderMid   = Border
	TextBright  = Text

	// Provider brand colors
	ClaudeBrand      = lipgloss.Color("#D4A27F") // Warm terracotta
	CodexBrand       = lipgloss.Color("#74AA9C") // OpenAI teal
	GeminiBrand      = lipgloss.Color("#8AB4F8") // Google blue
	KiroBrand        = lipgloss.Color("#FF6B6B") // Coral red
	QwenBrand        = lipgloss.Color("#7C3AED") // Purple
	AntigravityBrand = lipgloss.Color("#F59E0B") // Amber
	MiniMaxBrand     = lipgloss.Color("#EC4899") // Pink
	ZhipuBrand       = lipgloss.Color("#06B6D4") // Teal

	// Legacy color mappings
	PrimaryColor   = Accent
	SecondaryColor = Secondary
	AccentColor    = Tertiary
	SuccessColor   = Green
	ErrorColor     = Red
	WarningColor   = Yellow
	InfoColor      = Blue
	TextColor      = Text
	MutedColor     = TextMuted
	DimColor       = TextDim
	BgColor        = BgDark
	SurfaceColor   = BgPanel
	BorderColor    = Border
	HighlightBg    = BgSelected

	// Gradient shades for animations and progress bars
	GreenMid  = lipgloss.Color("#8BD48B") // Mid green for gradients
	GreenDark = lipgloss.Color("#76C776") // Dark green for gradients
)

// ═══════════════════════════════════════════════════════════════════════════════
// BORDERS - Clean, minimal styles
// ═══════════════════════════════════════════════════════════════════════════════

var (
	// Standard rounded border
	RoundedBorder = lipgloss.RoundedBorder()

	// Simple line border
	SimpleBorder = lipgloss.NormalBorder()

	// Thick border for emphasis
	ThickBorder = lipgloss.ThickBorder()

	// Compatibility
	SoftBorder  = RoundedBorder
	NeonBorder  = lipgloss.DoubleBorder()
	CyberBorder = ThickBorder
)

// ═══════════════════════════════════════════════════════════════════════════════
// BASE STYLES - Reusable building blocks
// ═══════════════════════════════════════════════════════════════════════════════

var (
	// Title styles
	TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(Text)

	SubtitleStyle = lipgloss.NewStyle().
		Foreground(TextDim)

	// Panel/container styles
	PanelStyle = lipgloss.NewStyle().
		Border(RoundedBorder).
		BorderForeground(Border). // #585B70 - VISIBLE!
		Padding(0, 1)

	// Selected/active styles
	SelectedStyle = lipgloss.NewStyle().
		Background(BgSelected).
		Foreground(AccentBright).
		Bold(true)

	// Status badges
	SuccessBadge = lipgloss.NewStyle().
		Foreground(Green).
		Bold(true)

	ErrorBadge = lipgloss.NewStyle().
		Foreground(Red).
		Bold(true)

	WarningBadge = lipgloss.NewStyle().
		Foreground(Yellow).
		Bold(true)

	InfoBadge = lipgloss.NewStyle().
		Foreground(Blue).
		Bold(true)

	// Enhanced status badges with backgrounds
	// Success badge with filled background
	SuccessBadgeFilled = lipgloss.NewStyle().
		Foreground(BgDark).
		Background(Green).
		Bold(true).
		Padding(0, 1)

	// Error badge with filled background
	ErrorBadgeFilled = lipgloss.NewStyle().
		Foreground(BgDark).
		Background(Red).
		Bold(true).
		Padding(0, 1)

	// Warning badge with filled background
	WarningBadgeFilled = lipgloss.NewStyle().
		Foreground(BgDark).
		Background(Yellow).
		Bold(true).
		Padding(0, 1)

	// Info badge with filled background
	InfoBadgeFilled = lipgloss.NewStyle().
		Foreground(BgDark).
		Background(Blue).
		Bold(true).
		Padding(0, 1)

	// Connected status badge
	ConnectedBadge = lipgloss.NewStyle().
		Foreground(BgDark).
		Background(Green).
		Bold(true).
		Padding(0, 1)

	// Disconnected status badge
	DisconnectedBadge = lipgloss.NewStyle().
		Foreground(Text).
		Background(BgSelected).
		Padding(0, 1)

	// Navigation card style for selected items
	NavCardSelected = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Accent).
		Padding(0, 1)

	// Section title underline style
	SectionUnderlineStyle = lipgloss.NewStyle().
		Foreground(Accent)

	// Footer keyboard pill style
	KeyboardPillStyle = lipgloss.NewStyle().
		Foreground(BgDark).
		Background(BorderDim).
		Padding(0, 1)

	// Dotted separator style
	DottedSeparatorStyle = lipgloss.NewStyle().
		Foreground(BorderDim)

	// Help bar styles
	HelpStyle = lipgloss.NewStyle().
		Foreground(TextMuted)

	HelpKeyStyle = lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
		Foreground(TextMuted)

	// Input styles
	InputStyle = lipgloss.NewStyle().
		Border(SimpleBorder).
		BorderForeground(Border).
		Padding(0, 1)

	FocusedInputStyle = lipgloss.NewStyle().
		Border(SimpleBorder).
		BorderForeground(Accent).
		Padding(0, 1)

	// Button styles
	ButtonStyle = lipgloss.NewStyle().
		Foreground(Text).
		Background(BgPanel).
		Padding(0, 2)

	PrimaryButtonStyle = lipgloss.NewStyle().
		Foreground(BgDark).
		Background(Accent).
		Bold(true).
		Padding(0, 2)

	// Status bar
	StatusBarStyle = lipgloss.NewStyle().
		Background(BgPanel).
		Foreground(Text).
		Padding(0, 1)

	// Spinner
	SpinnerStyle = lipgloss.NewStyle().
		Foreground(Accent)

	// Table styles
	TableHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(Accent).
		BorderStyle(SimpleBorder).
		BorderBottom(true).
		BorderForeground(Border)

	TableCellStyle = lipgloss.NewStyle().
		Foreground(Text)

	TableSelectedRowStyle = lipgloss.NewStyle().
		Background(BgSelected).
		Foreground(AccentBright).
		Bold(true)
)

// ═══════════════════════════════════════════════════════════════════════════════
// MENU STYLES - Navigation elements
// ═══════════════════════════════════════════════════════════════════════════════

var (
	MenuItemStyle = lipgloss.NewStyle().
		Foreground(TextDim).
		PaddingLeft(2)

	SelectedItemStyle = lipgloss.NewStyle().
		Background(BgSelected).
		Foreground(AccentBright).
		Bold(true).
		PaddingLeft(2)

	ActiveItemStyle = lipgloss.NewStyle().
		Foreground(Green).
		PaddingLeft(2)

	DisabledItemStyle = lipgloss.NewStyle().
		Foreground(TextMuted).
		PaddingLeft(2)

	CursorStyle = lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true)

	NavIndicator = lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true)
)

// ═══════════════════════════════════════════════════════════════════════════════
// ADDITIONAL STYLES - Compatibility
// ═══════════════════════════════════════════════════════════════════════════════

var (
	HeaderStyle  = TitleStyle
	SectionTitle = TitleStyle

	BoxStyle = PanelStyle
	FocusedBoxStyle = lipgloss.NewStyle().
		Border(RoundedBorder).
		BorderForeground(Accent). // #89B4FA - BRIGHT BLUE!
		Padding(0, 1)

	CardStyle = PanelStyle

	GlowBox = FocusedBoxStyle
	GlowBoxMagenta = lipgloss.NewStyle().
		Border(RoundedBorder).
		BorderForeground(Secondary).
		Padding(0, 1)
	GlowBoxSuccess = lipgloss.NewStyle().
		Border(RoundedBorder).
		BorderForeground(Green).
		Padding(0, 1)
	GlowBoxError = lipgloss.NewStyle().
		Border(RoundedBorder).
		BorderForeground(Red).
		Padding(0, 1)

	TitleGlow = TitleStyle

	MagentaBadge = lipgloss.NewStyle().
		Foreground(Secondary).
		Bold(true)

	PrimaryBadge = lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true)

	MutedBadge = lipgloss.NewStyle().
		Foreground(TextMuted)

	OnlineBadge  = SuccessBadge
	OfflineBadge = ErrorBadge

	InputLabelStyle  = SubtitleStyle
	PlaceholderStyle = lipgloss.NewStyle().Foreground(TextMuted)

	DangerButtonStyle = lipgloss.NewStyle().
		Foreground(Text).
		Background(Red).
		Bold(true).
		Padding(0, 2)

	DisabledButtonStyle = lipgloss.NewStyle().
		Foreground(TextMuted).
		Background(BgPanel).
		Padding(0, 2)

	StatusTextStyle = SubtitleStyle
	FooterStyle     = HelpStyle

	ProgressBarStyle   = lipgloss.NewStyle().Foreground(Green)
	ProgressTrackStyle = lipgloss.NewStyle().Foreground(BorderDim)

	HintStyle = SubtitleStyle
	KeyHint   = HelpKeyStyle

	LogoStyle    = TitleStyle
	LogoAccent   = lipgloss.NewStyle().Foreground(Accent).Bold(true)
	VersionStyle = SubtitleStyle

	// ─────────────────────────────────────────────────────────────────────────
	// Tab styles for tabbed interfaces
	// ─────────────────────────────────────────────────────────────────────────

	// Active tab with accent background
	TabActiveStyle = lipgloss.NewStyle().
			Foreground(BgDark).
			Background(Accent).
			Bold(true).
			Padding(0, 1)

	// Inactive tab with muted styling
	TabInactiveStyle = lipgloss.NewStyle().
				Foreground(TextMuted).
				Background(BgPanel).
				Padding(0, 1)

	// Tab container for grouping tabs
	TabContainerStyle = lipgloss.NewStyle().
				MarginBottom(1)

	// ─────────────────────────────────────────────────────────────────────────
	// Tooltip and popover styles
	// ─────────────────────────────────────────────────────────────────────────

	// Tooltip container
	TooltipStyle = lipgloss.NewStyle().
			Border(RoundedBorder).
			BorderForeground(BorderBright).
			Background(BgPanel).
			Foreground(Text).
			Padding(0, 1)

	// Tooltip title
	TooltipTitleStyle = lipgloss.NewStyle().
				Foreground(Accent).
				Bold(true)

	// ─────────────────────────────────────────────────────────────────────────
	// Loading and skeleton state styles
	// ─────────────────────────────────────────────────────────────────────────

	// Loading spinner container
	LoadingContainerStyle = lipgloss.NewStyle().
				Foreground(Accent)

	// Loading text
	LoadingTextStyle = lipgloss.NewStyle().
				Foreground(TextMuted).
				Italic(true)

	// Skeleton placeholder for loading content
	SkeletonStyle = lipgloss.NewStyle().
			Foreground(BorderDim).
			Background(BgPanel)

	// ─────────────────────────────────────────────────────────────────────────
	// Additional button variants
	// ─────────────────────────────────────────────────────────────────────────

	// Secondary button (outline style)
	SecondaryButtonStyle = lipgloss.NewStyle().
				Foreground(Accent).
				Border(RoundedBorder).
				BorderForeground(Accent).
				Padding(0, 2)

	// Ghost button (minimal style)
	GhostButtonStyle = lipgloss.NewStyle().
				Foreground(TextDim).
				Padding(0, 2)

	// Success button
	SuccessButtonStyle = lipgloss.NewStyle().
				Foreground(BgDark).
				Background(Green).
				Bold(true).
				Padding(0, 2)

	// ─────────────────────────────────────────────────────────────────────────
	// Tag and chip styles
	// ─────────────────────────────────────────────────────────────────────────

	// Default tag/chip style
	TagStyle = lipgloss.NewStyle().
			Foreground(Text).
			Background(BgSelected).
			Padding(0, 1)

	// Accent tag
	TagAccentStyle = lipgloss.NewStyle().
			Foreground(BgDark).
			Background(Accent).
			Bold(true).
			Padding(0, 1)

	// ─────────────────────────────────────────────────────────────────────────
	// Divider styles
	// ─────────────────────────────────────────────────────────────────────────

	// Horizontal divider
	DividerStyle = lipgloss.NewStyle().
			Foreground(Border)

	// Thick divider for emphasis
	ThickDividerStyle = lipgloss.NewStyle().
				Foreground(Accent)
)

// ═══════════════════════════════════════════════════════════════════════════════
// NAVIGATION ICONS - Screen-specific icons
// ═══════════════════════════════════════════════════════════════════════════════

// Navigation icons - unselected state
var NavIcons = map[string]string{
	"dashboard": "◈",
	"server":    "▣",
	"providers": "◎",
	"agents":    "◬",
	"usage":     "◔",
	"logs":      "☰",
	"mappings":  "⇄",
	"setup":     "⚙",
}

// Navigation icons - selected state (filled variants)
var NavIconsSelected = map[string]string{
	"dashboard": "◆",
	"server":    "■",
	"providers": "●",
	"agents":    "▲",
	"usage":     "●",
	"logs":      "≡",
	"mappings":  "⇔",
	"setup":     "✦",
}

// Superscript digits for key hints
var SuperScriptMap = map[string]string{
	"1": "¹", "2": "²", "3": "³", "4": "⁴",
	"5": "⁵", "6": "⁶", "7": "⁷", "8": "⁸",
	"9": "⁹", "0": "⁰",
}

// GetNavIcon returns the navigation icon for a screen
func GetNavIcon(screen string, selected bool) string {
	screen = strings.ToLower(screen)
	if selected {
		if icon, ok := NavIconsSelected[screen]; ok {
			return icon
		}
	}
	if icon, ok := NavIcons[screen]; ok {
		return icon
	}
	return "◆"
}

// ═══════════════════════════════════════════════════════════════════════════════
// ICONS - Simple, clean icons
// ═══════════════════════════════════════════════════════════════════════════════

const (
	IconOnline  = "●"
	IconOffline = "○"
	IconPending = "◐"

	IconArrowRight = "→"
	IconArrowLeft  = "←"
	IconArrowUp    = "↑"
	IconArrowDown  = "↓"
	IconChevron    = "›"

	IconCheck    = "✓"
	IconCross    = "✗"
	IconWarning  = "!"
	IconInfo     = "i"
	IconStar     = "*"
	IconDiamond  = "◆"
	IconSquare   = "■"
	IconCircle   = "●"
	IconTriangle = "▲"

	IconBolt     = "⚡"
	IconFire     = "~"
	IconRocket   = ">"
	IconGear     = "*"
	IconLock     = "#"
	IconUnlock   = "#"
	IconKey      = "*"
	IconCloud    = "~"
	IconServer   = "■"
	IconTerminal = ">"
	IconCode     = "<>"
	IconPulse    = "~"

	// Trend indicators
	TrendUp   = "▲"
	TrendDown = "▼"
	TrendFlat = "▬"

	// Status pulse frames
	PulseFrame1 = "●"
	PulseFrame2 = "◉"
	PulseFrame3 = "○"

	// Line drawing
	LineHeavy  = "━"
	LineDashed = "┄"
	LineDouble = "═"

	// Progress bar
	ProgressFilled = "━"
	ProgressEmpty  = "─"
	ProgressHead   = "●"

	// Bar chart
	BarFull  = "█"
	BarEmpty = "░"
)

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

func Bold(text string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(Text).Render(text)
}

func Dim(text string) string {
	return lipgloss.NewStyle().Foreground(TextMuted).Render(text)
}

func Success(text string) string {
	return lipgloss.NewStyle().Foreground(Green).Render(text)
}

func Error(text string) string {
	return lipgloss.NewStyle().Foreground(Red).Render(text)
}

func Warning(text string) string {
	return lipgloss.NewStyle().Foreground(Yellow).Render(text)
}

func Info(text string) string {
	return lipgloss.NewStyle().Foreground(Blue).Render(text)
}

func Primary(text string) string {
	return lipgloss.NewStyle().Foreground(Accent).Render(text)
}

func Accent2(text string) string {
	return lipgloss.NewStyle().Foreground(Secondary).Render(text)
}

func Muted(text string) string {
	return lipgloss.NewStyle().Foreground(TextMuted).Render(text)
}

func Colorize(text string, color lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(color).Render(text)
}

func Glow(text string, color lipgloss.Color) string {
	return lipgloss.NewStyle().Bold(true).Foreground(color).Render(text)
}

func WithWidth(style lipgloss.Style, width int) lipgloss.Style {
	return style.Width(width)
}

func WithHeight(style lipgloss.Style, height int) lipgloss.Style {
	return style.Height(height)
}

func Centered(text string, width int) string {
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(text)
}

func RightAligned(text string, width int) string {
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Right).Render(text)
}

// Gradient functions (simplified - no actual gradient, just color)
func GradientText(text string, colors []lipgloss.Color) string {
	if len(colors) == 0 {
		return text
	}
	return lipgloss.NewStyle().Foreground(colors[0]).Render(text)
}

func CyanGradient(text string) string {
	return lipgloss.NewStyle().Foreground(Accent).Render(text)
}

func MagentaGradient(text string) string {
	return lipgloss.NewStyle().Foreground(Secondary).Render(text)
}

// LetterSpace adds spacing between characters
func LetterSpace(text string) string {
	var result strings.Builder
	runes := []rune(text)
	for i, r := range runes {
		result.WriteRune(r)
		if i < len(runes)-1 {
			result.WriteString(" ")
		}
	}
	return result.String()
}

// FormatCount formats numbers with K/M suffix
func FormatCount(count int64) string {
	if count >= 1000000 {
		return lipgloss.NewStyle().Foreground(Accent).Bold(true).Render(
			fmt.Sprintf("%.1fM", float64(count)/1000000),
		)
	}
	if count >= 1000 {
		return lipgloss.NewStyle().Foreground(Accent).Bold(true).Render(
			fmt.Sprintf("%.1fK", float64(count)/1000),
		)
	}
	return lipgloss.NewStyle().Foreground(Accent).Bold(true).Render(
		fmt.Sprintf("%d", count),
	)
}

// RenderKeyboardPill creates a styled keyboard shortcut pill
func RenderKeyboardPill(key string) string {
	return KeyboardPillStyle.Render(key)
}

// RenderDottedDivider creates a dotted line divider
func RenderDottedDivider(width int) string {
	return DottedSeparatorStyle.Render(strings.Repeat("┄", width))
}

// RenderSectionTitle creates a section title with ═══ underline
func RenderSectionTitle(title string) string {
	titleText := lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true).
		Render(title)
	underlineLen := len(title)
	if underlineLen < 4 {
		underlineLen = 4
	}
	underline := lipgloss.NewStyle().
		Foreground(Accent).
		Render(strings.Repeat("═", underlineLen))
	return titleText + "\n" + underline
}

// RenderProgressBar creates a progress bar: ━━━━●───
func RenderProgressBar(percent float64, width int) string {
	if width <= 0 {
		width = 10
	}
	filled := int(percent * float64(width-1))
	if filled < 0 {
		filled = 0
	}
	if filled >= width {
		filled = width - 1
	}

	filledStyle := lipgloss.NewStyle().Foreground(Accent)
	emptyStyle := lipgloss.NewStyle().Foreground(BorderDim)

	before := filledStyle.Render(strings.Repeat("━", filled))
	head := filledStyle.Render("●")
	after := emptyStyle.Render(strings.Repeat("─", width-filled-1))

	return before + head + after
}

// RenderIndeterminateProgress renders an indeterminate loading animation
func RenderIndeterminateProgress(width int, frame int) string {
	if width < 5 {
		width = 10
	}
	// Create a bouncing effect
	position := frame % (width * 2)
	if position >= width {
		position = (width * 2) - position - 1
	}

	var result strings.Builder
	for i := 0; i < width; i++ {
		if i >= position && i < position+3 {
			result.WriteString(ProgressBarStyle.Render("█"))
		} else {
			result.WriteString(ProgressTrackStyle.Render("░"))
		}
	}
	return result.String()
}

// RenderConnectionStrength creates ▁▃▅▇ indicator
func RenderConnectionStrength(level int) string {
	bars := []string{"▁", "▃", "▅", "▇"}
	var result strings.Builder

	for i, bar := range bars {
		if i < level {
			result.WriteString(lipgloss.NewStyle().Foreground(Green).Render(bar))
		} else {
			result.WriteString(lipgloss.NewStyle().Foreground(BorderDim).Render(bar))
		}
	}
	return result.String()
}

// ═══════════════════════════════════════════════════════════════════════════════
// ADDITIONAL RENDER HELPERS - Common UI patterns
// ═══════════════════════════════════════════════════════════════════════════════

// RenderDivider creates a horizontal divider line
func RenderDivider(width int) string {
	return DividerStyle.Render(strings.Repeat("─", width))
}

// RenderThickDivider creates a thick horizontal divider line
func RenderThickDivider(width int) string {
	return ThickDividerStyle.Render(strings.Repeat("━", width))
}

// RenderStatusIndicator creates a status indicator with icon and text
func RenderStatusIndicator(online bool) string {
	if online {
		return SuccessBadge.Render(IconOnline + " ONLINE")
	}
	return ErrorBadge.Render(IconOffline + " OFFLINE")
}

// RenderStatusDot creates a simple status dot indicator
func RenderStatusDot(online bool) string {
	if online {
		return lipgloss.NewStyle().Foreground(Green).Bold(true).Render("●")
	}
	return lipgloss.NewStyle().Foreground(Red).Bold(true).Render("○")
}

// RenderTab creates a tab with appropriate styling based on active state
func RenderTab(label string, active bool) string {
	if active {
		return TabActiveStyle.Render(label)
	}
	return TabInactiveStyle.Render(label)
}

// RenderTabs creates a row of tabs
func RenderTabs(tabs []string, activeIndex int) string {
	var rendered []string
	for i, tab := range tabs {
		rendered = append(rendered, RenderTab(tab, i == activeIndex))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

// RenderLoadingText creates styled loading text with spinner placeholder
func RenderLoadingText(message string) string {
	spinner := LoadingContainerStyle.Render("◐")
	text := LoadingTextStyle.Render(message)
	return spinner + " " + text
}

// RenderSkeletonLine creates a skeleton placeholder line
func RenderSkeletonLine(width int) string {
	return SkeletonStyle.Render(strings.Repeat("░", width))
}

// RenderBarChart creates a simple bar chart with filled/empty chars
func RenderBarChart(value, max int64, width int) string {
	if max <= 0 || width <= 0 {
		return ""
	}
	filled := int(float64(value) / float64(max) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	filledBar := lipgloss.NewStyle().Foreground(Accent).Render(strings.Repeat(BarFull, filled))
	emptyBar := lipgloss.NewStyle().Foreground(BorderDim).Render(strings.Repeat(BarEmpty, width-filled))
	return filledBar + emptyBar
}

// RenderPercentage formats a percentage with color based on threshold
func RenderPercentage(value float64) string {
	var color lipgloss.Color
	switch {
	case value >= 90:
		color = Green
	case value >= 70:
		color = Yellow
	case value >= 50:
		color = Amber
	default:
		color = Red
	}
	return lipgloss.NewStyle().Foreground(color).Bold(true).Render(fmt.Sprintf("%.1f%%", value))
}

// RenderTrend creates a trend indicator (up, down, flat)
func RenderTrend(delta float64) string {
	if delta > 0 {
		return lipgloss.NewStyle().Foreground(Green).Bold(true).Render(TrendUp)
	} else if delta < 0 {
		return lipgloss.NewStyle().Foreground(Red).Bold(true).Render(TrendDown)
	}
	return lipgloss.NewStyle().Foreground(TextMuted).Render(TrendFlat)
}

// RenderKeyValue creates a key: value pair with consistent styling
func RenderKeyValue(key, value string) string {
	keyStyled := lipgloss.NewStyle().Foreground(TextMuted).Render(key + ":")
	valueStyled := lipgloss.NewStyle().Foreground(Text).Bold(true).Render(value)
	return keyStyled + " " + valueStyled
}

// RenderTag creates a styled tag/chip
func RenderTag(text string, accent bool) string {
	if accent {
		return TagAccentStyle.Render(text)
	}
	return TagStyle.Render(text)
}
