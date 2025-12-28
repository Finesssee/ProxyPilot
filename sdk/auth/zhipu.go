package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

// ZhipuAuthenticator implements API key authentication for Zhipu AI (GLM models).
type ZhipuAuthenticator struct{}

// NewZhipuAuthenticator constructs a Zhipu AI authenticator.
func NewZhipuAuthenticator() *ZhipuAuthenticator {
	return &ZhipuAuthenticator{}
}

func (a *ZhipuAuthenticator) Provider() string {
	return "zhipu"
}

func (a *ZhipuAuthenticator) RefreshLead() *time.Duration {
	// API keys don't need refresh
	return nil
}

func (a *ZhipuAuthenticator) Login(ctx context.Context, cfg *config.Config, opts *LoginOptions) (*coreauth.Auth, error) {
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
		apiKey, err = opts.Prompt("Please enter your Zhipu AI API key:")
		if err != nil {
			return nil, err
		}
	}

	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("zhipu: API key is required")
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
		label = fmt.Sprintf("zhipu-%d", time.Now().UnixMilli())
	}

	fileName := fmt.Sprintf("zhipu-%s.json", label)
	metadata := map[string]any{
		"api_key":    apiKey,
		"label":      label,
		"type":       "zhipu",
		"created_at": time.Now().Format(time.RFC3339),
	}

	fmt.Println("Zhipu AI API key saved successfully")

	return &coreauth.Auth{
		ID:         fileName,
		Provider:   a.Provider(),
		FileName:   fileName,
		Metadata:   metadata,
		Attributes: map[string]string{"api_key": apiKey},
	}, nil
}
