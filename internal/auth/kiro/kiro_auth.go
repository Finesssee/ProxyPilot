// Package kiro provides OAuth2 authentication functionality for AWS CodeWhisperer (Kiro) API.
// This package implements the complete OAuth2 flow with PKCE (Proof Key for Code Exchange)
// for secure authentication with Kiro API, including token exchange, refresh, and storage.
package kiro

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	log "github.com/sirupsen/logrus"
)

const (
	// awsSSOOIDCEndpoint is the AWS SSO OIDC service endpoint
	awsSSOOIDCEndpoint = "https://oidc.us-east-1.amazonaws.com"
	// kiroAPIEndpoint is the Kiro API endpoint
	kiroAPIEndpoint = "https://q.us-east-1.amazonaws.com"
	// kiroClientName is the client name for registration
	kiroClientName = "Kiro"
	// defaultRegion is the default AWS region for Kiro services
	defaultRegion = "us-east-1"
	// googleAuthURL is the Google OAuth authorization endpoint
	googleAuthURL = "https://accounts.google.com/o/oauth2/v2/auth"
	// googleTokenURL is the Google OAuth token endpoint
	googleTokenURL = "https://oauth2.googleapis.com/token"
	// googleClientID is the Kiro Google OAuth client ID
	googleClientID = "kiro-google-client-id"
	// redirectURI is the OAuth callback URL
	redirectURI = "http://localhost:1455/auth/callback"
)

// registerClientResponse represents the response from AWS SSO OIDC client registration.
type registerClientResponse struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	ExpiresAt    int64  `json:"clientSecretExpiresAt"`
}

// startDeviceAuthResponse represents the response from AWS SSO OIDC device authorization start.
type startDeviceAuthResponse struct {
	DeviceCode              string `json:"deviceCode"`
	UserCode                string `json:"userCode"`
	VerificationURI         string `json:"verificationUri"`
	VerificationURIComplete string `json:"verificationUriComplete"`
	ExpiresIn               int    `json:"expiresIn"`
	Interval                int    `json:"interval"`
}

// tokenResponse represents the response structure from AWS SSO OIDC token endpoint.
type tokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	TokenType    string `json:"tokenType"`
	ExpiresIn    int    `json:"expiresIn"`
	IDToken      string `json:"idToken,omitempty"`
}

// KiroAuth handles the AWS Kiro authentication flows.
// It manages the HTTP client and provides methods for both AWS Builder ID
// device authorization and Google OAuth flows.
type KiroAuth struct {
	httpClient *http.Client
	cfg        *config.Config
}

// NewKiroAuth creates a new KiroAuth service instance.
// It initializes an HTTP client with proxy settings from the provided configuration.
func NewKiroAuth(cfg *config.Config) *KiroAuth {
	return &KiroAuth{
		httpClient: util.SetProxy(&cfg.SDKConfig, &http.Client{}),
		cfg:        cfg,
	}
}

// RegisterClient registers a new OAuth client with AWS SSO OIDC.
// This is the first step in the device authorization flow.
//
// Parameters:
//   - ctx: The context for the request
//   - clientName: The name of the client application
//   - clientType: The type of client (e.g., "public")
//
// Returns:
//   - *registerClientResponse: The client registration response with client ID and secret
//   - error: An error if registration fails
func (k *KiroAuth) RegisterClient(ctx context.Context, clientName, clientType string) (*registerClientResponse, error) {
	endpoint := fmt.Sprintf("%s/client/register", awsSSOOIDCEndpoint)

	reqBody := map[string]interface{}{
		"clientName": clientName,
		"clientType": clientType,
		"scopes":     []string{"sso:account:access"},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create registration request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client registration request failed: %w", err)
	}
	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			log.Errorf("failed to close response body: %v", errClose)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read registration response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("client registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	var regResp registerClientResponse
	if err = json.Unmarshal(body, &regResp); err != nil {
		return nil, fmt.Errorf("failed to parse registration response: %w", err)
	}

	return &regResp, nil
}

// StartDeviceAuthorization initiates the AWS Builder ID device authorization flow.
// It requests a device code and user code from AWS SSO OIDC.
func (k *KiroAuth) StartDeviceAuthorization(ctx context.Context, clientInfo *SSOOIDCClientInfo) (*DeviceAuthorizationResponse, error) {
	authURL := fmt.Sprintf("%s/device_authorization", awsSSOOIDCEndpoint)

	data := url.Values{
		"client_id":     {clientInfo.ClientID},
		"client_secret": {clientInfo.ClientSecret},
		"start_url":     {"https://view.awsapps.com/start"},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", authURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create device authorization request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device authorization request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read device authorization response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device authorization failed with status %d: %s", resp.StatusCode, string(body))
	}

	var authResp DeviceAuthorizationResponse
	if err = json.Unmarshal(body, &authResp); err != nil {
		return nil, fmt.Errorf("failed to parse device authorization response: %w", err)
	}

	return &authResp, nil
}

// PollDeviceToken polls AWS SSO OIDC for the access token after user authorization.
// This is called repeatedly until the user completes the device authorization flow.
func (k *KiroAuth) PollDeviceToken(ctx context.Context, clientInfo *SSOOIDCClientInfo, deviceCode string) (*KiroTokenData, error) {
	tokenURL := fmt.Sprintf("%s/token", awsSSOOIDCEndpoint)

	data := url.Values{
		"client_id":     {clientInfo.ClientID},
		"client_secret": {clientInfo.ClientSecret},
		"grant_type":    {"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code":   {deviceCode},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Parse error response
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if err = json.Unmarshal(body, &errResp); err == nil {
			switch errResp.Error {
			case "authorization_pending":
				return nil, ErrAuthorizationPending
			case "slow_down":
				return nil, ErrSlowDown
			case "expired_token":
				return nil, ErrExpiredToken
			default:
				return nil, fmt.Errorf("token request failed: %s - %s", errResp.Error, errResp.ErrorDescription)
			}
		}
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err = json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339)

	return &KiroTokenData{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		IDToken:      tokenResp.IDToken,
		ExpiresAt:    expiresAt,
		AuthMethod:   "builder_id",
		Provider:     "aws",
		ClientID:     clientInfo.ClientID,
		ClientSecret: clientInfo.ClientSecret,
	}, nil
}

// RefreshTokens refreshes an access token using a refresh token.
// This method handles both AWS Builder ID and Google OAuth token refresh flows.
func (k *KiroAuth) RefreshTokens(ctx context.Context, tokenData *KiroTokenData) (*KiroTokenData, error) {
	if tokenData == nil {
		return nil, fmt.Errorf("token data is required")
	}

	if tokenData.RefreshToken == "" {
		return nil, fmt.Errorf("refresh token is required")
	}

	// Route to appropriate refresh method based on auth method
	switch tokenData.AuthMethod {
	case "builder_id":
		return k.refreshBuilderIDTokens(ctx, tokenData)
	case "google":
		return k.refreshGoogleTokens(ctx, tokenData)
	default:
		return nil, fmt.Errorf("unknown auth method: %s", tokenData.AuthMethod)
	}
}

// refreshBuilderIDTokens refreshes tokens for AWS Builder ID authentication.
func (k *KiroAuth) refreshBuilderIDTokens(ctx context.Context, tokenData *KiroTokenData) (*KiroTokenData, error) {
	if tokenData.ClientID == "" || tokenData.ClientSecret == "" {
		return nil, fmt.Errorf("client credentials are required for Builder ID token refresh")
	}

	tokenURL := fmt.Sprintf("%s/token", awsSSOOIDCEndpoint)

	data := url.Values{
		"client_id":     {tokenData.ClientID},
		"client_secret": {tokenData.ClientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {tokenData.RefreshToken},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token refresh request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err = json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse refresh response: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339)

	return &KiroTokenData{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		IDToken:      tokenResp.IDToken,
		Email:        tokenData.Email,
		ExpiresAt:    expiresAt,
		AuthMethod:   "builder_id",
		Provider:     "aws",
		ClientID:     tokenData.ClientID,
		ClientSecret: tokenData.ClientSecret,
		ProfileArn:   tokenData.ProfileArn,
	}, nil
}

// refreshGoogleTokens refreshes tokens for Google OAuth authentication.
func (k *KiroAuth) refreshGoogleTokens(ctx context.Context, tokenData *KiroTokenData) (*KiroTokenData, error) {
	data := url.Values{
		"client_id":     {googleClientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {tokenData.RefreshToken},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", googleTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token refresh request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err = json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse refresh response: %w", err)
	}

	// Use the new refresh token if provided, otherwise keep the old one
	refreshToken := tokenResp.RefreshToken
	if refreshToken == "" {
		refreshToken = tokenData.RefreshToken
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339)

	return &KiroTokenData{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: refreshToken,
		IDToken:      tokenResp.IDToken,
		Email:        tokenData.Email,
		ExpiresAt:    expiresAt,
		AuthMethod:   "google",
		Provider:     "google",
	}, nil
}

// NewSSOOIDCClient creates a new AWS SSO OIDC client registration.
// This is the first step in the AWS Builder ID device authorization flow.
// It registers the client application with AWS SSO OIDC and returns client credentials.
func (k *KiroAuth) NewSSOOIDCClient(ctx context.Context) (*SSOOIDCClientInfo, error) {
	regResp, err := k.RegisterClient(ctx, kiroClientName, "public")
	if err != nil {
		return nil, err
	}
	return &SSOOIDCClientInfo{
		ClientID:              regResp.ClientID,
		ClientSecret:          regResp.ClientSecret,
		ClientIDIssuedAt:      0, // Set from regResp if available
		ClientSecretExpiresAt: regResp.ExpiresAt,
	}, nil
}

// NewKiroOAuth creates a Google OAuth flow for Kiro authentication.
// It generates the authorization URL with PKCE for secure authentication.
func (k *KiroAuth) NewKiroOAuth(state string, pkceCodes *PKCECodes) (string, error) {
	if pkceCodes == nil {
		return "", fmt.Errorf("PKCE codes are required")
	}

	params := url.Values{
		"client_id":             {googleClientID},
		"response_type":         {"code"},
		"redirect_uri":          {redirectURI},
		"scope":                 {"openid email profile"},
		"state":                 {state},
		"code_challenge":        {pkceCodes.CodeChallenge},
		"code_challenge_method": {"S256"},
		"prompt":                {"select_account"},
		"access_type":           {"offline"},
	}

	authURL := fmt.Sprintf("%s?%s", googleAuthURL, params.Encode())
	return authURL, nil
}

// ExchangeCodeForTokens exchanges an authorization code for access and refresh tokens.
// This is used in the Google OAuth flow after the user completes authorization.
func (k *KiroAuth) ExchangeCodeForTokens(ctx context.Context, code string, pkceCodes *PKCECodes) (*KiroAuthBundle, error) {
	if pkceCodes == nil {
		return nil, fmt.Errorf("PKCE codes are required for token exchange")
	}

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {googleClientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {pkceCodes.CodeVerifier},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", googleTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err = json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// TODO: Parse ID token to extract email and other claims
	email := ""
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339)

	tokenData := KiroTokenData{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		IDToken:      tokenResp.IDToken,
		Email:        email,
		ExpiresAt:    expiresAt,
		AuthMethod:   "google",
		Provider:     "google",
	}

	bundle := &KiroAuthBundle{
		TokenData:   tokenData,
		LastRefresh: time.Now().Format(time.RFC3339),
	}

	return bundle, nil
}

// CreateTokenStorage creates a new KiroTokenStorage from a KiroAuthBundle.
// It populates the storage struct with token data, user information, and timestamps.
func (k *KiroAuth) CreateTokenStorage(bundle *KiroAuthBundle) *KiroTokenStorage {
	storage := &KiroTokenStorage{
		Type:         "kiro",
		AccessToken:  bundle.TokenData.AccessToken,
		RefreshToken: bundle.TokenData.RefreshToken,
		IDToken:      bundle.TokenData.IDToken,
		Email:        bundle.TokenData.Email,
		ExpiresAt:    bundle.TokenData.ExpiresAt,
		AuthMethod:   bundle.TokenData.AuthMethod,
		ProfileArn:   bundle.TokenData.ProfileArn,
		ClientID:     bundle.TokenData.ClientID,
		ClientSecret: bundle.TokenData.ClientSecret,
		Provider:     bundle.TokenData.Provider,
		LastRefresh:  bundle.LastRefresh,
	}

	return storage
}

// RefreshTokensWithRetry refreshes tokens with a built-in retry mechanism.
// It attempts to refresh the tokens up to a specified maximum number of retries,
// with an exponential backoff strategy to handle transient network errors.
func (k *KiroAuth) RefreshTokensWithRetry(ctx context.Context, tokenData *KiroTokenData, maxRetries int) (*KiroTokenData, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}

		refreshedTokenData, err := k.RefreshTokens(ctx, tokenData)
		if err == nil {
			return refreshedTokenData, nil
		}

		lastErr = err
		log.Warnf("Token refresh attempt %d failed: %v", attempt+1, err)
	}

	return nil, fmt.Errorf("token refresh failed after %d attempts: %w", maxRetries, lastErr)
}

// UpdateTokenStorage updates an existing KiroTokenStorage with new token data.
// This is typically called after a successful token refresh to persist the new credentials.
func (k *KiroAuth) UpdateTokenStorage(storage *KiroTokenStorage, tokenData *KiroTokenData) {
	storage.AccessToken = tokenData.AccessToken
	storage.RefreshToken = tokenData.RefreshToken
	storage.IDToken = tokenData.IDToken
	storage.Email = tokenData.Email
	storage.ExpiresAt = tokenData.ExpiresAt
	storage.AuthMethod = tokenData.AuthMethod
	storage.ProfileArn = tokenData.ProfileArn
	storage.ClientID = tokenData.ClientID
	storage.ClientSecret = tokenData.ClientSecret
	storage.Provider = tokenData.Provider
	storage.LastRefresh = time.Now().Format(time.RFC3339)
}

// SSOOIDCClientInfo holds the AWS SSO OIDC client registration information.
type SSOOIDCClientInfo struct {
	ClientID              string `json:"clientId"`
	ClientSecret          string `json:"clientSecret"`
	ClientIDIssuedAt      int64  `json:"clientIdIssuedAt"`
	ClientSecretExpiresAt int64  `json:"clientSecretExpiresAt"`
}

// DeviceAuthorizationResponse holds the response from AWS SSO OIDC device authorization.
type DeviceAuthorizationResponse struct {
	DeviceCode              string `json:"deviceCode"`
	UserCode                string `json:"userCode"`
	VerificationURI         string `json:"verificationUri"`
	VerificationURIComplete string `json:"verificationUriComplete"`
	ExpiresIn               int    `json:"expiresIn"`
	Interval                int    `json:"interval"`
}
