package memory

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// FileModification represents a file change in the session
type FileModification struct {
	Path        string `json:"path"`
	Action      string `json:"action"` // created, modified, deleted, referenced
	Description string `json:"description"`
}

// Decision represents a decision made during the session
type Decision struct {
	Decision  string `json:"decision"`
	Rationale string `json:"rationale"`
}

// Task represents a next step or task
type Task struct {
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
}

// SummaryMetadata contains metadata about the summary
type SummaryMetadata struct {
	UpdatedAt        time.Time `json:"updated_at"`
	CompressionCount int       `json:"compression_count"`
	TokensUsed       int       `json:"tokens_used"`
}

// StructuredSummary represents a Factory.ai-style structured summary
type StructuredSummary struct {
	Version           string             `json:"version"`
	SessionIntent     string             `json:"session_intent"`
	FileModifications []FileModification `json:"file_modifications"`
	DecisionsMade     []Decision         `json:"decisions_made"`
	NextSteps         []Task             `json:"next_steps"`
	TechnicalDetails  string             `json:"technical_details"`
	Metadata          SummaryMetadata    `json:"metadata"`
}

// ParseStructuredSummary parses markdown text with XML markers into a StructuredSummary
func ParseStructuredSummary(text string) (*StructuredSummary, error) {
	// Extract content between XML markers
	xmlPattern := regexp.MustCompile(`(?s)<structured_summary>(.*?)</structured_summary>`)
	matches := xmlPattern.FindStringSubmatch(text)

	var content string
	if len(matches) > 1 {
		content = matches[1]
	} else {
		// If no XML markers, treat entire text as content
		content = text
	}

	summary := &StructuredSummary{
		Version:           "1.0",
		FileModifications: []FileModification{},
		DecisionsMade:     []Decision{},
		NextSteps:         []Task{},
		Metadata: SummaryMetadata{
			UpdatedAt: time.Now(),
		},
	}

	// Parse Session Intent
	summary.SessionIntent = parseSection(content, "Session Intent")

	// Parse File Modifications table
	summary.FileModifications = parseFileModificationsTable(content)

	// Parse Decisions Made
	summary.DecisionsMade = parseDecisions(content)

	// Parse Next Steps
	summary.NextSteps = parseNextSteps(content)

	// Parse Technical Details
	summary.TechnicalDetails = parseSection(content, "Technical Details")

	// Parse Metadata
	parseMetadata(content, &summary.Metadata)

	return summary, nil
}

// parseSection extracts content from a markdown section
func parseSection(content, sectionName string) string {
	// Match section header and capture content until next section or end
	pattern := regexp.MustCompile(`(?si)##\s*` + regexp.QuoteMeta(sectionName) + `\s*\n(.*?)(?:##\s|\z)`)
	matches := pattern.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// parseFileModificationsTable parses the file modifications markdown table
func parseFileModificationsTable(content string) []FileModification {
	var modifications []FileModification

	// Find the File Modifications section
	sectionContent := parseSection(content, "File Modifications")
	if sectionContent == "" {
		return modifications
	}

	// Parse table rows (skip header and separator)
	lines := strings.Split(sectionContent, "\n")
	inTable := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if this is a table row
		if strings.HasPrefix(line, "|") {
			// Skip header separator (contains dashes)
			if strings.Contains(line, "---") {
				inTable = true
				continue
			}

			// Skip header row (first pipe-delimited row before separator)
			if !inTable {
				continue
			}

			// Parse table row
			cells := strings.Split(line, "|")
			if len(cells) >= 4 {
				path := strings.TrimSpace(cells[1])
				action := strings.TrimSpace(cells[2])
				description := strings.TrimSpace(cells[3])

				// Remove backticks from path if present
				path = strings.Trim(path, "`")

				if path != "" {
					modifications = append(modifications, FileModification{
						Path:        path,
						Action:      action,
						Description: description,
					})
				}
			}
		}
	}

	return modifications
}

// parseDecisions parses the Decisions Made section
func parseDecisions(content string) []Decision {
	var decisions []Decision

	sectionContent := parseSection(content, "Decisions Made")
	if sectionContent == "" {
		return decisions
	}

	// Match list items with optional rationale in parentheses or after colon
	lines := strings.Split(sectionContent, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Remove list marker
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")

		// Check for numbered list
		numberedPattern := regexp.MustCompile(`^\d+\.\s*`)
		line = numberedPattern.ReplaceAllString(line, "")

		if line == "" {
			continue
		}

		decision := Decision{}

		// Try to extract rationale from parentheses
		rationalePattern := regexp.MustCompile(`^(.*?)\s*\((.*?)\)\s*$`)
		if matches := rationalePattern.FindStringSubmatch(line); len(matches) > 2 {
			decision.Decision = strings.TrimSpace(matches[1])
			decision.Rationale = strings.TrimSpace(matches[2])
		} else if colonIdx := strings.Index(line, ":"); colonIdx > 0 {
			// Try colon separator
			decision.Decision = strings.TrimSpace(line[:colonIdx])
			decision.Rationale = strings.TrimSpace(line[colonIdx+1:])
		} else {
			decision.Decision = line
		}

		if decision.Decision != "" {
			decisions = append(decisions, decision)
		}
	}

	return decisions
}

// parseNextSteps parses the Next Steps section with checkboxes
func parseNextSteps(content string) []Task {
	var tasks []Task

	sectionContent := parseSection(content, "Next Steps")
	if sectionContent == "" {
		return tasks
	}

	lines := strings.Split(sectionContent, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Remove list marker
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")

		task := Task{}

		// Check for checkbox
		if strings.HasPrefix(line, "[x]") || strings.HasPrefix(line, "[X]") {
			task.Completed = true
			line = strings.TrimPrefix(line, "[x]")
			line = strings.TrimPrefix(line, "[X]")
		} else if strings.HasPrefix(line, "[ ]") {
			task.Completed = false
			line = strings.TrimPrefix(line, "[ ]")
		}

		task.Description = strings.TrimSpace(line)

		if task.Description != "" {
			tasks = append(tasks, task)
		}
	}

	return tasks
}

// parseMetadata parses the Metadata section
func parseMetadata(content string, metadata *SummaryMetadata) {
	sectionContent := parseSection(content, "Metadata")
	if sectionContent == "" {
		return
	}

	lines := strings.Split(sectionContent, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Remove list markers
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")

		// Parse key-value pairs
		if colonIdx := strings.Index(line, ":"); colonIdx > 0 {
			key := strings.ToLower(strings.TrimSpace(line[:colonIdx]))
			value := strings.TrimSpace(line[colonIdx+1:])

			switch {
			case strings.Contains(key, "updated") || strings.Contains(key, "time"):
				if t, err := time.Parse(time.RFC3339, value); err == nil {
					metadata.UpdatedAt = t
				}
			case strings.Contains(key, "compression"):
				if count, err := strconv.Atoi(value); err == nil {
					metadata.CompressionCount = count
				}
			case strings.Contains(key, "token"):
				if tokens, err := strconv.Atoi(value); err == nil {
					metadata.TokensUsed = tokens
				}
			}
		}
	}
}

// RenderStructuredSummary generates markdown format with XML markers
func RenderStructuredSummary(summary *StructuredSummary) string {
	var sb strings.Builder

	sb.WriteString("<structured_summary>\n\n")

	// Session Intent
	sb.WriteString("## Session Intent\n")
	sb.WriteString(summary.SessionIntent)
	sb.WriteString("\n\n")

	// File Modifications
	sb.WriteString("## File Modifications\n")
	if len(summary.FileModifications) > 0 {
		sb.WriteString("| Path | Action | Description |\n")
		sb.WriteString("|------|--------|-------------|\n")
		for _, mod := range summary.FileModifications {
			sb.WriteString(fmt.Sprintf("| `%s` | %s | %s |\n", mod.Path, mod.Action, mod.Description))
		}
	} else {
		sb.WriteString("No file modifications recorded.\n")
	}
	sb.WriteString("\n")

	// Decisions Made
	sb.WriteString("## Decisions Made\n")
	if len(summary.DecisionsMade) > 0 {
		for _, decision := range summary.DecisionsMade {
			if decision.Rationale != "" {
				sb.WriteString(fmt.Sprintf("- %s (%s)\n", decision.Decision, decision.Rationale))
			} else {
				sb.WriteString(fmt.Sprintf("- %s\n", decision.Decision))
			}
		}
	} else {
		sb.WriteString("No decisions recorded.\n")
	}
	sb.WriteString("\n")

	// Next Steps
	sb.WriteString("## Next Steps\n")
	if len(summary.NextSteps) > 0 {
		for _, task := range summary.NextSteps {
			checkbox := "[ ]"
			if task.Completed {
				checkbox = "[x]"
			}
			sb.WriteString(fmt.Sprintf("- %s %s\n", checkbox, task.Description))
		}
	} else {
		sb.WriteString("No next steps recorded.\n")
	}
	sb.WriteString("\n")

	// Technical Details
	sb.WriteString("## Technical Details\n")
	if summary.TechnicalDetails != "" {
		sb.WriteString(summary.TechnicalDetails)
	} else {
		sb.WriteString("No technical details recorded.")
	}
	sb.WriteString("\n\n")

	// Metadata
	sb.WriteString("## Metadata\n")
	sb.WriteString(fmt.Sprintf("- Updated At: %s\n", summary.Metadata.UpdatedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- Compression Count: %d\n", summary.Metadata.CompressionCount))
	sb.WriteString(fmt.Sprintf("- Tokens Used: %d\n", summary.Metadata.TokensUsed))
	sb.WriteString("\n")

	sb.WriteString("</structured_summary>")

	return sb.String()
}

// MergeStructuredSummaries combines two summaries, preferring newer data
func MergeStructuredSummaries(existing, new *StructuredSummary) *StructuredSummary {
	if existing == nil {
		return new
	}
	if new == nil {
		return existing
	}

	merged := &StructuredSummary{
		Version: new.Version,
		Metadata: SummaryMetadata{
			UpdatedAt:        time.Now(),
			CompressionCount: existing.Metadata.CompressionCount + 1,
			TokensUsed:       new.Metadata.TokensUsed,
		},
	}

	// Prefer newer session intent
	if new.SessionIntent != "" {
		merged.SessionIntent = new.SessionIntent
	} else {
		merged.SessionIntent = existing.SessionIntent
	}

	// Merge file modifications, deduplicate by path, prefer newer
	fileModMap := make(map[string]FileModification)
	for _, mod := range existing.FileModifications {
		fileModMap[mod.Path] = mod
	}
	for _, mod := range new.FileModifications {
		fileModMap[mod.Path] = mod
	}

	merged.FileModifications = make([]FileModification, 0, len(fileModMap))
	for _, mod := range fileModMap {
		merged.FileModifications = append(merged.FileModifications, mod)
	}
	// Limit to max 12 file modifications
	if len(merged.FileModifications) > 12 {
		merged.FileModifications = merged.FileModifications[len(merged.FileModifications)-12:]
	}

	// Merge decisions, prefer newer, limit to 5
	merged.DecisionsMade = mergeDecisions(existing.DecisionsMade, new.DecisionsMade, 5)

	// Merge next steps, prefer newer, limit to 5
	merged.NextSteps = mergeTasks(existing.NextSteps, new.NextSteps, 5)

	// Prefer newer technical details
	if new.TechnicalDetails != "" {
		merged.TechnicalDetails = new.TechnicalDetails
	} else {
		merged.TechnicalDetails = existing.TechnicalDetails
	}

	return merged
}

// mergeDecisions merges two decision slices with deduplication and limit
func mergeDecisions(existing, new []Decision, maxItems int) []Decision {
	seen := make(map[string]bool)
	var result []Decision

	// Add new decisions first (preferred)
	for _, d := range new {
		key := strings.ToLower(d.Decision)
		if !seen[key] {
			seen[key] = true
			result = append(result, d)
		}
	}

	// Add existing decisions that aren't duplicates
	for _, d := range existing {
		key := strings.ToLower(d.Decision)
		if !seen[key] {
			seen[key] = true
			result = append(result, d)
		}
	}

	// Limit to maxItems
	if len(result) > maxItems {
		result = result[:maxItems]
	}

	return result
}

// mergeTasks merges two task slices with deduplication and limit
func mergeTasks(existing, new []Task, maxItems int) []Task {
	seen := make(map[string]bool)
	var result []Task

	// Add new tasks first (preferred)
	for _, t := range new {
		key := strings.ToLower(t.Description)
		if !seen[key] {
			seen[key] = true
			result = append(result, t)
		}
	}

	// Add existing incomplete tasks that aren't duplicates
	for _, t := range existing {
		key := strings.ToLower(t.Description)
		if !seen[key] && !t.Completed {
			seen[key] = true
			result = append(result, t)
		}
	}

	// Limit to maxItems
	if len(result) > maxItems {
		result = result[:maxItems]
	}

	return result
}
