package translator

import (
	"context"
	"strings"
	"testing"
)

func TestGenerateMarkdownDocs_EmptyRegistry(t *testing.T) {
	reg := NewRegistry()
	docs := reg.GenerateMarkdownDocs()

	if !strings.Contains(docs, "# Translation Registry") {
		t.Error("Docs should contain title")
	}
	if !strings.Contains(docs, "No translations registered") {
		t.Error("Empty registry should mention no translations")
	}
}

func TestGenerateMarkdownDocs_WithTranslations(t *testing.T) {
	reg := NewRegistry()
	reg.Register(FormatOpenAI, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{
		Stream: func(ctx context.Context, model string, origReq, convReq, resp []byte, param *any) []string {
			return []string{string(resp)}
		},
		NonStream: func(ctx context.Context, model string, origReq, convReq, resp []byte, param *any) string {
			return string(resp)
		},
	})

	docs := reg.GenerateMarkdownDocs()

	// Check for main sections
	if !strings.Contains(docs, "# Translation Registry") {
		t.Error("Docs should contain title")
	}
	if !strings.Contains(docs, "## Supported Translations") {
		t.Error("Docs should contain translations section")
	}
	if !strings.Contains(docs, "## Detailed Capabilities") {
		t.Error("Docs should contain capabilities section")
	}
	if !strings.Contains(docs, "## Supported Formats") {
		t.Error("Docs should contain formats section")
	}

	// Check for table headers
	if !strings.Contains(docs, "| From | To |") {
		t.Error("Docs should contain translation table")
	}

	// Check for format names
	if !strings.Contains(docs, "openai") {
		t.Error("Docs should contain OpenAI format")
	}
	if !strings.Contains(docs, "claude") {
		t.Error("Docs should contain Claude format")
	}
}

func TestGenerateMermaidDiagram_EmptyRegistry(t *testing.T) {
	reg := NewRegistry()
	diagram := reg.GenerateMermaidDiagram()

	if !strings.Contains(diagram, "```mermaid") {
		t.Error("Diagram should start with mermaid code block")
	}
	if !strings.Contains(diagram, "flowchart LR") {
		t.Error("Diagram should be a left-to-right flowchart")
	}
	if !strings.Contains(diagram, "No translations registered") {
		t.Error("Empty registry should mention no translations")
	}
	if !strings.Contains(diagram, "```") {
		t.Error("Diagram should end with code block")
	}
}

func TestGenerateMermaidDiagram_WithTranslations(t *testing.T) {
	reg := NewRegistry()
	reg.Register(FormatOpenAI, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{
		Stream: func(ctx context.Context, model string, origReq, convReq, resp []byte, param *any) []string {
			return []string{string(resp)}
		},
	})

	diagram := reg.GenerateMermaidDiagram()

	// Check structure
	if !strings.Contains(diagram, "```mermaid") {
		t.Error("Diagram should start with mermaid code block")
	}
	if !strings.Contains(diagram, "flowchart LR") {
		t.Error("Diagram should be a left-to-right flowchart")
	}

	// Check for node definitions
	if !strings.Contains(diagram, "openai") {
		t.Error("Diagram should contain OpenAI node")
	}
	if !strings.Contains(diagram, "claude") {
		t.Error("Diagram should contain Claude node")
	}

	// Check for edge with labels
	if !strings.Contains(diagram, "-->") {
		t.Error("Diagram should contain edges")
	}
}

func TestGenerateTranslationSummary(t *testing.T) {
	reg := NewRegistry()
	reg.Register(FormatOpenAI, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})
	reg.Register(FormatClaude, FormatGemini, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})

	summary := reg.GenerateTranslationSummary()

	if !strings.Contains(summary, "Translation Registry Summary") {
		t.Error("Summary should contain title")
	}
	if !strings.Contains(summary, "Total Formats:") {
		t.Error("Summary should contain format count")
	}
	if !strings.Contains(summary, "Total Translation Paths:") {
		t.Error("Summary should contain path count")
	}
	if !strings.Contains(summary, "Registered Paths:") {
		t.Error("Summary should contain registered paths section")
	}
}

func TestBoolToCheck(t *testing.T) {
	if boolToCheck(true) != "yes" {
		t.Error("boolToCheck(true) should return 'yes'")
	}
	if boolToCheck(false) != "-" {
		t.Error("boolToCheck(false) should return '-'")
	}
}

func TestSanitizeMermaidID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"openai", "openai"},
		{"open-ai", "open_ai"},
		{"format.v2", "format_v2"},
		{"test@format", "test_format"},
		{"ABC123", "ABC123"},
		{"a_b_c", "a_b_c"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeMermaidID(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeMermaidID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPackageLevelDocFunctions(t *testing.T) {
	// These verify package-level functions work
	markdown := GenerateMarkdownDocs()
	if markdown == "" {
		t.Error("GenerateMarkdownDocs should not return empty string")
	}

	mermaid := GenerateMermaidDiagram()
	if mermaid == "" {
		t.Error("GenerateMermaidDiagram should not return empty string")
	}

	summary := GenerateTranslationSummary()
	if summary == "" {
		t.Error("GenerateTranslationSummary should not return empty string")
	}
}

func TestGenerateMarkdownDocs_TableFormat(t *testing.T) {
	reg := NewRegistry()
	reg.Register(FormatOpenAI, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})

	docs := reg.GenerateMarkdownDocs()

	// Verify markdown table structure
	lines := strings.Split(docs, "\n")
	foundTableSeparator := false
	for _, line := range lines {
		if strings.Contains(line, "|---") {
			foundTableSeparator = true
			break
		}
	}

	if !foundTableSeparator {
		t.Error("Markdown should contain proper table separators")
	}
}

func TestGenerateMermaidDiagram_EdgeLabels(t *testing.T) {
	reg := NewRegistry()
	reg.Register(FormatOpenAI, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{
		Stream: func(ctx context.Context, model string, origReq, convReq, resp []byte, param *any) []string {
			return []string{string(resp)}
		},
		NonStream: func(ctx context.Context, model string, origReq, convReq, resp []byte, param *any) string {
			return string(resp)
		},
	})

	diagram := reg.GenerateMermaidDiagram()

	// Check for labeled edges
	if !strings.Contains(diagram, "|") {
		t.Error("Diagram should contain edge labels")
	}
	if !strings.Contains(diagram, "req") {
		t.Error("Diagram should indicate request capability")
	}
}
