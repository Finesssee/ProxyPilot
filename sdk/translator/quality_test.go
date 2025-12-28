package translator

import (
	"testing"
)

func TestScoreTranslation_PerfectMatch(t *testing.T) {
	before := []byte(`{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`)
	after := []byte(`{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`)

	report := ScoreTranslation(FormatOpenAI, FormatClaude, before, after)

	if report.Score < 0.9 {
		t.Errorf("Perfect match should have high score, got %v", report.Score)
	}
	if report.FieldsDropped != 0 {
		t.Errorf("Perfect match should have no dropped fields, got %d", report.FieldsDropped)
	}
	if report.FieldsAdded != 0 {
		t.Errorf("Perfect match should have no added fields, got %d", report.FieldsAdded)
	}
}

func TestScoreTranslation_DataLoss(t *testing.T) {
	before := []byte(`{
		"model": "gpt-4",
		"messages": [{"role": "user", "content": "Hello"}],
		"temperature": 0.7,
		"max_tokens": 100
	}`)
	after := []byte(`{
		"model": "gpt-4",
		"messages": [{"role": "user", "content": "Hello"}]
	}`)

	report := ScoreTranslation(FormatOpenAI, FormatClaude, before, after)

	if report.FieldsDropped == 0 {
		t.Error("Should detect dropped fields")
	}
	if report.Score >= 1.0 {
		t.Error("Score should be less than perfect due to data loss")
	}
	if len(report.DroppedFields) == 0 {
		t.Error("DroppedFields should list the dropped field paths")
	}
}

func TestScoreTranslation_FieldsAdded(t *testing.T) {
	before := []byte(`{"model": "gpt-4"}`)
	after := []byte(`{"model": "gpt-4", "extra_field": "value"}`)

	report := ScoreTranslation(FormatOpenAI, FormatClaude, before, after)

	if report.FieldsAdded == 0 {
		t.Error("Should detect added fields")
	}
	if len(report.AddedFields) == 0 {
		t.Error("AddedFields should list the added field paths")
	}
}

func TestScoreTranslation_InvalidJSON(t *testing.T) {
	tests := []struct {
		name   string
		before []byte
		after  []byte
	}{
		{
			name:   "Invalid source JSON",
			before: []byte(`{invalid}`),
			after:  []byte(`{"valid": true}`),
		},
		{
			name:   "Invalid target JSON",
			before: []byte(`{"valid": true}`),
			after:  []byte(`{invalid}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := ScoreTranslation(FormatOpenAI, FormatClaude, tt.before, tt.after)

			if report.Score != 0 {
				t.Error("Invalid JSON should result in 0 score")
			}
			if len(report.Warnings) == 0 {
				t.Error("Should have warning about parse failure")
			}
		})
	}
}

func TestScoreTranslation_Warnings(t *testing.T) {
	// Test critical field warning
	before := []byte(`{"model": "gpt-4", "messages": []}`)
	after := []byte(`{"different": "structure"}`)

	report := ScoreTranslation(FormatOpenAI, FormatClaude, before, after)

	if len(report.Warnings) == 0 {
		t.Error("Should generate warnings for dropped critical fields")
	}

	// Check for critical field warning
	foundCriticalWarning := false
	for _, w := range report.Warnings {
		if contains(w, "critical field") {
			foundCriticalWarning = true
			break
		}
	}
	if !foundCriticalWarning {
		t.Error("Should warn about dropped critical fields like 'model' or 'messages'")
	}
}

func TestScoreTranslation_LowQualityWarning(t *testing.T) {
	before := []byte(`{"a": 1, "b": 2, "c": 3, "d": 4, "e": 5}`)
	after := []byte(`{"x": 1}`)

	report := ScoreTranslation(FormatOpenAI, FormatClaude, before, after)

	if report.Score >= 0.5 {
		t.Error("Score should be low for significant data loss")
	}

	foundQualityWarning := false
	for _, w := range report.Warnings {
		if contains(w, "quality") {
			foundQualityWarning = true
			break
		}
	}
	if !foundQualityWarning {
		t.Error("Should warn about low translation quality")
	}
}

func TestExtractPaths(t *testing.T) {
	data := map[string]interface{}{
		"model": "gpt-4",
		"messages": []interface{}{
			map[string]interface{}{
				"role":    "user",
				"content": "Hello",
			},
		},
	}

	paths := extractPaths(data, "")

	expectedPaths := []string{
		"model",
		"messages",
		"messages[0]",
		"messages[0].role",
		"messages[0].content",
	}

	pathSet := make(map[string]bool)
	for _, p := range paths {
		pathSet[p] = true
	}

	for _, expected := range expectedPaths {
		if !pathSet[expected] {
			t.Errorf("Expected path %q not found in extracted paths", expected)
		}
	}
}

func TestCalculateScore(t *testing.T) {
	tests := []struct {
		name     string
		report   QualityReport
		minScore float64
		maxScore float64
	}{
		{
			name: "All fields mapped",
			report: QualityReport{
				FieldsMapped:  10,
				FieldsDropped: 0,
				FieldsAdded:   0,
			},
			minScore: 1.0,
			maxScore: 1.0,
		},
		{
			name: "Half fields dropped",
			report: QualityReport{
				FieldsMapped:  5,
				FieldsDropped: 5,
				FieldsAdded:   0,
			},
			minScore: 0.0,
			maxScore: 0.5,
		},
		{
			name: "Empty source",
			report: QualityReport{
				FieldsMapped:  0,
				FieldsDropped: 0,
				FieldsAdded:   0,
			},
			minScore: 1.0,
			maxScore: 1.0,
		},
		{
			name: "Empty source with added fields",
			report: QualityReport{
				FieldsMapped:  0,
				FieldsDropped: 0,
				FieldsAdded:   5,
			},
			minScore: 0.5,
			maxScore: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculateScore(tt.report)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("Score = %v, want between %v and %v", score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestCompareJSONStructures(t *testing.T) {
	before := []byte(`{"a": 1, "b": {"c": 2}}`)
	after := []byte(`{"a": 1, "b": {"d": 3}}`)

	comp, err := CompareJSONStructures(before, after)
	if err != nil {
		t.Fatalf("CompareJSONStructures failed: %v", err)
	}

	if comp.SourceType != "object" {
		t.Errorf("SourceType = %v, want object", comp.SourceType)
	}
	if comp.TargetType != "object" {
		t.Errorf("TargetType = %v, want object", comp.TargetType)
	}

	// Should have diffs for b.c (removed) and b.d (added)
	if len(comp.Diffs) < 2 {
		t.Errorf("Expected at least 2 diffs, got %d", len(comp.Diffs))
	}

	foundRemoved := false
	foundAdded := false
	for _, diff := range comp.Diffs {
		if diff.DiffType == "removed" && diff.Path == "b.c" {
			foundRemoved = true
		}
		if diff.DiffType == "added" && diff.Path == "b.d" {
			foundAdded = true
		}
	}

	if !foundRemoved {
		t.Error("Should detect removed field b.c")
	}
	if !foundAdded {
		t.Error("Should detect added field b.d")
	}
}

func TestCompareJSONStructures_TypeChange(t *testing.T) {
	before := []byte(`{"field": "string"}`)
	after := []byte(`{"field": 123}`)

	comp, err := CompareJSONStructures(before, after)
	if err != nil {
		t.Fatalf("CompareJSONStructures failed: %v", err)
	}

	foundTypeChange := false
	for _, diff := range comp.Diffs {
		if diff.DiffType == "type_changed" && diff.Path == "field" {
			foundTypeChange = true
			if diff.SourceType != "string" {
				t.Errorf("SourceType = %v, want string", diff.SourceType)
			}
			if diff.TargetType != "number" {
				t.Errorf("TargetType = %v, want number", diff.TargetType)
			}
		}
	}

	if !foundTypeChange {
		t.Error("Should detect type change")
	}
}

func TestCompareJSONStructures_ArrayDiffs(t *testing.T) {
	before := []byte(`{"items": [1, 2, 3]}`)
	after := []byte(`{"items": [1, 2]}`)

	comp, err := CompareJSONStructures(before, after)
	if err != nil {
		t.Fatalf("CompareJSONStructures failed: %v", err)
	}

	foundRemovedItem := false
	for _, diff := range comp.Diffs {
		if diff.DiffType == "removed" && diff.Path == "items[2]" {
			foundRemovedItem = true
		}
	}

	if !foundRemovedItem {
		t.Error("Should detect removed array item")
	}
}

func TestCompareJSONStructures_InvalidJSON(t *testing.T) {
	_, err := CompareJSONStructures([]byte(`{invalid}`), []byte(`{}`))
	if err == nil {
		t.Error("Should return error for invalid source JSON")
	}

	_, err = CompareJSONStructures([]byte(`{}`), []byte(`{invalid}`))
	if err == nil {
		t.Error("Should return error for invalid target JSON")
	}
}

func TestGetTypeName(t *testing.T) {
	tests := []struct {
		value    interface{}
		expected string
	}{
		{nil, "null"},
		{map[string]interface{}{}, "object"},
		{[]interface{}{}, "array"},
		{"string", "string"},
		{float64(123), "number"},
		{true, "boolean"},
	}

	for _, tt := range tests {
		result := getTypeName(tt.value)
		if result != tt.expected {
			t.Errorf("getTypeName(%v) = %v, want %v", tt.value, result, tt.expected)
		}
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
