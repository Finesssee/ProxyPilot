// Package handlers provides prompt cache tracking integration for API handlers.
// This file implements system prompt extraction from different API formats (Claude, OpenAI, Gemini)
// and integrates with the prompt cache infrastructure for tracking.
package handlers

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/cache"
	"github.com/tidwall/gjson"
)

// PromptCacheStatus represents the cache status for a system prompt.
type PromptCacheStatus string

const (
	// PromptCacheHit indicates the system prompt was already cached.
	PromptCacheHit PromptCacheStatus = "HIT"
	// PromptCacheMiss indicates this is a new/uncached system prompt.
	PromptCacheMiss PromptCacheStatus = "MISS"
	// PromptCacheDisabled indicates prompt caching is disabled.
	PromptCacheDisabled PromptCacheStatus = "DISABLED"
)

// SetPromptCacheHeader sets the X-Prompt-Cache-Status header on the response.
func SetPromptCacheHeader(c *gin.Context, status PromptCacheStatus) {
	c.Header("X-Prompt-Cache-Status", string(status))
}

// ExtractAndTrackSystemPrompt extracts the system prompt from raw JSON request
// based on the handler type, tracks it in the prompt cache, and returns the cache status.
// It handles different API formats:
// - Claude: "system" field at top level (string or array)
// - OpenAI: first message with role="system" in messages array
// - Gemini: "systemInstruction" field at top level
//
// Returns the cache status and the extracted system prompt hash.
func ExtractAndTrackSystemPrompt(handlerType string, rawJSON []byte, provider string) (PromptCacheStatus, string) {
	systemPrompt := extractSystemPrompt(handlerType, rawJSON)
	if systemPrompt == "" {
		return PromptCacheDisabled, ""
	}

	// Check if prompt caching is enabled
	promptCache := cache.GetDefaultPromptCache()
	if promptCache == nil || !promptCache.IsEnabled() {
		return PromptCacheDisabled, cache.HashPrompt(systemPrompt)
	}

	// Check if already cached before tracking
	hash, hitCount, found := cache.IsPromptCached(systemPrompt)

	// Track the system prompt (this also updates hit counts)
	cache.TrackSystemPrompt(systemPrompt, provider)

	if found && hitCount > 0 {
		return PromptCacheHit, hash
	}
	return PromptCacheMiss, hash
}

// extractSystemPrompt extracts the system prompt from different API formats.
func extractSystemPrompt(handlerType string, rawJSON []byte) string {
	root := gjson.ParseBytes(rawJSON)

	switch strings.ToLower(handlerType) {
	case "claude", "anthropic":
		return extractClaudeSystemPrompt(root)
	case "openai", "openai-responses", "openai_response":
		return extractOpenAISystemPrompt(root)
	case "gemini":
		return extractGeminiSystemPrompt(root)
	default:
		// Try all formats as fallback
		if prompt := extractClaudeSystemPrompt(root); prompt != "" {
			return prompt
		}
		if prompt := extractOpenAISystemPrompt(root); prompt != "" {
			return prompt
		}
		return extractGeminiSystemPrompt(root)
	}
}

// extractClaudeSystemPrompt extracts system prompt from Claude/Anthropic format.
// Claude has "system" as a top-level field, which can be a string or array of content blocks.
func extractClaudeSystemPrompt(root gjson.Result) string {
	system := root.Get("system")
	if !system.Exists() {
		return ""
	}

	// Simple string format
	if system.Type == gjson.String {
		return strings.TrimSpace(system.String())
	}

	// Array of content blocks format
	if system.IsArray() {
		var parts []string
		system.ForEach(func(_, value gjson.Result) bool {
			if text := value.Get("text").String(); text != "" {
				parts = append(parts, text)
			} else if value.Type == gjson.String {
				parts = append(parts, value.String())
			}
			return true
		})
		return strings.TrimSpace(strings.Join(parts, "\n"))
	}

	return ""
}

// extractOpenAISystemPrompt extracts system prompt from OpenAI format.
// OpenAI has system messages in the messages array with role="system".
func extractOpenAISystemPrompt(root gjson.Result) string {
	messages := root.Get("messages")
	if !messages.Exists() || !messages.IsArray() {
		// Also check for "instructions" field (used in some OpenAI variants like Responses API)
		if instructions := root.Get("instructions"); instructions.Exists() {
			return strings.TrimSpace(instructions.String())
		}
		return ""
	}

	var systemParts []string
	messages.ForEach(func(_, msg gjson.Result) bool {
		role := msg.Get("role").String()
		if role != "system" {
			return true
		}

		content := msg.Get("content")
		if content.Type == gjson.String {
			systemParts = append(systemParts, content.String())
		} else if content.IsArray() {
			content.ForEach(func(_, block gjson.Result) bool {
				if text := block.Get("text").String(); text != "" {
					systemParts = append(systemParts, text)
				}
				return true
			})
		}
		return true
	})

	return strings.TrimSpace(strings.Join(systemParts, "\n"))
}

// extractGeminiSystemPrompt extracts system prompt from Gemini format.
// Gemini has "systemInstruction" at top level with parts array.
func extractGeminiSystemPrompt(root gjson.Result) string {
	systemInstruction := root.Get("systemInstruction")
	if !systemInstruction.Exists() {
		// Also check for system_instruction (snake_case variant)
		systemInstruction = root.Get("system_instruction")
		if !systemInstruction.Exists() {
			return ""
		}
	}

	// String format
	if systemInstruction.Type == gjson.String {
		return strings.TrimSpace(systemInstruction.String())
	}

	// Object with parts array
	parts := systemInstruction.Get("parts")
	if !parts.Exists() || !parts.IsArray() {
		// Check for direct text field
		if text := systemInstruction.Get("text").String(); text != "" {
			return strings.TrimSpace(text)
		}
		return ""
	}

	var textParts []string
	parts.ForEach(func(_, part gjson.Result) bool {
		if text := part.Get("text").String(); text != "" {
			textParts = append(textParts, text)
		}
		return true
	})

	return strings.TrimSpace(strings.Join(textParts, "\n"))
}
