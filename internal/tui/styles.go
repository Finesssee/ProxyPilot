package tui

import (
	"fmt"

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
)

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
