// Package cmd provides CLI command implementations for ProxyPilot.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

// ExportBundle represents exported accounts
type ExportBundle struct {
	Version    string            `json:"version"`
	ExportedAt string            `json:"exported_at"`
	Accounts   []ExportedAccount `json:"accounts"`
}

// ExportedAccount represents a single exported account
type ExportedAccount struct {
	ID         string            `json:"id"`
	Provider   string            `json:"provider"`
	Label      string            `json:"label,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Metadata   map[string]any    `json:"metadata,omitempty"`
	CreatedAt  time.Time         `json:"created_at,omitempty"`
}

var sensitiveKeys = []string{
	"access_token", "refresh_token", "token", "api_key", "secret",
	"password", "credential", "session", "cookie", "id_token", "bearer",
}

// ExportAccounts exports all accounts to JSON
func ExportAccounts(outputPath string, includeTokens bool, jsonOutput bool) error {
	store := sdkAuth.NewFileTokenStore()
	store.SetBaseDir(util.DefaultAuthDir())

	auths, err := store.List(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list accounts: %w", err)
	}

	bundle := ExportBundle{
		Version:    "1.0",
		ExportedAt: time.Now().Format(time.RFC3339),
		Accounts:   make([]ExportedAccount, 0, len(auths)),
	}

	for _, auth := range auths {
		exported := ExportedAccount{
			ID:         auth.ID,
			Provider:   auth.Provider,
			Label:      auth.Label,
			Attributes: auth.Attributes,
			CreatedAt:  auth.CreatedAt,
		}

		if auth.Metadata != nil {
			exported.Metadata = make(map[string]any, len(auth.Metadata))
			for k, v := range auth.Metadata {
				if !includeTokens && isSensitiveKey(k) {
					exported.Metadata[k] = "[REDACTED]"
				} else {
					exported.Metadata[k] = v
				}
			}
		}
		bundle.Accounts = append(bundle.Accounts, exported)
	}

	// Determine output target
	var out io.Writer = os.Stdout
	if outputPath != "" && outputPath != "-" {
		f, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(bundle); err != nil {
		return fmt.Errorf("failed to encode accounts: %w", err)
	}

	if !jsonOutput && outputPath != "" && outputPath != "-" {
		fmt.Fprintf(os.Stderr, "%sExported %d accounts to %s%s\n",
			colorGreen, len(bundle.Accounts), outputPath, colorReset)
		if !includeTokens {
			fmt.Fprintf(os.Stderr, "%sNote: Sensitive tokens redacted. Use --include-tokens to include.%s\n",
				colorYellow, colorReset)
		}
	}

	return nil
}

// ImportAccounts imports accounts from a JSON bundle
func ImportAccounts(inputPath string, force bool, jsonOutput bool) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read import file: %w", err)
	}

	var bundle ExportBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return fmt.Errorf("failed to parse import file: %w", err)
	}

	store := sdkAuth.NewFileTokenStore()
	store.SetBaseDir(util.DefaultAuthDir())

	ctx := context.Background()
	existing, err := store.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list existing accounts: %w", err)
	}

	existingIDs := make(map[string]bool)
	for _, auth := range existing {
		existingIDs[auth.ID] = true
	}

	var imported, skipped, skippedRedacted int
	var results []map[string]any

	for _, acc := range bundle.Accounts {
		// Skip if has redacted tokens
		hasRedacted := false
		if acc.Metadata != nil {
			for k, v := range acc.Metadata {
				if s, ok := v.(string); ok && s == "[REDACTED]" && isSensitiveKey(k) {
					hasRedacted = true
					break
				}
			}
		}
		if hasRedacted {
			skippedRedacted++
			if jsonOutput {
				results = append(results, map[string]any{
					"id": acc.ID, "status": "skipped", "reason": "redacted tokens",
				})
			}
			continue
		}

		// Check if exists
		if existingIDs[acc.ID] && !force {
			skipped++
			if jsonOutput {
				results = append(results, map[string]any{
					"id": acc.ID, "status": "skipped", "reason": "already exists",
				})
			} else {
				fmt.Fprintf(os.Stderr, "%sSkipping %s (exists). Use --force to overwrite.%s\n",
					colorYellow, acc.ID, colorReset)
			}
			continue
		}

		// Convert to Auth
		auth := &cliproxyauth.Auth{
			ID:         acc.ID,
			Provider:   acc.Provider,
			Label:      acc.Label,
			Attributes: acc.Attributes,
			Metadata:   acc.Metadata,
			CreatedAt:  acc.CreatedAt,
			UpdatedAt:  time.Now(),
		}

		// Set FileName based on authdir
		auth.FileName = filepath.Join(util.DefaultAuthDir(), acc.ID+".json")

		if _, err := store.Save(ctx, auth); err != nil {
			if jsonOutput {
				results = append(results, map[string]any{
					"id": acc.ID, "status": "error", "error": err.Error(),
				})
			} else {
				fmt.Fprintf(os.Stderr, "%sFailed to import %s: %v%s\n",
					colorRed, acc.ID, err, colorReset)
			}
			continue
		}

		imported++
		if jsonOutput {
			results = append(results, map[string]any{
				"id": acc.ID, "status": "imported",
			})
		}
	}

	if jsonOutput {
		return outputJSON(map[string]any{
			"imported":         imported,
			"skipped":          skipped,
			"skipped_redacted": skippedRedacted,
			"results":          results,
		})
	}

	fmt.Printf("\n%s%sImport Summary%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s─────────────────────────%s\n", colorDim, colorReset)
	fmt.Printf("  Imported: %s%d%s\n", colorGreen, imported, colorReset)
	if skipped > 0 {
		fmt.Printf("  Skipped (existing): %s%d%s\n", colorYellow, skipped, colorReset)
	}
	if skippedRedacted > 0 {
		fmt.Printf("  Skipped (redacted): %s%d%s\n", colorYellow, skippedRedacted, colorReset)
	}
	fmt.Println()

	return nil
}

func isSensitiveKey(key string) bool {
	keyLower := strings.ToLower(key)
	for _, s := range sensitiveKeys {
		if strings.Contains(keyLower, s) {
			return true
		}
	}
	return false
}
