package translator

import (
	"context"
	"testing"
)

func TestGetCompatibilityMatrix(t *testing.T) {
	// Create a test registry with known translations
	reg := NewRegistry()
	reg.Register(FormatOpenAI, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})
	reg.Register(FormatOpenAI, FormatGemini, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})
	reg.Register(FormatClaude, FormatOpenAI, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})

	matrix := reg.GetCompatibilityMatrix()

	// Check OpenAI targets
	openAITargets := matrix[FormatOpenAI.String()]
	if len(openAITargets) != 2 {
		t.Errorf("OpenAI should have 2 targets, got %d", len(openAITargets))
	}

	// Check Claude targets
	claudeTargets := matrix[FormatClaude.String()]
	if len(claudeTargets) != 1 {
		t.Errorf("Claude should have 1 target, got %d", len(claudeTargets))
	}

	// Verify targets are sorted
	if len(openAITargets) >= 2 && openAITargets[0] > openAITargets[1] {
		t.Error("Targets should be sorted alphabetically")
	}
}

func TestGetSupportedFormats(t *testing.T) {
	reg := NewRegistry()
	reg.Register(FormatOpenAI, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})
	reg.Register(FormatGemini, FormatOpenAI, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})

	formats := reg.GetSupportedFormats()

	// Should have 3 unique formats: OpenAI, Claude, Gemini
	if len(formats) != 3 {
		t.Errorf("Expected 3 formats, got %d", len(formats))
	}

	// Check all expected formats are present
	formatSet := make(map[Format]bool)
	for _, f := range formats {
		formatSet[f] = true
	}

	if !formatSet[FormatOpenAI] {
		t.Error("OpenAI format should be present")
	}
	if !formatSet[FormatClaude] {
		t.Error("Claude format should be present")
	}
	if !formatSet[FormatGemini] {
		t.Error("Gemini format should be present")
	}
}

func TestIsTranslationSupported(t *testing.T) {
	reg := NewRegistry()
	reg.Register(FormatOpenAI, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})

	// Registered translation should be supported
	if !reg.IsTranslationSupported(FormatOpenAI, FormatClaude) {
		t.Error("OpenAI -> Claude should be supported")
	}

	// Non-registered translation should not be supported
	if reg.IsTranslationSupported(FormatClaude, FormatOpenAI) {
		t.Error("Claude -> OpenAI should not be supported")
	}

	// Unknown formats should not be supported
	if reg.IsTranslationSupported("unknown", FormatOpenAI) {
		t.Error("unknown -> OpenAI should not be supported")
	}
}

func TestGetTranslationInfo(t *testing.T) {
	reg := NewRegistry()
	reg.Register(FormatOpenAI, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{
		Stream:     func(ctx context.Context, model string, origReq, convReq, resp []byte, param *any) []string { return []string{string(resp)} },
		NonStream:  func(ctx context.Context, model string, origReq, convReq, resp []byte, param *any) string { return string(resp) },
		TokenCount: func(ctx context.Context, count int64) string { return "" },
	})

	info := reg.GetTranslationInfo(FormatOpenAI, FormatClaude)

	if info.From != FormatOpenAI {
		t.Errorf("From = %v, want %v", info.From, FormatOpenAI)
	}
	if info.To != FormatClaude {
		t.Errorf("To = %v, want %v", info.To, FormatClaude)
	}
	if !info.HasRequest {
		t.Error("HasRequest should be true")
	}
	if !info.HasResponse {
		t.Error("HasResponse should be true")
	}
	if !info.HasStream {
		t.Error("HasStream should be true")
	}
	if !info.HasNonStream {
		t.Error("HasNonStream should be true")
	}
	if !info.HasTokenCount {
		t.Error("HasTokenCount should be true")
	}
}

func TestGetTranslationInfo_PartialResponse(t *testing.T) {
	reg := NewRegistry()
	reg.Register(FormatOpenAI, FormatClaude, nil, ResponseTransform{
		Stream: func(ctx context.Context, model string, origReq, convReq, resp []byte, param *any) []string { return []string{string(resp)} },
		// NonStream and TokenCount not registered
	})

	info := reg.GetTranslationInfo(FormatOpenAI, FormatClaude)

	if info.HasRequest {
		t.Error("HasRequest should be false")
	}
	if !info.HasResponse {
		t.Error("HasResponse should be true")
	}
	if !info.HasStream {
		t.Error("HasStream should be true")
	}
	if info.HasNonStream {
		t.Error("HasNonStream should be false")
	}
	if info.HasTokenCount {
		t.Error("HasTokenCount should be false")
	}
}

func TestGetAllTranslations(t *testing.T) {
	reg := NewRegistry()
	reg.Register(FormatOpenAI, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})
	reg.Register(FormatClaude, FormatGemini, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})
	reg.Register(FormatGemini, FormatOpenAI, nil, ResponseTransform{
		NonStream: func(ctx context.Context, model string, origReq, convReq, resp []byte, param *any) string { return string(resp) },
	})

	translations := reg.GetAllTranslations()

	if len(translations) != 3 {
		t.Errorf("Expected 3 translations, got %d", len(translations))
	}

	// Verify sorting (by From, then To)
	for i := 1; i < len(translations); i++ {
		prev := translations[i-1]
		curr := translations[i]
		if prev.From.String() > curr.From.String() {
			t.Error("Translations should be sorted by From format")
		}
		if prev.From == curr.From && prev.To.String() > curr.To.String() {
			t.Error("Translations with same From should be sorted by To format")
		}
	}
}

func TestPackageLevelAPIFunctions(t *testing.T) {
	// These just verify the package-level functions don't panic
	// and delegate to the default registry

	matrix := GetCompatibilityMatrix()
	if matrix == nil {
		t.Error("GetCompatibilityMatrix should not return nil")
	}

	formats := GetSupportedFormats()
	if formats == nil {
		t.Error("GetSupportedFormats should not return nil")
	}

	// IsTranslationSupported should work
	_ = IsTranslationSupported(FormatOpenAI, FormatClaude)

	// GetTranslationInfo should return a valid pointer
	info := GetTranslationInfo(FormatOpenAI, FormatClaude)
	if info == nil {
		t.Error("GetTranslationInfo should not return nil")
	}

	// GetAllTranslations should work
	translations := GetAllTranslations()
	if translations == nil {
		t.Error("GetAllTranslations should not return nil")
	}
}

func TestEmptyRegistry_API(t *testing.T) {
	reg := NewRegistry()

	matrix := reg.GetCompatibilityMatrix()
	if len(matrix) != 0 {
		t.Error("Empty registry should have empty matrix")
	}

	formats := reg.GetSupportedFormats()
	if len(formats) != 0 {
		t.Error("Empty registry should have no formats")
	}

	translations := reg.GetAllTranslations()
	if len(translations) != 0 {
		t.Error("Empty registry should have no translations")
	}
}
