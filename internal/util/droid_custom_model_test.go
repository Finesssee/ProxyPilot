package util

import "testing"

func TestNormalizeDroidCustomModel_GeminiClaude(t *testing.T) {
	in := "custom:CLIProxy-(local):-gemini-claude-opus-4-5-thinking-12"
	if got := NormalizeDroidCustomModel(in); got != "gemini-claude-opus-4-5-thinking" {
		t.Fatalf("NormalizeDroidCustomModel=%q", got)
	}
}

func TestNormalizeDroidCustomModel_GPTReasoning(t *testing.T) {
	in := "custom:CLIProxy-(local):-gpt-5.2-(reasoning:-medium)-2"
	if got := NormalizeDroidCustomModel(in); got != "gpt-5.2(medium)" {
		t.Fatalf("NormalizeDroidCustomModel=%q", got)
	}
}
