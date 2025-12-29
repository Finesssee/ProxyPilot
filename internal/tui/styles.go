package tui

import "github.com/charmbracelet/lipgloss"

// Color palette - Clean and professional color scheme
var (
	// Primary colors
	PrimaryColor   = lipgloss.Color("#00D4FF") // Cyan
	SecondaryColor = lipgloss.Color("#6C757D") // Gray
	AccentColor    = lipgloss.Color("#7C3AED") // Purple accent

	// Status colors
	SuccessColor = lipgloss.Color("#10B981") // Green
	ErrorColor   = lipgloss.Color("#EF4444") // Red
	WarningColor = lipgloss.Color("#F59E0B") // Yellow/Amber
	InfoColor    = lipgloss.Color("#3B82F6") // Blue

	// Neutral colors
	TextColor     = lipgloss.Color("#E5E7EB") // Light gray text
	MutedColor    = lipgloss.Color("#9CA3AF") // Muted text
	DimColor      = lipgloss.Color("#6B7280") // Dim text
	BgColor       = lipgloss.Color("#1F2937") // Dark background
	SurfaceColor  = lipgloss.Color("#374151") // Surface/card background
	BorderColor   = lipgloss.Color("#4B5563") // Border gray
	HighlightBg   = lipgloss.Color("#2D3748") // Highlight background
)

// Title styles
var (
	// TitleStyle is the main application title style
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor).
			MarginBottom(1)

	// SubtitleStyle for secondary headings
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			MarginBottom(1)

	// HeaderStyle for section headers
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(TextColor).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(BorderColor).
			PaddingBottom(1).
			MarginBottom(1)
)

// Menu and list item styles
var (
	// MenuItemStyle for normal menu items
	MenuItemStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			PaddingLeft(2)

	// SelectedItemStyle for currently selected/focused items
	SelectedItemStyle = lipgloss.NewStyle().
				Foreground(PrimaryColor).
				Bold(true).
				PaddingLeft(2).
				Background(HighlightBg)

	// ActiveItemStyle for active/enabled items
	ActiveItemStyle = lipgloss.NewStyle().
			Foreground(SuccessColor).
			PaddingLeft(2)

	// DisabledItemStyle for disabled/inactive items
	DisabledItemStyle = lipgloss.NewStyle().
				Foreground(DimColor).
				PaddingLeft(2)

	// CursorStyle for the selection cursor
	CursorStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true)
)

// Status badge styles
var (
	// SuccessBadge for success status indicators
	SuccessBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(SuccessColor).
			Bold(true).
			Padding(0, 1)

	// ErrorBadge for error status indicators
	ErrorBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(ErrorColor).
			Bold(true).
			Padding(0, 1)

	// WarningBadge for warning status indicators
	WarningBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(WarningColor).
			Bold(true).
			Padding(0, 1)

	// InfoBadge for informational status indicators
	InfoBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(InfoColor).
			Bold(true).
			Padding(0, 1)

	// PrimaryBadge for primary action indicators
	PrimaryBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(PrimaryColor).
			Bold(true).
			Padding(0, 1)

	// MutedBadge for neutral/inactive indicators
	MutedBadge = lipgloss.NewStyle().
			Foreground(TextColor).
			Background(SurfaceColor).
			Padding(0, 1)
)

// Border styles
var (
	// RoundedBorder is a standard rounded border style
	RoundedBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(1, 2)

	// ThickBorder is a thicker border for emphasis
	ThickBorder = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(PrimaryColor).
			Padding(1, 2)

	// DoubleBorder for important sections
	DoubleBorder = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(PrimaryColor).
			Padding(1, 2)

	// NormalBorder for standard sections
	NormalBorder = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(BorderColor).
			Padding(1, 2)

	// HiddenBorder maintains spacing without visible border
	HiddenBorder = lipgloss.NewStyle().
			Border(lipgloss.HiddenBorder()).
			Padding(1, 2)
)

// Box and container styles
var (
	// BoxStyle is a general purpose container
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(1, 2).
			MarginBottom(1)

	// FocusedBoxStyle for focused/active containers
	FocusedBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			Padding(1, 2).
			MarginBottom(1)

	// CardStyle for card-like containers
	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Background(SurfaceColor).
			Padding(1, 2).
			MarginBottom(1)
)

// Input styles
var (
	// InputStyle for text input fields
	InputStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(BorderColor).
			Padding(0, 1)

	// FocusedInputStyle for focused text input fields
	FocusedInputStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(PrimaryColor).
				Padding(0, 1)

	// InputLabelStyle for input labels
	InputLabelStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			MarginBottom(1)

	// PlaceholderStyle for placeholder text
	PlaceholderStyle = lipgloss.NewStyle().
				Foreground(DimColor)
)

// Button styles
var (
	// ButtonStyle for standard buttons
	ButtonStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			Background(SurfaceColor).
			Padding(0, 2).
			MarginRight(1)

	// PrimaryButtonStyle for primary action buttons
	PrimaryButtonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000000")).
				Background(PrimaryColor).
				Bold(true).
				Padding(0, 2).
				MarginRight(1)

	// DangerButtonStyle for destructive action buttons
	DangerButtonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(ErrorColor).
				Bold(true).
				Padding(0, 2).
				MarginRight(1)

	// DisabledButtonStyle for disabled buttons
	DisabledButtonStyle = lipgloss.NewStyle().
				Foreground(DimColor).
				Background(SurfaceColor).
				Padding(0, 2).
				MarginRight(1)
)

// Help and hint styles
var (
	// HelpStyle for help text
	HelpStyle = lipgloss.NewStyle().
			Foreground(DimColor).
			MarginTop(1)

	// HelpKeyStyle for keyboard shortcut keys
	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Bold(true)

	// HelpDescStyle for help descriptions
	HelpDescStyle = lipgloss.NewStyle().
			Foreground(DimColor)

	// HintStyle for inline hints
	HintStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Italic(true)
)

// Status line and footer styles
var (
	// StatusBarStyle for the bottom status bar
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			Background(SurfaceColor).
			Padding(0, 1)

	// StatusTextStyle for status bar text
	StatusTextStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	// FooterStyle for footer content
	FooterStyle = lipgloss.NewStyle().
			Foreground(DimColor).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(BorderColor).
			PaddingTop(1).
			MarginTop(1)
)

// Spinner and progress styles
var (
	// SpinnerStyle for loading spinners
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor)

	// ProgressBarStyle for progress indicators
	ProgressBarStyle = lipgloss.NewStyle().
				Foreground(PrimaryColor)

	// ProgressTrackStyle for progress bar track
	ProgressTrackStyle = lipgloss.NewStyle().
				Foreground(SurfaceColor)
)

// Table styles
var (
	// TableHeaderStyle for table headers
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(TextColor).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(BorderColor).
				PaddingRight(2)

	// TableCellStyle for table cells
	TableCellStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			PaddingRight(2)

	// TableSelectedRowStyle for selected table rows
	TableSelectedRowStyle = lipgloss.NewStyle().
				Foreground(PrimaryColor).
				Background(HighlightBg).
				Bold(true)
)

// Logo and branding
var (
	// LogoStyle for the application logo/brand
	LogoStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true)

	// VersionStyle for version information
	VersionStyle = lipgloss.NewStyle().
			Foreground(DimColor)
)

// Helper functions for dynamic styling

// Colorize returns text with the specified color
func Colorize(text string, color lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(color).Render(text)
}

// Bold returns bold text
func Bold(text string) string {
	return lipgloss.NewStyle().Bold(true).Render(text)
}

// Dim returns dimmed text
func Dim(text string) string {
	return lipgloss.NewStyle().Foreground(DimColor).Render(text)
}

// Success returns success-colored text
func Success(text string) string {
	return lipgloss.NewStyle().Foreground(SuccessColor).Render(text)
}

// Error returns error-colored text
func Error(text string) string {
	return lipgloss.NewStyle().Foreground(ErrorColor).Render(text)
}

// Warning returns warning-colored text
func Warning(text string) string {
	return lipgloss.NewStyle().Foreground(WarningColor).Render(text)
}

// Info returns info-colored text
func Info(text string) string {
	return lipgloss.NewStyle().Foreground(InfoColor).Render(text)
}

// Primary returns primary-colored text
func Primary(text string) string {
	return lipgloss.NewStyle().Foreground(PrimaryColor).Render(text)
}

// Muted returns muted text
func Muted(text string) string {
	return lipgloss.NewStyle().Foreground(MutedColor).Render(text)
}

// WithWidth returns a style with the specified width
func WithWidth(style lipgloss.Style, width int) lipgloss.Style {
	return style.Width(width)
}

// WithHeight returns a style with the specified height
func WithHeight(style lipgloss.Style, height int) lipgloss.Style {
	return style.Height(height)
}

// Centered returns centered text within the given width
func Centered(text string, width int) string {
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(text)
}

// RightAligned returns right-aligned text within the given width
func RightAligned(text string, width int) string {
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Right).Render(text)
}
