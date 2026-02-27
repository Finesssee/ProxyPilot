package memory

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEvent_JSON_Serialization(t *testing.T) {
	tests := []struct {
		name    string
		event   Event
		wantErr bool
	}{
		{
			name: "full event with all fields",
			event: Event{
				TS:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Kind: "message",
				Role: "user",
				Type: "text",
				Text: "Hello, world!",
				Meta: map[string]string{"key": "value", "model": "gpt-4"},
			},
			wantErr: false,
		},
		{
			name: "minimal event with only required fields",
			event: Event{
				TS:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Kind: "system",
			},
			wantErr: false,
		},
		{
			name: "event with empty optional fields",
			event: Event{
				TS:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Kind: "assistant",
				Role: "",
				Type: "",
				Text: "",
				Meta: nil,
			},
			wantErr: false,
		},
		{
			name: "event with zero timestamp",
			event: Event{
				Kind: "message",
				Text: "test",
			},
			wantErr: false,
		},
		{
			name: "event with unicode text",
			event: Event{
				TS:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Kind: "message",
				Text: "Hello \u4e16\u754c! \U0001F600",
			},
			wantErr: false,
		},
		{
			name: "event with multiline text",
			event: Event{
				TS:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Kind: "message",
				Text: "Line 1\nLine 2\nLine 3",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			data, err := json.Marshal(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Unmarshal back to Event
			var decoded Event
			err = json.Unmarshal(data, &decoded)
			if err != nil {
				t.Errorf("json.Unmarshal() error = %v", err)
				return
			}

			// Compare fields
			if !tt.event.TS.Equal(decoded.TS) {
				t.Errorf("TS mismatch: got %v, want %v", decoded.TS, tt.event.TS)
			}
			if decoded.Kind != tt.event.Kind {
				t.Errorf("Kind mismatch: got %q, want %q", decoded.Kind, tt.event.Kind)
			}
			if decoded.Role != tt.event.Role {
				t.Errorf("Role mismatch: got %q, want %q", decoded.Role, tt.event.Role)
			}
			if decoded.Type != tt.event.Type {
				t.Errorf("Type mismatch: got %q, want %q", decoded.Type, tt.event.Type)
			}
			if decoded.Text != tt.event.Text {
				t.Errorf("Text mismatch: got %q, want %q", decoded.Text, tt.event.Text)
			}
			if len(tt.event.Meta) != len(decoded.Meta) {
				t.Errorf("Meta length mismatch: got %d, want %d", len(decoded.Meta), len(tt.event.Meta))
			}
			for k, v := range tt.event.Meta {
				if decoded.Meta[k] != v {
					t.Errorf("Meta[%q] mismatch: got %q, want %q", k, decoded.Meta[k], v)
				}
			}
		})
	}
}

func TestEvent_JSON_OmitEmpty(t *testing.T) {
	tests := []struct {
		name          string
		event         Event
		shouldContain []string
		shouldOmit    []string
	}{
		{
			name: "omits empty optional fields",
			event: Event{
				TS:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Kind: "message",
			},
			shouldContain: []string{"ts", "kind"},
			shouldOmit:    []string{"role", "type", "text", "meta"},
		},
		{
			name: "includes non-empty optional fields",
			event: Event{
				TS:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Kind: "message",
				Role: "user",
				Text: "hello",
			},
			shouldContain: []string{"ts", "kind", "role", "text"},
			shouldOmit:    []string{"type", "meta"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			jsonStr := string(data)

			for _, field := range tt.shouldContain {
				if !containsField(jsonStr, field) {
					t.Errorf("JSON should contain field %q, got: %s", field, jsonStr)
				}
			}
			for _, field := range tt.shouldOmit {
				if containsField(jsonStr, field) {
					t.Errorf("JSON should omit field %q, got: %s", field, jsonStr)
				}
			}
		})
	}
}

func TestEvent_Fields(t *testing.T) {
	tests := []struct {
		name     string
		event    Event
		wantKind string
		wantRole string
		wantType string
		wantText string
	}{
		{
			name: "user message event",
			event: Event{
				Kind: "message",
				Role: "user",
				Type: "text",
				Text: "What is the weather?",
			},
			wantKind: "message",
			wantRole: "user",
			wantType: "text",
			wantText: "What is the weather?",
		},
		{
			name: "assistant response event",
			event: Event{
				Kind: "message",
				Role: "assistant",
				Type: "text",
				Text: "The weather is sunny.",
			},
			wantKind: "message",
			wantRole: "assistant",
			wantType: "text",
			wantText: "The weather is sunny.",
		},
		{
			name: "system event",
			event: Event{
				Kind: "system",
				Type: "config",
				Text: "Session started",
			},
			wantKind: "system",
			wantRole: "",
			wantType: "config",
			wantText: "Session started",
		},
		{
			name: "tool use event",
			event: Event{
				Kind: "tool_use",
				Role: "assistant",
				Type: "bash",
				Text: "ls -la",
			},
			wantKind: "tool_use",
			wantRole: "assistant",
			wantType: "bash",
			wantText: "ls -la",
		},
		{
			name: "tool result event",
			event: Event{
				Kind: "tool_result",
				Role: "user",
				Type: "bash",
				Text: "file1.txt\nfile2.txt",
			},
			wantKind: "tool_result",
			wantRole: "user",
			wantType: "bash",
			wantText: "file1.txt\nfile2.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.event.Kind != tt.wantKind {
				t.Errorf("Kind = %q, want %q", tt.event.Kind, tt.wantKind)
			}
			if tt.event.Role != tt.wantRole {
				t.Errorf("Role = %q, want %q", tt.event.Role, tt.wantRole)
			}
			if tt.event.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", tt.event.Type, tt.wantType)
			}
			if tt.event.Text != tt.wantText {
				t.Errorf("Text = %q, want %q", tt.event.Text, tt.wantText)
			}
		})
	}
}

func TestEvent_MetaField(t *testing.T) {
	tests := []struct {
		name    string
		meta    map[string]string
		key     string
		wantVal string
		wantOk  bool
	}{
		{
			name:    "existing key",
			meta:    map[string]string{"model": "gpt-4", "tokens": "100"},
			key:     "model",
			wantVal: "gpt-4",
			wantOk:  true,
		},
		{
			name:    "missing key",
			meta:    map[string]string{"model": "gpt-4"},
			key:     "tokens",
			wantVal: "",
			wantOk:  false,
		},
		{
			name:    "nil meta",
			meta:    nil,
			key:     "any",
			wantVal: "",
			wantOk:  false,
		},
		{
			name:    "empty meta",
			meta:    map[string]string{},
			key:     "any",
			wantVal: "",
			wantOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := Event{Meta: tt.meta}
			val, ok := event.Meta[tt.key]
			if ok != tt.wantOk {
				t.Errorf("Meta[%q] ok = %v, want %v", tt.key, ok, tt.wantOk)
			}
			if val != tt.wantVal {
				t.Errorf("Meta[%q] = %q, want %q", tt.key, val, tt.wantVal)
			}
		})
	}
}

func TestEvent_Timestamp(t *testing.T) {
	tests := []struct {
		name   string
		ts     time.Time
		isZero bool
	}{
		{
			name:   "valid timestamp",
			ts:     time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			isZero: false,
		},
		{
			name:   "zero timestamp",
			ts:     time.Time{},
			isZero: true,
		},
		{
			name:   "unix epoch",
			ts:     time.Unix(0, 0).UTC(),
			isZero: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := Event{TS: tt.ts}
			if event.TS.IsZero() != tt.isZero {
				t.Errorf("TS.IsZero() = %v, want %v", event.TS.IsZero(), tt.isZero)
			}
		})
	}
}

// containsField checks if a JSON string contains a specific field key
func containsField(jsonStr, field string) bool {
	return json.Valid([]byte(jsonStr)) && (len(jsonStr) > 0 && (jsonStr[0] == '{' || jsonStr[0] == '[')) &&
		(len(field) > 0 && (jsonFieldPresent(jsonStr, field)))
}

func jsonFieldPresent(jsonStr, field string) bool {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return false
	}
	_, ok := m[field]
	return ok
}
