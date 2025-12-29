package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
)

// DoZhipuLogin handles Zhipu AI API key authentication.
// It prompts for an API key and saves it to the configured auth directory.
//
// Parameters:
//   - cfg: The application configuration
//   - options: Login options including prompts
func DoZhipuLogin(cfg *config.Config, options *LoginOptions) {
	if options == nil {
		options = &LoginOptions{}
	}

	manager := newAuthManager()

	promptFn := options.Prompt
	if promptFn == nil {
		promptFn = func(prompt string) (string, error) {
			fmt.Println()
			fmt.Println(prompt)
			reader := bufio.NewReader(os.Stdin)
			value, err := reader.ReadString('\n')
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(value), nil
		}
	}

	authOpts := &sdkAuth.LoginOptions{
		Metadata: map[string]string{},
		Prompt:   promptFn,
	}

	_, savedPath, err := manager.Login(context.Background(), "zhipu", cfg, authOpts)
	if err != nil {
		fmt.Printf("Zhipu authentication failed: %v\n", err)
		return
	}

	if savedPath != "" {
		fmt.Printf("Authentication saved to %s\n", savedPath)
	}

	fmt.Println("Zhipu AI API key saved successfully!")
}
