package translator

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// QualityReport contains the results of analyzing a translation for data loss or transformation issues.
type QualityReport struct {
	// FieldsMapped is the count of fields that were successfully translated.
	FieldsMapped int `json:"fields_mapped"`

	// FieldsDropped is the count of fields from the source that are missing in the target.
	FieldsDropped int `json:"fields_dropped"`

	// FieldsAdded is the count of fields in the target that were not in the source.
	FieldsAdded int `json:"fields_added"`

	// Score is a quality score from 0.0 (total loss) to 1.0 (perfect translation).
	Score float64 `json:"score"`

	// Warnings contains human-readable warnings about the translation.
	Warnings []string `json:"warnings,omitempty"`

	// DroppedFields lists the specific field paths that were dropped.
	DroppedFields []string `json:"dropped_fields,omitempty"`

	// AddedFields lists the specific field paths that were added.
	AddedFields []string `json:"added_fields,omitempty"`

	// MappedFields lists the specific field paths that were successfully mapped.
	MappedFields []string `json:"mapped_fields,omitempty"`
}

// ScoreTranslation analyzes the quality of a translation by comparing JSON structures.
// It detects data loss by comparing fields in the source and target payloads.
func ScoreTranslation(from, to Format, before, after []byte) QualityReport {
	report := QualityReport{
		Warnings:      []string{},
		DroppedFields: []string{},
		AddedFields:   []string{},
		MappedFields:  []string{},
	}

	// Parse source JSON
	var beforeData interface{}
	if err := json.Unmarshal(before, &beforeData); err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("failed to parse source JSON: %v", err))
		report.Score = 0.0
		return report
	}

	// Parse target JSON
	var afterData interface{}
	if err := json.Unmarshal(after, &afterData); err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("failed to parse target JSON: %v", err))
		report.Score = 0.0
		return report
	}

	// Extract all field paths from both structures
	beforePaths := extractPaths(beforeData, "")
	afterPaths := extractPaths(afterData, "")

	// Convert to sets for comparison
	beforeSet := make(map[string]struct{})
	for _, p := range beforePaths {
		beforeSet[p] = struct{}{}
	}

	afterSet := make(map[string]struct{})
	for _, p := range afterPaths {
		afterSet[p] = struct{}{}
	}

	// Find mapped fields (present in both)
	for path := range beforeSet {
		if _, exists := afterSet[path]; exists {
			report.MappedFields = append(report.MappedFields, path)
			report.FieldsMapped++
		}
	}

	// Find dropped fields (in source but not target)
	for path := range beforeSet {
		if _, exists := afterSet[path]; !exists {
			report.DroppedFields = append(report.DroppedFields, path)
			report.FieldsDropped++
		}
	}

	// Find added fields (in target but not source)
	for path := range afterSet {
		if _, exists := beforeSet[path]; !exists {
			report.AddedFields = append(report.AddedFields, path)
			report.FieldsAdded++
		}
	}

	// Sort all field lists for consistent output
	sort.Strings(report.MappedFields)
	sort.Strings(report.DroppedFields)
	sort.Strings(report.AddedFields)

	// Calculate score
	report.Score = calculateScore(report)

	// Generate warnings based on analysis
	report.Warnings = generateWarnings(from, to, report)

	return report
}

// extractPaths recursively extracts all field paths from a JSON structure.
// It returns normalized path strings like "messages[0].content" or "model".
func extractPaths(data interface{}, prefix string) []string {
	var paths []string

	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			newPrefix := key
			if prefix != "" {
				newPrefix = prefix + "." + key
			}
			// Add the key itself as a path
			paths = append(paths, newPrefix)
			// Recursively extract nested paths
			paths = append(paths, extractPaths(value, newPrefix)...)
		}
	case []interface{}:
		for i, item := range v {
			newPrefix := fmt.Sprintf("%s[%d]", prefix, i)
			paths = append(paths, newPrefix)
			paths = append(paths, extractPaths(item, newPrefix)...)
		}
	default:
		// Leaf value - the path was already added by the parent
	}

	return paths
}

// calculateScore computes a quality score based on field mapping statistics.
func calculateScore(report QualityReport) float64 {
	totalSource := report.FieldsMapped + report.FieldsDropped
	if totalSource == 0 {
		// No source fields means we can't calculate a meaningful score
		if report.FieldsAdded > 0 {
			return 0.5 // Some data was generated
		}
		return 1.0 // Empty to empty is perfect
	}

	// Base score is the ratio of mapped fields to total source fields
	baseScore := float64(report.FieldsMapped) / float64(totalSource)

	// Penalize heavily for dropped fields
	droppedPenalty := 0.0
	if report.FieldsDropped > 0 {
		droppedPenalty = float64(report.FieldsDropped) / float64(totalSource) * 0.5
	}

	score := baseScore - droppedPenalty

	// Clamp to [0, 1]
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

// generateWarnings creates human-readable warnings based on the analysis.
func generateWarnings(from, to Format, report QualityReport) []string {
	var warnings []string

	if report.FieldsDropped > 0 {
		warnings = append(warnings, fmt.Sprintf("%d fields from %s format were not translated to %s", report.FieldsDropped, from, to))
	}

	if report.FieldsAdded > 0 {
		warnings = append(warnings, fmt.Sprintf("%d new fields were added during translation to %s", report.FieldsAdded, to))
	}

	if report.Score < 0.5 {
		warnings = append(warnings, "translation quality is low - significant data loss detected")
	} else if report.Score < 0.8 {
		warnings = append(warnings, "translation quality is moderate - some data may have been lost")
	}

	// Check for common critical fields that might be dropped
	criticalFields := []string{"model", "messages", "prompt", "content", "role"}
	for _, field := range criticalFields {
		for _, dropped := range report.DroppedFields {
			if strings.HasSuffix(dropped, field) || dropped == field {
				warnings = append(warnings, fmt.Sprintf("critical field '%s' was dropped during translation", field))
				break
			}
		}
	}

	return warnings
}

// CompareJSONStructures provides a detailed comparison of two JSON structures.
// This is useful for debugging translation issues.
func CompareJSONStructures(before, after []byte) (*StructureComparison, error) {
	var beforeData, afterData interface{}

	if err := json.Unmarshal(before, &beforeData); err != nil {
		return nil, fmt.Errorf("failed to parse source JSON: %w", err)
	}

	if err := json.Unmarshal(after, &afterData); err != nil {
		return nil, fmt.Errorf("failed to parse target JSON: %w", err)
	}

	comp := &StructureComparison{
		SourceType: getTypeName(beforeData),
		TargetType: getTypeName(afterData),
		Diffs:      []FieldDiff{},
	}

	compareRecursive(beforeData, afterData, "", &comp.Diffs)

	return comp, nil
}

// StructureComparison contains detailed comparison results.
type StructureComparison struct {
	SourceType string      `json:"source_type"`
	TargetType string      `json:"target_type"`
	Diffs      []FieldDiff `json:"diffs"`
}

// FieldDiff describes a difference at a specific path.
type FieldDiff struct {
	Path       string `json:"path"`
	DiffType   string `json:"diff_type"` // "added", "removed", "type_changed", "value_changed"
	SourceType string `json:"source_type,omitempty"`
	TargetType string `json:"target_type,omitempty"`
}

// getTypeName returns a human-readable type name for a value.
func getTypeName(v interface{}) string {
	if v == nil {
		return "null"
	}
	t := reflect.TypeOf(v)
	switch t.Kind() {
	case reflect.Map:
		return "object"
	case reflect.Slice:
		return "array"
	case reflect.String:
		return "string"
	case reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	default:
		return t.String()
	}
}

// compareRecursive recursively compares two structures and records differences.
func compareRecursive(before, after interface{}, path string, diffs *[]FieldDiff) {
	beforeType := getTypeName(before)
	afterType := getTypeName(after)

	if beforeType != afterType {
		*diffs = append(*diffs, FieldDiff{
			Path:       path,
			DiffType:   "type_changed",
			SourceType: beforeType,
			TargetType: afterType,
		})
		return
	}

	switch b := before.(type) {
	case map[string]interface{}:
		a := after.(map[string]interface{})

		// Check for removed keys
		for key := range b {
			newPath := key
			if path != "" {
				newPath = path + "." + key
			}
			if _, exists := a[key]; !exists {
				*diffs = append(*diffs, FieldDiff{
					Path:     newPath,
					DiffType: "removed",
				})
			} else {
				compareRecursive(b[key], a[key], newPath, diffs)
			}
		}

		// Check for added keys
		for key := range a {
			newPath := key
			if path != "" {
				newPath = path + "." + key
			}
			if _, exists := b[key]; !exists {
				*diffs = append(*diffs, FieldDiff{
					Path:     newPath,
					DiffType: "added",
				})
			}
		}

	case []interface{}:
		a := after.([]interface{})
		maxLen := len(b)
		if len(a) > maxLen {
			maxLen = len(a)
		}

		for i := 0; i < maxLen; i++ {
			newPath := fmt.Sprintf("%s[%d]", path, i)
			if i >= len(b) {
				*diffs = append(*diffs, FieldDiff{
					Path:     newPath,
					DiffType: "added",
				})
			} else if i >= len(a) {
				*diffs = append(*diffs, FieldDiff{
					Path:     newPath,
					DiffType: "removed",
				})
			} else {
				compareRecursive(b[i], a[i], newPath, diffs)
			}
		}
	}
}
