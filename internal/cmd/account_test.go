package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestParseAccounts_Empty(t *testing.T) {
	accounts := parseAccounts(nil)
	if len(accounts) != 0 {
		t.Errorf("parseAccounts(nil) = %d accounts, want 0", len(accounts))
	}
}

func TestAccountInfo_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "future date not expired",
			expiresAt: time.Now().Add(24 * time.Hour),
			want:      false,
		},
		{
			name:      "past date is expired",
			expiresAt: time.Now().Add(-24 * time.Hour),
			want:      true,
		},
		{
			name:      "zero time not expired",
			expiresAt: time.Time{},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc := AccountInfo{
				ExpiresAt: tt.expiresAt,
				IsExpired: !tt.expiresAt.IsZero() && time.Now().After(tt.expiresAt),
			}
			if acc.IsExpired != tt.want {
				t.Errorf("IsExpired = %v, want %v", acc.IsExpired, tt.want)
			}
		})
	}
}

func TestOutputJSON(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	data := map[string]string{"test": "value"}
	err := outputJSON(data)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("outputJSON() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var result map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result["test"] != "value" {
		t.Errorf("JSON output = %v, want {\"test\":\"value\"}", result)
	}
}

func TestOutputTable_Empty(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputTable(nil)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("outputTable() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	output := buf.String()
	if output == "" {
		t.Error("outputTable(nil) should produce output")
	}
}

func TestOutputTable_WithAccounts(t *testing.T) {
	accounts := []AccountInfo{
		{
			ID:        "test-id",
			Provider:  "gemini",
			Email:     "test@example.com",
			ProjectID: "my-project",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			IsExpired: false,
		},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputTable(accounts)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("outputTable() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	output := buf.String()
	if output == "" {
		t.Error("outputTable should produce output")
	}
	// Check for expected content
	if !bytes.Contains(buf.Bytes(), []byte("gemini")) {
		t.Error("output should contain provider name")
	}
	if !bytes.Contains(buf.Bytes(), []byte("test@example.com")) {
		t.Error("output should contain email")
	}
}

func TestTruncation(t *testing.T) {
	// Test that long emails get truncated in table output
	accounts := []AccountInfo{
		{
			Provider:  "gemini",
			Email:     "very-long-email-address-that-exceeds-the-limit@example.com",
			ProjectID: "very-long-project-id-that-also-exceeds-limits",
			IsExpired: false,
		},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = outputTable(accounts)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)

	// Should contain truncated email with "..."
	if !bytes.Contains(buf.Bytes(), []byte("...")) {
		t.Error("long fields should be truncated with ...")
	}
}
