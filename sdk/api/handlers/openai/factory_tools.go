package openai

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func maybeInjectFactoryTools(c *gin.Context, rawJSON []byte) []byte {
	if c == nil {
		return rawJSON
	}
	ua := strings.ToLower(c.GetHeader("User-Agent"))
	isFactory := strings.Contains(ua, "factory-cli") || strings.Contains(ua, "droid") || c.GetHeader("X-Stainless-Lang") != "" || c.GetHeader("X-Stainless-Package-Version") != ""
	if !isFactory {
		return rawJSON
	}
	if gjson.GetBytes(rawJSON, "tools").Exists() {
		return rawJSON
	}

	// Minimal set of tools Droid commonly uses. This enables native tool calling
	// so the model returns structured function_call items instead of plain text.
	tools := []any{
		map[string]any{
			"type":        "function",
			"name":        "Shell",
			"description": "Run a shell command in the workspace",
			"parameters": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"command": map[string]any{"type": "string"},
				},
				"required": []any{"command"},
			},
		},
		map[string]any{
			"type":        "function",
			"name":        "Read",
			"description": "Read a file by path with optional line range",
			"parameters": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"file_path":   map[string]any{"type": "string"},
					"start_line":  map[string]any{"type": "integer"},
					"end_line":    map[string]any{"type": "integer"},
					"start_byte":  map[string]any{"type": "integer"},
					"end_byte":    map[string]any{"type": "integer"},
					"max_bytes":   map[string]any{"type": "integer"},
					"max_lines":   map[string]any{"type": "integer"},
					"encoding":    map[string]any{"type": "string"},
					"strip_ansi":  map[string]any{"type": "boolean"},
					"strip_utf8":  map[string]any{"type": "boolean"},
					"follow_syml": map[string]any{"type": "boolean"},
				},
				"required": []any{"file_path"},
			},
		},
		map[string]any{
			"type":        "function",
			"name":        "Grep",
			"description": "Search files for a pattern",
			"parameters": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"pattern":    map[string]any{"type": "string"},
					"path":       map[string]any{"type": "string"},
					"glob":       map[string]any{"type": "string"},
					"regex":      map[string]any{"type": "boolean"},
					"ignoreCase": map[string]any{"type": "boolean"},
				},
				"required": []any{"pattern"},
			},
		},
		map[string]any{
			"type":        "function",
			"name":        "Write",
			"description": "Write a file at file_path with provided content",
			"parameters": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"file_path": map[string]any{"type": "string"},
					"content":   map[string]any{"type": "string"},
				},
				"required": []any{"file_path", "content"},
			},
		},
		map[string]any{
			"type":        "function",
			"name":        "Edit",
			"description": "Edit a file with a patch or line-based edits",
			"parameters": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"file_path": map[string]any{"type": "string"},
					"patch":     map[string]any{"type": "string"},
				},
				"required": []any{"file_path"},
			},
		},
	}

	out, err := sjson.SetBytes(rawJSON, "tools", tools)
	if err != nil {
		return rawJSON
	}
	// Encourage tool selection when tools exist (but do not force).
	// If middleware previously set tool_choice:"none" while trimming, override to "auto" for Droid/Factory.
	tc := gjson.GetBytes(out, "tool_choice")
	if !tc.Exists() || tc.String() == "" || strings.EqualFold(tc.String(), "none") {
		if with, err2 := sjson.SetBytes(out, "tool_choice", "auto"); err2 == nil {
			out = with
		}
	}
	return out
}
