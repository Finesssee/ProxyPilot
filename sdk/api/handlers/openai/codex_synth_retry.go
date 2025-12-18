package openai

import (
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type codexExecFunc func(model string, req []byte) ([]byte, *interfaces.ErrorMessage)

const codexSilentMaxTokensFallback = "No visible output was produced before the upstream hit MAX_TOKENS. Try reducing prompt/context size, lowering reasoning effort, or switching to a non-thinking model."

func codexDetThinkingModel(model string) string {
	model = strings.TrimSpace(model)
	if strings.HasSuffix(model, "-thinking") {
		return strings.TrimSuffix(model, "-thinking")
	}
	return model
}

func responsesHasToolOrFunctionCalls(resp []byte) bool {
	out := gjson.GetBytes(resp, "output")
	if !out.Exists() || !out.IsArray() {
		return false
	}
	for _, item := range out.Array() {
		t := item.Get("type").String()
		if t == "function_call" || t == "tool_call" {
			return true
		}
	}
	return false
}

func codexIsSilentMaxTokens(resp []byte) bool {
	// Mirrors the streaming translator heuristic: no message output, no tool calls.
	// We only use it inside the Codex synthetic path (compaction/huge), where a single retry is acceptable.
	if resp == nil {
		return false
	}
	outputText := strings.TrimSpace(gjson.GetBytes(resp, "output_text").String())
	if outputText != "" {
		return false
	}
	if responsesHasToolOrFunctionCalls(resp) {
		return false
	}

	// Strong signal: explicit incomplete_details reason.
	if r := gjson.GetBytes(resp, "incomplete_details.reason"); r.Exists() && strings.EqualFold(r.String(), "max_output_tokens") {
		return true
	}

	// Heuristic: very high output token usage with no visible output.
	// Gemini often reports output_tokens even when the "text" part is empty.
	if ot := gjson.GetBytes(resp, "usage.output_tokens"); ot.Exists() && ot.Int() >= 8000 {
		return true
	}

	return false
}

func setResponseOutputText(resp []byte, text string) []byte {
	out, err := sjson.SetBytes(resp, "output_text", text)
	if err != nil {
		return resp
	}
	return out
}

func codexNonStreamWithSingleRetry(model string, req []byte, exec codexExecFunc) (resp []byte, retryUsed bool, usedModel string, errMsg *interfaces.ErrorMessage) {
	usedModel = model
	resp, errMsg = exec(model, req)
	if errMsg != nil {
		return resp, false, usedModel, errMsg
	}
	if !codexIsSilentMaxTokens(resp) {
		return resp, false, usedModel, nil
	}

	altModel := codexDetThinkingModel(model)
	if altModel == "" || altModel == model {
		return resp, false, usedModel, nil
	}
	retryReq, err := sjson.SetBytes(req, "model", altModel)
	if err != nil {
		retryReq = req
	}
	resp2, err2 := exec(altModel, retryReq)
	if err2 != nil {
		return resp2, true, altModel, err2
	}
	return resp2, true, altModel, nil
}
