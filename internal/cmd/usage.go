// Package cmd provides CLI command implementations for ProxyPilot.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
)

// UsageOutput represents the JSON output structure for usage stats
type UsageOutput struct {
	TotalRequests     int64                    `json:"total_requests"`
	SuccessCount      int64                    `json:"success_count"`
	FailureCount      int64                    `json:"failure_count"`
	TotalTokens       int64                    `json:"total_tokens"`
	TotalInputTokens  int64                    `json:"total_input_tokens"`
	TotalOutputTokens int64                    `json:"total_output_tokens"`
	ByProvider        map[string]ProviderStats `json:"by_provider"`
	ByDay             map[string]DayStats      `json:"by_day,omitempty"`
}

// ProviderStats holds usage stats for a single provider
type ProviderStats struct {
	Requests int64            `json:"requests"`
	Tokens   int64            `json:"tokens"`
	Models   map[string]int64 `json:"models,omitempty"`
}

// DayStats holds daily usage stats
type DayStats struct {
	Requests     int64 `json:"requests"`
	Tokens       int64 `json:"tokens"`
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

// ShowUsage displays token usage stats per account/provider
func ShowUsage(jsonOutput bool) error {
	stats := usage.GetRequestStatistics()
	if stats == nil {
		if jsonOutput {
			return outputJSON(UsageOutput{
				ByProvider: make(map[string]ProviderStats),
				ByDay:      make(map[string]DayStats),
			})
		}
		fmt.Printf("%sNo usage data available%s\n", colorYellow, colorReset)
		return nil
	}

	snapshot := stats.Snapshot()

	if jsonOutput {
		return outputUsageJSON(snapshot)
	}

	return outputUsageTable(snapshot)
}

func outputUsageJSON(snapshot usage.StatisticsSnapshot) error {
	output := UsageOutput{
		TotalRequests:     snapshot.TotalRequests,
		SuccessCount:      snapshot.SuccessCount,
		FailureCount:      snapshot.FailureCount,
		TotalTokens:       snapshot.TotalTokens,
		TotalInputTokens:  snapshot.TotalInputTokens,
		TotalOutputTokens: snapshot.TotalOutputTokens,
		ByProvider:        make(map[string]ProviderStats),
		ByDay:             make(map[string]DayStats),
	}

	// Aggregate by provider
	for apiName, apiSnapshot := range snapshot.APIs {
		provider := inferProvider(apiName)
		ps := output.ByProvider[provider]
		ps.Requests += apiSnapshot.TotalRequests
		ps.Tokens += apiSnapshot.TotalTokens
		if ps.Models == nil {
			ps.Models = make(map[string]int64)
		}
		for modelName, modelSnapshot := range apiSnapshot.Models {
			ps.Models[modelName] += modelSnapshot.TotalRequests
		}
		output.ByProvider[provider] = ps
	}

	// Daily stats
	for day, requests := range snapshot.RequestsByDay {
		ds := output.ByDay[day]
		ds.Requests = requests
		ds.Tokens = snapshot.TokensByDay[day]
		ds.InputTokens = snapshot.InputTokensByDay[day]
		ds.OutputTokens = snapshot.OutputTokensByDay[day]
		output.ByDay[day] = ds
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func outputUsageTable(snapshot usage.StatisticsSnapshot) error {
	if snapshot.TotalRequests == 0 {
		fmt.Printf("%sNo usage data available%s\n", colorYellow, colorReset)
		fmt.Printf("%sUsage statistics are collected during proxy operation.%s\n", colorDim, colorReset)
		return nil
	}

	fmt.Printf("\n%s%sProxyPilot Usage Statistics%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s─────────────────────────────────────────────────────────%s\n\n", colorDim, colorReset)

	// Summary
	fmt.Printf("%sTotal Requests:%s  %d", colorBold, colorReset, snapshot.TotalRequests)
	if snapshot.FailureCount > 0 {
		fmt.Printf(" (%s%d failed%s)", colorRed, snapshot.FailureCount, colorReset)
	}
	fmt.Println()
	fmt.Printf("%sTotal Tokens:%s    %s\n", colorBold, colorReset, formatTokenCount(snapshot.TotalTokens))
	fmt.Printf("  Input:          %s\n", formatTokenCount(snapshot.TotalInputTokens))
	fmt.Printf("  Output:         %s\n", formatTokenCount(snapshot.TotalOutputTokens))

	// By provider
	if len(snapshot.APIs) > 0 {
		fmt.Printf("\n%s%sByProvider%s\n", colorBold, colorCyan, colorReset)
		fmt.Printf("%s─────────────────────────────────────────────────────────%s\n", colorDim, colorReset)

		// Aggregate by provider
		providerStats := make(map[string]struct {
			requests int64
			tokens   int64
		})

		for apiName, apiSnapshot := range snapshot.APIs {
			provider := inferProvider(apiName)
			ps := providerStats[provider]
			ps.requests += apiSnapshot.TotalRequests
			ps.tokens += apiSnapshot.TotalTokens
			providerStats[provider] = ps
		}

		// Sort providers
		providers := make([]string, 0, len(providerStats))
		for p := range providerStats {
			providers = append(providers, p)
		}
		sort.Strings(providers)

		fmt.Printf("  %-15s %10s %15s\n", "Provider", "Requests", "Tokens")
		fmt.Printf("  %s───────────────────────────────────────────%s\n", colorDim, colorReset)

		for _, provider := range providers {
			ps := providerStats[provider]
			fmt.Printf("  %-15s %10d %15s\n", provider, ps.requests, formatTokenCount(ps.tokens))
		}
	}

	// Recent daily stats
	if len(snapshot.RequestsByDay) > 0 {
		fmt.Printf("\n%s%sRecent Activity%s\n", colorBold, colorCyan, colorReset)
		fmt.Printf("%s─────────────────────────────────────────────────────────%s\n", colorDim, colorReset)

		// Sort days
		days := make([]string, 0, len(snapshot.RequestsByDay))
		for day := range snapshot.RequestsByDay {
			days = append(days, day)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(days)))

		// Show last 7 days max
		if len(days) > 7 {
			days = days[:7]
		}

		fmt.Printf("  %-12s %10s %15s\n", "Date", "Requests", "Tokens")
		fmt.Printf("  %s─────────────────────────────────────%s\n", colorDim, colorReset)

		for _, day := range days {
			requests := snapshot.RequestsByDay[day]
			tokens := snapshot.TokensByDay[day]
			fmt.Printf("  %-12s %10d %15s\n", day, requests, formatTokenCount(tokens))
		}
	}

	fmt.Printf("\n%s─────────────────────────────────────────────────────────%s\n\n", colorDim, colorReset)

	return nil
}

func inferProvider(apiName string) string {
	switch {
	case contains(apiName, "claude", "anthropic"):
		return "anthropic"
	case contains(apiName, "gpt", "openai", "codex"):
		return "openai"
	case contains(apiName, "gemini", "google"):
		return "google"
	case contains(apiName, "qwen"):
		return "qwen"
	case contains(apiName, "deepseek"):
		return "deepseek"
	case contains(apiName, "kiro", "amazon"):
		return "kiro"
	default:
		return apiName
	}
}

func contains(s string, substrs ...string) bool {
	lower := toLower(s)
	for _, sub := range substrs {
		if indexOfLower(lower, sub) >= 0 {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func indexOfLower(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			if s[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func formatTokenCount(tokens int64) string {
	if tokens < 1000 {
		return fmt.Sprintf("%d", tokens)
	}
	if tokens < 1000000 {
		return fmt.Sprintf("%.1fK", float64(tokens)/1000)
	}
	return fmt.Sprintf("%.2fM", float64(tokens)/1000000)
}
