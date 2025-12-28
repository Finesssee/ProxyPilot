package translator

import (
	"fmt"
	"sort"
	"strings"
)

// GenerateMarkdownDocs generates a markdown documentation string for all registered translations.
// The output includes a table showing all translation paths and their capabilities.
func (r *Registry) GenerateMarkdownDocs() string {
	translations := r.GetAllTranslations()
	if len(translations) == 0 {
		return "# Translation Registry\n\nNo translations registered.\n"
	}

	var sb strings.Builder

	sb.WriteString("# Translation Registry\n\n")
	sb.WriteString("## Supported Translations\n\n")
	sb.WriteString("| From | To | Registered |\n")
	sb.WriteString("|------|-----|------------|\n")

	for _, t := range translations {
		registered := ""
		if t.HasRequest || t.HasResponse {
			registered = "yes"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", t.From, t.To, registered))
	}

	sb.WriteString("\n## Detailed Capabilities\n\n")
	sb.WriteString("| From | To | Request | Stream | Non-Stream | Token Count |\n")
	sb.WriteString("|------|-----|---------|--------|------------|-------------|\n")

	for _, t := range translations {
		request := boolToCheck(t.HasRequest)
		stream := boolToCheck(t.HasStream)
		nonStream := boolToCheck(t.HasNonStream)
		tokenCount := boolToCheck(t.HasTokenCount)
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
			t.From, t.To, request, stream, nonStream, tokenCount))
	}

	sb.WriteString("\n## Supported Formats\n\n")
	formats := r.GetSupportedFormats()
	for _, f := range formats {
		sb.WriteString(fmt.Sprintf("- `%s`\n", f))
	}

	return sb.String()
}

// GenerateMermaidDiagram generates a Mermaid flowchart diagram showing translation paths.
// The output can be rendered by Mermaid-compatible markdown viewers.
func (r *Registry) GenerateMermaidDiagram() string {
	translations := r.GetAllTranslations()
	if len(translations) == 0 {
		return "```mermaid\nflowchart LR\n    NoTranslations[\"No translations registered\"]\n```\n"
	}

	var sb strings.Builder
	sb.WriteString("```mermaid\n")
	sb.WriteString("flowchart LR\n")

	// Collect unique formats for node definitions
	formatSet := make(map[Format]struct{})
	for _, t := range translations {
		formatSet[t.From] = struct{}{}
		formatSet[t.To] = struct{}{}
	}

	// Sort formats for consistent output
	formats := make([]Format, 0, len(formatSet))
	for f := range formatSet {
		formats = append(formats, f)
	}
	sort.Slice(formats, func(i, j int) bool {
		return formats[i].String() < formats[j].String()
	})

	// Define nodes with sanitized IDs
	for _, f := range formats {
		nodeID := sanitizeMermaidID(f.String())
		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", nodeID, f))
	}

	sb.WriteString("\n")

	// Define edges
	for _, t := range translations {
		fromID := sanitizeMermaidID(t.From.String())
		toID := sanitizeMermaidID(t.To.String())

		// Add edge label based on capabilities
		var labels []string
		if t.HasRequest {
			labels = append(labels, "req")
		}
		if t.HasStream {
			labels = append(labels, "stream")
		}
		if t.HasNonStream {
			labels = append(labels, "non-stream")
		}

		if len(labels) > 0 {
			sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", fromID, strings.Join(labels, ", "), toID))
		} else {
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", fromID, toID))
		}
	}

	sb.WriteString("```\n")

	return sb.String()
}

// GenerateMarkdownDocs generates markdown docs from the default registry.
func GenerateMarkdownDocs() string {
	return defaultRegistry.GenerateMarkdownDocs()
}

// GenerateMermaidDiagram generates a mermaid diagram from the default registry.
func GenerateMermaidDiagram() string {
	return defaultRegistry.GenerateMermaidDiagram()
}

// boolToCheck converts a boolean to a checkmark or empty string.
func boolToCheck(b bool) string {
	if b {
		return "yes"
	}
	return "-"
}

// sanitizeMermaidID converts a format string into a valid Mermaid node ID.
// Mermaid node IDs must be alphanumeric with underscores.
func sanitizeMermaidID(s string) string {
	result := strings.Builder{}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		} else {
			result.WriteRune('_')
		}
	}
	return result.String()
}

// GenerateTranslationSummary returns a concise summary of translation capabilities.
func (r *Registry) GenerateTranslationSummary() string {
	matrix := r.GetCompatibilityMatrix()
	formats := r.GetSupportedFormats()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Translation Registry Summary\n"))
	sb.WriteString(fmt.Sprintf("============================\n\n"))
	sb.WriteString(fmt.Sprintf("Total Formats: %d\n", len(formats)))

	totalPaths := 0
	for _, targets := range matrix {
		totalPaths += len(targets)
	}
	sb.WriteString(fmt.Sprintf("Total Translation Paths: %d\n\n", totalPaths))

	sb.WriteString("Registered Paths:\n")
	for from, targets := range matrix {
		if len(targets) > 0 {
			sb.WriteString(fmt.Sprintf("  %s -> %s\n", from, strings.Join(targets, ", ")))
		}
	}

	return sb.String()
}

// GenerateTranslationSummary returns a summary from the default registry.
func GenerateTranslationSummary() string {
	return defaultRegistry.GenerateTranslationSummary()
}
