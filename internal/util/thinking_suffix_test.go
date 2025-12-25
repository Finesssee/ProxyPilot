package util

import (
	"reflect"
	"testing"
)

func TestNormalizeThinkingModel(t *testing.T) {
	cases := []struct {
		name       string
		model      string
		wantBase   string
		wantMeta   map[string]any
	}{
		{
			name:     "no suffix",
			model:    "claude-3-opus",
			wantBase: "claude-3-opus",
			wantMeta: nil,
		},
		{
			name:     "parens budget",
			model:    "claude-3-opus(16000)",
			wantBase: "claude-3-opus",
			wantMeta: map[string]any{
				ThinkingOriginalModelMetadataKey: "claude-3-opus(16000)",
				ThinkingBudgetMetadataKey:        16000,
			},
		},
		{
			name:     "parens effort",
			model:    "gpt-5(high)",
			wantBase: "gpt-5",
			wantMeta: map[string]any{
				ThinkingOriginalModelMetadataKey: "gpt-5(high)",
				ReasoningEffortMetadataKey:       "high",
			},
		},
		{
			name:     "hyphen budget",
			model:    "claude-sonnet-4-5-thinking-20000",
			wantBase: "claude-sonnet-4-5",
			wantMeta: map[string]any{
				ThinkingOriginalModelMetadataKey: "claude-sonnet-4-5-thinking-20000",
				ThinkingBudgetMetadataKey:        20000,
			},
		},
		{
			name:     "hyphen effort",
			model:    "o1-preview-thinking-medium",
			wantBase: "o1-preview",
			wantMeta: map[string]any{
				ThinkingOriginalModelMetadataKey: "o1-preview-thinking-medium",
				ReasoningEffortMetadataKey:       "medium",
			},
		},
		{
			name:     "incomplete parens",
			model:    "model(123",
			wantBase: "model(123",
			wantMeta: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotBase, gotMeta := NormalizeThinkingModel(tc.model)
			if gotBase != tc.wantBase {
				t.Errorf("base model mismatch: got %q, want %q", gotBase, tc.wantBase)
			}
			if !reflect.DeepEqual(gotMeta, tc.wantMeta) {
				t.Errorf("metadata mismatch: got %v, want %v", gotMeta, tc.wantMeta)
			}
		})
	}
}
