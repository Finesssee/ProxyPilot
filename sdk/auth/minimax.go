package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

// MiniMaxAuthenticator implements API key authentication for MiniMax.
type MiniMaxAuthenticator struct{}

// NewMiniMaxAuthenticator constructs a MiniMax authenticator.
func NewMiniMaxAuthenticator() *MiniMaxAuthenticator {
	return &MiniMaxAuthenticator{}
}

func (a *MiniMaxAuthenticator) Provider() string {
	return "minimax"
}

func (a *MiniMaxAuthenticator) RefreshLead() *time.Duration {
	// API keys don't need refresh
	return nil
}

func (a *MiniMaxAuthenticator) Login(ctx context.Context, cfg *config.Config, opts *LoginOptions) (*coreauth.Auth, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cliproxy auth: configuration is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if opts == nil {
		opts = &LoginOptions{}
	}

	var apiKey string
	if opts.Metadata != nil {
		apiKey = opts.Metadata["api_key"]
	}

	if apiKey == "" && opts.Prompt != nil {
		var err error
		apiKey, err = opts.Prompt("Please enter your MiniMax API key:")
		if err != nil {
			return nil, err
		}
	}

	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("minimax: API key is required")
	}

	var label string
	if opts.Metadata != nil {
		label = opts.Metadata["label"]
	}
	if label == "" && opts.Prompt != nil {
		var err error
		label, err = opts.Prompt("Please enter a label for this API key (optional, press Enter to skip):")
		if err != nil {
			return nil, err
		}
	}
	label = strings.TrimSpace(label)
	if label == "" {
		label = fmt.Sprintf("minimax-%d", time.Now().UnixMilli())
	}

	fileName := fmt.Sprintf("minimax-%s.json", label)
	metadata := map[string]any{
		"api_key":    apiKey,
		"label":      label,
		"type":       "minimax",
		"created_at": time.Now().Format(time.RFC3339),
	}

	fmt.Println("MiniMax API key saved successfully")

	return &coreauth.Auth{
		ID:         fileName,
		Provider:   a.Provider(),
		FileName:   fileName,
		Metadata:   metadata,
		Attributes: map[string]string{"api_key": apiKey},
	}, nil
}
