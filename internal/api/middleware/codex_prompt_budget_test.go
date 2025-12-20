package middleware

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAppendScaffoldStateChat(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)
	out := appendScaffoldState("chat", body, "<proxypilot_state>ok</proxypilot_state>", 10000)

	msgs := gjson.GetBytes(out, "messages").Array()
	require.Len(t, msgs, 2)
	require.Equal(t, "system", msgs[1].Get("role").String())
	require.Contains(t, msgs[1].Get("content").String(), "proxypilot_state")
}

func TestAppendScaffoldStateResponses(t *testing.T) {
	body := []byte(`{"input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)
	out := appendScaffoldState("responses", body, "<proxypilot_state>ok</proxypilot_state>", 10000)

	input := gjson.GetBytes(out, "input").Array()
	require.Len(t, input, 2)
	require.Equal(t, "system", input[1].Get("role").String())
	require.Contains(t, input[1].Get("content.0.text").String(), "proxypilot_state")
}

func TestSpecModeHeader(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "/v1/responses", nil)
	req.Header.Set("X-CLIProxyAPI-Spec-Mode", "true")
	require.True(t, agenticSpecModeEnabled(req, []byte(`{}`)))
}

func TestAppendSystemBlockChat(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)
	out, ok := appendSystemBlock("chat", body, "<proxypilot_anchor>ok</proxypilot_anchor>", 10000)
	require.True(t, ok)
	msgs := gjson.GetBytes(out, "messages").Array()
	require.Len(t, msgs, 2)
	require.Equal(t, "system", msgs[1].Get("role").String())
	require.Contains(t, msgs[1].Get("content").String(), "proxypilot_anchor")
}
