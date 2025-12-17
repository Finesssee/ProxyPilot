package executor

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestGeminiToAntigravity_TextPartsRemainScalar(t *testing.T) {
	in := []byte(`{"request":{"contents":[{"role":"user","parts":[{"text":"hello"},{"text":""}]}],"systemInstruction":{"parts":[{"text":"sys"}]}}}`)

	outNonStream := geminiToAntigravity("claude-opus-4-5-thinking", in, "proj", false)
	if gjson.GetBytes(outNonStream, "request.contents.0.parts.0.text").Type != gjson.String {
		t.Fatalf("expected non-stream text to remain scalar string")
	}
	if gjson.GetBytes(outNonStream, "request.contents.0.parts.1.text").Type != gjson.String {
		t.Fatalf("expected non-stream empty text to remain scalar string")
	}
	if gjson.GetBytes(outNonStream, "request.systemInstruction.parts.0.text").Type != gjson.String {
		t.Fatalf("expected systemInstruction text to remain scalar string")
	}

	outStream := geminiToAntigravity("claude-opus-4-5-thinking", in, "proj", true)
	if gjson.GetBytes(outStream, "request.contents.0.parts.0.text").Type != gjson.String {
		t.Fatalf("expected stream text to remain scalar string")
	}
	if gjson.GetBytes(outStream, "request.systemInstruction.parts.0.text").Type != gjson.String {
		t.Fatalf("expected stream systemInstruction text to remain scalar string")
	}

	// Ensure we didn't accidentally create nested object text.
	if gjson.GetBytes(outStream, "request.contents.0.parts.0.text.text").Exists() {
		t.Fatalf("did not expect nested text.text object")
	}
}
