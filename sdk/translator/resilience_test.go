package translator

import (
	"errors"
	"testing"
)

func TestTranslateRequestWithRecovery_DirectHit(t *testing.T) {
	reg := NewRegistry()
	reg.Register(FormatOpenAI, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return append([]byte("translated:"), data...)
	}, ResponseTransform{})

	result, err := reg.TranslateRequestWithRecovery(FormatOpenAI, FormatClaude, "model", []byte("test"), false, nil)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("Result should be successful")
	}
	if !result.DirectHit {
		t.Error("Should be a direct hit")
	}
	if result.FallbackHit {
		t.Error("Should not be a fallback hit")
	}
	if string(result.Payload) != "translated:test" {
		t.Errorf("Payload = %q, want %q", string(result.Payload), "translated:test")
	}
	if len(result.UsedPath) != 2 {
		t.Error("UsedPath should have 2 elements")
	}
}

func TestTranslateRequestWithRecovery_FallbackChain(t *testing.T) {
	reg := NewRegistry()

	// Register indirect path: OpenAI -> Gemini -> Claude
	reg.Register(FormatOpenAI, FormatGemini, func(model string, data []byte, stream bool) []byte {
		return append(data, []byte("-gemini")...)
	}, ResponseTransform{})
	reg.Register(FormatGemini, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return append(data, []byte("-claude")...)
	}, ResponseTransform{})

	// Register fallback chain
	fallbackReg := NewFallbackRegistry()
	fallbackReg.RegisterChain(FormatOpenAI, FormatClaude, []Format{FormatGemini})

	result, err := reg.TranslateRequestWithRecovery(FormatOpenAI, FormatClaude, "model", []byte("test"), false, fallbackReg)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("Result should be successful")
	}
	if result.DirectHit {
		t.Error("Should not be a direct hit")
	}
	if !result.FallbackHit {
		t.Error("Should be a fallback hit")
	}
	if string(result.Payload) != "test-gemini-claude" {
		t.Errorf("Payload = %q, want %q", string(result.Payload), "test-gemini-claude")
	}
}

func TestTranslateRequestWithRecovery_AutoDetect(t *testing.T) {
	reg := NewRegistry()

	// Register Claude -> OpenAI translator
	reg.Register(FormatClaude, FormatOpenAI, func(model string, data []byte, stream bool) []byte {
		return append([]byte("from-claude:"), data...)
	}, ResponseTransform{})

	// Payload that will be auto-detected as Claude format
	payload := []byte(`{"model": "claude-3", "messages": [], "anthropic_version": "2023-06-01"}`)

	// Request translation from unknown format to OpenAI
	// The auto-detect should identify it as Claude and use that translator
	result, err := reg.TranslateRequestWithRecovery("unknown", FormatOpenAI, "model", payload, false, nil)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("Result should be successful via auto-detect")
	}
	if !result.AutoDetectHit {
		t.Error("Should be an auto-detect hit")
	}
	if result.Detected != FormatClaude {
		t.Errorf("Detected = %v, want %v", result.Detected, FormatClaude)
	}
}

func TestTranslateRequestWithRecovery_AllStrategiesFail(t *testing.T) {
	reg := NewRegistry()
	// Empty registry - no translators

	result, err := reg.TranslateRequestWithRecovery(FormatOpenAI, FormatClaude, "model", []byte("test"), false, nil)

	if err == nil {
		t.Error("Should return error when all strategies fail")
	}
	if result.Success {
		t.Error("Result should not be successful")
	}

	// Check it's a TranslationError
	var translationErr *TranslationError
	if !errors.As(err, &translationErr) {
		t.Error("Error should be a TranslationError")
	}
}

func TestTranslationError_Error(t *testing.T) {
	err := &TranslationError{
		From:           FormatOpenAI,
		To:             FormatClaude,
		AttemptedPaths: [][]Format{{FormatOpenAI, FormatClaude}, {FormatOpenAI, FormatGemini, FormatClaude}},
		DetectedFormat: FormatGemini,
		Cause:          ErrNoTranslator,
	}

	errStr := err.Error()

	if !containsStr(errStr, "openai") {
		t.Error("Error message should contain source format")
	}
	if !containsStr(errStr, "claude") {
		t.Error("Error message should contain target format")
	}
	if !containsStr(errStr, "detected: gemini") {
		t.Error("Error message should contain detected format")
	}
	if !containsStr(errStr, "attempted paths") {
		t.Error("Error message should mention attempted paths")
	}
}

func TestTranslationError_Unwrap(t *testing.T) {
	cause := errors.New("root cause")
	err := &TranslationError{
		From:  FormatOpenAI,
		To:    FormatClaude,
		Cause: cause,
	}

	unwrapped := err.Unwrap()
	if unwrapped != cause {
		t.Error("Unwrap should return the cause")
	}
}

func TestUnregister(t *testing.T) {
	reg := NewRegistry()

	reg.Register(FormatOpenAI, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})

	// Verify they exist
	if !reg.HasRequestTranslator(FormatOpenAI, FormatClaude) {
		t.Fatal("Request translator should exist before unregister")
	}

	reg.Unregister(FormatOpenAI, FormatClaude)

	// Verify they're gone
	if reg.HasRequestTranslator(FormatOpenAI, FormatClaude) {
		t.Error("Request translator should be removed after unregister")
	}
}

func TestCloneRegistry(t *testing.T) {
	reg := NewRegistry()

	reg.Register(FormatOpenAI, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})
	reg.SetDebugMode(true)
	reg.SetDryRunMode(true)

	clone := reg.Clone()

	// Verify clone has same translators
	if !clone.HasRequestTranslator(FormatOpenAI, FormatClaude) {
		t.Error("Clone should have the same translators")
	}

	// Verify clone has same settings
	if !clone.debugMode.Load() {
		t.Error("Clone should have same debug mode")
	}
	if !clone.dryRunMode.Load() {
		t.Error("Clone should have same dry-run mode")
	}

	// Modify original, clone should be unaffected
	reg.Unregister(FormatOpenAI, FormatClaude)

	if !clone.HasRequestTranslator(FormatOpenAI, FormatClaude) {
		t.Error("Clone should be independent of original")
	}
}

func TestReplaceRegistry(t *testing.T) {
	// Save original
	original := defaultRegistry

	newReg := NewRegistry()
	newReg.Register("test", "test2", func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})

	old := ReplaceRegistry(newReg)

	if old != original {
		t.Error("ReplaceRegistry should return the old registry")
	}

	if !defaultRegistry.HasRequestTranslator("test", "test2") {
		t.Error("New registry should be active")
	}

	// Restore original
	ReplaceRegistry(original)
}

func TestReplaceRegistry_Nil(t *testing.T) {
	result := ReplaceRegistry(nil)
	if result != nil {
		t.Error("ReplaceRegistry(nil) should return nil")
	}
}

func TestListRegisteredPairs(t *testing.T) {
	reg := NewRegistry()

	reg.Register(FormatOpenAI, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})
	reg.Register(FormatClaude, FormatGemini, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})

	pairs := reg.ListRegisteredPairs()

	if len(pairs) != 2 {
		t.Errorf("Expected 2 pairs, got %d", len(pairs))
	}

	foundPairs := make(map[string]bool)
	for _, p := range pairs {
		key := string(p[0]) + "->" + string(p[1])
		foundPairs[key] = true
	}

	if !foundPairs["openai->claude"] {
		t.Error("Should contain openai->claude pair")
	}
	if !foundPairs["claude->gemini"] {
		t.Error("Should contain claude->gemini pair")
	}
}

func TestClear(t *testing.T) {
	reg := NewRegistry()

	reg.Register(FormatOpenAI, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})

	reg.Clear()

	pairs := reg.ListRegisteredPairs()
	if len(pairs) != 0 {
		t.Error("Clear should remove all translators")
	}
}

func TestHasRequestTransformer(t *testing.T) {
	reg := NewRegistry()

	reg.Register(FormatOpenAI, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})

	// HasRequestTransformer is an alias for HasRequestTranslator
	if !reg.HasRequestTransformer(FormatOpenAI, FormatClaude) {
		t.Error("HasRequestTransformer should return true for registered translator")
	}
	if reg.HasRequestTransformer(FormatClaude, FormatOpenAI) {
		t.Error("HasRequestTransformer should return false for unregistered translator")
	}
}

func TestPackageLevelResilienceFunctions(t *testing.T) {
	// Test package-level Unregister (on default registry)
	// This just verifies it doesn't panic
	Unregister("nonexistent", "format")

	// Test package-level HasRequestTransformer
	_ = HasRequestTransformer(FormatOpenAI, FormatClaude)

	// Test package-level ListRegisteredPairs
	pairs := ListRegisteredPairs()
	if pairs == nil {
		t.Error("ListRegisteredPairs should not return nil")
	}

	// Test package-level TranslateRequestWithRecovery
	result, _ := TranslateRequestWithRecovery(FormatOpenAI, FormatClaude, "model", []byte("test"), false)
	if result == nil {
		t.Error("TranslateRequestWithRecovery should not return nil result")
	}

	// Test package-level CloneRegistry
	clone := CloneRegistry()
	if clone == nil {
		t.Error("CloneRegistry should not return nil")
	}
}

// Helper function
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
