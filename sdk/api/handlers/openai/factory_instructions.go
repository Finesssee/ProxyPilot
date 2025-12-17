package openai

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const factoryDefaultInstructions = "You are Droid, an AI software engineering agent. When you need to read files or run commands, emit a tool call immediately (do not just acknowledge). Use <tool_call> JSON blocks with a tool name and arguments."

func maybeInjectFactoryInstructions(c *gin.Context, rawJSON []byte) []byte {
	if c == nil {
		return rawJSON
	}
	ua := strings.ToLower(c.GetHeader("User-Agent"))
	isFactory := strings.Contains(ua, "factory-cli") || strings.Contains(ua, "droid") || c.GetHeader("X-Stainless-Lang") != "" || c.GetHeader("X-Stainless-Package-Version") != ""
	if !isFactory {
		return rawJSON
	}
	if gjson.GetBytes(rawJSON, "instructions").Exists() {
		return rawJSON
	}
	updated, err := sjson.SetBytes(rawJSON, "instructions", factoryDefaultInstructions)
	if err != nil {
		return rawJSON
	}
	return updated
}
