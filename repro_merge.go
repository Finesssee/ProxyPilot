package main

import (
	"fmt"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func itoa(i int) string { return fmt.Sprintf("%d", i) }

func main() {
	rawJSON := []byte(`{
        "messages": [
            {"role": "user", "content": "hi"},
            {"role": "assistant", "content": "Thinking..."},
            {"role": "assistant", "tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "test", "arguments": "{}"}}]}
        ]
    }`)

	out := []byte(`{"contents":[]}`)

	messages := gjson.GetBytes(rawJSON, "messages")
	arr := messages.Array()

	for i := 0; i < len(arr); i++ {
		m := arr[i]
		role := m.Get("role").String()
		content := m.Get("content")

		var targetRole string
		partsNode := []byte(`[]`)
		hasContent := false

		if role == "user" {
			targetRole = "user"
			partsNode, _ = sjson.SetBytes(partsNode, "-1.text", content.String())
			hasContent = true
		} else if role == "assistant" {
			targetRole = "model"
			if content.Type == gjson.String {
				partsNode, _ = sjson.SetBytes(partsNode, "-1.text", content.String())
				hasContent = true
			} else if !content.Exists() || content.Type == gjson.Null {
				tcs := m.Get("tool_calls")
				if tcs.IsArray() {
					for _, tc := range tcs.Array() {
						idx := int(gjson.ParseBytes(partsNode).Get("#").Int())
						partsNode, _ = sjson.SetBytes(partsNode, itoa(idx)+".functionCall.name", tc.Get("function.name").String())
						hasContent = true
					}
				}
			}
		}

		if hasContent && targetRole != "" {
			contents := gjson.GetBytes(out, "contents")
			var lastRole string
			var lastIdx int = -1

			if contents.IsArray() {
				arr := contents.Array()
				if len(arr) > 0 {
					lastIdx = len(arr) - 1
					lastRole = arr[lastIdx].Get("role").String()
				}
			}

			fmt.Printf("i=%d Role=%s Target=%s LastRole=%s LastIdx=%d\n", i, role, targetRole, lastRole, lastIdx)

			if lastIdx >= 0 && lastRole == targetRole {
				fmt.Println("  > MERGING")
				newParts := gjson.ParseBytes(partsNode).Array()
				for _, np := range newParts {
					out, _ = sjson.SetRawBytes(out, "contents."+itoa(lastIdx)+".parts.-1", []byte(np.Raw))
				}
			} else {
				fmt.Println("  > NEW TURN")
				node := []byte(`{"role":"` + targetRole + `","parts":[]}`)
				node, _ = sjson.SetRawBytes(node, "parts", partsNode)
				out, _ = sjson.SetRawBytes(out, "contents.-1", node)
			}
		}
	}
	fmt.Println(string(out))
}
