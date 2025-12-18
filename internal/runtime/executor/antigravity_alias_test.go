package executor

import "testing"

func TestAntigravityModelAlias_Gemini3ProLow(t *testing.T) {
	if got := modelName2Alias("gemini-3-pro-low"); got != "gemini-3-pro-low-preview" {
		t.Fatalf("modelName2Alias(gemini-3-pro-low)=%q", got)
	}
	if got := alias2ModelName("gemini-3-pro-low-preview"); got != "gemini-3-pro-low" {
		t.Fatalf("alias2ModelName(gemini-3-pro-low-preview)=%q", got)
	}
}
