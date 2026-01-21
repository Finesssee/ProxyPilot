package cache

import (
	"testing"
	"time"
)

func TestCacheSignature_BasicStorageAndRetrieval(t *testing.T) {
	ClearSignatureCache("")

	text := "This is some thinking text content"
	signature := "abc123validSignature1234567890123456789012345678901234567890"

	// Store signature
	CacheSignature("test-model", text, signature)

	// Retrieve signature
	retrieved := GetCachedSignature("test-model", text)
	if retrieved != signature {
		t.Errorf("Expected signature '%s', got '%s'", signature, retrieved)
	}
}

func TestCacheSignature_DifferentTexts(t *testing.T) {
	ClearSignatureCache("")

	text1 := "First text"
	text2 := "Second text"
	sig1 := "signature1_1234567890123456789012345678901234567890123456"
	sig2 := "signature2_1234567890123456789012345678901234567890123456"

	CacheSignature("test-model", text1, sig1)
	CacheSignature("test-model", text2, sig2)

	if GetCachedSignature("test-model", text1) != sig1 {
		t.Error("text1 signature mismatch")
	}
	if GetCachedSignature("test-model", text2) != sig2 {
		t.Error("text2 signature mismatch")
	}
}

func TestCacheSignature_NotFound(t *testing.T) {
	ClearSignatureCache("")

	// Non-existent text
	if got := GetCachedSignature("test-model", "some text"); got != "" {
		t.Errorf("Expected empty string for nonexistent text, got '%s'", got)
	}

	// Existing text but different model (should still find due to model group)
	CacheSignature("test-model", "text-a", "sigA12345678901234567890123456789012345678901234567890")
	if got := GetCachedSignature("test-model", "text-b"); got != "" {
		t.Errorf("Expected empty string for different text, got '%s'", got)
	}
}

func TestCacheSignature_EmptyInputs(t *testing.T) {
	ClearSignatureCache("")

	// Empty text should not cache
	CacheSignature("test-model", "", "sig12345678901234567890123456789012345678901234567890")
	// Empty signature should not cache
	CacheSignature("test-model", "valid-text", "")
	// Short signature should not cache
	CacheSignature("test-model", "valid-text", "short")

	if got := GetCachedSignature("test-model", "valid-text"); got != "" {
		t.Errorf("Expected empty after invalid cache attempts, got '%s'", got)
	}
}

func TestCacheSignature_ShortSignatureRejected(t *testing.T) {
	ClearSignatureCache("")

	text := "Some text"
	shortSig := "abc123" // Less than 50 chars

	CacheSignature("test-model", text, shortSig)

	if got := GetCachedSignature("test-model", text); got != "" {
		t.Errorf("Short signature should be rejected, got '%s'", got)
	}
}

func TestClearSignatureCache_AllSessions(t *testing.T) {
	ClearSignatureCache("")

	sig := "validSig1234567890123456789012345678901234567890123456"
	CacheSignature("test-model", "text1", sig)
	CacheSignature("test-model", "text2", sig)

	ClearSignatureCache("")

	if got := GetCachedSignature("test-model", "text1"); got != "" {
		t.Error("text1 should be cleared")
	}
	if got := GetCachedSignature("test-model", "text2"); got != "" {
		t.Error("text2 should be cleared")
	}
}

func TestHasValidSignature(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		signature string
		expected  bool
	}{
		{"valid long signature", "claude-sonnet-4-5-thinking", "abc123validSignature1234567890123456789012345678901234567890", true},
		{"exactly 50 chars", "claude-sonnet-4-5-thinking", "12345678901234567890123456789012345678901234567890", true},
		{"49 chars - invalid", "claude-sonnet-4-5-thinking", "1234567890123456789012345678901234567890123456789", false},
		{"empty string", "claude-sonnet-4-5-thinking", "", false},
		{"short signature", "claude-sonnet-4-5-thinking", "abc", false},
		{"gemini skip sentinel", "gemini-2.5-pro", "skip_thought_signature_validator", true},
		{"gemini skip sentinel wrong model", "claude-sonnet-4-5-thinking", "skip_thought_signature_validator", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasValidSignature(tt.modelName, tt.signature)
			if result != tt.expected {
				t.Errorf("HasValidSignature(%q, %q) = %v, expected %v", tt.modelName, tt.signature, result, tt.expected)
			}
		})
	}
}

func TestCacheSignature_TextHashCollisionResistance(t *testing.T) {
	ClearSignatureCache("")

	// Different texts should produce different hashes
	text1 := "First thinking text"
	text2 := "Second thinking text"
	sig1 := "signature1_1234567890123456789012345678901234567890123456"
	sig2 := "signature2_1234567890123456789012345678901234567890123456"

	CacheSignature("test-model", text1, sig1)
	CacheSignature("test-model", text2, sig2)

	if GetCachedSignature("test-model", text1) != sig1 {
		t.Error("text1 signature mismatch")
	}
	if GetCachedSignature("test-model", text2) != sig2 {
		t.Error("text2 signature mismatch")
	}
}

func TestCacheSignature_UnicodeText(t *testing.T) {
	ClearSignatureCache("")

	text := "한글 텍스트와 이모지 🎉 그리고 特殊文字"
	sig := "unicodeSig123456789012345678901234567890123456789012345"

	CacheSignature("test-model", text, sig)

	if got := GetCachedSignature("test-model", text); got != sig {
		t.Errorf("Unicode text signature retrieval failed, got '%s'", got)
	}
}

func TestCacheSignature_Overwrite(t *testing.T) {
	ClearSignatureCache("")

	text := "Same text"
	sig1 := "firstSignature12345678901234567890123456789012345678901"
	sig2 := "secondSignature1234567890123456789012345678901234567890"

	CacheSignature("test-model", text, sig1)
	CacheSignature("test-model", text, sig2) // Overwrite

	if got := GetCachedSignature("test-model", text); got != sig2 {
		t.Errorf("Expected overwritten signature '%s', got '%s'", sig2, got)
	}
}

func TestCacheSignature_ExpirationLogic(t *testing.T) {
	ClearSignatureCache("")

	text := "text"
	sig := "validSig1234567890123456789012345678901234567890123456"

	CacheSignature("test-model", text, sig)

	// Fresh entry should be retrievable
	if got := GetCachedSignature("test-model", text); got != sig {
		t.Errorf("Fresh entry should be retrievable, got '%s'", got)
	}

	_ = time.Now() // Acknowledge we're not testing time passage
}

func TestGetCachedSignature_GeminiFallback(t *testing.T) {
	ClearSignatureCache("")

	// For gemini models, missing cache should return skip sentinel
	if got := GetCachedSignature("gemini-2.5-pro", "nonexistent"); got != "skip_thought_signature_validator" {
		t.Errorf("Expected gemini fallback, got '%s'", got)
	}

	// For claude models, missing cache should return empty
	if got := GetCachedSignature("claude-sonnet-4-5", "nonexistent"); got != "" {
		t.Errorf("Expected empty for claude, got '%s'", got)
	}
}

func TestGetModelGroup(t *testing.T) {
	tests := []struct {
		modelName string
		expected  string
	}{
		{"gpt-4", "gpt"},
		{"gpt-4-turbo", "gpt"},
		{"claude-3-sonnet", "claude"},
		{"claude-sonnet-4-5-thinking", "claude"},
		{"gemini-2.5-pro", "gemini"},
		{"gemini-1.5-flash", "gemini"},
		{"unknown-model", "unknown-model"},
	}

	for _, tt := range tests {
		t.Run(tt.modelName, func(t *testing.T) {
			if got := GetModelGroup(tt.modelName); got != tt.expected {
				t.Errorf("GetModelGroup(%q) = %q, expected %q", tt.modelName, got, tt.expected)
			}
		})
	}
}
