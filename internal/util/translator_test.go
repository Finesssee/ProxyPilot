package util_test

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
)

func TestNormalizeNullableTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple nullable string",
			input:    `{"type":["STRING","NULL"]}`,
			expected: `{"type":"STRING"}`,
		},
		{
			name:     "Nested nullable in properties",
			input:    `{"parameters":{"type":"object","properties":{"foo":{"type":["STRING","NULL"]}}}}`,
			expected: `{"parameters":{"type":"object","properties":{"foo":{"type":"STRING"}}}}`,
		},
		{
			name:     "Lowercase null",
			input:    `{"type":["string","null"]}`,
			expected: `{"type":"string"}`,
		},
		{
			name:     "Null first",
			input:    `{"type":["null","STRING"]}`,
			expected: `{"type":"STRING"}`,
		},
		{
			name:     "Non-nullable type unchanged",
			input:    `{"type":"STRING"}`,
			expected: `{"type":"STRING"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := util.NormalizeNullableTypes(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeNullableTypes(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}
