package util

import (
	"encoding/json"
	"strings"
)

const redactedValue = "[REDACTED]"

// RedactSensitiveJSON attempts to redact sensitive fields from a JSON payload.
// If the payload is not valid JSON, it returns the original bytes.
func RedactSensitiveJSON(body []byte) []byte {
	trim := strings.TrimSpace(string(body))
	if trim == "" {
		return body
	}
	if !strings.HasPrefix(trim, "{") && !strings.HasPrefix(trim, "[") {
		return body
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return body
	}
	redacted := redactValue(v)
	out, err := json.Marshal(redacted)
	if err != nil {
		return body
	}
	return out
}

func redactValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if isSensitiveKey(k) {
				t[k] = redactedValue
				continue
			}
			t[k] = redactValue(val)
		}
		return t
	case []any:
		for i := range t {
			t[i] = redactValue(t[i])
		}
		return t
	default:
		return v
	}
}

func isSensitiveKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	switch {
	case strings.Contains(k, "authorization"),
		strings.Contains(k, "cookie"),
		strings.Contains(k, "api_key"),
		strings.Contains(k, "apikey"),
		strings.Contains(k, "secret"),
		strings.Contains(k, "token"),
		strings.Contains(k, "password"):
		return true
	default:
		return false
	}
}
