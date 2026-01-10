package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	antigravityauth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/antigravity"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/browser"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/misc"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

const (
	antigravityClientID     = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
	antigravityClientSecret = "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"
	antigravityCallbackPort = 51121
)

var antigravityScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
	"https://www.googleapis.com/auth/cclog",
	"https://www.googleapis.com/auth/experimentsandconfigs",
}

// AntigravityAuthenticator implements OAuth login for the antigravity provider.
type AntigravityAuthenticator struct{}

// NewAntigravityAuthenticator constructs a new authenticator instance.
func NewAntigravityAuthenticator() Authenticator { return &AntigravityAuthenticator{} }

// Provider returns the provider key for antigravity.
func (AntigravityAuthenticator) Provider() string { return "antigravity" }

// RefreshLead instructs the manager to refresh five minutes before expiry.
func (AntigravityAuthenticator) RefreshLead() *time.Duration {
	lead := 5 * time.Minute
	return &lead
}

// Login launches a local OAuth flow to obtain antigravity tokens and persists them.
func (AntigravityAuthenticator) Login(ctx context.Context, cfg *config.Config, opts *LoginOptions) (*coreauth.Auth, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cliproxy auth: configuration is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if opts == nil {
		opts = &LoginOptions{}
	}

	httpClient := util.SetProxy(&cfg.SDKConfig, &http.Client{})

	state, err := misc.GenerateRandomState()
	if err != nil {
		return nil, fmt.Errorf("antigravity: failed to generate state: %w", err)
	}

	srv, port, cbChan, errServer := startAntigravityCallbackServer()
	if errServer != nil {
		return nil, fmt.Errorf("antigravity: failed to start callback server: %w", errServer)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	redirectURI := fmt.Sprintf("http://localhost:%d/oauth-callback", port)
	authURL := buildAntigravityAuthURL(redirectURI, state)

	if !opts.NoBrowser {
		fmt.Println("Opening browser for antigravity authentication")
		if !browser.IsAvailable() {
			log.Warn("No browser available; please open the URL manually")
			util.PrintSSHTunnelInstructions(port)
			fmt.Printf("Visit the following URL to continue authentication:\n%s\n", authURL)
		} else if errOpen := browser.OpenURL(authURL); errOpen != nil {
			log.Warnf("Failed to open browser automatically: %v", errOpen)
			util.PrintSSHTunnelInstructions(port)
			fmt.Printf("Visit the following URL to continue authentication:\n%s\n", authURL)
		}
	} else {
		util.PrintSSHTunnelInstructions(port)
		fmt.Printf("Visit the following URL to continue authentication:\n%s\n", authURL)
	}

	fmt.Println("Waiting for antigravity authentication callback...")

	var cbRes callbackResult
	timeoutTimer := time.NewTimer(5 * time.Minute)
	defer timeoutTimer.Stop()

	var manualPromptTimer *time.Timer
	var manualPromptC <-chan time.Time
	if opts.Prompt != nil {
		manualPromptTimer = time.NewTimer(15 * time.Second)
		manualPromptC = manualPromptTimer.C
		defer manualPromptTimer.Stop()
	}

waitForCallback:
	for {
		select {
		case res := <-cbChan:
			cbRes = res
			break waitForCallback
		case <-manualPromptC:
			manualPromptC = nil
			if manualPromptTimer != nil {
				manualPromptTimer.Stop()
			}
			select {
			case res := <-cbChan:
				cbRes = res
				break waitForCallback
			default:
			}
			input, errPrompt := opts.Prompt("Paste the antigravity callback URL (or press Enter to keep waiting): ")
			if errPrompt != nil {
				return nil, errPrompt
			}
			parsed, errParse := misc.ParseOAuthCallback(input)
			if errParse != nil {
				return nil, errParse
			}
			if parsed == nil {
				continue
			}
			cbRes = callbackResult{
				Code:  parsed.Code,
				State: parsed.State,
				Error: parsed.Error,
			}
			break waitForCallback
		case <-timeoutTimer.C:
			return nil, fmt.Errorf("antigravity: authentication timed out")
		}
	}

	if cbRes.Error != "" {
		return nil, fmt.Errorf("antigravity: authentication failed: %s", cbRes.Error)
	}
	if cbRes.State != state {
		return nil, fmt.Errorf("antigravity: invalid state")
	}
	if cbRes.Code == "" {
		return nil, fmt.Errorf("antigravity: missing authorization code")
	}

	tokenResp, errToken := exchangeAntigravityCode(ctx, cbRes.Code, redirectURI, httpClient)
	if errToken != nil {
		return nil, fmt.Errorf("antigravity: token exchange failed: %w", errToken)
	}

	email := ""
	if tokenResp.AccessToken != "" {
		if info, errInfo := fetchAntigravityUserInfo(ctx, tokenResp.AccessToken, httpClient); errInfo == nil && strings.TrimSpace(info.Email) != "" {
			email = strings.TrimSpace(info.Email)
		}
	}

	// Fetch project/tier info via loadCodeAssist (same approach as Gemini CLI)
	projectID := ""
	tierID := ""
	allowedTiers := []map[string]any(nil)
	if tokenResp.AccessToken != "" {
		accountInfo, errProject := fetchAntigravityAccountInfo(ctx, tokenResp.AccessToken, httpClient)
		if errProject != nil {
			log.Warnf("antigravity: failed to fetch account info: %v", errProject)
		} else if accountInfo != nil {
			projectID = accountInfo.ProjectID
			tierID = accountInfo.TierID
			allowedTiers = accountInfo.AllowedTiers
			if projectID != "" {
				log.Infof("antigravity: obtained project ID %s", projectID)
			}
		}
	}

	now := time.Now()
	metadata := map[string]any{
		"type":          "antigravity",
		"access_token":  tokenResp.AccessToken,
		"refresh_token": tokenResp.RefreshToken,
		"expires_in":    tokenResp.ExpiresIn,
		"timestamp":     now.UnixMilli(),
		"expired":       now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339),
	}
	if email != "" {
		metadata["email"] = email
	}
	if projectID != "" {
		metadata["project_id"] = projectID
	}
	if tierID != "" {
		metadata["tier_id"] = tierID
	}
	if len(allowedTiers) > 0 {
		metadata["allowed_tiers"] = allowedTiers
	}

	fileName := sanitizeAntigravityFileName(email)
	label := email
	if label == "" {
		label = "antigravity"
	}

	fmt.Println("Antigravity authentication successful")
	if projectID != "" {
		fmt.Printf("Using GCP project: %s\n", projectID)
	}
	return &coreauth.Auth{
		ID:       fileName,
		Provider: "antigravity",
		FileName: fileName,
		Label:    label,
		Metadata: metadata,
	}, nil
}

type callbackResult struct {
	Code  string
	Error string
	State string
}

func startAntigravityCallbackServer() (*http.Server, int, <-chan callbackResult, error) {
	addr := fmt.Sprintf(":%d", antigravityCallbackPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, 0, nil, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	resultCh := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth-callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		res := callbackResult{
			Code:  strings.TrimSpace(q.Get("code")),
			Error: strings.TrimSpace(q.Get("error")),
			State: strings.TrimSpace(q.Get("state")),
		}
		resultCh <- res
		if res.Code != "" && res.Error == "" {
			_, _ = w.Write([]byte("<h1>Login successful</h1><p>You can close this window.</p>"))
		} else {
			_, _ = w.Write([]byte("<h1>Login failed</h1><p>Please check the CLI output.</p>"))
		}
	})

	srv := &http.Server{Handler: mux}
	go func() {
		if errServe := srv.Serve(listener); errServe != nil && !strings.Contains(errServe.Error(), "Server closed") {
			log.Warnf("antigravity callback server error: %v", errServe)
		}
	}()

	return srv, port, resultCh, nil
}

type antigravityTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

func exchangeAntigravityCode(ctx context.Context, code, redirectURI string, httpClient *http.Client) (*antigravityTokenResponse, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", antigravityClientID)
	data.Set("client_secret", antigravityClientSecret)
	data.Set("redirect_uri", redirectURI)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, errDo := httpClient.Do(req)
	if errDo != nil {
		return nil, errDo
	}
	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			log.Errorf("antigravity token exchange: close body error: %v", errClose)
		}
	}()

	var token antigravityTokenResponse
	if errDecode := json.NewDecoder(resp.Body).Decode(&token); errDecode != nil {
		return nil, errDecode
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("oauth token exchange failed: status %d", resp.StatusCode)
	}
	return &token, nil
}

type antigravityUserInfo struct {
	Email string `json:"email"`
}

func fetchAntigravityUserInfo(ctx context.Context, accessToken string, httpClient *http.Client) (*antigravityUserInfo, error) {
	if strings.TrimSpace(accessToken) == "" {
		return &antigravityUserInfo{}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v1/userinfo?alt=json", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, errDo := httpClient.Do(req)
	if errDo != nil {
		return nil, errDo
	}
	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			log.Errorf("antigravity userinfo: close body error: %v", errClose)
		}
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &antigravityUserInfo{}, nil
	}
	var info antigravityUserInfo
	if errDecode := json.NewDecoder(resp.Body).Decode(&info); errDecode != nil {
		return nil, errDecode
	}
	return &info, nil
}

func buildAntigravityAuthURL(redirectURI, state string) string {
	params := url.Values{}
	params.Set("access_type", "offline")
	params.Set("client_id", antigravityClientID)
	params.Set("prompt", "consent")
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", strings.Join(antigravityScopes, " "))
	params.Set("state", state)
	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}

func sanitizeAntigravityFileName(email string) string {
	if strings.TrimSpace(email) == "" {
		return "antigravity.json"
	}
	replacer := strings.NewReplacer("@", "_", ".", "_")
	return fmt.Sprintf("antigravity-%s.json", replacer.Replace(email))
}

// Antigravity API constants for project discovery
const (
	antigravityAPIEndpoint    = "https://cloudcode-pa.googleapis.com"
	antigravityAPIVersion     = "v1internal"
	antigravityAPIUserAgent   = "google-api-nodejs-client/9.15.1"
	antigravityAPIClient      = "google-cloud-sdk vscode_cloudshelleditor/0.1"
	antigravityClientMetadata = `{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}`
)

// AntigravityAccountInfo captures metadata returned by loadCodeAssist.
type AntigravityAccountInfo struct {
	ProjectID    string
	TierID       string
	AllowedTiers []map[string]any
}

// FetchAntigravityProjectID exposes project discovery for external callers.
func FetchAntigravityProjectID(ctx context.Context, accessToken string, httpClient *http.Client) (string, error) {
	info, err := fetchAntigravityAccountInfo(ctx, accessToken, httpClient)
	if err != nil {
		return "", err
	}
	return info.ProjectID, nil
}

// FetchAntigravityAccountInfo exposes project and tier discovery for external callers.
func FetchAntigravityAccountInfo(ctx context.Context, accessToken string, httpClient *http.Client) (*AntigravityAccountInfo, error) {
	return fetchAntigravityAccountInfo(ctx, accessToken, httpClient)
}

// fetchAntigravityAccountInfo retrieves the project/tier metadata for the authenticated user via loadCodeAssist.
// This uses the same approach as Gemini CLI to get the cloudaicompanionProject.
func fetchAntigravityAccountInfo(ctx context.Context, accessToken string, httpClient *http.Client) (*AntigravityAccountInfo, error) {
	// Call loadCodeAssist to get the project
	loadReqBody := map[string]any{
		"metadata": map[string]string{
			"ideType":    "IDE_UNSPECIFIED",
			"platform":   "PLATFORM_UNSPECIFIED",
			"pluginType": "GEMINI",
		},
	}

	rawBody, errMarshal := json.Marshal(loadReqBody)
	if errMarshal != nil {
		return nil, fmt.Errorf("marshal request body: %w", errMarshal)
	}

	endpointURL := fmt.Sprintf("%s/%s:loadCodeAssist", antigravityAPIEndpoint, antigravityAPIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, strings.NewReader(string(rawBody)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", antigravityAPIUserAgent)
	req.Header.Set("X-Goog-Api-Client", antigravityAPIClient)
	req.Header.Set("Client-Metadata", antigravityClientMetadata)

	resp, errDo := httpClient.Do(req)
	if errDo != nil {
		return nil, fmt.Errorf("execute request: %w", errDo)
	}
	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			log.Errorf("antigravity loadCodeAssist: close body error: %v", errClose)
		}
	}()

	bodyBytes, errRead := io.ReadAll(resp.Body)
	if errRead != nil {
		return nil, fmt.Errorf("read response: %w", errRead)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var loadResp map[string]any
	if errDecode := json.Unmarshal(bodyBytes, &loadResp); errDecode != nil {
		return nil, fmt.Errorf("decode response: %w", errDecode)
	}

	info := parseAntigravityAccountInfo(loadResp)
	if info.ProjectID == "" {
		return nil, fmt.Errorf("no cloudaicompanionProject in response")
	}

	return info, nil
}

func parseAntigravityAccountInfo(loadResp map[string]any) *AntigravityAccountInfo {
	info := &AntigravityAccountInfo{TierID: "legacy-tier"}
	if loadResp == nil {
		return info
	}
	if id, ok := loadResp["cloudaicompanionProject"].(string); ok {
		info.ProjectID = strings.TrimSpace(id)
	}
	if info.ProjectID == "" {
		if projectMap, ok := loadResp["cloudaicompanionProject"].(map[string]any); ok {
			if id, okID := projectMap["id"].(string); okID {
				info.ProjectID = strings.TrimSpace(id)
			}
		}
	}

	// Check for paidTier first (Google AI Ultra subscription)
	// This takes priority as it indicates the user has a paid subscription
	if paidTier, ok := loadResp["paidTier"].(map[string]any); ok {
		if id, okID := paidTier["id"].(string); okID && strings.TrimSpace(id) != "" {
			info.TierID = strings.TrimSpace(id)
			log.Debugf("antigravity: using paidTier %s", info.TierID)
		}
	}

	// If no paidTier, check currentTier
	if info.TierID == "legacy-tier" {
		if currentTier, ok := loadResp["currentTier"].(map[string]any); ok {
			if id, okID := currentTier["id"].(string); okID && strings.TrimSpace(id) != "" {
				info.TierID = strings.TrimSpace(id)
				log.Debugf("antigravity: using currentTier %s", info.TierID)
			}
		}
	}

	if tiers, okTiers := loadResp["allowedTiers"].([]any); okTiers {
		for _, rawTier := range tiers {
			tier, okTier := rawTier.(map[string]any)
			if !okTier {
				continue
			}
			entry := map[string]any{}
			if id, okID := tier["id"].(string); okID {
				id = strings.TrimSpace(id)
				if id != "" {
					entry["id"] = id
				}
			}
			if name, okName := tier["name"].(string); okName {
				name = strings.TrimSpace(name)
				if name != "" {
					entry["name"] = name
				}
			}
			if display, okDisplay := tier["displayName"].(string); okDisplay {
				display = strings.TrimSpace(display)
				if display != "" {
					entry["display_name"] = display
				}
			}
			if isDefault, okDefault := tier["isDefault"].(bool); okDefault {
				entry["is_default"] = isDefault
				// Only use isDefault tier if we haven't found a better tier
				if isDefault && info.TierID == "legacy-tier" {
					if id, okID := entry["id"].(string); okID && id != "" {
						info.TierID = id
					}
				}
			}
			if len(entry) > 0 {
				info.AllowedTiers = append(info.AllowedTiers, entry)
			}
		}
	}
	return info
}

// ImportFromAntigravityIDE imports token from Antigravity IDE's token file.
// This is useful for users who have already logged in via Antigravity IDE
// and want to use the same credentials in ProxyPilot.
//
// Parameters:
//   - ctx: The context for the operation
//   - cfg: The application configuration
//
// Returns:
//   - *coreauth.Auth: The imported auth record
//   - error: An error if the import fails
func (AntigravityAuthenticator) ImportFromAntigravityIDE(ctx context.Context, cfg *config.Config) (*coreauth.Auth, error) {
	// Load token from Antigravity IDE
	tokenData, err := antigravityauth.LoadAntigravityToken()
	if err != nil {
		return nil, fmt.Errorf("failed to load Antigravity IDE token: %w", err)
	}

	// Get email from token or fetch from API
	email := tokenData.Email
	projectID := tokenData.ProjectID
	tierID := ""
	var allowedTiers []map[string]any

	// If email is not in token, try to fetch user info
	if email == "" && tokenData.GetAccessToken() != "" {
		httpClient := util.SetProxy(&cfg.SDKConfig, &http.Client{})
		if info, errInfo := fetchAntigravityUserInfo(ctx, tokenData.GetAccessToken(), httpClient); errInfo == nil && strings.TrimSpace(info.Email) != "" {
			email = strings.TrimSpace(info.Email)
		}
	}

	// Try to fetch project/tier info if not in token
	if projectID == "" && tokenData.GetAccessToken() != "" {
		httpClient := util.SetProxy(&cfg.SDKConfig, &http.Client{})
		if accountInfo, errProject := fetchAntigravityAccountInfo(ctx, tokenData.GetAccessToken(), httpClient); errProject == nil && accountInfo != nil {
			projectID = accountInfo.ProjectID
			tierID = accountInfo.TierID
			allowedTiers = accountInfo.AllowedTiers
		}
	}

	// Parse expiry time
	expiresAt := tokenData.GetExpiry()
	if expiresAt.IsZero() {
		// Default to 1 hour from now if no expiry
		expiresAt = time.Now().Add(1 * time.Hour)
	}

	// Calculate expires_in (seconds until expiry)
	expiresIn := int64(time.Until(expiresAt).Seconds())
	if expiresIn < 0 {
		expiresIn = 0
	}

	now := time.Now()
	metadata := map[string]any{
		"type":          "antigravity",
		"access_token":  tokenData.GetAccessToken(),
		"refresh_token": tokenData.GetRefreshToken(),
		"expires_in":    expiresIn,
		"timestamp":     now.UnixMilli(),
		"expired":       expiresAt.Format(time.RFC3339),
	}
	if email != "" {
		metadata["email"] = email
	}
	if projectID != "" {
		metadata["project_id"] = projectID
	}
	if tierID != "" {
		metadata["tier_id"] = tierID
	}
	if len(allowedTiers) > 0 {
		metadata["allowed_tiers"] = allowedTiers
	}

	// Add client credentials if available for token refresh
	if tokenData.Token != nil {
		if tokenData.Token.ClientID != "" {
			metadata["client_id"] = tokenData.Token.ClientID
		}
		if tokenData.Token.ClientSecret != "" {
			metadata["client_secret"] = tokenData.Token.ClientSecret
		}
	}

	fileName := sanitizeAntigravityFileName(email)
	label := email
	if label == "" {
		label = "antigravity"
	}

	// Check if token is expired and warn user
	if tokenData.IsExpired() {
		fmt.Println("Warning: The imported token is expired. ProxyPilot will attempt to refresh it on first use.")
	}

	record := &coreauth.Auth{
		ID:        fileName,
		Provider:  "antigravity",
		FileName:  fileName,
		Label:     label,
		Status:    coreauth.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  metadata,
		Attributes: map[string]string{
			"source": "antigravity-ide-import",
			"email":  email,
		},
		// NextRefreshAfter is set to 5 minutes before expiry
		NextRefreshAfter: expiresAt.Add(-5 * time.Minute),
	}

	// Display success message
	if email != "" {
		fmt.Printf("\n✓ Imported Antigravity token from Antigravity IDE (Account: %s)\n", email)
	} else {
		fmt.Println("\n✓ Imported Antigravity token from Antigravity IDE")
	}
	if projectID != "" {
		fmt.Printf("  Project: %s\n", projectID)
	}

	return record, nil
}
