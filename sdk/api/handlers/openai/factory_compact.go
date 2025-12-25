package openai

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	factoryMaxInputTextChars = 80_000
	factoryKeepHeadChars     = 6_000
	factoryKeepTailChars     = 10_000
)

func maybeCompactFactoryInput(c *gin.Context, rawJSON []byte) []byte {
	if c == nil {
		return rawJSON
	}
	ua := strings.ToLower(c.GetHeader("User-Agent"))
	isFactory := strings.Contains(ua, "factory-cli") || strings.Contains(ua, "droid") || c.GetHeader("X-Stainless-Lang") != "" || c.GetHeader("X-Stainless-Package-Version") != ""
	if !isFactory {
		return rawJSON
	}

	out := rawJSON

	// 1. Try "input" array (Factory native / Responses API)
	if input := gjson.GetBytes(rawJSON, "input"); input.Exists() && input.IsArray() {
		messages := input.Array()
		for mi := range messages {
			content := gjson.GetBytes(out, "input."+itoa(mi)+".content")
			if !content.Exists() || !content.IsArray() {
				continue
			}
			parts := content.Array()
			for pi, part := range parts {
				partType := part.Get("type").String()
				if partType == "" && part.Get("text").Exists() {
					partType = "input_text"
				}
				if partType != "input_text" {
					continue
				}
				text := part.Get("text").String()
				newText, changed := processFactoryText(text)
				if changed {
					updated, err := sjson.SetBytes(out, "input."+itoa(mi)+".content."+itoa(pi)+".text", newText)
					if err == nil {
						out = updated
					}
				}
			}
		}
		return out
	}

	// 2. Try "messages" array (OpenAI Chat Completions)
	if messages := gjson.GetBytes(rawJSON, "messages"); messages.Exists() && messages.IsArray() {
		msgs := messages.Array()
		for mi, msg := range msgs {
			content := msg.Get("content")

			// Case A: content is a simple string
			if content.Type == gjson.String {
				text := content.String()
				newText, changed := processFactoryText(text)
				if changed {
					updated, err := sjson.SetBytes(out, "messages."+itoa(mi)+".content", newText)
					if err == nil {
						out = updated
					}
				}
				continue
			}

			// Case B: content is an array of parts
			if content.IsArray() {
				parts := content.Array()
				for pi, part := range parts {
					partType := part.Get("type").String()
					// OpenAI uses "text", Factory sometimes uses "input_text"
					if partType != "text" && partType != "input_text" {
						continue
					}
					if !part.Get("text").Exists() {
						continue
					}
					text := part.Get("text").String()
					newText, changed := processFactoryText(text)
					if changed {
						updated, err := sjson.SetBytes(out, "messages."+itoa(mi)+".content."+itoa(pi)+".text", newText)
						if err == nil {
							out = updated
						}
					}
				}
			}
		}
		return out
	}

	return rawJSON
}

func processFactoryText(text string) (string, bool) {
	text = stripToolCallTags(text)
	// Special case: Droid /compact dumps a huge "previous instance summary" into a single input_text.
	// Keeping the head is counterproductive (it's mostly stale); keep only the tail which contains
	// the latest user intent and near-term steps.
	if strings.Contains(text, "A previous instance of Droid") && strings.Contains(text, "<summary>") {
		if len(text) > factoryKeepTailChars {
			return compactText(text, 0, factoryKeepTailChars), true
		}
	} else if len(text) > factoryMaxInputTextChars {
		return compactText(text, factoryKeepHeadChars, factoryKeepTailChars), true
	}
	return text, false
}

func compactText(text string, keepHead int, keepTail int) string {
	if keepHead < 0 {
		keepHead = 0
	}
	if keepTail < 0 {
		keepTail = 0
	}
	if keepHead+keepTail+64 >= len(text) {
		return text
	}
	head := text[:keepHead]
	tail := text[len(text)-keepTail:]
	return head + "\n\n...[ProxyPilot truncated large history]...\n\n" + tail
}

func stripToolCallTags(text string) string {
	// Droid sometimes embeds <tool_call> blocks inside the running transcript (especially after /compact).
	// Those blocks are not actionable by the model and can confuse tool selection.
	for {
		start := strings.Index(text, "<tool_call>")
		if start < 0 {
			return text
		}
		end := strings.Index(text[start:], "</tool_call>")
		if end < 0 {
			return text
		}
		endAbs := start + end + len("</tool_call>")
		text = text[:start] + text[endAbs:]
	}
}
