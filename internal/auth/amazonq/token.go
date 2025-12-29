// Package amazonq provides authentication functionality for Amazon Q CLI.
// Amazon Q CLI stores its tokens in a SQLite database at ~/.local/share/amazon-q/data.sqlite3
// This package reads tokens from that database for use with ProxyPilot.
package amazonq

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	kiroauth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	_ "modernc.org/sqlite"
)

const (
	// TokenKey is the SQLite key for the OAuth token in the auth_kv table
	TokenKey = "codewhisperer:odic:token"
	// DeviceRegistrationKey is the SQLite key for device registration
	DeviceRegistrationKey = "codewhisperer:odic:device-registration"
)

// AmazonQToken represents the token data stored by Amazon Q CLI
type AmazonQToken struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresAt    string   `json:"expires_at"` // RFC3339 timestamp string
	Region       string   `json:"region"`
	StartURL     *string  `json:"start_url"` // Can be null
	OAuthFlow    string   `json:"oauth_flow"`
	Scopes       []string `json:"scopes"`
}

// DeviceRegistration represents the device registration data stored by Amazon Q CLI
type DeviceRegistration struct {
	ClientID              string   `json:"client_id"`
	ClientSecret          string   `json:"client_secret"`
	ClientSecretExpiresAt string   `json:"client_secret_expires_at"` // RFC3339 timestamp
	Region                string   `json:"region"`
	OAuthFlow             string   `json:"oauth_flow"`
	Scopes                []string `json:"scopes"`
}

// IsExpired checks if the token has expired
func (t *AmazonQToken) IsExpired() bool {
	if t.ExpiresAt == "" {
		return true
	}
	// ExpiresAt is in RFC3339 format
	expiresAt, err := time.Parse(time.RFC3339, t.ExpiresAt)
	if err != nil {
		// Try RFC3339Nano for more precision
		expiresAt, err = time.Parse(time.RFC3339Nano, t.ExpiresAt)
		if err != nil {
			return true
		}
	}
	// Consider expired if less than 5 minutes remaining
	return time.Now().Add(5 * time.Minute).After(expiresAt)
}

// ToKiroTokenData converts AmazonQToken to kiro.KiroTokenData for use with the Kiro executor
func (t *AmazonQToken) ToKiroTokenData() *kiroauth.KiroTokenData {
	startURL := ""
	if t.StartURL != nil {
		startURL = *t.StartURL
	}

	return &kiroauth.KiroTokenData{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		ExpiresAt:    t.ExpiresAt, // Already in RFC3339 format
		AuthMethod:   "builder-id",
		Provider:     "AmazonQ-CLI",
		StartURL:     startURL,
		Region:       t.Region,
	}
}

// TokenReader reads Amazon Q CLI tokens from the SQLite database
type TokenReader struct {
	dbPath    string
	wslDistro string // WSL distro name (Windows only)
	wslUser   string // WSL username (Windows only)
}

// NewTokenReader creates a new TokenReader
func NewTokenReader() (*TokenReader, error) {
	if runtime.GOOS == "windows" {
		return newWSLTokenReader()
	}

	dbPath, err := getNativeDatabasePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get database path: %w", err)
	}
	return &TokenReader{dbPath: dbPath}, nil
}

// newWSLTokenReader creates a token reader that queries via WSL on Windows
func newWSLTokenReader() (*TokenReader, error) {
	distro, err := getDefaultWSLDistro()
	if err != nil {
		return nil, fmt.Errorf("no WSL distribution found: %w", err)
	}

	user, err := getWSLUsername(distro)
	if err != nil {
		return nil, fmt.Errorf("failed to get WSL user: %w", err)
	}

	// Store the native WSL path for queries
	dbPath := fmt.Sprintf("/home/%s/.local/share/amazon-q/data.sqlite3", user)

	return &TokenReader{
		dbPath:    dbPath,
		wslDistro: distro,
		wslUser:   user,
	}, nil
}

// NewTokenReaderWithPath creates a TokenReader with a custom database path
func NewTokenReaderWithPath(dbPath string) *TokenReader {
	return &TokenReader{dbPath: dbPath}
}

// GetDatabasePath returns the path to the Amazon Q SQLite database
func GetDatabasePath() (string, error) {
	if runtime.GOOS == "windows" {
		return getWSLDatabasePath()
	}
	return getNativeDatabasePath()
}

// getNativeDatabasePath returns the database path for Linux/macOS
func getNativeDatabasePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".local", "share", "amazon-q", "data.sqlite3"), nil
}

// getWSLDatabasePath returns the database path via WSL on Windows
func getWSLDatabasePath() (string, error) {
	// Get default WSL distro
	distro, err := getDefaultWSLDistro()
	if err != nil {
		return "", fmt.Errorf("no WSL distribution found: %w", err)
	}

	// Get WSL username
	user, err := getWSLUsername(distro)
	if err != nil {
		return "", fmt.Errorf("failed to get WSL user: %w", err)
	}

	// Windows UNC path to WSL filesystem
	return fmt.Sprintf(`\\wsl$\%s\home\%s\.local\share\amazon-q\data.sqlite3`, distro, user), nil
}

// getDefaultWSLDistro returns the default WSL distribution name
func getDefaultWSLDistro() (string, error) {
	cmd := exec.Command("wsl", "-l", "-q")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// WSL outputs UTF-16LE on Windows, convert by removing null bytes
	cleaned := strings.ReplaceAll(string(output), "\x00", "")
	// Also handle Windows CRLF line endings
	cleaned = strings.ReplaceAll(cleaned, "\r\n", "\n")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")

	lines := strings.Split(strings.TrimSpace(cleaned), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && line != "docker-desktop" && line != "docker-desktop-data" &&
			!strings.HasPrefix(line, "podman") {
			return line, nil
		}
	}

	return "", fmt.Errorf("no WSL distribution found")
}

// getWSLUsername returns the default username in the WSL distribution
func getWSLUsername(distro string) (string, error) {
	cmd := exec.Command("wsl", "-d", distro, "--", "whoami")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// ReadToken reads the current OAuth token from the database
func (r *TokenReader) ReadToken() (*AmazonQToken, error) {
	// On Windows, use WSL to query the database
	if r.wslDistro != "" {
		return r.readTokenViaWSL()
	}

	return r.readTokenNative()
}

// readTokenViaWSL queries the database using WSL Python (avoids cross-filesystem SQLite issues)
func (r *TokenReader) readTokenViaWSL() (*AmazonQToken, error) {
	// Use Python to query SQLite since it's more reliably available than sqlite3 CLI
	pythonScript := fmt.Sprintf(`import sqlite3; import json; conn = sqlite3.connect('%s'); cursor = conn.cursor(); cursor.execute('SELECT value FROM auth_kv WHERE key = "%s"'); row = cursor.fetchone(); print(row[0] if row else '')`, r.dbPath, TokenKey)

	cmd := exec.Command("wsl", "-d", r.wslDistro, "python3", "-c", pythonScript)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to query database via WSL: %w", err)
	}

	valueJSON := strings.TrimSpace(string(output))
	if valueJSON == "" {
		return nil, fmt.Errorf("no token found in Amazon Q CLI database (run 'q login' first)")
	}

	var token AmazonQToken
	if err := json.Unmarshal([]byte(valueJSON), &token); err != nil {
		return nil, fmt.Errorf("failed to parse token JSON: %w", err)
	}

	if token.AccessToken == "" {
		return nil, fmt.Errorf("access token is empty")
	}

	return &token, nil
}

// readTokenNative reads the token using native SQLite access (Linux/macOS)
func (r *TokenReader) readTokenNative() (*AmazonQToken, error) {
	// Check if database exists
	if _, err := os.Stat(r.dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("Amazon Q CLI database not found at %s", r.dbPath)
	}

	// Open database with read-only mode
	dsn := r.dbPath + "?mode=ro&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Read token from auth_kv table
	var valueJSON string
	err = db.QueryRow("SELECT value FROM auth_kv WHERE key = ?", TokenKey).Scan(&valueJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no token found in Amazon Q CLI database (run 'q login' first)")
		}
		return nil, fmt.Errorf("failed to read token: %w", err)
	}

	// Parse JSON
	var token AmazonQToken
	if err := json.Unmarshal([]byte(valueJSON), &token); err != nil {
		return nil, fmt.Errorf("failed to parse token JSON: %w", err)
	}

	if token.AccessToken == "" {
		return nil, fmt.Errorf("access token is empty")
	}

	return &token, nil
}

// ReadDeviceRegistration reads the device registration data from the database
func (r *TokenReader) ReadDeviceRegistration() (*DeviceRegistration, error) {
	// On Windows, use WSL to query the database
	if r.wslDistro != "" {
		return r.readDeviceRegistrationViaWSL()
	}

	return r.readDeviceRegistrationNative()
}

// readDeviceRegistrationViaWSL queries device registration via WSL
func (r *TokenReader) readDeviceRegistrationViaWSL() (*DeviceRegistration, error) {
	pythonScript := fmt.Sprintf(`import sqlite3; import json; conn = sqlite3.connect('%s'); cursor = conn.cursor(); cursor.execute('SELECT value FROM auth_kv WHERE key = "%s"'); row = cursor.fetchone(); print(row[0] if row else '')`, r.dbPath, DeviceRegistrationKey)

	cmd := exec.Command("wsl", "-d", r.wslDistro, "python3", "-c", pythonScript)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to query database via WSL: %w", err)
	}

	valueJSON := strings.TrimSpace(string(output))
	if valueJSON == "" {
		return nil, fmt.Errorf("no device registration found")
	}

	var reg DeviceRegistration
	if err := json.Unmarshal([]byte(valueJSON), &reg); err != nil {
		return nil, fmt.Errorf("failed to parse device registration JSON: %w", err)
	}

	return &reg, nil
}

// readDeviceRegistrationNative reads device registration using native SQLite
func (r *TokenReader) readDeviceRegistrationNative() (*DeviceRegistration, error) {
	if _, err := os.Stat(r.dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("Amazon Q CLI database not found at %s", r.dbPath)
	}

	dsn := r.dbPath + "?mode=ro&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	var valueJSON string
	err = db.QueryRow("SELECT value FROM auth_kv WHERE key = ?", DeviceRegistrationKey).Scan(&valueJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no device registration found")
		}
		return nil, fmt.Errorf("failed to read device registration: %w", err)
	}

	var reg DeviceRegistration
	if err := json.Unmarshal([]byte(valueJSON), &reg); err != nil {
		return nil, fmt.Errorf("failed to parse device registration JSON: %w", err)
	}

	return &reg, nil
}

// ReadKiroToken reads and converts the token to Kiro format for use with the executor
func (r *TokenReader) ReadKiroToken() (*kiroauth.KiroTokenData, error) {
	token, err := r.ReadToken()
	if err != nil {
		return nil, err
	}

	if token.IsExpired() {
		return nil, fmt.Errorf("Amazon Q CLI token has expired (run 'q login' to refresh)")
	}

	kiroToken := token.ToKiroTokenData()

	// Try to get client credentials for token refresh
	reg, err := r.ReadDeviceRegistration()
	if err == nil {
		kiroToken.ClientID = reg.ClientID
		kiroToken.ClientSecret = reg.ClientSecret
	}

	return kiroToken, nil
}

// IsAvailable checks if Amazon Q CLI tokens are available
func IsAvailable() bool {
	reader, err := NewTokenReader()
	if err != nil {
		return false
	}

	token, err := reader.ReadToken()
	if err != nil {
		return false
	}

	return !token.IsExpired()
}

// LoadAmazonQToken is a convenience function to load the Amazon Q token
func LoadAmazonQToken() (*kiroauth.KiroTokenData, error) {
	reader, err := NewTokenReader()
	if err != nil {
		return nil, err
	}
	return reader.ReadKiroToken()
}
