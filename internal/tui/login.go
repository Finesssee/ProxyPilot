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

// Provider brand colors are defined in styles.go

// getProviderColor returns the brand color for a provider
func getProviderColor(providerID string) lipgloss.Color {
	switch providerID {
	case "claude":
		return ClaudeBrand
	case "codex":
		return CodexBrand
	case "gemini":
		return GeminiBrand
	case "kiro":
		return KiroBrand
	case "qwen":
		return QwenBrand
	case "antigravity":
		return AntigravityBrand
	case "minimax":
		return MiniMaxBrand
	case "zhipu":
		return ZhipuBrand
	default:
		return Cyan
	}
}

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

// ═══════════════════════════════════════════════════════════════════════════════
// VIEW RENDERING - Returns content only (no outer borders)
// The main TUI wraps this in a bordered panel
// ═══════════════════════════════════════════════════════════════════════════════

// View renders the login screen content
func (m LoginModel) View() string {
	return m.ViewWithSize(m.width, m.height)
}

// ViewWithSize renders with explicit dimensions for responsive layout
func (m LoginModel) ViewWithSize(width, height int) string {
	var b strings.Builder

	// Section title (compact)
	title := lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true).
		Render("Provider Authentication")
	subtitle := lipgloss.NewStyle().
		Foreground(TextMuted).
		Render(" - Connect via OAuth")
	b.WriteString(title + subtitle + "\n\n")

	// Loading state
	if m.loading {
		b.WriteString(lipgloss.NewStyle().Foreground(Accent).Render("◐") + " ")
		b.WriteString(lipgloss.NewStyle().Foreground(TextMuted).Render("Scanning authentication status..."))
		return b.String()
	}

	// Logging in state
	if m.loggingIn {
		providerColor := getProviderColor(m.loginProvider)
		b.WriteString(lipgloss.NewStyle().Foreground(providerColor).Bold(true).Render("◐") + " ")
		b.WriteString(lipgloss.NewStyle().Foreground(Text).Render(fmt.Sprintf("Authenticating with %s...", m.loginProvider)))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(TextMuted).Render("Complete authentication in your browser"))
		return b.String()
	}

	// Calculate visible rows based on height
	maxRows := height - 5
	if maxRows < 4 {
		maxRows = 4
	}
	if maxRows > len(m.providers) {
		maxRows = len(m.providers)
	}

	// Provider list - responsive rows
	for i := 0; i < maxRows; i++ {
		if i >= len(m.providers) {
			break
		}
		provider := m.providers[i]
		isSelected := i == m.cursor
		row := m.renderProviderRowWithWidth(provider, isSelected, width)
		b.WriteString(row + "\n")
	}

	// Status message
	if m.message != "" {
		b.WriteString("\n")
		if m.messageIsError {
			b.WriteString(lipgloss.NewStyle().Foreground(Red).Render(IconCross + " " + m.message))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(Green).Render(IconCheck + " " + m.message))
		}
	}

	return b.String()
}

// renderProviderRow creates a simple row for the provider list
func (m LoginModel) renderProviderRow(provider ProviderInfo, isSelected bool) string {
	return m.renderProviderRowWithWidth(provider, isSelected, m.width)
}

// renderProviderRowWithWidth creates a row that adapts to available width
func (m LoginModel) renderProviderRowWithWidth(provider ProviderInfo, isSelected bool, width int) string {
	providerColor := getProviderColor(provider.ID)

	// Cursor indicator
	var cursor string
	if isSelected {
		cursor = lipgloss.NewStyle().Foreground(Accent).Bold(true).Render(IconChevron + " ")
	} else {
		cursor = "  "
	}

	// Calculate responsive widths based on available space
	nameWidth := 12
	if width > 60 {
		nameWidth = 16
	}
	if width > 80 {
		nameWidth = 18
	}

	// Provider name
	var nameStyle lipgloss.Style
	if isSelected {
		nameStyle = lipgloss.NewStyle().Foreground(providerColor).Bold(true).Width(nameWidth)
	} else {
		nameStyle = lipgloss.NewStyle().Foreground(Text).Width(nameWidth)
	}
	name := nameStyle.Render(provider.Name)

	// Status badge - use shorter text on narrow terminals
	var status string
	if provider.Authenticated {
		if width > 50 {
			status = lipgloss.NewStyle().
				Foreground(BgDark).
				Background(Green).
				Bold(true).
				Padding(0, 1).
				Render(IconOnline + " CONNECTED")
		} else {
			status = lipgloss.NewStyle().
				Foreground(BgDark).
				Background(Green).
				Bold(true).
				Padding(0, 1).
				Render(IconOnline)
		}
	} else {
		if width > 50 {
			status = lipgloss.NewStyle().
				Foreground(BgDark).
				Background(Red).
				Bold(true).
				Padding(0, 1).
				Render(IconOffline + " DISCONNECTED")
		} else {
			status = lipgloss.NewStyle().
				Foreground(BgDark).
				Background(Red).
				Bold(true).
				Padding(0, 1).
				Render(IconOffline)
		}
	}

	// Email info - only show on wider terminals
	var emailInfo string
	if width > 70 && provider.Authenticated && provider.Email != "" {
		// Truncate email if terminal is not wide enough
		email := provider.Email
		maxEmailLen := width - nameWidth - 30 // Reserve space for cursor, name, status
		if maxEmailLen < 10 {
			maxEmailLen = 10
		}
		if len(email) > maxEmailLen {
			email = email[:maxEmailLen-3] + "..."
		}
		emailInfo = lipgloss.NewStyle().Foreground(TextMuted).Render(" " + email)
	}

	// Compose row
	row := cursor + name + status + emailInfo

	// Selection highlight
	if isSelected {
		row = lipgloss.NewStyle().Background(BgSelected).Render(row)
	}

	return row
}

// renderStatusBadge creates a styled status badge for auth state
func (m LoginModel) renderStatusBadge(provider ProviderInfo) string {
	if provider.Authenticated {
		// Connected badge - NeonGreen with filled circle
		return lipgloss.NewStyle().
			Foreground(DeepBlack).
			Background(NeonGreen).
			Bold(true).
			Padding(0, 1).
			Render(IconOnline + " CONNECTED")
	}

	// Disconnected badge - HotCoral with hollow circle
	return lipgloss.NewStyle().
		Foreground(TextBright).
		Background(HotCoral).
		Bold(true).
		Padding(0, 1).
		Render(IconOffline + " DISCONNECTED")
}

// renderKeyInfo displays masked key or email info
func (m LoginModel) renderKeyInfo(provider ProviderInfo) string {
	keyStyle := lipgloss.NewStyle().Foreground(TextDim)
	valueStyle := lipgloss.NewStyle().Foreground(TextMuted)

	if provider.Email != "" {
		// Show email
		return keyStyle.Render("Email: ") + valueStyle.Render(provider.Email)
	}

	// Show masked key placeholder
	maskedKey := "****-****-****-" + lipgloss.NewStyle().Foreground(Cyan).Render("****")
	return keyStyle.Render("Token: ") + maskedKey
}

// renderActionHints creates the keyboard shortcut hints
func (m LoginModel) renderActionHints() string {
	keyStyle := lipgloss.NewStyle().
		Foreground(Violet).
		Background(Surface).
		Bold(true).
		Padding(0, 1)

	descStyle := lipgloss.NewStyle().
		Foreground(TextDim)

	separator := lipgloss.NewStyle().Foreground(BorderDim).Render("  ")

	hints := []struct {
		key  string
		desc string
	}{
		{"Enter", "Login"},
		{"r", "Refresh"},
		{"Esc", "Back"},
		{"q", "Quit"},
	}

	var parts []string
	for _, hint := range hints {
		parts = append(parts, keyStyle.Render(hint.key)+descStyle.Render(" "+hint.desc))
	}

	return strings.Join(parts, separator)
}

// ═══════════════════════════════════════════════════════════════════════════════
// PUBLIC ACCESSOR METHODS
// ═══════════════════════════════════════════════════════════════════════════════

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
