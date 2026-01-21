package claude

import (
	"context"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/cache"
)

// ============================================================================
// Signature Caching Tests
// ============================================================================

func TestConvertAntigravityResponseToClaude_ThinkingTextAccumulated(t *testing.T) {
	cache.ClearSignatureCache("")

	requestJSON := []byte(`{
		"model": "claude-sonnet-4-5-thinking",
		"messages": [{"role": "user", "content": [{"type": "text", "text": "Test"}]}]
	}`)

	// First thinking chunk
	chunk1 := []byte(`{
		"response": {
			"candidates": [{
				"content": {
					"parts": [{"text": "First part of thinking...", "thought": true}]
				}
			}]
		}
	}`)

	// Second thinking chunk (continuation)
	chunk2 := []byte(`{
		"response": {
			"candidates": [{
				"content": {
					"parts": [{"text": " Second part of thinking.", "thought": true}]
				}
			}]
		}
	}`)

	var param any
	ctx := context.Background()

	// Process both chunks
	ConvertAntigravityResponseToClaude(ctx, "claude-sonnet-4-5-thinking", requestJSON, requestJSON, chunk1, &param)
	ConvertAntigravityResponseToClaude(ctx, "claude-sonnet-4-5-thinking", requestJSON, requestJSON, chunk2, &param)

	// Verify thinking text was accumulated
	params := param.(*Params)
	text := params.CurrentThinkingText.String()
	if !strings.Contains(text, "First part") || !strings.Contains(text, "Second part") {
		t.Errorf("Thinking text should accumulate both parts, got: %s", text)
	}
}

func TestConvertAntigravityResponseToClaude_SignatureCached(t *testing.T) {
	cache.ClearSignatureCache("")

	requestJSON := []byte(`{
		"model": "claude-sonnet-4-5-thinking",
		"messages": [{"role": "user", "content": [{"type": "text", "text": "Cache test"}]}]
	}`)

	// Thinking chunk
	thinkingChunk := []byte(`{
		"response": {
			"candidates": [{
				"content": {
					"parts": [{"text": "My thinking process here", "thought": true}]
				}
			}]
		}
	}`)

	// Signature chunk
	validSignature := "abc123validSignature1234567890123456789012345678901234567890"
	signatureChunk := []byte(`{
		"response": {
			"candidates": [{
				"content": {
					"parts": [{"text": "", "thought": true, "thoughtSignature": "` + validSignature + `"}]
				}
			}]
		}
	}`)

	var param any
	ctx := context.Background()

	// Process thinking chunk
	ConvertAntigravityResponseToClaude(ctx, "claude-sonnet-4-5-thinking", requestJSON, requestJSON, thinkingChunk, &param)
	params := param.(*Params)
	thinkingText := params.CurrentThinkingText.String()

	if thinkingText == "" {
		t.Fatal("Thinking text should be accumulated")
	}

	// Process signature chunk - should cache the signature
	ConvertAntigravityResponseToClaude(ctx, "claude-sonnet-4-5-thinking", requestJSON, requestJSON, signatureChunk, &param)

	// Verify signature was cached
	cachedSig := cache.GetCachedSignature("claude-sonnet-4-5-thinking", thinkingText)
	if cachedSig != validSignature {
		t.Errorf("Expected cached signature '%s', got '%s'", validSignature, cachedSig)
	}

	// Verify thinking text was reset after caching
	if params.CurrentThinkingText.Len() != 0 {
		t.Error("Thinking text should be reset after signature is cached")
	}
}
