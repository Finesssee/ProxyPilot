package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	log "github.com/sirupsen/logrus"
)

// DoKiroLogin triggers the Kiro (AWS CodeWhisperer) OAuth flow through the shared authentication manager.
// It initiates the AWS SSO OIDC device authorization flow for Kiro services and saves
// the authentication tokens to the configured auth directory.
//
// Parameters:
//   - cfg: The application configuration
//   - options: Login options including browser behavior and prompts
func DoKiroLogin(cfg *config.Config, options *LoginOptions) {
	if options == nil {
		options = &LoginOptions{}
	}

	manager := newAuthManager()

	authOpts := &sdkAuth.LoginOptions{
		NoBrowser: options.NoBrowser,
		Metadata:  map[string]string{},
		Prompt:    options.Prompt,
	}

	_, savedPath, err := manager.Login(context.Background(), "kiro", cfg, authOpts)
	if err != nil {
		var authErr *kiro.AuthenticationError
		if errors.As(err, &authErr) {
			log.Error(kiro.GetUserFriendlyMessage(authErr))
			if authErr.Type == kiro.ErrServerStartFailed.Type {
				os.Exit(kiro.ErrServerStartFailed.Code)
			}
			return
		}
		fmt.Printf("Kiro authentication failed: %v\n", err)
		return
	}

	if savedPath != "" {
		fmt.Printf("Authentication saved to %s\n", savedPath)
	}
	fmt.Println("Kiro authentication successful!")
}
