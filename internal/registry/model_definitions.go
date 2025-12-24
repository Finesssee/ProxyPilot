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
			Thinking:                   &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true},
		},
		{
			ID:                         "gemini-3-pro-low-preview",
			Object:                     "model",
			Created:                    1737158400,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/gemini-3-pro-low",
			Version:                    "3.0",
			DisplayName:                "Gemini 3 Pro Low Preview",
			Description:                "Gemini 3 Pro Low Preview",
			InputTokenLimit:            1048576,
			OutputTokenLimit:           65536,
			SupportedGenerationMethods: []string{"generateContent", "countTokens", "createCachedContent", "batchGenerateContent"},
			Thinking:                   &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true},
		},
		{
			ID:                         "gemini-3-flash-preview",
			Object:                     "model",
			Created:                    1737158400,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/gemini-3-flash-preview",
			Version:                    "3.0",
			DisplayName:                "Gemini 3 Flash Preview",
			Description:                "Gemini 3 Flash Preview",
			InputTokenLimit:            1048576,
			OutputTokenLimit:           65536,
			SupportedGenerationMethods: []string{"generateContent", "countTokens", "createCachedContent", "batchGenerateContent"},
		},
		{
			ID:                         "gemini-3-flash",
			Object:                     "model",
			Created:                    1737158400,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/gemini-3-flash",
			Version:                    "3.0",
			DisplayName:                "Gemini 3 Flash",
			Description:                "Gemini 3 Flash",
			InputTokenLimit:            1048576,
			OutputTokenLimit:           65536,
			SupportedGenerationMethods: []string{"generateContent", "countTokens", "createCachedContent", "batchGenerateContent"},
		},
		{
			ID:                         "antigravity-claude-sonnet-4-5-thinking",
			Object:                     "model",
			Created:                    1759104000,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/antigravity-claude-sonnet-4-5-thinking",
			Version:                    "4.5",
			DisplayName:                "Antigravity Claude 4.5 Sonnet (Thinking)",
			Description:                "Antigravity-routed Claude 4.5 Sonnet with thinking support",
			InputTokenLimit:            200000,
			OutputTokenLimit:           64000,
			SupportedGenerationMethods: []string{"generateContent", "countTokens", "createCachedContent", "batchGenerateContent"},
			Thinking:                   &ThinkingSupport{Min: 1024, Max: 200000, ZeroAllowed: false, DynamicAllowed: true},
		},
		{
			ID:                         "antigravity-claude-opus-4-5-thinking",
			Object:                     "model",
			Created:                    1761955200,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/antigravity-claude-opus-4-5-thinking",
			Version:                    "4.5",
			DisplayName:                "Antigravity Claude 4.5 Opus (Thinking)",
			Description:                "Antigravity-routed Claude 4.5 Opus with thinking support",
			InputTokenLimit:            200000,
			OutputTokenLimit:           64000,
			SupportedGenerationMethods: []string{"generateContent", "countTokens", "createCachedContent", "batchGenerateContent"},
			Thinking:                   &ThinkingSupport{Min: 1024, Max: 200000, ZeroAllowed: false, DynamicAllowed: true},
		},
		{
			ID:                         "text-embedding-004",
			Object:                     "model",
			Created:                    1715644800,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/text-embedding-004",
			Version:                    "004",
			DisplayName:                "Text Embedding 004",
			Description:                "Google's state-of-the-art text embedding model",
			InputTokenLimit:            2048,
			OutputTokenLimit:           768,
			SupportedGenerationMethods: []string{"embedContent", "batchEmbedContents"},
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
			Thinking:                   &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true},
		},
	}
}

func GetGeminiVertexModels() []*ModelInfo {
	return []*ModelInfo{
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
			Thinking:                   &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true},
		},
		{
			ID:                         "gemini-3-pro-low-preview",
			Object:                     "model",
			Created:                    1737158400,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/gemini-3-pro-low",
			Version:                    "3.0",
			DisplayName:                "Gemini 3 Pro Low Preview",
			Description:                "Gemini 3 Pro Low Preview",
			InputTokenLimit:            1048576,
			OutputTokenLimit:           65536,
			SupportedGenerationMethods: []string{"generateContent", "countTokens", "createCachedContent", "batchGenerateContent"},
			Thinking:                   &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true},
		},
		{
			ID:                         "gemini-3-flash-preview",
			Object:                     "model",
			Created:                    1737158400,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/gemini-3-flash-preview",
			Version:                    "3.0",
			DisplayName:                "Gemini 3 Flash Preview",
			Description:                "Gemini 3 Flash Preview",
			InputTokenLimit:            1048576,
			OutputTokenLimit:           65536,
			SupportedGenerationMethods: []string{"generateContent", "countTokens", "createCachedContent", "batchGenerateContent"},
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
			Thinking:                   &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true},
		},
	}
}

// GetGeminiCLIModels returns the standard Gemini model definitions
func GetGeminiCLIModels() []*ModelInfo {
	return []*ModelInfo{
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
			Thinking:                   &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true},
		},
		        		{
		        			ID:                         "gemini-3-flash",
		        			Object:                     "model",
		        			Created:                    1737158400,
		        			OwnedBy:                    "google",
		        			Type:                       "gemini",
		        			Name:                       "models/gemini-3-flash",
		        			Version:                    "3.0",
		        			DisplayName:                "Gemini 3 Flash",
		        			Description:                "Gemini 3 Flash",
		        			InputTokenLimit:            1048576,
		        			OutputTokenLimit:           65536,
		        			SupportedGenerationMethods: []string{"generateContent", "countTokens", "createCachedContent", "batchGenerateContent"},
		        		},
		        		{
		        			ID:                         "text-embedding-004",			Object:                     "model",
			Created:                    1715644800,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/text-embedding-004",
			Version:                    "004",
			DisplayName:                "Text Embedding 004",
			Description:                "Google's state-of-the-art text embedding model",
			InputTokenLimit:            2048,
			OutputTokenLimit:           768,
			SupportedGenerationMethods: []string{"embedContent", "batchEmbedContents"},
		},
	}
}
// GetAIStudioModels returns the Gemini model definitions for AI Studio integrations
func GetAIStudioModels() []*ModelInfo {
	return []*ModelInfo{
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
			Thinking:                   &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true},
		},
		{
			ID:                         "gemini-3-flash",
			Object:                     "model",
			Created:                    1737158400,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/gemini-3-flash",
			Version:                    "3.0",
			DisplayName:                "Gemini 3 Flash",
			Description:                "Gemini 3 Flash",
			InputTokenLimit:            1048576,
			OutputTokenLimit:           65536,
			SupportedGenerationMethods: []string{"generateContent", "countTokens", "createCachedContent", "batchGenerateContent"},
		},
		{
			ID:                         "gemini-pro-latest",
			Object:                     "model",
			Created:                    1750118400,
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
			Created:                    1750118400,
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
		{
			ID:                         "gemini-flash-lite-latest",
			Object:                     "model",
			Created:                    1753142400,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/gemini-flash-lite-latest",
			Version:                    "2.5",
			DisplayName:                "Gemini Flash-Lite Latest",
			Description:                "Latest release of Gemini Flash-Lite",
			InputTokenLimit:            1048576,
			OutputTokenLimit:           65536,
			SupportedGenerationMethods: []string{"generateContent", "countTokens", "createCachedContent", "batchGenerateContent"},
			Thinking:                   &ThinkingSupport{Min: 512, Max: 24576, ZeroAllowed: true, DynamicAllowed: true},
		},
		{
			ID:                         "text-embedding-004",
			Object:                     "model",
			Created:                    1715644800,
			OwnedBy:                    "google",
			Type:                       "gemini",
			Name:                       "models/text-embedding-004",
			Version:                    "004",
			DisplayName:                "Text Embedding 004",
			Description:                "Google's state-of-the-art text embedding model",
			InputTokenLimit:            2048,
			OutputTokenLimit:           768,
			SupportedGenerationMethods: []string{"embedContent", "batchEmbedContents"},
		},
	}
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
			Version:             "gpt-5.2-codex",
			DisplayName:         "GPT 5.2 Codex",
			Description:         "GPT 5.2 specialized for code",
			ContextLength:       400000,
			MaxCompletionTokens: 128000,
			SupportedParameters: []string{"tools"},
			Thinking:            &ThinkingSupport{Levels: []string{"none", "low", "medium", "high", "xhigh"}},
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
		{
			ID:                  "vision-model",
			Object:              "model",
			Created:             1758672000,
			OwnedBy:             "qwen",
			Type:                "qwen",
			Version:             "3.0",
			DisplayName:         "Qwen3 Vision Model",
			Description:         "Vision model model",
			ContextLength:       32768,
			MaxCompletionTokens: 2048,
			SupportedParameters: []string{"temperature", "top_p", "max_tokens", "stream", "stop"},
		},
	}
}

// GetIFlowModels returns supported models for iFlow OAuth accounts.
func GetIFlowModels() []*ModelInfo {
	entries := []struct {
		ID          string
		DisplayName string
		Description string
		Created     int64
		Thinking    *ThinkingSupport
	}{
		{ID: "tstars2.0", DisplayName: "TStars-2.0", Description: "iFlow TStars-2.0 multimodal assistant", Created: 1746489600},
		{ID: "qwen3-coder-plus", DisplayName: "Qwen3-Coder-Plus", Description: "Qwen3 Coder Plus code generation", Created: 1753228800},
		{ID: "qwen3-max", DisplayName: "Qwen3-Max", Description: "Qwen3 flagship model", Created: 1758672000},
		{ID: "qwen3-vl-plus", DisplayName: "Qwen3-VL-Plus", Description: "Qwen3 multimodal vision-language", Created: 1758672000},
		{ID: "qwen3-max-preview", DisplayName: "Qwen3-Max-Preview", Description: "Qwen3 Max preview build", Created: 1757030400},
		{ID: "kimi-k2-0905", DisplayName: "Kimi-K2-Instruct-0905", Description: "Moonshot Kimi K2 instruct 0905", Created: 1757030400},
		{ID: "glm-4.6", DisplayName: "GLM-4.6", Description: "Zhipu GLM 4.6 general model", Created: 1759190400},
		{ID: "glm-4.7", DisplayName: "GLM-4.7", Description: "Zhipu GLM 4.7 flagship model", Created: 1765000000},
		{ID: "kimi-k2", DisplayName: "Kimi-K2", Description: "Moonshot Kimi K2 general model", Created: 1752192000},
		{ID: "kimi-k2-thinking", DisplayName: "Kimi-K2-Thinking", Description: "Moonshot Kimi K2 thinking model", Created: 1762387200, Thinking: &ThinkingSupport{Levels: []string{"low", "medium", "high"}}},
		{ID: "qwen3-32b", DisplayName: "Qwen3-32B", Description: "Qwen3 32B", Created: 1747094400},
		{ID: "qwen3-235b-a22b-instruct", DisplayName: "Qwen3-235B-A22B-Instruct", Description: "Qwen3 235B A22B Instruct", Created: 1753401600},
		{ID: "qwen3-235b", DisplayName: "Qwen3-235B-A22B", Description: "Qwen3 235B A22B", Created: 1753401600},
		{ID: "minimax-m2", DisplayName: "MiniMax-M2", Description: "MiniMax M2", Created: 1758672000, Thinking: &ThinkingSupport{Levels: []string{"low", "medium", "high"}}},
		{ID: "minimax-m2.1", DisplayName: "MiniMax-M2.1", Description: "MiniMax M2.1 flagship", Created: 1765000000, Thinking: &ThinkingSupport{Levels: []string{"low", "medium", "high"}}},
	}
	models := make([]*ModelInfo, 0, len(entries))
	for _, entry := range entries {
		models = append(models, &ModelInfo{
			ID:          entry.ID,
			Object:      "model",
			Created:     entry.Created,
			OwnedBy:     "iflow",
			Type:        "iflow",
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
	Name                string
}

// GetAntigravityModelConfig returns static configuration for antigravity models.
// Keys use the ALIASED model names (after modelName2Alias conversion) for direct lookup.
func GetAntigravityModelConfig() map[string]*AntigravityModelConfig {
	return map[string]*AntigravityModelConfig{
		"gemini-2.5-flash":                        {Thinking: &ThinkingSupport{Min: 0, Max: 24576, ZeroAllowed: true, DynamicAllowed: true}, Name: "models/gemini-2.5-flash"},
		"gemini-2.5-flash-lite":                   {Thinking: &ThinkingSupport{Min: 0, Max: 24576, ZeroAllowed: true, DynamicAllowed: true}, Name: "models/gemini-2.5-flash-lite"},
		"gemini-2.5-computer-use-preview-10-2025": {Name: "models/gemini-2.5-computer-use-preview-10-2025"},
		"gemini-2.5-image-pro-preview":            {Name: "models/gemini-2.5-image-pro-preview"},
		"gemini-3-pro-preview":                    {Thinking: &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true, Levels: []string{"high"}}, Name: "models/gemini-3-pro-preview"},
		"gemini-3-pro-low-preview":                {Thinking: &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true}, Name: "models/gemini-3-pro-low"},
		"gemini-3-pro-image-preview":              {Thinking: &ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false, DynamicAllowed: true}, Name: "models/gemini-3-pro-image-preview"},
		"gemini-3-flash-preview":                  {Name: "models/gemini-3-flash-preview"},
		"gemini-3-flash":                          {Name: "models/gemini-3-flash"},
		"antigravity-claude-sonnet-4-5-thinking":  {Thinking: &ThinkingSupport{Min: 1024, Max: 200000, ZeroAllowed: false, DynamicAllowed: true}, MaxCompletionTokens: 64000},
		"antigravity-claude-opus-4-5-thinking":    {Thinking: &ThinkingSupport{Min: 1024, Max: 200000, ZeroAllowed: false, DynamicAllowed: true}, MaxCompletionTokens: 64000},
	}
}
