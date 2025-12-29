// Package tui provides terminal user interface utilities for ProxyPilot.
// It includes helper functions for server health checks, provider authentication
// status, agent configuration detection, and display formatting.
package tui

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Default server configuration
const (
	DefaultHost = "127.0.0.1"
	DefaultPort = 8317
)

// ServerStatus represents the current state of the ProxyPilot server.
type ServerStatus struct {
	Running bool
	Host    string
	Port    int
	Error   error
}

// ProviderAuthStatus represents the authentication status of a provider.
type ProviderAuthStatus struct {
	Provider    string
	Configured  bool
	Email       string
	ProjectID   string // For Gemini/Vertex
	AccountID   string // For Codex
	TokenType   string
	AuthFile    string
	ExpiresAt   string
	LastRefresh string
	Error       error
}

// AgentSwitchStatus represents the configuration mode of an agent.
type AgentSwitchStatus struct {
	Agent      string
	Configured bool
	Mode       string // "proxy", "native", or "unknown"
	ConfigPath string
	Message    string
	Error      error
}

// CheckServerRunning checks if the ProxyPilot server is running by attempting
// to connect to the specified host and port. It uses a short timeout to avoid
// blocking the TUI.
func CheckServerRunning(host string, port int) ServerStatus {
	if host == "" {
		host = DefaultHost
	}
	if port == 0 {
		port = DefaultPort
	}

	status := ServerStatus{
		Host: host,
		Port: port,
	}

	addr := fmt.Sprintf("%s:%d", host, port)

	// Try TCP connection with short timeout
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		status.Running = false
		status.Error = err
		return status
	}
	_ = conn.Close()

	// Optionally verify it's actually ProxyPilot by hitting a known endpoint
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s/api/status", addr))
	if err != nil {
		// Connection succeeded but HTTP failed - server might still be starting
		status.Running = true
		return status
	}
	_ = resp.Body.Close()

	status.Running = resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized
	return status
}

// CheckServerRunningDefault checks if the server is running on default settings.
func CheckServerRunningDefault() ServerStatus {
	return CheckServerRunning(DefaultHost, DefaultPort)
}

// GetProviderAuthStatus reads authentication files and returns the status
// for all configured providers.
func GetProviderAuthStatus(authDir string) []ProviderAuthStatus {
	if authDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		authDir = filepath.Join(home, ".proxypilot", "auth")
	}

	// Expand ~ if present
	authDir = expandPath(authDir)

	var statuses []ProviderAuthStatus

	// Check for auth files
	entries, err := os.ReadDir(authDir)
	if err != nil {
		// Auth directory doesn't exist yet
		return []ProviderAuthStatus{
			{Provider: "claude", Configured: false},
			{Provider: "codex", Configured: false},
			{Provider: "gemini", Configured: false},
			{Provider: "vertex", Configured: false},
		}
	}

	// Track which providers we've found
	foundProviders := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(authDir, entry.Name())
		status := parseAuthFile(filePath)
		if status.Provider != "" {
			foundProviders[status.Provider] = true
			statuses = append(statuses, status)
		}
	}

	// Add entries for providers not found
	defaultProviders := []string{"claude", "codex", "gemini", "vertex"}
	for _, p := range defaultProviders {
		if !foundProviders[p] {
			statuses = append(statuses, ProviderAuthStatus{
				Provider:   p,
				Configured: false,
			})
		}
	}

	return statuses
}

// parseAuthFile reads and parses an auth file to extract provider status.
func parseAuthFile(filePath string) ProviderAuthStatus {
	status := ProviderAuthStatus{
		AuthFile: filePath,
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		status.Error = err
		return status
	}

	var authData map[string]any
	if err := json.Unmarshal(data, &authData); err != nil {
		status.Error = err
		return status
	}

	// Determine provider type
	if typeVal, ok := authData["type"].(string); ok {
		status.Provider = typeVal
		status.TokenType = typeVal
	}

	// Extract common fields
	if email, ok := authData["email"].(string); ok {
		status.Email = email
	}

	if projectID, ok := authData["project_id"].(string); ok {
		status.ProjectID = projectID
	}

	if accountID, ok := authData["account_id"].(string); ok {
		status.AccountID = accountID
	}

	if expire, ok := authData["expired"].(string); ok {
		status.ExpiresAt = expire
	}

	if lastRefresh, ok := authData["last_refresh"].(string); ok {
		status.LastRefresh = lastRefresh
	}

	// Check if configured (has access token or service account)
	if _, hasAccess := authData["access_token"]; hasAccess {
		status.Configured = true
	}
	if _, hasServiceAccount := authData["service_account"]; hasServiceAccount {
		status.Configured = true
	}
	if _, hasToken := authData["token"]; hasToken {
		status.Configured = true
	}

	return status
}

// GetAgentSwitchStatus returns the configuration status for all supported agents.
// This reuses the logic from the agent_switch command.
func GetAgentSwitchStatus() []AgentSwitchStatus {
	agents := []string{"claude", "gemini", "codex", "opencode", "droid", "cursor", "kilo", "roocode"}
	var statuses []AgentSwitchStatus

	for _, agent := range agents {
		status := getAgentStatus(agent)
		statuses = append(statuses, status)
	}

	return statuses
}

// getAgentStatus checks the configuration status of a single agent.
func getAgentStatus(agent string) AgentSwitchStatus {
	status := AgentSwitchStatus{
		Agent: agent,
	}

	configPath := getAgentConfigPath(agent)
	if configPath == "" {
		// VS Code extensions that need manual config
		status.Mode = "manual"
		status.Message = "VS Code extension - manual config required"
		return status
	}

	status.ConfigPath = configPath

	// Check if config file exists
	if !fileExists(configPath) {
		status.Configured = false
		status.Mode = "not installed"
		status.Message = "Agent not installed or not configured"
		return status
	}

	status.Configured = true

	// Detect current mode by reading config
	mode := detectAgentMode(configPath)
	status.Mode = mode

	return status
}

// getAgentConfigPath returns the configuration file path for an agent.
func getAgentConfigPath(agent string) string {
	switch strings.ToLower(agent) {
	case "claude", "claude-code":
		return expandPath("~/.claude/settings.json")
	case "gemini", "gemini-cli":
		return expandPath("~/.gemini/settings.json")
	case "codex", "codex-cli":
		return expandPath("~/.codex/config.toml")
	case "opencode":
		return expandPath("~/.config/opencode/opencode.json")
	case "droid", "factory-droid":
		return expandPath("~/.factory/config.json")
	case "cursor":
		return getCursorSettingsPath()
	case "kilo", "kilo-code", "kilocode":
		return "" // VS Code extension - manual config
	case "roo", "roocode", "roo-code":
		return "" // VS Code extension - manual config
	default:
		return ""
	}
}

// getCursorSettingsPath finds the Cursor settings path across platforms.
func getCursorSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	var paths []string
	if appData := os.Getenv("APPDATA"); appData != "" {
		paths = append(paths, filepath.Join(appData, "Cursor", "User", "settings.json"))
	}
	paths = append(paths,
		filepath.Join(home, ".config", "Cursor", "User", "settings.json"),
		filepath.Join(home, "Library", "Application Support", "Cursor", "User", "settings.json"),
	)

	for _, p := range paths {
		if fileExists(p) || dirExists(filepath.Dir(p)) {
			return p
		}
	}

	if len(paths) > 0 {
		return paths[0]
	}
	return ""
}

// detectAgentMode reads the agent config and determines if it's using proxy or native mode.
func detectAgentMode(configPath string) string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "unknown"
	}

	content := string(data)

	// Check for ProxyPilot markers
	if strings.Contains(content, "127.0.0.1:8317") ||
		strings.Contains(content, "127.0.0.1:8318") ||
		strings.Contains(content, "proxypal-local") ||
		(strings.Contains(content, "ANTHROPIC_BASE_URL") && strings.Contains(content, "127.0.0.1")) {
		return "proxy"
	}

	return "native"
}

// FormatDuration formats a duration into a human-readable string.
// Examples: "2h 30m", "5m 10s", "< 1s"
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return "< 1s"
	}

	d = d.Round(time.Second)

	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute
	d -= minutes * time.Minute
	seconds := d / time.Second

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}

	if minutes > 0 {
		if seconds > 0 && minutes < 10 {
			return fmt.Sprintf("%dm %ds", minutes, seconds)
		}
		return fmt.Sprintf("%dm", minutes)
	}

	return fmt.Sprintf("%ds", seconds)
}

// FormatCount formats a count with appropriate suffix (K, M, B).
// Examples: "1,234", "12.3K", "1.5M"
func FormatCount(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}

	if n < 1000000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}

	return fmt.Sprintf("%.1fB", float64(n)/1000000000)
}

// FormatCountWithCommas formats a count with comma separators.
// Examples: "1,234", "1,234,567"
func FormatCountWithCommas(n int64) string {
	if n < 0 {
		return "-" + FormatCountWithCommas(-n)
	}

	str := fmt.Sprintf("%d", n)
	if len(str) <= 3 {
		return str
	}

	var result strings.Builder
	remainder := len(str) % 3
	if remainder > 0 {
		result.WriteString(str[:remainder])
		if len(str) > remainder {
			result.WriteString(",")
		}
	}

	for i := remainder; i < len(str); i += 3 {
		if i > remainder {
			result.WriteString(",")
		}
		result.WriteString(str[i : i+3])
	}

	return result.String()
}

// FormatBytes formats bytes into human-readable format.
// Examples: "1.5 KB", "2.3 MB", "1.0 GB"
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	if bytes < KB {
		return fmt.Sprintf("%d B", bytes)
	}

	if bytes < MB {
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	}

	if bytes < GB {
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	}

	if bytes < TB {
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	}

	return fmt.Sprintf("%.1f TB", float64(bytes)/TB)
}

// FormatRelativeTime formats a time as a relative duration from now.
// Examples: "just now", "5m ago", "2h ago", "3d ago"
func FormatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	d := time.Since(t)

	if d < time.Minute {
		return "just now"
	}

	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}

	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}

	days := int(d.Hours() / 24)
	if days == 1 {
		return "1d ago"
	}

	if days < 30 {
		return fmt.Sprintf("%dd ago", days)
	}

	months := days / 30
	if months == 1 {
		return "1mo ago"
	}

	if months < 12 {
		return fmt.Sprintf("%dmo ago", months)
	}

	years := months / 12
	if years == 1 {
		return "1y ago"
	}

	return fmt.Sprintf("%dy ago", years)
}

// TUIError represents an error that can be displayed in the TUI.
type TUIError struct {
	Title   string
	Message string
	Details string
	Style   lipgloss.Style
}

// NewTUIError creates a new TUI error with default styling.
func NewTUIError(title, message string) TUIError {
	return TUIError{
		Title:   title,
		Message: message,
		Style:   ErrorBadge,
	}
}

// NewTUIErrorWithDetails creates a new TUI error with additional details.
func NewTUIErrorWithDetails(title, message, details string) TUIError {
	return TUIError{
		Title:   title,
		Message: message,
		Details: details,
		Style:   ErrorBadge,
	}
}

// Render returns a formatted string representation of the error.
func (e TUIError) Render() string {
	var sb strings.Builder

	// Title with error badge
	if e.Title != "" {
		sb.WriteString(e.Style.Render(e.Title))
		sb.WriteString("\n")
	}

	// Message
	if e.Message != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(ErrorColor).Render(e.Message))
		sb.WriteString("\n")
	}

	// Details (dimmed)
	if e.Details != "" {
		sb.WriteString(Dim(e.Details))
	}

	return sb.String()
}

// WrapError wraps an error for TUI display with context.
func WrapError(err error, context string) TUIError {
	if err == nil {
		return TUIError{}
	}

	return TUIError{
		Title:   "Error",
		Message: context,
		Details: err.Error(),
		Style:   ErrorBadge,
	}
}

// FormatErrorList formats multiple errors into a single display string.
func FormatErrorList(errors []error) string {
	if len(errors) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, err := range errors {
		if err == nil {
			continue
		}
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(lipgloss.NewStyle().Foreground(ErrorColor).Render("- " + err.Error()))
	}

	return sb.String()
}

// StatusIndicator returns a styled status indicator character.
func StatusIndicator(ok bool) string {
	if ok {
		return lipgloss.NewStyle().Foreground(SuccessColor).Render("*")
	}
	return lipgloss.NewStyle().Foreground(ErrorColor).Render("x")
}

// StatusText returns styled status text.
func StatusText(ok bool, okText, failText string) string {
	if ok {
		return lipgloss.NewStyle().Foreground(SuccessColor).Render(okText)
	}
	return lipgloss.NewStyle().Foreground(ErrorColor).Render(failText)
}

// ProviderStatusBadge returns a styled badge for provider status.
func ProviderStatusBadge(configured bool) string {
	if configured {
		return SuccessBadge.Render("OK")
	}
	return MutedBadge.Render("--")
}

// AgentModeBadge returns a styled badge for agent mode.
func AgentModeBadge(mode string) string {
	switch mode {
	case "proxy":
		return SuccessBadge.Render("PROXY")
	case "native":
		return WarningBadge.Render("NATIVE")
	case "manual":
		return InfoBadge.Render("MANUAL")
	default:
		return MutedBadge.Render("--")
	}
}

// Utility functions

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
