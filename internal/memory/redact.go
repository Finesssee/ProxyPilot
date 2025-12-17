package memory

import (
	"regexp"
)

var (
	reBearer = regexp.MustCompile(`(?i)\b(bearer)\s+([A-Za-z0-9_\-\.=]{12,})`)
	reSK     = regexp.MustCompile(`\b(sk-[A-Za-z0-9]{12,})\b`)
	reAIza   = regexp.MustCompile(`\b(AIza[A-Za-z0-9_\-]{16,})\b`)
)

func RedactText(s string) string {
	if s == "" {
		return s
	}
	s = reBearer.ReplaceAllString(s, "$1 [REDACTED]")
	s = reSK.ReplaceAllString(s, "[REDACTED]")
	s = reAIza.ReplaceAllString(s, "[REDACTED]")
	return s
}
