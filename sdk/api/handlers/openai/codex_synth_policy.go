package openai

import (
	"os"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

const (
	codexSynthHugeBytesDefault = 250_000
)

func codexSynthEnabled() bool {
	v := strings.TrimSpace(os.Getenv("CLIPROXY_CODEX_SYNTH_ENABLE"))
	if v == "" {
		return true
	}
	switch strings.ToLower(v) {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}

func codexSynthHugeBytes() int {
	v := strings.TrimSpace(os.Getenv("CLIPROXY_CODEX_SYNTH_HUGE_BYTES"))
	if v == "" {
		return codexSynthHugeBytesDefault
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return codexSynthHugeBytesDefault
	}
	// Clamp to a sane range: 32KB..5MB
	if n < 32*1024 {
		n = 32 * 1024
	}
	if n > 5*1024*1024 {
		n = 5 * 1024 * 1024
	}
	return n
}

func isCodexCLIUserAgent(userAgent string) bool {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	return strings.Contains(ua, "openai codex")
}

// codexSynthReason returns one of: "", "compaction", "huge".
func codexSynthReason(rawJSON []byte) string {
	if !codexSynthEnabled() {
		return ""
	}
	if codexIsCompactionOrCheckpoint(rawJSON) {
		return "compaction"
	}
	if len(rawJSON) >= codexSynthHugeBytes() {
		return "huge"
	}
	return ""
}

func codexIsCompactionOrCheckpoint(rawJSON []byte) bool {
	// Tight keyword whitelist to avoid flipping normal turns.
	needle := func(s string) bool {
		s = strings.ToLower(s)
		s = strings.TrimSpace(s)
		if s == "" {
			return false
		}
		if strings.Contains(s, "context checkpoint compaction") {
			return true
		}
		if strings.Contains(s, "handoff summary") {
			return true
		}
		if strings.Contains(s, "resume the task") {
			return true
		}
		if strings.Contains(s, "/compact") {
			return true
		}
		// Treat explicit "checkpoint"+"compaction" as strong signal.
		if strings.Contains(s, "checkpoint") && strings.Contains(s, "compaction") {
			return true
		}
		return false
	}

	if rawJSON == nil {
		return false
	}

	if v := gjson.GetBytes(rawJSON, "instructions"); v.Exists() && v.Type == gjson.String {
		if needle(v.String()) {
			return true
		}
	}

	// Responses: scan last user input text (best-effort).
	if t := extractLastUserInputTextFromResponses(rawJSON); t != "" {
		if needle(t) {
			return true
		}
	}

	return false
}

func extractLastUserInputTextFromResponses(rawJSON []byte) string {
	input := gjson.GetBytes(rawJSON, "input")
	if !input.Exists() || !input.IsArray() {
		// Some clients send input as a string; treat it as user content.
		if input.Type == gjson.String {
			return input.String()
		}
		return ""
	}
	arr := input.Array()
	for i := len(arr) - 1; i >= 0; i-- {
		if !strings.EqualFold(arr[i].Get("role").String(), "user") {
			continue
		}
		content := arr[i].Get("content")
		if !content.Exists() || !content.IsArray() {
			continue
		}
		parts := content.Array()
		for j := 0; j < len(parts); j++ {
			t := parts[j].Get("type").String()
			if t == "" && parts[j].Get("text").Exists() {
				t = "input_text"
			}
			if t != "input_text" {
				continue
			}
			return parts[j].Get("text").String()
		}
	}
	return ""
}
