package openai

import (
	"strings"
	"testing"
)

func TestCodexIsCompactionOrCheckpoint_InInstructions(t *testing.T) {
	raw := []byte(`{"instructions":"CONTEXT CHECKPOINT COMPACTION\\nplease summarize","input":[]}`)
	if !codexIsCompactionOrCheckpoint(raw) {
		t.Fatalf("expected compaction detection from instructions")
	}
}

func TestCodexIsCompactionOrCheckpoint_InLastUserInput(t *testing.T) {
	raw := []byte(`{
  "input":[
    {"role":"assistant","content":[{"type":"output_text","text":"x"}]},
    {"role":"user","content":[{"type":"input_text","text":"/compact please"}]}
  ]
}`)
	if !codexIsCompactionOrCheckpoint(raw) {
		t.Fatalf("expected compaction detection from last user input_text")
	}
}

func TestCodexSynthReason_Huge(t *testing.T) {
	t.Setenv("CLIPROXY_CODEX_SYNTH_ENABLE", "1")
	// codexSynthHugeBytes clamps to >=32KB
	t.Setenv("CLIPROXY_CODEX_SYNTH_HUGE_BYTES", "32768")
	raw := []byte(`{"input":[{"role":"user","content":[{"type":"input_text","text":"` + strings.Repeat("x", 33000) + `"}]}]}`)
	if got := codexSynthReason(raw); got != "huge" {
		t.Fatalf("expected huge reason, got=%q", got)
	}
}

func TestCodexSynthReason_Disabled(t *testing.T) {
	t.Setenv("CLIPROXY_CODEX_SYNTH_ENABLE", "0")
	t.Setenv("CLIPROXY_CODEX_SYNTH_HUGE_BYTES", "32768")
	raw := []byte(`{"instructions":"CONTEXT CHECKPOINT COMPACTION","input":[{"role":"user","content":[{"type":"input_text","text":"x"}]}]}`)
	if got := codexSynthReason(raw); got != "" {
		t.Fatalf("expected empty reason when disabled, got=%q", got)
	}
}
