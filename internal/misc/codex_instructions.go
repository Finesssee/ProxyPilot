// Package misc provides miscellaneous utility functions and embedded data for the CLI Proxy API.
// This package contains general-purpose helpers and embedded resources that do not fit into
// more specific domain packages. It includes embedded instructional text for Codex-related operations.
package misc

import (
	"embed"
	"strings"
	"sync/atomic"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// codexInstructionsEnabled controls whether CodexInstructionsForModel returns custom instructions.
// When false (default), CodexInstructionsForModel returns (true, "") immediately.
// Set via SetCodexInstructionsEnabled from config.
var codexInstructionsEnabled atomic.Bool

// SetCodexInstructionsEnabled sets whether codex instructions processing is enabled.
func SetCodexInstructionsEnabled(enabled bool) {
	codexInstructionsEnabled.Store(enabled)
}

// GetCodexInstructionsEnabled returns whether codex instructions processing is enabled.
func GetCodexInstructionsEnabled() bool {
	return codexInstructionsEnabled.Load()
}

//go:embed codex_instructions
var codexInstructionsDir embed.FS

//go:embed opencode_codex_instructions.txt
var opencodeCodexInstructions string

const (
	codexUserAgentKey  = "__cpa_user_agent"
	userAgentOpenAISDK = "ai-sdk/openai/"
)

// InjectCodexUserAgent injects the user agent into the request payload for later extraction.
func InjectCodexUserAgent(raw []byte, userAgent string) []byte {
	if len(raw) == 0 {
		return raw
	}
	trimmed := strings.TrimSpace(userAgent)
	if trimmed == "" {
		return raw
	}
	updated, err := sjson.SetBytes(raw, codexUserAgentKey, trimmed)
	if err != nil {
		return raw
	}
	return updated
}

// ExtractCodexUserAgent extracts the user agent from the request payload.
func ExtractCodexUserAgent(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	return strings.TrimSpace(gjson.GetBytes(raw, codexUserAgentKey).String())
}

// StripCodexUserAgent removes the user agent from the request payload.
func StripCodexUserAgent(raw []byte) []byte {
	if len(raw) == 0 {
		return raw
	}
	if !gjson.GetBytes(raw, codexUserAgentKey).Exists() {
		return raw
	}
	updated, err := sjson.DeleteBytes(raw, codexUserAgentKey)
	if err != nil {
		return raw
	}
	return updated
}

func codexInstructionsForOpenCode(systemInstructions string) (bool, string) {
	if opencodeCodexInstructions == "" {
		return false, ""
	}
	if strings.HasPrefix(systemInstructions, opencodeCodexInstructions) {
		return true, ""
	}
	return false, opencodeCodexInstructions
}

func useOpenCodeInstructions(userAgent string) bool {
	return strings.Contains(strings.ToLower(userAgent), userAgentOpenAISDK)
}

func codexInstructionsForCodex(modelName, systemInstructions string) (bool, string) {
	entries, _ := codexInstructionsDir.ReadDir("codex_instructions")

	lastPrompt := ""
	lastCodexPrompt := ""
	lastCodexMaxPrompt := ""
	last51Prompt := ""
	last52Prompt := ""
	last52CodexPrompt := ""
	for _, entry := range entries {
		content, _ := codexInstructionsDir.ReadFile("codex_instructions/" + entry.Name())
		if strings.HasPrefix(systemInstructions, string(content)) {
			return true, ""
		}
		if strings.HasPrefix(entry.Name(), "gpt_5_codex_prompt.md") {
			lastCodexPrompt = string(content)
		} else if strings.HasPrefix(entry.Name(), "gpt-5.1-codex-max_prompt.md") {
			lastCodexMaxPrompt = string(content)
		} else if strings.HasPrefix(entry.Name(), "prompt.md") {
			lastPrompt = string(content)
		} else if strings.HasPrefix(entry.Name(), "gpt_5_1_prompt.md") {
			last51Prompt = string(content)
		} else if strings.HasPrefix(entry.Name(), "gpt_5_2_prompt.md") {
			last52Prompt = string(content)
		} else if strings.HasPrefix(entry.Name(), "gpt-5.2-codex_prompt.md") {
			last52CodexPrompt = string(content)
		}
	}
	if strings.Contains(modelName, "codex-max") {
		return false, lastCodexMaxPrompt
	} else if strings.Contains(modelName, "5.2-codex") {
		return false, last52CodexPrompt
	} else if strings.Contains(modelName, "codex") {
		return false, lastCodexPrompt
	} else if strings.Contains(modelName, "5.1") {
		return false, last51Prompt
	} else if strings.Contains(modelName, "5.2") {
		return false, last52Prompt
	} else {
		return false, lastPrompt
	}
}

// CodexInstructionsForModel returns the appropriate instructions for the given model and user agent.
// For OpenCode clients (identified by user agent), it returns OpenCode-specific instructions.
// For Codex clients, it returns the standard Codex instructions based on model name.
func CodexInstructionsForModel(modelName, systemInstructions, userAgent string) (bool, string) {
	if !GetCodexInstructionsEnabled() {
		return true, ""
	}
	if useOpenCodeInstructions(userAgent) {
		return codexInstructionsForOpenCode(systemInstructions)
	}
	return codexInstructionsForCodex(modelName, systemInstructions)
}
