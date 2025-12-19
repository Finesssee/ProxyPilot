package kiro

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/browser"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	"golang.org/x/oauth2"
)

const (
	// Kiro uses a specific client ID for AWS Builder ID / CodeWhisperer integration (placeholder)
	// In a real scenario, this would be the actual Client ID for Kiro.
	kiroClientID     = "kiro-client-id-placeholder"
	kiroClientSecret = "kiro-client-secret-placeholder"
	kiroAuthURL      = "https://kiro.ai/auth"  // Placeholder
	kiroTokenURL     = "https://kiro.ai/token" // Placeholder
)

type KiroTokenStorage struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Expiry       string `json:"expiry"`
	TokenType    string `json:"token_type"`
}

// SaveTokenToFile persists authentication tokens to the specified file path.
func (ts *KiroTokenStorage) SaveTokenToFile(authFilePath string) error {
	// Logic to save token to file would go here if needed,
	// but the core auth manager handles persistence via the Storage field
	// being marshalled into the Auth struct.
	// This method satisfies the TokenStorage interface.
	return nil
}

type KiroAuth struct{}

func NewKiroAuth() *KiroAuth {
	return &KiroAuth{}
}

func (k *KiroAuth) GetAuthenticatedClient(ctx context.Context, ts *KiroTokenStorage, cfg *config.Config, noBrowser ...bool) (*http.Client, error) {
	// Setup OAuth config
	conf := &oauth2.Config{
		ClientID:     kiroClientID,
		ClientSecret: kiroClientSecret,
		RedirectURL:  "http://localhost:8086/oauth2callback",
		Scopes:       []string{"openid", "profile", "email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  kiroAuthURL,
			TokenURL: kiroTokenURL,
		},
	}

	var token *oauth2.Token
	var err error

	// If no token, start flow
	if ts.AccessToken == "" {
		fmt.Printf("Could not load Kiro token, starting OAuth flow.\n")
		token, err = k.getTokenFromWeb(ctx, conf, noBrowser...)
		if err != nil {
			return nil, fmt.Errorf("failed to get token from web: %w", err)
		}

		// Save token back to storage (conceptually - the caller handles persistence typically)
		ts.AccessToken = token.AccessToken
		ts.RefreshToken = token.RefreshToken
		ts.TokenType = token.TokenType
		ts.Expiry = token.Expiry.Format(time.RFC3339)
	} else {
		expiry, _ := time.Parse(time.RFC3339, ts.Expiry)
		token = &oauth2.Token{
			AccessToken:  ts.AccessToken,
			RefreshToken: ts.RefreshToken,
			TokenType:    ts.TokenType,
			Expiry:       expiry,
		}
	}

	return conf.Client(ctx, token), nil
}

func (k *KiroAuth) getTokenFromWeb(ctx context.Context, config *oauth2.Config, noBrowser ...bool) (*oauth2.Token, error) {
	server := NewOAuthServer(8086) // Use port 8086 for Kiro
	if err := server.Start(); err != nil {
		return nil, fmt.Errorf("failed to start local server: %w", err)
	}
	defer server.Stop(ctx)

	state := "state-token-kiro"
	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline)

	if len(noBrowser) == 1 && !noBrowser[0] {
		fmt.Println("Opening browser for Kiro authentication...")
		if !browser.IsAvailable() {
			util.PrintSSHTunnelInstructions(8086)
			fmt.Printf("Please manually open this URL in your browser:\n\n%s\n", authURL)
		} else {
			if err := browser.OpenURL(authURL); err != nil {
				util.PrintSSHTunnelInstructions(8086)
				fmt.Printf("Please manually open this URL in your browser:\n\n%s\n", authURL)
			}
		}
	} else {
		util.PrintSSHTunnelInstructions(8086)
		fmt.Printf("Please open this URL in your browser:\n\n%s\n", authURL)
	}

	result, err := server.WaitForCallback(5 * time.Minute)
	if err != nil {
		return nil, err
	}

	if result.Error != "" {
		return nil, fmt.Errorf("oauth error: %s", result.Error)
	}

	token, err := config.Exchange(ctx, result.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}

	return token, nil
}
