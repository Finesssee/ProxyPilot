package openai

import (
	"context"
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertKiroStreamToOpenAI_MapsSparseContentBlockIndexesToToolOrdinals(t *testing.T) {
	var param any

	events := [][]byte{
		[]byte(`event: message_start
data: {"type":"message_start"}`),
		[]byte(`event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"tool_a","name":"Read"}}`),
		[]byte(`event: content_block_start
data: {"type":"content_block_start","index":3,"content_block":{"type":"tool_use","id":"tool_b","name":"Write"}}`),
		[]byte(`event: content_block_delta
data: {"type":"content_block_delta","index":3,"delta":{"type":"input_json_delta","partial_json":"{\"path\":\"/tmp/x\"}"}}`),
	}

	var lastChunk string
	for _, event := range events {
		chunks := ConvertKiroStreamToOpenAI(context.Background(), "kiro-model", nil, nil, event, &param)
		if len(chunks) > 0 {
			lastChunk = chunks[len(chunks)-1]
		}
	}

	if lastChunk == "" {
		t.Fatal("expected a tool call delta chunk")
	}
	if got := gjson.Get(lastChunk, "choices.0.delta.tool_calls.0.index").Int(); got != 1 {
		t.Fatalf("expected sparse block index 3 to map to tool ordinal 1, got %d chunk=%s", got, lastChunk)
	}
	if got := gjson.Get(lastChunk, "choices.0.delta.tool_calls.0.function.arguments").String(); got != `{"path":"/tmp/x"}` {
		t.Fatalf("unexpected arguments delta %q chunk=%s", got, lastChunk)
	}
}
