package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

// KeyMap defines the key bindings for the TUI application
type KeyMap struct {
	// Navigation keys
	Up   key.Binding
	Down key.Binding

	// Selection and action keys
	Select key.Binding
	Back   key.Binding

	// Application control keys
	Quit    key.Binding
	Refresh key.Binding

	// Help toggle
	Help key.Binding
}

// DefaultKeyMap returns the default key bindings for the TUI
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("^/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("v/j", "down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "backspace"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}
}

// ShortHelp returns the short help text for the key bindings
// This implements the help.KeyMap interface
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Down,
		k.Select,
		k.Back,
		k.Quit,
	}
}

// FullHelp returns the full help text for all key bindings
// This implements the help.KeyMap interface
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Navigation column
		{k.Up, k.Down},
		// Action column
		{k.Select, k.Back, k.Refresh},
		// Application column
		{k.Help, k.Quit},
	}
}

// NavigationHelp returns help text specifically for navigation keys
func (k KeyMap) NavigationHelp() string {
	return formatBindingHelp(k.Up, k.Down)
}

// ActionHelp returns help text specifically for action keys
func (k KeyMap) ActionHelp() string {
	return formatBindingHelp(k.Select, k.Back, k.Refresh)
}

// GenerateHelpText generates a formatted help string from the key bindings
func (k KeyMap) GenerateHelpText() string {
	var parts []string

	parts = append(parts, formatSingleBinding(k.Up))
	parts = append(parts, formatSingleBinding(k.Down))
	parts = append(parts, formatSingleBinding(k.Select))
	parts = append(parts, formatSingleBinding(k.Back))
	parts = append(parts, formatSingleBinding(k.Refresh))
	parts = append(parts, formatSingleBinding(k.Help))
	parts = append(parts, formatSingleBinding(k.Quit))

	return strings.Join(parts, " | ")
}

// GenerateShortHelpText generates a compact help string for display in status bars
func (k KeyMap) GenerateShortHelpText() string {
	var parts []string

	parts = append(parts, formatCompactBinding(k.Up, k.Down, "navigate"))
	parts = append(parts, formatSingleBinding(k.Select))
	parts = append(parts, formatSingleBinding(k.Quit))

	return strings.Join(parts, " | ")
}

// GenerateMenuHelpText generates help text for menu screens
func (k KeyMap) GenerateMenuHelpText() string {
	var parts []string

	parts = append(parts, formatCompactBinding(k.Up, k.Down, "navigate"))
	parts = append(parts, formatSingleBinding(k.Select))
	parts = append(parts, formatSingleBinding(k.Quit))

	return strings.Join(parts, " | ")
}

// GenerateDetailHelpText generates help text for detail/sub screens
func (k KeyMap) GenerateDetailHelpText() string {
	var parts []string

	parts = append(parts, formatSingleBinding(k.Back))
	parts = append(parts, formatSingleBinding(k.Refresh))
	parts = append(parts, formatSingleBinding(k.Quit))

	return strings.Join(parts, " | ")
}

// formatBindingHelp formats multiple key bindings into a help string
func formatBindingHelp(bindings ...key.Binding) string {
	var parts []string
	for _, b := range bindings {
		if b.Enabled() {
			parts = append(parts, formatSingleBinding(b))
		}
	}
	return strings.Join(parts, " | ")
}

// formatSingleBinding formats a single key binding for help display
func formatSingleBinding(b key.Binding) string {
	if !b.Enabled() {
		return ""
	}
	help := b.Help()
	return HelpKeyStyle.Render(help.Key) + HelpDescStyle.Render(": "+help.Desc)
}

// formatCompactBinding combines two bindings with a shared description
func formatCompactBinding(b1, b2 key.Binding, desc string) string {
	h1 := b1.Help()
	h2 := b2.Help()
	return HelpKeyStyle.Render(h1.Key+"/"+h2.Key) + HelpDescStyle.Render(": "+desc)
}

// SetEnabled enables or disables all key bindings
func (k *KeyMap) SetEnabled(enabled bool) {
	k.Up.SetEnabled(enabled)
	k.Down.SetEnabled(enabled)
	k.Select.SetEnabled(enabled)
	k.Back.SetEnabled(enabled)
	k.Quit.SetEnabled(enabled)
	k.Refresh.SetEnabled(enabled)
	k.Help.SetEnabled(enabled)
}

// SetNavigationEnabled enables or disables navigation keys
func (k *KeyMap) SetNavigationEnabled(enabled bool) {
	k.Up.SetEnabled(enabled)
	k.Down.SetEnabled(enabled)
}

// SetBackEnabled enables or disables the back key
func (k *KeyMap) SetBackEnabled(enabled bool) {
	k.Back.SetEnabled(enabled)
}

// SetRefreshEnabled enables or disables the refresh key
func (k *KeyMap) SetRefreshEnabled(enabled bool) {
	k.Refresh.SetEnabled(enabled)
}

// Matches checks if a key message matches any of the given bindings
func Matches(msg interface{ String() string }, bindings ...key.Binding) bool {
	for _, b := range bindings {
		if key.Matches(msg.(interface {
			String() string
			Type() interface{}
			Runes() []rune
		}), b) {
			return true
		}
	}
	return false
}

// IsQuit checks if the key message is a quit command
func (k KeyMap) IsQuit(msg interface{ String() string }) bool {
	keyStr := msg.String()
	for _, key := range k.Quit.Keys() {
		if keyStr == key {
			return true
		}
	}
	return false
}

// IsBack checks if the key message is a back command
func (k KeyMap) IsBack(msg interface{ String() string }) bool {
	keyStr := msg.String()
	for _, key := range k.Back.Keys() {
		if keyStr == key {
			return true
		}
	}
	return false
}

// IsUp checks if the key message is an up navigation command
func (k KeyMap) IsUp(msg interface{ String() string }) bool {
	keyStr := msg.String()
	for _, key := range k.Up.Keys() {
		if keyStr == key {
			return true
		}
	}
	return false
}

// IsDown checks if the key message is a down navigation command
func (k KeyMap) IsDown(msg interface{ String() string }) bool {
	keyStr := msg.String()
	for _, key := range k.Down.Keys() {
		if keyStr == key {
			return true
		}
	}
	return false
}

// IsSelect checks if the key message is a select command
func (k KeyMap) IsSelect(msg interface{ String() string }) bool {
	keyStr := msg.String()
	for _, key := range k.Select.Keys() {
		if keyStr == key {
			return true
		}
	}
	return false
}

// IsRefresh checks if the key message is a refresh command
func (k KeyMap) IsRefresh(msg interface{ String() string }) bool {
	keyStr := msg.String()
	for _, key := range k.Refresh.Keys() {
		if keyStr == key {
			return true
		}
	}
	return false
}

// IsHelp checks if the key message is a help toggle command
func (k KeyMap) IsHelp(msg interface{ String() string }) bool {
	keyStr := msg.String()
	for _, key := range k.Help.Keys() {
		if keyStr == key {
			return true
		}
	}
	return false
}
