package openai

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func TestMaybeCompactFactoryInput_Messages_CompactSummary(t *testing.T) {
	// Construct a huge summary
	hugeSummary := "<summary>A previous instance of Droid is doing things...</summary>\n" + strings.Repeat("blah ", 2000)
	rawJSON := `{"model":"gpt-4","messages":[{"role":"user","content":"` + hugeSummary + `"}]}`

	c := &gin.Context{}
	c.Request = &http.Request{
		Header: make(http.Header),
	}
	c.Request.Header.Set("User-Agent", "droid-cli")

	compacted := maybeCompactFactoryInput(c, []byte(rawJSON))

	text := gjson.GetBytes(compacted, "messages.0.content").String()
	if !strings.Contains(text, "...[ProxyPilot truncated large history]...") {
		t.Errorf("Expected compacted text, got length %d", len(text))
	}
	if len(text) >= len(hugeSummary) {
		t.Errorf("Expected text to be shorter than original (%d), got %d", len(hugeSummary), len(text))
	}
}

func TestMaybeCompactFactoryInput_Input_CompactSummary(t *testing.T) {
	// Construct a huge summary
	hugeSummary := "<summary>A previous instance of Droid is doing things...</summary>\n" + strings.Repeat("blah ", 2000)
	rawJSON := `{"model":"gpt-4","input":[{"content":[{"type":"input_text","text":"` + hugeSummary + `"}]}]}`

	c := &gin.Context{}
	c.Request = &http.Request{
		Header: make(http.Header),
	}
	c.Request.Header.Set("User-Agent", "droid-cli")

	compacted := maybeCompactFactoryInput(c, []byte(rawJSON))

	text := gjson.GetBytes(compacted, "input.0.content.0.text").String()
	if !strings.Contains(text, "...[ProxyPilot truncated large history]...") {
		t.Errorf("Expected compacted text, got length %d", len(text))
	}
	if len(text) >= len(hugeSummary) {
		t.Errorf("Expected text to be shorter than original (%d), got %d", len(hugeSummary), len(text))
	}
}

func TestMaybeCompactFactoryInput_Messages_ArrayContent(t *testing.T) {
	// Construct a huge summary
	hugeSummary := "<summary>A previous instance of Droid is doing things...</summary>\n" + strings.Repeat("blah ", 2000)
	rawJSON := `{"model":"gpt-4","messages":[{"role":"user","content":[{"type":"text","text":"` + hugeSummary + `"}]}]}`

	c := &gin.Context{}
	c.Request = &http.Request{
		Header: make(http.Header),
	}
	c.Request.Header.Set("User-Agent", "droid-cli")

	compacted := maybeCompactFactoryInput(c, []byte(rawJSON))

	text := gjson.GetBytes(compacted, "messages.0.content.0.text").String()
	if !strings.Contains(text, "...[ProxyPilot truncated large history]...") {
		t.Errorf("Expected compacted text, got length %d", len(text))
	}
}
