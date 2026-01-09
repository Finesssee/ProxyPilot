package executor

import (
	"strings"

	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func parseProjectIDCandidates(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key := strings.ToLower(part)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, part)
	}
	return out
}

func projectIDCandidatesFromAuth(auth *cliproxyauth.Auth) []string {
	if auth == nil || auth.Metadata == nil {
		return nil
	}
	if raw, ok := auth.Metadata["project_id"].(string); ok {
		return parseProjectIDCandidates(raw)
	}
	return nil
}

func quotaPreviewFallbackOrder(model string) []string {
	model = strings.TrimSpace(model)
	switch model {
	// Gemini 3: fallback to pro preview (observed highest availability).
	case "gemini-3-flash", "gemini-3-flash-high", "gemini-3-flash-preview":
		return []string{"gemini-3-pro-preview"}
	case "gemini-3-pro", "gemini-3-pro-high":
		return []string{"gemini-3-pro-preview"}
	default:
		return nil
	}
}

