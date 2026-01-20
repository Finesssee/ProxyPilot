// Package registry provides model definitions for various AI service providers.
// This file contains static model definitions that can be used by clients
// when registering their supported models.
package registry

// GetClaudeModels returns the standard Claude model definitions
func GetClaudeModels() []*ModelInfo {
	return []*ModelInfo{

		{
			ID:                  "claude-haiku-4-5-20251001",
			Object:              "model",
			Created:             1759276800, // 2025-10-01
			OwnedBy:             "anthropic",
			Type:                "claude",
			DisplayName:         "Claude 4.5 Haiku",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
			// Thinking: not supported for Haiku models
		},
		{
			ID:                  "claude-sonnet-4-5-20250929",
			Object:              "model",
			Created:             1759104000, // 2025-09-29
			OwnedBy:             "anthropic",
			Type:                "claude",
			DisplayName:         "Claude 4.5 Sonnet",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
			Thinking:            &ThinkingSupport{Min: 1024, Max: 100000, ZeroAllowed: false, DynamicAllowed: true},
		},
		{
			ID:                  "claude-opus-4-5-20251101",
			Object:              "model",
			Created:             1761955200, // 2025-11-01
			OwnedBy:             "anthropic",
			Type:                "claude",
			DisplayName:         "Claude 4.5 Opus",
			Description:         "Premium model combining maximum intelligence with practical performance",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
			Thinking:            &ThinkingSupport{Min: 1024, Max: 100000, ZeroAllowed: false, DynamicAllowed: true},
		},
	}
}

// GetGeminiModels returns the standard Gemini model definitions
func GetGeminiModels() []*ModelInfo {
	return []*ModelInfo{
		{
			ID:                         "gemini-2.5-pro",
			Object:                     "model",
			Created:                    1750118400,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/gemini-2.5-pro",
			Version:                    "2.5",
			DisplayName:                "Gemini 2.5 Pro",
			Description:                "Stable release (June 17th, 2025) of Gemini 2.5 Pro",
			InputTokenLimit:            1048576,
			OutputTokenLimit:           65536,
			SupportedGenerationMethods: []string{"generateContent", "countTokens", "createCachedContent", "batchGenerateContent"},
			Thinking:                   &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true},
		},
		{
			ID:                         "gemini-2.5-flash",
			Object:                     "model",
			Created:                    1750118400,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/gemini-2.5-flash",
			Version:                    "001",
			DisplayName:                "Gemini 2.5 Flash",
			Description:                "Stable version of Gemini 2.5 Flash, our mid-size multimodal model that supports up to 1 million tokens, released in June of 2025.",
			InputTokenLimit:            1048576,
			OutputTokenLimit:           65536,
			SupportedGenerationMethods: []string{"generateContent", "countTokens", "createCachedContent", "batchGenerateContent"},
			Thinking:                   &ThinkingSupport{Min: 0, Max: 24576, ZeroAllowed: true, DynamicAllowed: true},
		},
		{
			ID:                         "gemini-2.5-flash-lite",
			Object:                     "model",
			Created:                    1753142400,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/gemini-2.5-flash-lite",
			Version:                    "2.5",
			DisplayName:                "Gemini 2.5 Flash Lite",
			Description:                "Our smallest and most cost effective model, built for at scale usage.",
			InputTokenLimit:            1048576,
			OutputTokenLimit:           65536,
			SupportedGenerationMethods: []string{"generateContent", "countTokens", "createCachedContent", "batchGenerateContent"},
			Thinking:                   &ThinkingSupport{Min: 0, Max: 24576, ZeroAllowed: true, DynamicAllowed: true},
		},
		{
			ID:                         "gemini-3-pro-preview",
			Object:                     "model",
			Created:                    1737158400,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/gemini-3-pro-preview",
			Version:                    "3.0",
			DisplayName:                "Gemini 3 Pro Preview",
			Description:                "Gemini 3 Pro Preview",
			InputTokenLimit:            1048576,
			OutputTokenLimit:           65536,
			SupportedGenerationMethods: []string{"generateContent", "countTokens", "createCachedContent", "batchGenerateContent"},
			Thinking:                   &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true, Levels: []string{"low", "high"}},
		},
		{
			ID:                         "gemini-3-flash-preview",
			Object:                     "model",
			Created:                    1765929600,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/gemini-3-flash-preview",
			Version:                    "3.0",
			DisplayName:                "Gemini 3 Flash Preview",
			Description:                "Gemini 3 Flash Preview",
			InputTokenLimit:            1048576,
			OutputTokenLimit:           65536,
			SupportedGenerationMethods: []string{"generateContent", "countTokens", "createCachedContent", "batchGenerateContent"},
			Thinking:                   &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true, Levels: []string{"minimal", "low", "medium", "high"}},
		},
		{
			ID:                         "gemini-3-pro-image-preview",
			Object:                     "model",
			Created:                    1737158400,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/gemini-3-pro-image-preview",
			Version:                    "3.0",
			DisplayName:                "Gemini 3 Pro Image Preview",
			Description:                "Gemini 3 Pro Image Preview",
			InputTokenLimit:            1048576,
			OutputTokenLimit:           65536,
			SupportedGenerationMethods: []string{"generateContent", "countTokens", "createCachedContent", "batchGenerateContent"},
			Thinking:                   &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true, Levels: []string{"low", "high"}},
		},
	}
}

// GetGeminiVertexModels returns GetGeminiModels - consolidated
func GetGeminiVertexModels() []*ModelInfo {
	return GetGeminiModels()
}

// GetGeminiCLIModels returns GetGeminiModels - consolidated
func GetGeminiCLIModels() []*ModelInfo {
	return GetGeminiModels()
}

// GetAIStudioModels returns the Gemini model definitions for AI Studio integrations
// Includes base models plus -latest aliases specific to AI Studio
func GetAIStudioModels() []*ModelInfo {
	// Start with base Gemini models
	models := GetGeminiModels()

	// Add AI Studio specific -latest aliases
	now := int64(1750118400)
	aiStudioExtras := []*ModelInfo{
		{
			ID:                         "gemini-pro-latest",
			Object:                     "model",
			Created:                    now,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/gemini-pro-latest",
			Version:                    "2.5",
			DisplayName:                "Gemini Pro Latest",
			Description:                "Latest release of Gemini Pro",
			InputTokenLimit:            1048576,
			OutputTokenLimit:           65536,
			SupportedGenerationMethods: []string{"generateContent", "countTokens", "createCachedContent", "batchGenerateContent"},
			Thinking:                   &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true},
		},
		{
			ID:                         "gemini-flash-latest",
			Object:                     "model",
			Created:                    now,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/gemini-flash-latest",
			Version:                    "2.5",
			DisplayName:                "Gemini Flash Latest",
			Description:                "Latest release of Gemini Flash",
			InputTokenLimit:            1048576,
			OutputTokenLimit:           65536,
			SupportedGenerationMethods: []string{"generateContent", "countTokens", "createCachedContent", "batchGenerateContent"},
			Thinking:                   &ThinkingSupport{Min: 0, Max: 24576, ZeroAllowed: true, DynamicAllowed: true},
		},
	}
	return append(models, aiStudioExtras...)
}

// GetOpenAIModels returns the standard OpenAI model definitions
func GetOpenAIModels() []*ModelInfo {
	return []*ModelInfo{
		{
			ID:                  "gpt-5.2",
			Object:              "model",
			Created:             1765440000,
			OwnedBy:             "openai",
			Type:                "openai",
			Version:             "gpt-5.2",
			DisplayName:         "GPT 5.2",
			Description:         "Stable version of GPT 5.2",
			ContextLength:       400000,
			MaxCompletionTokens: 128000,
			SupportedParameters: []string{"tools"},
			Thinking:            &ThinkingSupport{Levels: []string{"none", "low", "medium", "high", "xhigh"}},
		},
		{
			ID:                  "gpt-5.2-codex",
			Object:              "model",
			Created:             1765440000,
			OwnedBy:             "openai",
			Type:                "openai",
			Version:             "gpt-5.2",
			DisplayName:         "GPT 5.2 Codex",
			Description:         "Stable version of GPT 5.2 Codex, The best model for coding and agentic tasks across domains.",
			ContextLength:       400000,
			MaxCompletionTokens: 128000,
			SupportedParameters: []string{"tools"},
			Thinking:            &ThinkingSupport{Levels: []string{"low", "medium", "high", "xhigh"}},
		},
	}
}

// GetQwenModels returns the standard Qwen model definitions
func GetQwenModels() []*ModelInfo {
	return []*ModelInfo{
		{
			ID:                  "qwen3-coder-plus",
			Object:              "model",
			Created:             1753228800,
			OwnedBy:             "qwen",
			Type:                "qwen",
			Version:             "3.0",
			DisplayName:         "Qwen3 Coder Plus",
			Description:         "Advanced code generation and understanding model",
			ContextLength:       32768,
			MaxCompletionTokens: 8192,
			SupportedParameters: []string{"temperature", "top_p", "max_tokens", "stream", "stop"},
		},
		{
			ID:                  "qwen3-coder-flash",
			Object:              "model",
			Created:             1753228800,
			OwnedBy:             "qwen",
			Type:                "qwen",
			Version:             "3.0",
			DisplayName:         "Qwen3 Coder Flash",
			Description:         "Fast code generation model",
			ContextLength:       8192,
			MaxCompletionTokens: 2048,
			SupportedParameters: []string{"temperature", "top_p", "max_tokens", "stream", "stop"},
		},
	}
}

// GetMiniMaxModels returns supported models for MiniMax API key accounts.
func GetMiniMaxModels() []*ModelInfo {
	entries := []struct {
		ID          string
		DisplayName string
		Description string
		Created     int64
		Thinking    *ThinkingSupport
	}{
		{ID: "minimax-m2", DisplayName: "MiniMax-M2", Description: "MiniMax M2 base model", Created: 1758672000},
		{ID: "minimax-m2.1", DisplayName: "MiniMax-M2.1", Description: "MiniMax M2.1 with reasoning", Created: 1766448000, Thinking: &ThinkingSupport{Levels: []string{"none", "auto", "low", "medium", "high"}}},
		{ID: "minimax-m2.1-lightning", DisplayName: "MiniMax-M2.1-Lightning", Description: "MiniMax M2.1 fast variant", Created: 1766448000},
	}
	models := make([]*ModelInfo, 0, len(entries))
	for _, entry := range entries {
		models = append(models, &ModelInfo{
			ID:          entry.ID,
			Object:      "model",
			Created:     entry.Created,
			OwnedBy:     "minimax",
			Type:        "minimax",
			DisplayName: entry.DisplayName,
			Description: entry.Description,
			Thinking:    entry.Thinking,
		})
	}
	return models
}

// GetZhipuModels returns supported models for Zhipu AI (GLM) API key accounts.
func GetZhipuModels() []*ModelInfo {
	entries := []struct {
		ID          string
		DisplayName string
		Description string
		Created     int64
		Thinking    *ThinkingSupport
	}{
		{ID: "glm-4.7", DisplayName: "GLM-4.7", Description: "Zhipu GLM 4.7 flagship model with thinking", Created: 1766448000, Thinking: &ThinkingSupport{Levels: []string{"none", "auto", "low", "medium", "high"}}},
		{ID: "glm-4.6", DisplayName: "GLM-4.6", Description: "Zhipu GLM 4.6 high performance model", Created: 1759190400, Thinking: &ThinkingSupport{Levels: []string{"none", "auto", "low", "medium", "high"}}},
		{ID: "glm-4.5", DisplayName: "GLM-4.5", Description: "Zhipu GLM 4.5 excellent for coding", Created: 1752192000},
		{ID: "glm-4-long", DisplayName: "GLM-4-Long", Description: "Zhipu GLM 4 with 1M context", Created: 1745280000},
		{ID: "glm-4.6v", DisplayName: "GLM-4.6V", Description: "Zhipu GLM 4.6 vision model", Created: 1759190400},
	}
	models := make([]*ModelInfo, 0, len(entries))
	for _, entry := range entries {
		models = append(models, &ModelInfo{
			ID:          entry.ID,
			Object:      "model",
			Created:     entry.Created,
			OwnedBy:     "zhipu",
			Type:        "zhipu",
			DisplayName: entry.DisplayName,
			Description: entry.Description,
			Thinking:    entry.Thinking,
		})
	}
	return models
}

// AntigravityModelConfig captures static antigravity model overrides, including
// Thinking budget limits and provider max completion tokens.
type AntigravityModelConfig struct {
	Thinking            *ThinkingSupport
	MaxCompletionTokens int
}

// GetAntigravityModelConfig returns static configuration for antigravity models.
// Keys use the ALIASED model names (after modelName2Alias conversion) for direct lookup.
func GetAntigravityModelConfig() map[string]*AntigravityModelConfig {
	return map[string]*AntigravityModelConfig{
		"gemini-2.5-flash":                        {Thinking: &ThinkingSupport{Min: 0, Max: 24576, ZeroAllowed: true, DynamicAllowed: true}},
		"gemini-2.5-flash-lite":                   {Thinking: &ThinkingSupport{Min: 0, Max: 24576, ZeroAllowed: true, DynamicAllowed: true}},
		"gemini-2.5-computer-use-preview-10-2025": {},
		"rev19-uic3-1p":                           {Thinking: &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true}},
		"gemini-3-pro-preview":                    {Thinking: &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true, Levels: []string{"low", "high"}}},
		"gemini-3-pro-high":                       {Thinking: &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true, Levels: []string{"low", "high"}}},
		"gemini-3-pro-image-preview":              {Thinking: &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true, Levels: []string{"low", "high"}}},
		"gemini-3-pro-image":                      {Thinking: &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true, Levels: []string{"low", "high"}}},
		"gemini-3-flash-preview":                  {Thinking: &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true, Levels: []string{"minimal", "low", "medium", "high"}}},
		"gemini-3-flash":                          {Thinking: &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true, Levels: []string{"minimal", "low", "medium", "high"}}},
		"claude-sonnet-4-5-thinking":              {Thinking: &ThinkingSupport{Min: 1024, Max: 128000, ZeroAllowed: true, DynamicAllowed: true}, MaxCompletionTokens: 64000},
		"claude-opus-4-5-thinking":                {Thinking: &ThinkingSupport{Min: 1024, Max: 128000, ZeroAllowed: true, DynamicAllowed: true}, MaxCompletionTokens: 64000},
		"gemini-claude-sonnet-4-5-thinking":       {Thinking: &ThinkingSupport{Min: 1024, Max: 128000, ZeroAllowed: true, DynamicAllowed: true}, MaxCompletionTokens: 64000},
		"gemini-claude-opus-4-5-thinking":         {Thinking: &ThinkingSupport{Min: 1024, Max: 128000, ZeroAllowed: true, DynamicAllowed: true}, MaxCompletionTokens: 64000},
		"claude-sonnet-4-5":                       {MaxCompletionTokens: 64000},
		"gpt-oss-120b-medium":                     {},
		"tab_flash_lite_preview":                  {},
	}
}

// GetGitHubCopilotModels returns the available models for GitHub Copilot.
// These models are available through the GitHub Copilot API at api.githubcopilot.com.
func GetGitHubCopilotModels() []*ModelInfo {
	now := int64(1732752000) // 2024-11-27
	return []*ModelInfo{
		{
			ID:                  "gpt-5.2",
			Object:              "model",
			Created:             now,
			OwnedBy:             "github-copilot",
			Type:                "github-copilot",
			DisplayName:         "GPT-5.2",
			Description:         "OpenAI GPT-5.2 via GitHub Copilot",
			ContextLength:       200000,
			MaxCompletionTokens: 32768,
		},
		{
			ID:                  "claude-haiku-4.5",
			Object:              "model",
			Created:             now,
			OwnedBy:             "github-copilot",
			Type:                "github-copilot",
			DisplayName:         "Claude Haiku 4.5",
			Description:         "Anthropic Claude Haiku 4.5 via GitHub Copilot",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
		},
		{
			ID:                  "claude-sonnet-4.5",
			Object:              "model",
			Created:             now,
			OwnedBy:             "github-copilot",
			Type:                "github-copilot",
			DisplayName:         "Claude Sonnet 4.5",
			Description:         "Anthropic Claude Sonnet 4.5 via GitHub Copilot",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
		},
		{
			ID:                  "claude-opus-4.5",
			Object:              "model",
			Created:             now,
			OwnedBy:             "github-copilot",
			Type:                "github-copilot",
			DisplayName:         "Claude Opus 4.5",
			Description:         "Anthropic Claude Opus 4.5 via GitHub Copilot",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
		},
		{
			ID:                  "gemini-2.5-pro",
			Object:              "model",
			Created:             now,
			OwnedBy:             "github-copilot",
			Type:                "github-copilot",
			DisplayName:         "Gemini 2.5 Pro",
			Description:         "Google Gemini 2.5 Pro via GitHub Copilot",
			ContextLength:       1048576,
			MaxCompletionTokens: 65536,
		},
		{
			ID:                  "gemini-3-pro",
			Object:              "model",
			Created:             now,
			OwnedBy:             "github-copilot",
			Type:                "github-copilot",
			DisplayName:         "Gemini 3 Pro",
			Description:         "Google Gemini 3 Pro via GitHub Copilot",
			ContextLength:       1048576,
			MaxCompletionTokens: 65536,
		},
	}
}

// GetKiroModels returns the Kiro (AWS CodeWhisperer) model definitions
func GetKiroModels() []*ModelInfo {
	return []*ModelInfo{
		// --- Base Models ---
		{
			ID:                  "kiro-claude-opus-4-5",
			Object:              "model",
			Created:             1732752000,
			OwnedBy:             "aws",
			Type:                "kiro",
			DisplayName:         "Kiro Claude Opus 4.5",
			Description:         "Claude Opus 4.5 via Kiro (2.2x credit)",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
			Thinking:            &ThinkingSupport{Min: 1024, Max: 32000, ZeroAllowed: true, DynamicAllowed: true},
		},
		{
			ID:                  "kiro-claude-sonnet-4-5",
			Object:              "model",
			Created:             1732752000,
			OwnedBy:             "aws",
			Type:                "kiro",
			DisplayName:         "Kiro Claude Sonnet 4.5",
			Description:         "Claude Sonnet 4.5 via Kiro (1.3x credit)",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
			Thinking:            &ThinkingSupport{Min: 1024, Max: 32000, ZeroAllowed: true, DynamicAllowed: true},
		},
		{
			ID:                  "kiro-claude-sonnet-4",
			Object:              "model",
			Created:             1732752000,
			OwnedBy:             "aws",
			Type:                "kiro",
			DisplayName:         "Kiro Claude Sonnet 4",
			Description:         "Claude Sonnet 4 via Kiro (1.3x credit)",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
			Thinking:            &ThinkingSupport{Min: 1024, Max: 32000, ZeroAllowed: true, DynamicAllowed: true},
		},
		{
			ID:                  "kiro-claude-haiku-4-5",
			Object:              "model",
			Created:             1732752000,
			OwnedBy:             "aws",
			Type:                "kiro",
			DisplayName:         "Kiro Claude Haiku 4.5",
			Description:         "Claude Haiku 4.5 via Kiro (0.4x credit)",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
			Thinking:            &ThinkingSupport{Min: 1024, Max: 32000, ZeroAllowed: true, DynamicAllowed: true},
		},
		// --- Thinking Variants (Extended thinking enabled, ZeroAllowed=false forces thinking) ---
		{
			ID:                  "kiro-claude-opus-4-5-thinking",
			Object:              "model",
			Created:             1732752000,
			OwnedBy:             "aws",
			Type:                "kiro",
			DisplayName:         "Kiro Claude Opus 4.5 (Thinking)",
			Description:         "Claude Opus 4.5 via Kiro with extended thinking enabled",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
			Thinking:            &ThinkingSupport{Min: 1024, Max: 128000, ZeroAllowed: false, DynamicAllowed: true},
		},
		{
			ID:                  "kiro-claude-sonnet-4-5-thinking",
			Object:              "model",
			Created:             1732752000,
			OwnedBy:             "aws",
			Type:                "kiro",
			DisplayName:         "Kiro Claude Sonnet 4.5 (Thinking)",
			Description:         "Claude Sonnet 4.5 via Kiro with extended thinking enabled",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
			Thinking:            &ThinkingSupport{Min: 1024, Max: 128000, ZeroAllowed: false, DynamicAllowed: true},
		},
		// --- Agentic Variants (Optimized for coding agents with chunked writes) ---
		{
			ID:                  "kiro-claude-opus-4-5-agentic",
			Object:              "model",
			Created:             1732752000,
			OwnedBy:             "aws",
			Type:                "kiro",
			DisplayName:         "Kiro Claude Opus 4.5 (Agentic)",
			Description:         "Claude Opus 4.5 optimized for coding agents (chunked writes)",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
			Thinking:            &ThinkingSupport{Min: 1024, Max: 32000, ZeroAllowed: true, DynamicAllowed: true},
		},
		{
			ID:                  "kiro-claude-sonnet-4-5-agentic",
			Object:              "model",
			Created:             1732752000,
			OwnedBy:             "aws",
			Type:                "kiro",
			DisplayName:         "Kiro Claude Sonnet 4.5 (Agentic)",
			Description:         "Claude Sonnet 4.5 optimized for coding agents (chunked writes)",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
			Thinking:            &ThinkingSupport{Min: 1024, Max: 32000, ZeroAllowed: true, DynamicAllowed: true},
		},
		{
			ID:                  "kiro-claude-sonnet-4-agentic",
			Object:              "model",
			Created:             1732752000,
			OwnedBy:             "aws",
			Type:                "kiro",
			DisplayName:         "Kiro Claude Sonnet 4 (Agentic)",
			Description:         "Claude Sonnet 4 optimized for coding agents (chunked writes)",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
			Thinking:            &ThinkingSupport{Min: 1024, Max: 32000, ZeroAllowed: true, DynamicAllowed: true},
		},
		{
			ID:                  "kiro-claude-haiku-4-5-agentic",
			Object:              "model",
			Created:             1732752000,
			OwnedBy:             "aws",
			Type:                "kiro",
			DisplayName:         "Kiro Claude Haiku 4.5 (Agentic)",
			Description:         "Claude Haiku 4.5 optimized for coding agents (chunked writes)",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
			Thinking:            &ThinkingSupport{Min: 1024, Max: 32000, ZeroAllowed: true, DynamicAllowed: true},
		},
	}
}

// GetAmazonQModels returns the Amazon Q (AWS CodeWhisperer) model definitions.
// These models use the same API as Kiro and share the same executor.
func GetAmazonQModels() []*ModelInfo {
	return []*ModelInfo{
		{
			ID:                  "amazonq-claude-opus-4.5",
			Object:              "model",
			Created:             1732752000,
			OwnedBy:             "aws",
			Type:                "kiro",
			DisplayName:         "Amazon Q Claude Opus 4.5",
			Description:         "Claude Opus 4.5 via Amazon Q (2.2x credit)",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
		},
		{
			ID:                  "amazonq-claude-sonnet-4.5",
			Object:              "model",
			Created:             1732752000,
			OwnedBy:             "aws",
			Type:                "kiro",
			DisplayName:         "Amazon Q Claude Sonnet 4.5",
			Description:         "Claude Sonnet 4.5 via Amazon Q (1.3x credit)",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
		},
		{
			ID:                  "amazonq-claude-sonnet-4",
			Object:              "model",
			Created:             1732752000,
			OwnedBy:             "aws",
			Type:                "kiro",
			DisplayName:         "Amazon Q Claude Sonnet 4",
			Description:         "Claude Sonnet 4 via Amazon Q (1.3x credit)",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
		},
		{
			ID:                  "amazonq-claude-haiku-4.5",
			Object:              "model",
			Created:             1732752000,
			OwnedBy:             "aws",
			Type:                "kiro",
			DisplayName:         "Amazon Q Claude Haiku 4.5",
			Description:         "Claude Haiku 4.5 via Amazon Q (0.4x credit)",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
		},
	}
}
