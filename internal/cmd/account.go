// Package cmd provides CLI command implementations for ProxyPilot.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

// ANSI color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

// AccountInfo holds parsed account information for display
type AccountInfo struct {
	ID        string
	Provider  string
	Email     string
	ProjectID string
	ExpiresAt time.Time
	IsExpired bool
	FilePath  string
}

// ListAccounts lists all authenticated accounts with their status
func ListAccounts(jsonOutput bool) error {
	store := sdkAuth.NewFileTokenStore()
	store.SetBaseDir(util.DefaultAuthDir())

	auths, err := store.List(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list accounts: %w", err)
	}

	accounts := parseAccounts(auths)

	if jsonOutput {
		return outputJSON(accounts)
	}

	return outputTable(accounts)
}

// ShowStatus shows a summary of account health
func ShowStatus(jsonOutput bool) error {
	store := sdkAuth.NewFileTokenStore()
	store.SetBaseDir(util.DefaultAuthDir())

	auths, err := store.List(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list accounts: %w", err)
	}

	accounts := parseAccounts(auths)

	// Count by provider and status
	stats := make(map[string]struct{ active, expired int })
	for _, acc := range accounts {
		s := stats[acc.Provider]
		if acc.IsExpired {
			s.expired++
		} else {
			s.active++
		}
		stats[acc.Provider] = s
	}

	if jsonOutput {
		result := map[string]any{
			"total_files":    len(auths),
			"total_accounts": len(accounts),
			"by_provider":    stats,
		}
		return outputJSON(result)
	}

	// Terminal output
	fmt.Printf("\n%s%sProxyPilot Account Status%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s─────────────────────────────%s\n\n", colorDim, colorReset)

	totalActive := 0
	totalExpired := 0

	// Sort providers for consistent output
	providers := make([]string, 0, len(stats))
	for p := range stats {
		providers = append(providers, p)
	}
	sort.Strings(providers)

	for _, provider := range providers {
		s := stats[provider]
		totalActive += s.active
		totalExpired += s.expired

		status := fmt.Sprintf("%s%d active%s", colorGreen, s.active, colorReset)
		if s.expired > 0 {
			status += fmt.Sprintf(", %s%d expired%s", colorRed, s.expired, colorReset)
		}
		fmt.Printf("  %-15s %s\n", provider+":", status)
	}

	fmt.Printf("\n%s─────────────────────────────%s\n", colorDim, colorReset)
	fmt.Printf("  %-15s %s%d active%s", "Total:", colorBold+colorGreen, totalActive, colorReset)
	if totalExpired > 0 {
		fmt.Printf(", %s%d expired%s", colorRed, totalExpired, colorReset)
	}
	fmt.Printf(" (%d files)\n\n", len(auths))

	return nil
}

// CleanupExpired removes all expired auth files
func CleanupExpired(dryRun bool) error {
	store := sdkAuth.NewFileTokenStore()
	store.SetBaseDir(util.DefaultAuthDir())

	auths, err := store.List(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list accounts: %w", err)
	}

	accounts := parseAccounts(auths)
	var expired []AccountInfo

	for _, acc := range accounts {
		if acc.IsExpired {
			expired = append(expired, acc)
		}
	}

	if len(expired) == 0 {
		fmt.Printf("%s✓ No expired accounts found%s\n", colorGreen, colorReset)
		return nil
	}

	fmt.Printf("\n%sExpired accounts:%s\n", colorYellow, colorReset)
	for _, acc := range expired {
		fmt.Printf("  • %s (%s) - expired %s\n",
			acc.Email,
			acc.Provider,
			acc.ExpiresAt.Format("2006-01-02"))
	}

	if dryRun {
		fmt.Printf("\n%s[dry-run] Would remove %d expired account(s)%s\n", colorCyan, len(expired), colorReset)
		return nil
	}

	fmt.Printf("\nRemoving %d expired account(s)...\n", len(expired))

	for _, acc := range expired {
		if err := os.Remove(acc.FilePath); err != nil {
			fmt.Printf("  %s✗ Failed to remove %s: %v%s\n", colorRed, acc.ID, err, colorReset)
		} else {
			fmt.Printf("  %s✓ Removed %s%s\n", colorGreen, acc.ID, colorReset)
		}
	}

	return nil
}

// RemoveAccount removes a specific account by email or filename
func RemoveAccount(identifier string) error {
	store := sdkAuth.NewFileTokenStore()
	store.SetBaseDir(util.DefaultAuthDir())

	auths, err := store.List(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list accounts: %w", err)
	}

	identifier = strings.TrimSpace(strings.ToLower(identifier))

	var toRemove *cliproxyauth.Auth
	for _, auth := range auths {
		// Match by ID (filename), email, or label
		id := strings.ToLower(auth.ID)
		email := strings.ToLower(auth.Attributes["email"])
		label := strings.ToLower(auth.Label)

		if id == identifier || strings.Contains(id, identifier) ||
			email == identifier || strings.Contains(email, identifier) ||
			label == identifier || strings.Contains(label, identifier) {
			toRemove = auth
			break
		}
	}

	if toRemove == nil {
		return fmt.Errorf("account not found: %s", identifier)
	}

	path := toRemove.Attributes["path"]
	if path == "" {
		return fmt.Errorf("no file path for account: %s", identifier)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to remove %s: %w", path, err)
	}

	fmt.Printf("%s✓ Removed account: %s (%s)%s\n", colorGreen, toRemove.Label, toRemove.Provider, colorReset)
	return nil
}

// parseAccounts converts Auth entries to AccountInfo for display
func parseAccounts(auths []*cliproxyauth.Auth) []AccountInfo {
	var accounts []AccountInfo

	for _, auth := range auths {
		email := auth.Attributes["email"]
		if email == "" {
			email = auth.Label
		}

		// Get project IDs (may be comma-separated for multi-project)
		projectID := ""
		if pid, ok := auth.Metadata["project_id"].(string); ok {
			projectID = pid
		}

		// Count projects for multi-project files
		projectCount := 1
		if strings.Contains(projectID, ",") {
			projectCount = len(strings.Split(projectID, ","))
		}

		// Get expiry
		expiresAt := auth.TokenExpiresAt
		isExpired := false
		if !expiresAt.IsZero() {
			isExpired = time.Now().After(expiresAt)
		}

		// For multi-project files, create one entry per project
		if projectCount > 1 {
			projects := strings.Split(projectID, ",")
			for _, proj := range projects {
				accounts = append(accounts, AccountInfo{
					ID:        auth.ID,
					Provider:  auth.Provider,
					Email:     email,
					ProjectID: strings.TrimSpace(proj),
					ExpiresAt: expiresAt,
					IsExpired: isExpired,
					FilePath:  auth.Attributes["path"],
				})
			}
		} else {
			accounts = append(accounts, AccountInfo{
				ID:        auth.ID,
				Provider:  auth.Provider,
				Email:     email,
				ProjectID: projectID,
				ExpiresAt: expiresAt,
				IsExpired: isExpired,
				FilePath:  auth.Attributes["path"],
			})
		}
	}

	return accounts
}

// outputJSON outputs data as JSON
func outputJSON(data any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// outputTable outputs accounts as a formatted table
func outputTable(accounts []AccountInfo) error {
	if len(accounts) == 0 {
		fmt.Printf("%sNo accounts found%s\n", colorYellow, colorReset)
		return nil
	}

	fmt.Printf("\n%s%s%-12s %-30s %-25s %s%s\n",
		colorBold, colorCyan,
		"PROVIDER", "EMAIL", "PROJECT", "STATUS",
		colorReset)
	fmt.Printf("%s────────────────────────────────────────────────────────────────────────────%s\n", colorDim, colorReset)

	for _, acc := range accounts {
		email := acc.Email
		if len(email) > 28 {
			email = email[:25] + "..."
		}

		project := acc.ProjectID
		if len(project) > 23 {
			project = project[:20] + "..."
		}

		status := colorGreen + "active" + colorReset
		if acc.IsExpired {
			status = colorRed + "expired" + colorReset
		}

		fmt.Printf("%-12s %-30s %-25s %s\n",
			acc.Provider, email, project, status)
	}

	fmt.Printf("%s────────────────────────────────────────────────────────────────────────────%s\n", colorDim, colorReset)
	fmt.Printf("Total: %d account(s)\n\n", len(accounts))

	return nil
}
