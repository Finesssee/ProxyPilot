package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the TUI application.
func Run(proxyURL, managementKey string) error {
	if proxyURL == "" {
		proxyURL = "http://127.0.0.1:8318"
	}

	model := NewModel(proxyURL, managementKey)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	return nil
}

// RunWithStdio runs the TUI with custom stdio (for embedding in other programs).
func RunWithStdio(proxyURL, managementKey string, in *os.File, out *os.File) error {
	if proxyURL == "" {
		proxyURL = "http://127.0.0.1:8318"
	}

	model := NewModel(proxyURL, managementKey)
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithInput(in),
		tea.WithOutput(out),
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	return nil
}
