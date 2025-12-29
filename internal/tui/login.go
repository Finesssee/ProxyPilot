package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

// ProviderInfo holds display information for a provider
type ProviderInfo struct {
	ID            string
	Name          string
	Authenticated bool
	Email         string
}

// LoginModel is the model for the provider login screen
type LoginModel struct {
	providers      []ProviderInfo
	cursor         int
	width          int
	height         int
	loading        bool
	loggingIn      bool
	loginProvider  string
	message        string
	messageIsError bool
	cfg            *config.Config
	authManager    *sdkAuth.Manager
	store          coreauth.Store
}

// loginResultMsg is sent when a login completes
type loginResultMsg struct {
	provider string
	success  bool
	email    string
	err      error
}

// authStatusMsg is sent when auth status check completes
type authStatusMsg struct {
	providers []ProviderInfo
}

// NewLoginModel creates a new login screen model
func NewLoginModel(cfg *config.Config) LoginModel {
	store := sdkAuth.GetTokenStore()
	if setter, ok := store.(interface{ SetBaseDir(string) }); ok && cfg != nil {
		setter.SetBaseDir(cfg.AuthDir)
	}

	manager := sdkAuth.NewManager(store,
		sdkAuth.NewClaudeAuthenticator(),
		sdkAuth.NewCodexAuthenticator(),
		sdkAuth.NewGeminiAuthenticator(),
		sdkAuth.NewKiroAuthenticator(),
		sdkAuth.NewQwenAuthenticator(),
		sdkAuth.NewAntigravityAuthenticator(),
		sdkAuth.NewMiniMaxAuthenticator(),
		sdkAuth.NewZhipuAuthenticator(),
	)

	return LoginModel{
		providers: []ProviderInfo{
			{ID: "claude", Name: "Claude"},
			{ID: "codex", Name: "Codex"},
			{ID: "gemini", Name: "Gemini"},
			{ID: "kiro", Name: "Kiro"},
			{ID: "qwen", Name: "Qwen"},
			{ID: "antigravity", Name: "Antigravity"},
			{ID: "minimax", Name: "MiniMax"},
			{ID: "zhipu", Name: "Zhipu"},
		},
		cursor:      0,
		width:       80,
		height:      24,
		loading:     true,
		cfg:         cfg,
		authManager: manager,
		store:       store,
	}
}

// Init initializes the login model
func (m LoginModel) Init() tea.Cmd {
	return m.checkAuthStatus
}

// checkAuthStatus checks the authentication status for all providers
func (m LoginModel) checkAuthStatus() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	providers := make([]ProviderInfo, len(m.providers))
	copy(providers, m.providers)

	// Get all stored auth records
	auths, err := m.store.List(ctx)
	if err != nil {
		// Return providers with unknown status
		return authStatusMsg{providers: providers}
	}

	// Build a map of provider -> auth info
	authMap := make(map[string]*coreauth.Auth)
	for _, auth := range auths {
		if auth == nil {
			continue
		}
		provider := strings.ToLower(auth.Provider)
		// Keep the most recent auth for each provider
		if existing, ok := authMap[provider]; !ok || auth.UpdatedAt.After(existing.UpdatedAt) {
			authMap[provider] = auth
		}
	}

	// Update provider info with auth status
	for i := range providers {
		providerID := strings.ToLower(providers[i].ID)
		if auth, ok := authMap[providerID]; ok {
			providers[i].Authenticated = true
			// Extract email from metadata if available
			if auth.Metadata != nil {
				if email, ok := auth.Metadata["email"].(string); ok && email != "" {
					providers[i].Email = email
				}
			}
		}
	}

	return authStatusMsg{providers: providers}
}

// Update handles messages for the login model
func (m LoginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case authStatusMsg:
		m.providers = msg.providers
		m.loading = false
		return m, nil
	case loginResultMsg:
		m.loggingIn = false
		m.loginProvider = ""
		if msg.err != nil {
			m.message = fmt.Sprintf("Login failed: %v", msg.err)
			m.messageIsError = true
		} else if msg.success {
			m.message = fmt.Sprintf("Successfully logged in to %s", msg.provider)
			if msg.email != "" {
				m.message += fmt.Sprintf(" (%s)", msg.email)
			}
			m.messageIsError = false
			// Refresh auth status
			return m, m.checkAuthStatus
		}
		return m, nil
	}
	return m, nil
}

// handleKeyPress handles keyboard input for the login screen
func (m LoginModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Don't allow navigation while logging in
	if m.loggingIn {
		return m, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc", "b":
		// Return to main menu - handled by parent
		return m, nil
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		m.message = ""
	case "down", "j":
		if m.cursor < len(m.providers)-1 {
			m.cursor++
		}
		m.message = ""
	case "enter", " ":
		if m.cursor >= 0 && m.cursor < len(m.providers) {
			provider := m.providers[m.cursor]
			m.loggingIn = true
			m.loginProvider = provider.ID
			m.message = ""
			return m, m.startLogin(provider.ID)
		}
	case "r":
		// Refresh auth status
		m.loading = true
		return m, m.checkAuthStatus
	}
	return m, nil
}

// startLogin initiates the OAuth login flow for the selected provider
func (m LoginModel) startLogin(providerID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		opts := &sdkAuth.LoginOptions{
			NoBrowser: false,
		}

		record, _, err := m.authManager.Login(ctx, providerID, m.cfg, opts)
		if err != nil {
			return loginResultMsg{
				provider: providerID,
				success:  false,
				err:      err,
			}
		}

		email := ""
		if record != nil && record.Metadata != nil {
			if e, ok := record.Metadata["email"].(string); ok {
				email = e
			}
		}

		return loginResultMsg{
			provider: providerID,
			success:  true,
			email:    email,
		}
	}
}

// View renders the login screen
func (m LoginModel) View() string {
	var b strings.Builder

	// Title
	b.WriteString(TitleStyle.Render("Provider Login"))
	b.WriteString("\n")

	// Divider
	b.WriteString(lipgloss.NewStyle().Foreground(BorderColor).Render(strings.Repeat("\u2500", 40)))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(SpinnerStyle.Render("Loading authentication status..."))
		b.WriteString("\n")
	} else if m.loggingIn {
		b.WriteString(SpinnerStyle.Render(fmt.Sprintf("Logging in to %s...", m.loginProvider)))
		b.WriteString("\n")
		b.WriteString(Muted("Please complete authentication in your browser."))
		b.WriteString("\n")
	} else {
		// Provider list
		for i, provider := range m.providers {
			var line string
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			// Provider name
			name := provider.Name

			// Status indicator
			var status string
			if provider.Authenticated {
				statusText := "Authenticated"
				if provider.Email != "" {
					statusText = provider.Email
				}
				status = Success(fmt.Sprintf("[%s %s]", "\u2713", statusText))
			} else {
				status = Error(fmt.Sprintf("[%s Not logged in]", "\u2717"))
			}

			// Pad the name for alignment
			paddedName := fmt.Sprintf("%-15s", name)

			if i == m.cursor {
				line = CursorStyle.Render(cursor) + Primary(paddedName) + " " + status
			} else {
				line = Dim(cursor) + paddedName + " " + status
			}

			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Message area
	if m.message != "" {
		b.WriteString("\n")
		if m.messageIsError {
			b.WriteString(Error(m.message))
		} else {
			b.WriteString(Success(m.message))
		}
		b.WriteString("\n")
	}

	// Help text
	b.WriteString("\n")
	helpText := "[Enter] Login  [r] Refresh  [Esc] Back  [q] Quit"
	b.WriteString(HelpStyle.Render(helpText))

	return b.String()
}

// GetSelectedProvider returns the currently selected provider ID
func (m LoginModel) GetSelectedProvider() string {
	if m.cursor >= 0 && m.cursor < len(m.providers) {
		return m.providers[m.cursor].ID
	}
	return ""
}

// IsAuthenticated returns whether the selected provider is authenticated
func (m LoginModel) IsAuthenticated() bool {
	if m.cursor >= 0 && m.cursor < len(m.providers) {
		return m.providers[m.cursor].Authenticated
	}
	return false
}

// IsLoading returns whether the model is currently loading
func (m LoginModel) IsLoading() bool {
	return m.loading
}

// IsLoggingIn returns whether a login is in progress
func (m LoginModel) IsLoggingIn() bool {
	return m.loggingIn
}
