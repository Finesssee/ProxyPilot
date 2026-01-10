package cmd

import (
	"context"
	"fmt"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

// DoAntigravityLogin triggers the OAuth flow for the antigravity provider and saves tokens.
func DoAntigravityLogin(cfg *config.Config, options *LoginOptions) {
	if options == nil {
		options = &LoginOptions{}
	}

	promptFn := options.Prompt
	if promptFn == nil {
		promptFn = defaultProjectPrompt()
	}

	manager := newAuthManager()
	authOpts := &sdkAuth.LoginOptions{
		NoBrowser: options.NoBrowser,
		Metadata:  map[string]string{},
		Prompt:    promptFn,
	}

	record, savedPath, err := manager.Login(context.Background(), "antigravity", cfg, authOpts)
	if err != nil {
		log.Errorf("Antigravity authentication failed: %v", err)
		return
	}

	if savedPath != "" {
		fmt.Printf("Authentication saved to %s\n", savedPath)
	}
	if record != nil && record.Label != "" {
		fmt.Printf("Authenticated as %s\n", record.Label)
	}
	fmt.Println("Antigravity authentication successful!")
}

// DoAntigravityImport imports tokens from Antigravity IDE's storage
// and saves them as Antigravity credentials for use with ProxyPilot.
func DoAntigravityImport(cfg *config.Config) {
	manager := newAuthManager()

	authenticator := sdkAuth.NewAntigravityAuthenticator()
	auth, ok := authenticator.(interface {
		ImportFromAntigravityIDE(ctx context.Context, cfg *config.Config) (*coreauth.Auth, error)
	})
	if !ok {
		log.Error("Antigravity authenticator does not support ImportFromAntigravityIDE")
		return
	}

	record, err := auth.ImportFromAntigravityIDE(context.Background(), cfg)
	if err != nil {
		log.Errorf("Failed to import from Antigravity IDE: %v", err)
		return
	}

	savedPath, errSave := manager.SaveAuth(record, cfg)
	if errSave != nil {
		log.Errorf("Failed to save imported token: %v", errSave)
		return
	}

	fmt.Printf("Token saved to %s\n", savedPath)
	fmt.Println("You can now use Antigravity provider with your Antigravity IDE credentials!")
}
