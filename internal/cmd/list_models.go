// Package cmd provides CLI command implementations for ProxyPilot.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
)

// ModelGroup represents a provider and its models
type ModelGroup struct {
	Provider string       `json:"provider"`
	Models   []ModelEntry `json:"models"`
}

// ModelEntry represents a single model entry for output
type ModelEntry struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
	Thinking    bool   `json:"thinking,omitempty"`
}

// ListModels lists all available models grouped by provider
func ListModels(jsonOutput bool) error {
	groups := collectModelGroups()
	if jsonOutput {
		return outputModelsAsJSON(groups)
	}
	return outputModelsTable(groups)
}

func collectModelGroups() []ModelGroup {
	providers := []struct {
		name   string
		models []*registry.ModelInfo
	}{
		{"Claude", registry.GetClaudeModels()},
		{"Gemini", registry.GetGeminiModels()},
		{"AI Studio", registry.GetAIStudioModels()},
		{"OpenAI (Codex)", registry.GetOpenAIModels()},
		{"Qwen", registry.GetQwenModels()},
		{"MiniMax", registry.GetMiniMaxModels()},
		{"Zhipu", registry.GetZhipuModels()},
		{"Kiro", registry.GetKiroModels()},
		{"GitHub Copilot", registry.GetGitHubCopilotModels()},
	}

	var groups []ModelGroup
	for _, p := range providers {
		if len(p.models) == 0 {
			continue
		}
		entries := make([]ModelEntry, 0, len(p.models))
		for _, m := range p.models {
			entry := ModelEntry{
				ID:          m.ID,
				DisplayName: m.DisplayName,
				Description: m.Description,
				Thinking:    m.Thinking != nil,
			}
			entries = append(entries, entry)
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].ID < entries[j].ID
		})
		groups = append(groups, ModelGroup{Provider: p.name, Models: entries})
	}
	return groups
}

func outputModelsTable(groups []ModelGroup) error {
	if len(groups) == 0 {
		fmt.Printf("%sNo models available%s\n", colorYellow, colorReset)
		return nil
	}

	for _, g := range groups {
		fmt.Printf("\n%s%s%s %s(%d models)%s\n",
			colorBold, g.Provider, colorReset, colorDim, len(g.Models), colorReset)
		fmt.Printf("%s─────────────────────────────────────────%s\n", colorDim, colorReset)

		for _, m := range g.Models {
			thinkTag := ""
			if m.Thinking {
				thinkTag = fmt.Sprintf(" %s[thinking]%s", colorCyan, colorReset)
			}
			name := m.ID
			if m.DisplayName != "" && m.DisplayName != m.ID {
				name = fmt.Sprintf("%s %s(%s)%s", m.ID, colorDim, m.DisplayName, colorReset)
			}
			fmt.Printf("  %s%s%s\n", colorGreen, name, thinkTag)
		}
	}
	fmt.Println()
	return nil
}

func outputModelsAsJSON(groups []ModelGroup) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(groups)
}
