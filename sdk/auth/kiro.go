package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

const (
	kiroDeviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"
	kiroClientName      = "kiro-cli-proxy"
	kiroClientType      = "public"
)

// KiroAuthenticator implements the AWS SSO OIDC device authorization flow for Kiro accounts.
type KiroAuthenticator struct {
	// PollInterval is the interval between token polling attempts (default: 5 seconds)
	PollInterval time.Duration
	// PollTimeout is the maximum time to wait for user authorization (default: 5 minutes)
	PollTimeout time.Duration
}

// NewKiroAuthenticator constructs a Kiro authenticator with default settings.
func NewKiroAuthenticator() *KiroAuthenticator {
	return &KiroAuthenticator{
		PollInterval: 5 * time.Second,
		PollTimeout:  5 * time.Minute,
	}
}

func (a *KiroAuthenticator) Provider() string {
	return "kiro"
}

func (a *KiroAuthenticator) RefreshLead() *time.Duration {
	d := 24 * time.Hour
	return &d
}

func (a *KiroAuthenticator) Login(ctx context.Context, cfg *config.Config, opts *LoginOptions) (*coreauth.Auth, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cliproxy auth: configuration is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if opts == nil {
		opts = &LoginOptions{}
	}

	authSvc := kiro.NewKiroAuth(cfg)

	// Step 1: Register client
	fmt.Println("Registering Kiro client...")
	clientInfo, err := authSvc.NewSSOOIDCClient(ctx)
	if err != nil {
		return nil, kiro.NewAuthenticationError(kiro.ErrCodeExchangeFailed, fmt.Errorf("client registration failed: %w", err))
	}

	// Step 2: Start device authorization
	fmt.Println("Starting device authorization...")
	deviceResp, err := authSvc.StartDeviceAuthorization(ctx, clientInfo)
	if err != nil {
		return nil, kiro.NewAuthenticationError(kiro.ErrCodeExchangeFailed, fmt.Errorf("device authorization failed: %w", err))
	}

	// Step 3: Display authorization instructions to user
	fmt.Println("\n=== Kiro Authentication ===")
	fmt.Printf("Please visit: %s\n", deviceResp.VerificationURI)
	fmt.Printf("And enter code: %s\n", deviceResp.UserCode)
	if deviceResp.VerificationURIComplete != "" {
		fmt.Printf("\nOr visit this URL directly:\n%s\n", deviceResp.VerificationURIComplete)
	}
	fmt.Println("\nWaiting for authorization...")

	// Step 4: Poll for token
	pollInterval := a.PollInterval
	if deviceResp.Interval > 0 {
		pollInterval = time.Duration(deviceResp.Interval) * time.Second
	}

	timeout := a.PollTimeout
	if deviceResp.ExpiresIn > 0 {
		timeout = time.Duration(deviceResp.ExpiresIn) * time.Second
	}

	deadline := time.Now().Add(timeout)
	var tokenResp *kiro.KiroTokenData

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}

		resp, pollErr := authSvc.PollDeviceToken(ctx, clientInfo, deviceResp.DeviceCode)
		if pollErr != nil {
			if pollErr == kiro.ErrAuthorizationPending {
				log.Debug("Authorization pending, continuing to poll...")
				continue
			}
			// Other errors are fatal
			return nil, kiro.NewAuthenticationError(kiro.ErrCodeExchangeFailed, pollErr)
		}

		// Success - use token data directly
		tokenResp = resp
		break
	}

	if tokenResp == nil {
		return nil, kiro.NewAuthenticationError(kiro.ErrCallbackTimeout, fmt.Errorf("authorization timed out"))
	}

	// Create auth bundle and storage
	bundle := &kiro.KiroAuthBundle{
		TokenData:   *tokenResp,
		LastRefresh: time.Now().Format(time.RFC3339),
	}

	tokenStorage := authSvc.CreateTokenStorage(bundle)

	// Generate a filename based on auth method
	fileName := "kiro-builder-id.json"
	if tokenStorage.Email != "" {
		fileName = fmt.Sprintf("kiro-%s.json", tokenStorage.Email)
	}

	metadata := map[string]any{
		"auth_method":   "builder_id",
		"client_id":     clientInfo.ClientID,
		"client_secret": clientInfo.ClientSecret,
	}
	if tokenStorage.Email != "" {
		metadata["email"] = tokenStorage.Email
	}

	fmt.Println("\nKiro authentication successful!")

	return &coreauth.Auth{
		ID:       fileName,
		Provider: a.Provider(),
		FileName: fileName,
		Storage:  tokenStorage,
		Metadata: metadata,
	}, nil
}
