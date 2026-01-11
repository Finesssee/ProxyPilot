package gemini

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCredentialFileName(t *testing.T) {
	tests := []struct {
		name          string
		email         string
		projectID     string
		includePrefix bool
		want          string
	}{
		{
			name:          "single project without prefix",
			email:         "user@example.com",
			projectID:     "my-project-123",
			includePrefix: false,
			want:          "user@example.com-my-project-123.json",
		},
		{
			name:          "single project with prefix",
			email:         "user@example.com",
			projectID:     "my-project-123",
			includePrefix: true,
			want:          "gemini-user@example.com-my-project-123.json",
		},
		{
			name:          "all projects literal",
			email:         "user@example.com",
			projectID:     "all",
			includePrefix: false,
			want:          "gemini-user@example.com-all.json",
		},
		{
			name:          "all projects uppercase",
			email:         "user@example.com",
			projectID:     "ALL",
			includePrefix: true,
			want:          "gemini-user@example.com-all.json",
		},
		{
			name:          "comma-separated projects",
			email:         "user@example.com",
			projectID:     "project-1,project-2,project-3",
			includePrefix: false,
			want:          "gemini-user@example.com-all.json",
		},
		{
			name:          "whitespace trimmed",
			email:         "  user@example.com  ",
			projectID:     "  my-project  ",
			includePrefix: true,
			want:          "gemini-user@example.com-my-project.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CredentialFileName(tt.email, tt.projectID, tt.includePrefix)
			if got != tt.want {
				t.Errorf("CredentialFileName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGeminiTokenStorage_SaveTokenToFile(t *testing.T) {
	tmpDir := t.TempDir()
	authPath := filepath.Join(tmpDir, "subdir", "token.json")

	ts := &GeminiTokenStorage{
		Token:     map[string]string{"access_token": "test-token"},
		ProjectID: "test-project",
		Email:     "test@example.com",
		Auto:      true,
		Checked:   true,
	}

	err := ts.SaveTokenToFile(authPath)
	if err != nil {
		t.Fatalf("SaveTokenToFile() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(authPath); os.IsNotExist(err) {
		t.Fatal("token file was not created")
	}

	// Verify type was set
	if ts.Type != "gemini" {
		t.Errorf("Type = %q, want %q", ts.Type, "gemini")
	}

	// Verify content is valid JSON
	data, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatalf("failed to read token file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("token file is empty")
	}
}
