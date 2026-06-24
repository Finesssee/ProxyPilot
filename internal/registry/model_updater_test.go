package registry

import "testing"

func TestMergeMissingForkModelSectionsPreservesQwenAndIFlow(t *testing.T) {
	data := &staticModelsJSON{
		Claude:      []*ModelInfo{{ID: "claude-1"}},
		Gemini:      []*ModelInfo{{ID: "gemini-1"}},
		Vertex:      []*ModelInfo{{ID: "vertex-1"}},
		AIStudio:    []*ModelInfo{{ID: "aistudio-1"}},
		CodexFree:   []*ModelInfo{{ID: "codex-free-1"}},
		CodexTeam:   []*ModelInfo{{ID: "codex-team-1"}},
		CodexPlus:   []*ModelInfo{{ID: "codex-plus-1"}},
		CodexPro:    []*ModelInfo{{ID: "codex-pro-1"}},
		Kimi:        []*ModelInfo{{ID: "kimi-1"}},
		Antigravity: []*ModelInfo{{ID: "antigravity-1"}},
	}
	fallback := &staticModelsJSON{
		Qwen:  []*ModelInfo{{ID: "qwen-fallback"}},
		IFlow: []*ModelInfo{{ID: "iflow-fallback"}},
	}

	mergeMissingForkModelSections(data, fallback)

	if err := validateModelsCatalog(data); err != nil {
		t.Fatalf("expected merged catalog to validate: %v", err)
	}
	if got := data.Qwen[0].ID; got != "qwen-fallback" {
		t.Fatalf("expected qwen fallback model, got %q", got)
	}
	if got := data.IFlow[0].ID; got != "iflow-fallback" {
		t.Fatalf("expected iflow fallback model, got %q", got)
	}
}
