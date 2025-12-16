package gemini

import (
	"fmt"
	"testing"

	"github.com/tidwall/gjson"
)

func TestFixCLIToolResponse_DoesNotDropEarlyResponses(t *testing.T) {
	in := `{
  "request": {
    "contents": [
      {"role":"model","parts":[{"functionCall":{"id":"a","name":"A","args":{}}}]},
      {"role":"function","parts":[{"functionResponse":{"id":"a","name":"A","response":{"result":"outA"}}}]},
      {"role":"function","parts":[{"functionResponse":{"id":"b","name":"B","response":{"result":"outB"}}}]},
      {"role":"model","parts":[{"functionCall":{"id":"b","name":"B","args":{}}}]}
    ]
  }
}`

	out, err := fixCLIToolResponse(in)
	if err != nil {
		t.Fatalf("fixCLIToolResponse error: %v", err)
	}

	contents := gjson.Get(out, "request.contents")
	if !contents.Exists() || !contents.IsArray() {
		t.Fatalf("expected request.contents array, got body=%s", out)
	}

	var callBIndex int64 = -1
	contents.ForEach(func(k, v gjson.Result) bool {
		if v.Get("role").String() != "model" {
			return true
		}
		v.Get("parts").ForEach(func(_, p gjson.Result) bool {
			if p.Get("functionCall.id").String() == "b" {
				callBIndex = k.Int()
				return false
			}
			return true
		})
		return callBIndex == -1
	})
	if callBIndex < 0 {
		t.Fatalf("expected to find functionCall id=b in output body=%s", out)
	}

	next := gjson.Get(out, fmt.Sprintf("request.contents.%d.parts", callBIndex+1))
	if !next.Exists() || !next.IsArray() {
		t.Fatalf("expected tool response content after call b body=%s", out)
	}

	found := false
	next.ForEach(func(_, p gjson.Result) bool {
		if p.Get("functionResponse.id").String() == "b" {
			found = true
			return false
		}
		return true
	})
	if !found {
		t.Fatalf("expected functionResponse id=b immediately after call b body=%s", out)
	}
}

func TestFixCLIToolResponse_PullsResponsesBeforeUserText(t *testing.T) {
	in := `{
  "request": {
    "contents": [
      {"role":"model","parts":[{"functionCall":{"id":"a","name":"A","args":{}}}]},
      {"role":"user","parts":[{"text":"(some user text that must not split tool call/result)"}]},
      {"role":"function","parts":[{"functionResponse":{"id":"a","name":"A","response":{"result":"outA"}}}]}
    ]
  }
}`

	out, err := fixCLIToolResponse(in)
	if err != nil {
		t.Fatalf("fixCLIToolResponse error: %v", err)
	}

	contents := gjson.Get(out, "request.contents")
	if !contents.Exists() || !contents.IsArray() {
		t.Fatalf("expected request.contents array, got body=%s", out)
	}

	if contents.Get("0.role").String() != "model" {
		t.Fatalf("expected first role=model, got %s body=%s", contents.Get("0.role").String(), out)
	}
	if contents.Get("1.role").String() != "user" {
		t.Fatalf("expected second role=user, got %s body=%s", contents.Get("1.role").String(), out)
	}
	if contents.Get("1.parts.0.functionResponse.id").String() != "a" {
		t.Fatalf("expected second message to contain functionResponse id=a body=%s", out)
	}
	if contents.Get("2.parts.0.text").String() == "" {
		t.Fatalf("expected user text to follow tool result body=%s", out)
	}
}
