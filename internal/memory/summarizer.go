package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

// SummarizerConfig holds configuration for LLM-based context summarization.
type SummarizerConfig struct {
	Enabled              bool          // Enable LLM summarization (default: true)
	ThresholdPercent     float64       // Trigger at N% of context window (default: 0.75)
	MaxSummaryTokens     int           // Max tokens for summary output (default: 2000)
	FallbackToRegex      bool          // Use regex-based summary if LLM fails (default: true)
	SummarizationTimeout time.Duration // Timeout for summarization call (default: 30s)
}

// DefaultSummarizerConfig returns the default summarizer configuration.
func DefaultSummarizerConfig() SummarizerConfig {
	return SummarizerConfig{
		Enabled:              true,
		ThresholdPercent:     0.75,
		MaxSummaryTokens:     2000,
		FallbackToRegex:      true,
		SummarizationTimeout: 30 * time.Second,
	}
}

// Summarizer handles LLM-based context summarization.
type Summarizer struct {
	config   SummarizerConfig
	executor SummarizerExecutor
}

// NewSummarizer creates a new Summarizer with the given config and executor.
func NewSummarizer(config SummarizerConfig, executor SummarizerExecutor) *Summarizer {
	return &Summarizer{
		config:   config,
		executor: executor,
	}
}

// GenerateInitialSummary creates the first summary from dropped events.
func (s *Summarizer) GenerateInitialSummary(ctx context.Context, model string, dropped []Event, latestIntent string) (*StructuredSummary, error) {
	if len(dropped) == 0 {
		return &StructuredSummary{
			Version: "1.0",
			Metadata: SummaryMetadata{
				UpdatedAt:        time.Now(),
				CompressionCount: 1,
				TokensUsed:       0,
			},
		}, nil
	}

	prompt := buildInitialPrompt(dropped, latestIntent)

	ctx, cancel := context.WithTimeout(ctx, s.config.SummarizationTimeout)
	defer cancel()

	response, err := s.executor.Summarize(ctx, model, prompt)
	if err != nil {
		log.Printf("[summarizer] LLM summarization failed: %v", err)
		if s.config.FallbackToRegex {
			log.Printf("[summarizer] falling back to regex-based summary")
			return regexFallback(nil, dropped, latestIntent), nil
		}
		return nil, fmt.Errorf("summarization failed: %w", err)
	}

	summary, err := parseSummaryResponse(response)
	if err != nil {
		log.Printf("[summarizer] failed to parse LLM response: %v", err)
		if s.config.FallbackToRegex {
			log.Printf("[summarizer] falling back to regex-based summary")
			return regexFallback(nil, dropped, latestIntent), nil
		}
		return nil, fmt.Errorf("failed to parse summary response: %w", err)
	}

	summary.Version = "1.0"
	summary.Metadata = SummaryMetadata{
		UpdatedAt:        time.Now(),
		CompressionCount: 1,
		TokensUsed:       estimateTokens(response),
	}

	return ensureTokenBudget(summary), nil
}

// MergeSummary performs an incremental merge of new events into an existing summary.
func (s *Summarizer) MergeSummary(ctx context.Context, model string, existing *StructuredSummary, dropped []Event, latestIntent string) (*StructuredSummary, error) {
	if existing == nil {
		return s.GenerateInitialSummary(ctx, model, dropped, latestIntent)
	}

	if len(dropped) == 0 {
		return existing, nil
	}

	prompt := buildMergePrompt(existing, dropped, latestIntent)

	ctx, cancel := context.WithTimeout(ctx, s.config.SummarizationTimeout)
	defer cancel()

	response, err := s.executor.Summarize(ctx, model, prompt)
	if err != nil {
		log.Printf("[summarizer] LLM merge summarization failed: %v", err)
		if s.config.FallbackToRegex {
			log.Printf("[summarizer] falling back to regex-based merge")
			return regexFallback(existing, dropped, latestIntent), nil
		}
		return nil, fmt.Errorf("merge summarization failed: %w", err)
	}

	summary, err := parseSummaryResponse(response)
	if err != nil {
		log.Printf("[summarizer] failed to parse LLM merge response: %v", err)
		if s.config.FallbackToRegex {
			log.Printf("[summarizer] falling back to regex-based merge")
			return regexFallback(existing, dropped, latestIntent), nil
		}
		return nil, fmt.Errorf("failed to parse merge response: %w", err)
	}

	summary.Version = existing.Version
	summary.Metadata = SummaryMetadata{
		UpdatedAt:        time.Now(),
		CompressionCount: existing.Metadata.CompressionCount + 1,
		TokensUsed:       estimateTokens(response),
	}

	return ensureTokenBudget(summary), nil
}

// buildInitialPrompt constructs the prompt for generating an initial summary.
func buildInitialPrompt(dropped []Event, latestIntent string) string {
	var sb strings.Builder

	sb.WriteString(`You are a context summarizer. Generate a structured summary of the following conversation events.

Output a JSON object with this exact structure:
{
  "session_intent": "Brief description of the user's main goal",
  "file_modifications": [
    {"path": "/exact/file/path.go", "action": "created|modified|deleted", "description": "Brief change description"}
  ],
  "decisions_made": [
    {"decision": "Decision description", "rationale": "Why this decision was made"}
  ],
  "next_steps": [
    {"description": "Task description", "completed": false}
  ],
  "technical_details": "Important technical context as a single string"
}

Guidelines:
- Keep each section concise (max 5 items per list, max 12 files)
- Preserve file paths exactly as they appear
- Focus on actionable information
- session_intent should be 1-2 sentences max

`)

	if latestIntent != "" {
		sb.WriteString("Latest user intent: ")
		sb.WriteString(latestIntent)
		sb.WriteString("\n\n")
	}

	sb.WriteString("Events to summarize:\n")
	sb.WriteString(formatDroppedEvents(dropped))

	return sb.String()
}

// buildMergePrompt constructs the prompt for merging new events into an existing summary.
func buildMergePrompt(existing *StructuredSummary, dropped []Event, latestIntent string) string {
	var sb strings.Builder

	sb.WriteString(`You are a context summarizer performing an incremental update. Merge the new events into the existing summary.

Output a JSON object with this exact structure:
{
  "session_intent": "Updated description of the user's main goal",
  "file_modifications": [
    {"path": "/exact/file/path.go", "action": "created|modified|deleted", "description": "Brief change description"}
  ],
  "decisions_made": [
    {"decision": "Decision description", "rationale": "Why this decision was made"}
  ],
  "next_steps": [
    {"description": "Task description", "completed": false}
  ],
  "technical_details": "Important technical context as a single string"
}

Guidelines:
- Update existing sections with new information
- Consolidate duplicates (merge multiple modifications to same file)
- Keep each section concise (max 5 items per list, max 12 files)
- Preserve file paths exactly as they appear
- Mark completed tasks as completed: true
- Remove obsolete information
- session_intent should reflect the evolved understanding

`)

	sb.WriteString("Existing summary:\n")
	existingJSON, _ := json.MarshalIndent(existing, "", "  ")
	sb.WriteString(string(existingJSON))
	sb.WriteString("\n\n")

	if latestIntent != "" {
		sb.WriteString("Latest user intent: ")
		sb.WriteString(latestIntent)
		sb.WriteString("\n\n")
	}

	sb.WriteString("New events to merge:\n")
	sb.WriteString(formatDroppedEvents(dropped))

	return sb.String()
}

// formatDroppedEvents formats events for inclusion in prompts.
func formatDroppedEvents(dropped []Event) string {
	var sb strings.Builder

	for i, e := range dropped {
		sb.WriteString(fmt.Sprintf("[%d] ", i+1))
		if e.Role != "" {
			sb.WriteString(fmt.Sprintf("[%s] ", e.Role))
		}
		if e.Kind != "" {
			sb.WriteString(fmt.Sprintf("(%s) ", e.Kind))
		}
		if e.Type != "" {
			sb.WriteString(fmt.Sprintf("<%s> ", e.Type))
		}
		sb.WriteString(e.Text)
		if len(e.Meta) > 0 {
			metaJSON, _ := json.Marshal(e.Meta)
			sb.WriteString(fmt.Sprintf(" [meta: %s]", string(metaJSON)))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// ensureTokenBudget trims the summary if it exceeds the token budget.
func ensureTokenBudget(summary *StructuredSummary) *StructuredSummary {
	if summary == nil {
		return nil
	}

	// Limit file modifications to 12
	if len(summary.FileModifications) > 12 {
		summary.FileModifications = summary.FileModifications[:12]
	}

	// Limit decisions to 5
	if len(summary.DecisionsMade) > 5 {
		summary.DecisionsMade = summary.DecisionsMade[:5]
	}

	// Limit next steps to 5
	if len(summary.NextSteps) > 5 {
		summary.NextSteps = summary.NextSteps[:5]
	}

	// Truncate overly long strings
	if len(summary.SessionIntent) > 500 {
		summary.SessionIntent = summary.SessionIntent[:500] + "..."
	}

	for i := range summary.FileModifications {
		if len(summary.FileModifications[i].Description) > 200 {
			summary.FileModifications[i].Description = summary.FileModifications[i].Description[:200] + "..."
		}
	}

	for i := range summary.DecisionsMade {
		if len(summary.DecisionsMade[i].Decision) > 200 {
			summary.DecisionsMade[i].Decision = summary.DecisionsMade[i].Decision[:200] + "..."
		}
		if len(summary.DecisionsMade[i].Rationale) > 200 {
			summary.DecisionsMade[i].Rationale = summary.DecisionsMade[i].Rationale[:200] + "..."
		}
	}

	for i := range summary.NextSteps {
		if len(summary.NextSteps[i].Description) > 200 {
			summary.NextSteps[i].Description = summary.NextSteps[i].Description[:200] + "..."
		}
	}

	// Truncate technical details if too long
	if len(summary.TechnicalDetails) > 1500 {
		summary.TechnicalDetails = summary.TechnicalDetails[:1500] + "..."
	}

	return summary
}

// regexFallback generates a summary using regex-based extraction when LLM fails.
func regexFallback(existing *StructuredSummary, dropped []Event, latestIntent string) *StructuredSummary {
	summary := &StructuredSummary{
		Version:           "1.0",
		FileModifications: make([]FileModification, 0),
		DecisionsMade:     make([]Decision, 0),
		NextSteps:         make([]Task, 0),
		TechnicalDetails:  "",
	}

	// Copy existing data if available
	if existing != nil {
		summary.Version = existing.Version
		summary.SessionIntent = existing.SessionIntent
		summary.FileModifications = append(summary.FileModifications, existing.FileModifications...)
		summary.DecisionsMade = append(summary.DecisionsMade, existing.DecisionsMade...)
		summary.NextSteps = append(summary.NextSteps, existing.NextSteps...)
		summary.TechnicalDetails = existing.TechnicalDetails
		summary.Metadata = existing.Metadata
	}

	// Update session intent if provided
	if latestIntent != "" {
		if summary.SessionIntent == "" {
			summary.SessionIntent = latestIntent
		} else {
			summary.SessionIntent = latestIntent + " (previously: " + summary.SessionIntent + ")"
		}
	}

	// Regex patterns for file path extraction
	filePathRegex := regexp.MustCompile(`(?i)(?:file|path|modified|created|edited|deleted|updated)[:=\s]+["']?([/\\]?[\w./\\-]+\.\w+)["']?`)
	absolutePathRegex := regexp.MustCompile(`(?m)^([A-Za-z]:)?[/\\][\w./\\-]+\.\w+`)

	seenFiles := make(map[string]bool)
	for _, fm := range summary.FileModifications {
		seenFiles[fm.Path] = true
	}

	var technicalDetailsBuilder strings.Builder
	if summary.TechnicalDetails != "" {
		technicalDetailsBuilder.WriteString(summary.TechnicalDetails)
		technicalDetailsBuilder.WriteString("\n")
	}

	// Extract information from dropped events
	for _, e := range dropped {
		text := e.Text

		// Extract file paths
		matches := filePathRegex.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 1 {
				path := strings.TrimSpace(match[1])
				if path != "" && !seenFiles[path] && len(summary.FileModifications) < 12 {
					seenFiles[path] = true
					action := "modified"
					lowerText := strings.ToLower(text)
					if strings.Contains(lowerText, "creat") || strings.Contains(lowerText, "new") {
						action = "created"
					} else if strings.Contains(lowerText, "delet") || strings.Contains(lowerText, "remov") {
						action = "deleted"
					}
					summary.FileModifications = append(summary.FileModifications, FileModification{
						Path:        path,
						Action:      action,
						Description: truncateString(text, 100),
					})
				}
			}
		}

		// Also check for absolute paths
		absMatches := absolutePathRegex.FindAllString(text, -1)
		for _, path := range absMatches {
			path = strings.TrimSpace(path)
			if path != "" && !seenFiles[path] && len(summary.FileModifications) < 12 {
				seenFiles[path] = true
				summary.FileModifications = append(summary.FileModifications, FileModification{
					Path:        path,
					Action:      "referenced",
					Description: truncateString(text, 100),
				})
			}
		}

		// Extract decisions from assistant messages
		if e.Role == "assistant" && len(text) > 20 && len(summary.DecisionsMade) < 5 {
			if strings.Contains(strings.ToLower(text), "decid") ||
				strings.Contains(strings.ToLower(text), "chose") ||
				strings.Contains(strings.ToLower(text), "will use") ||
				strings.Contains(strings.ToLower(text), "going to") {
				decision := Decision{
					Decision:  truncateString(text, 150),
					Rationale: "Extracted from conversation",
				}
				summary.DecisionsMade = append(summary.DecisionsMade, decision)
			}
		}

		// Extract technical details from tool results or code blocks
		if e.Kind == "tool_result" || strings.Contains(text, "```") {
			detail := truncateString(text, 200)
			if technicalDetailsBuilder.Len()+len(detail) < 1500 {
				technicalDetailsBuilder.WriteString(detail)
				technicalDetailsBuilder.WriteString("\n")
			}
		}
	}

	summary.TechnicalDetails = strings.TrimSpace(technicalDetailsBuilder.String())

	// Update metadata
	now := time.Now()
	if existing == nil {
		summary.Metadata = SummaryMetadata{
			UpdatedAt:        now,
			CompressionCount: 1,
			TokensUsed:       0,
		}
	} else {
		summary.Metadata.UpdatedAt = now
		summary.Metadata.CompressionCount++
	}

	return ensureTokenBudget(summary)
}

// parseSummaryResponse parses the LLM response into a StructuredSummary.
func parseSummaryResponse(response string) (*StructuredSummary, error) {
	response = strings.TrimSpace(response)

	// Try to extract JSON from the response
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")

	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no valid JSON found in response")
	}

	jsonStr := response[jsonStart : jsonEnd+1]

	var summary StructuredSummary
	if err := json.Unmarshal([]byte(jsonStr), &summary); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return &summary, nil
}

// truncateString truncates a string to the specified length.
func truncateString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// estimateTokens provides a rough token count estimate based on character count.
// Uses the rough approximation of ~4 characters per token for English text.
func estimateTokens(text string) int {
	return len(text) / 4
}
