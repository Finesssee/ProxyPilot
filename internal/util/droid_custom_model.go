package util

import (
	"strings"
	"unicode"
)

// NormalizeDroidCustomModel attempts to convert Factory Droid "custom:*" model ids
// into the underlying model id that this proxy understands.
//
// Examples:
//   custom:CLIProxy-(local):-gemini-claude-opus-4-5-thinking-12 -> gemini-claude-opus-4-5-thinking
//   custom:CLIProxy-(local):-gpt-5.2-(reasoning:-medium)-2      -> gpt-5.2(medium)
func NormalizeDroidCustomModel(model string) string {
	model = strings.TrimSpace(model)
	if !strings.HasPrefix(strings.ToLower(model), "custom:") {
		return model
	}
	raw := strings.TrimPrefix(model, "custom:")

	// Prefer well-known ":-<family>" markers so we don't get confused by "reasoning:-low".
	candidate := ""
	lower := strings.ToLower(raw)
	for _, marker := range []string{":-antigravity-", ":-gemini-", ":-gpt-", ":-claude-", ":-qwen", ":-deepseek", ":-kimi", ":-glm-", ":-minimax", ":-tstars"} {
		if idx := strings.LastIndex(lower, marker); idx >= 0 {
			candidate = raw[idx+2:] // drop the leading ":-"
			break
		}
	}
	if candidate == "" {
		// Fallback: take the substring after the last ":-" if present.
		if idx := strings.LastIndex(raw, ":-"); idx >= 0 && idx+2 < len(raw) {
			candidate = raw[idx+2:]
		} else {
			candidate = raw
		}
	}
	candidate = strings.TrimSpace(candidate)

	// Strip trailing "-<digits>" index suffix (Droid custom model numbering).
	candidate = stripTrailingDashDigits(candidate)

	// Map Droid's "(reasoning:-level)" encoding to our "(level)" suffix.
	// Example: gpt-5.2-(reasoning:-medium) -> gpt-5.2(medium)
	if strings.Contains(strings.ToLower(candidate), "-(reasoning:-") {
		lc := strings.ToLower(candidate)
		idx := strings.Index(lc, "-(reasoning:-")
		if idx >= 0 {
			base := strings.TrimSpace(candidate[:idx])
			rest := candidate[idx+len("-(reasoning:-"):]
			// rest should look like: medium) or medium)-something
			level := rest
			if end := strings.Index(level, ")"); end >= 0 {
				level = level[:end]
			}
			level = strings.TrimSpace(strings.Trim(level, "-"))
			levelLower := strings.ToLower(level)
			if base != "" && (levelLower == "low" || levelLower == "medium" || levelLower == "high" || levelLower == "xhigh" || levelLower == "minimal" || levelLower == "none") {
				return base + "(" + levelLower + ")"
			}
		}
	}

	return candidate
}

func stripTrailingDashDigits(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// Find final "-<digits>" segment.
	lastDash := strings.LastIndex(s, "-")
	if lastDash < 0 || lastDash == len(s)-1 {
		return s
	}
	suffix := s[lastDash+1:]
	if suffix == "" {
		return s
	}
	for _, r := range suffix {
		if !unicode.IsDigit(r) {
			return s
		}
	}
	return strings.TrimSpace(s[:lastDash])
}

