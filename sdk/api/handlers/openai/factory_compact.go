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
	isFactory := strings.Contains(ua, "factory-cli") || c.GetHeader("X-Stainless-Lang") != "" || c.GetHeader("X-Stainless-Package-Version") != ""
	if !isFactory {
		return rawJSON
	}

	input := gjson.GetBytes(rawJSON, "input")
	if !input.Exists() || !input.IsArray() {
		return rawJSON
	}

	out := rawJSON
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
				// Some Factory/Droid payloads omit the "type" field and just send {text:"..."} items.
				partType = "input_text"
			}
			if partType != "input_text" {
				continue
			}
			text := part.Get("text").String()
			text = stripToolCallTags(text)
			// Special case: Droid /compact dumps a huge "previous instance summary" into a single input_text.
			// Keeping the head is counterproductive (it's mostly stale); keep only the tail which contains
			// the latest user intent and near-term steps.
			if strings.Contains(text, "A previous instance of Droid") && strings.Contains(text, "<summary>") {
				if len(text) > factoryKeepTailChars {
					text = compactText(text, 0, factoryKeepTailChars)
				}
			} else if len(text) > factoryMaxInputTextChars {
				text = compactText(text, factoryKeepHeadChars, factoryKeepTailChars)
			} else {
				continue
			}
			updated, err := sjson.SetBytes(out, "input."+itoa(mi)+".content."+itoa(pi)+".text", text)
			if err == nil {
				out = updated
			}
		}
	}
	return out
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
