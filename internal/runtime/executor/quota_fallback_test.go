package executor

import "testing"

func TestParseProjectIDCandidates(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "empty", in: "", want: nil},
		{name: "single", in: "proj-a", want: []string{"proj-a"}},
		{name: "trim", in: "  proj-a  ", want: []string{"proj-a"}},
		{name: "list", in: "a,b,c", want: []string{"a", "b", "c"}},
		{name: "list_trim", in: " a,  b ,c ", want: []string{"a", "b", "c"}},
		{name: "dedupe_case_insensitive", in: "a,A,a", want: []string{"a"}},
		{name: "skip_empty_entries", in: "a,,b, ,c", want: []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseProjectIDCandidates(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len=%d, want %d (got=%v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("idx %d: got %q want %q (got=%v)", i, got[i], tt.want[i], got)
				}
			}
		})
	}
}

func TestQuotaPreviewFallbackOrder(t *testing.T) {
	if got := quotaPreviewFallbackOrder("gemini-3-flash"); len(got) != 1 || got[0] != "gemini-3-pro-preview" {
		t.Fatalf("unexpected fallback for gemini-3-flash: %v", got)
	}
	if got := quotaPreviewFallbackOrder("gemini-3-pro-preview"); got != nil {
		t.Fatalf("expected nil fallback for gemini-3-pro-preview, got %v", got)
	}
}

