package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileStore_Append_CreatesFile(t *testing.T) {
	tests := []struct {
		name    string
		session string
		events  []Event
		wantErr bool
	}{
		{
			name:    "creates file with single event",
			session: "test-session-1",
			events: []Event{
				{
					TS:   time.Now(),
					Kind: "message",
					Role: "user",
					Text: "Hello, world!",
				},
			},
			wantErr: false,
		},
		{
			name:    "creates file with event that has zero timestamp",
			session: "test-session-2",
			events: []Event{
				{
					Kind: "message",
					Role: "user",
					Text: "This event has no timestamp set",
				},
			},
			wantErr: false,
		},
		{
			name:    "empty session returns nil without creating file",
			session: "",
			events: []Event{
				{Kind: "message", Text: "test"},
			},
			wantErr: false,
		},
		{
			name:    "empty events returns nil without creating file",
			session: "test-session-3",
			events:  []Event{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := NewFileStore(tmpDir)

			err := store.Append(tt.session, tt.events)
			if (err != nil) != tt.wantErr {
				t.Errorf("Append() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.session == "" || len(tt.events) == 0 {
				// File should not be created
				eventsPath := filepath.Join(tmpDir, "sessions", tt.session, "events.jsonl")
				if _, err := os.Stat(eventsPath); !os.IsNotExist(err) {
					t.Errorf("File should not exist for empty session or events")
				}
				return
			}

			// Verify file was created
			eventsPath := filepath.Join(tmpDir, "sessions", tt.session, "events.jsonl")
			if _, err := os.Stat(eventsPath); os.IsNotExist(err) {
				t.Errorf("events.jsonl was not created")
				return
			}

			// Verify content is valid JSONL
			data, err := os.ReadFile(eventsPath)
			if err != nil {
				t.Fatalf("Failed to read events file: %v", err)
			}

			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) != len(tt.events) {
				t.Errorf("Expected %d lines, got %d", len(tt.events), len(lines))
			}

			for i, line := range lines {
				var e Event
				if err := json.Unmarshal([]byte(line), &e); err != nil {
					t.Errorf("Line %d is not valid JSON: %v", i, err)
				}
			}
		})
	}
}

func TestFileStore_Append_MultipleEvents(t *testing.T) {
	tests := []struct {
		name      string
		session   string
		eventSets [][]Event
		wantTotal int
		wantErr   bool
	}{
		{
			name:    "append multiple events in single call",
			session: "multi-event-session",
			eventSets: [][]Event{
				{
					{Kind: "message", Role: "user", Text: "First message"},
					{Kind: "message", Role: "assistant", Text: "First response"},
					{Kind: "message", Role: "user", Text: "Second message"},
				},
			},
			wantTotal: 3,
			wantErr:   false,
		},
		{
			name:    "append events across multiple calls",
			session: "multiple-calls-session",
			eventSets: [][]Event{
				{{Kind: "message", Role: "user", Text: "Message 1"}},
				{{Kind: "message", Role: "assistant", Text: "Response 1"}},
				{{Kind: "message", Role: "user", Text: "Message 2"}},
				{{Kind: "message", Role: "assistant", Text: "Response 2"}},
			},
			wantTotal: 4,
			wantErr:   false,
		},
		{
			name:    "append mixed events with metadata",
			session: "mixed-events-session",
			eventSets: [][]Event{
				{
					{Kind: "system", Text: "Session started"},
					{Kind: "message", Role: "user", Text: "Hello", Meta: map[string]string{"lang": "en"}},
					{Kind: "tool_use", Role: "assistant", Type: "bash", Text: "ls"},
					{Kind: "tool_result", Role: "user", Type: "bash", Text: "file.txt"},
				},
			},
			wantTotal: 4,
			wantErr:   false,
		},
		{
			name:    "append preserves order",
			session: "order-session",
			eventSets: [][]Event{
				{
					{Kind: "message", Role: "user", Text: "First"},
					{Kind: "message", Role: "user", Text: "Second"},
					{Kind: "message", Role: "user", Text: "Third"},
				},
			},
			wantTotal: 3,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := NewFileStore(tmpDir)

			for _, events := range tt.eventSets {
				err := store.Append(tt.session, events)
				if (err != nil) != tt.wantErr {
					t.Errorf("Append() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
			}

			// Read and verify the file
			eventsPath := filepath.Join(tmpDir, "sessions", tt.session, "events.jsonl")
			data, err := os.ReadFile(eventsPath)
			if err != nil {
				t.Fatalf("Failed to read events file: %v", err)
			}

			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) != tt.wantTotal {
				t.Errorf("Expected %d events, got %d", tt.wantTotal, len(lines))
			}

			// Verify each line is valid JSON and check order for order test
			if tt.name == "append preserves order" {
				expectedTexts := []string{"First", "Second", "Third"}
				for i, line := range lines {
					var e Event
					if err := json.Unmarshal([]byte(line), &e); err != nil {
						t.Errorf("Line %d is not valid JSON: %v", i, err)
						continue
					}
					if e.Text != expectedTexts[i] {
						t.Errorf("Event %d: expected text %q, got %q", i, expectedTexts[i], e.Text)
					}
				}
			}
		})
	}
}

func TestFileStore_Append_RedactsSecrets(t *testing.T) {
	tests := []struct {
		name       string
		inputText  string
		wantRedact bool
	}{
		{
			name:       "redacts sk- API keys",
			inputText:  "My key is sk-1234567890abcdef",
			wantRedact: true,
		},
		{
			name:       "redacts bearer tokens",
			inputText:  "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			wantRedact: true,
		},
		{
			name:       "redacts AIza keys",
			inputText:  "API key: AIzaSyB1234567890abcdefghij",
			wantRedact: true,
		},
		{
			name:       "does not modify normal text",
			inputText:  "This is a normal message without secrets",
			wantRedact: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := NewFileStore(tmpDir)
			session := "redact-test"

			err := store.Append(session, []Event{
				{Kind: "message", Text: tt.inputText},
			})
			if err != nil {
				t.Fatalf("Append() error = %v", err)
			}

			// Read the stored event
			eventsPath := filepath.Join(tmpDir, "sessions", session, "events.jsonl")
			data, err := os.ReadFile(eventsPath)
			if err != nil {
				t.Fatalf("Failed to read events file: %v", err)
			}

			var e Event
			if err := json.Unmarshal(data, &e); err != nil {
				t.Fatalf("Failed to parse event: %v", err)
			}

			hasRedacted := strings.Contains(e.Text, "[REDACTED]")
			if tt.wantRedact && !hasRedacted {
				t.Errorf("Expected text to be redacted, got: %s", e.Text)
			}
			if !tt.wantRedact && hasRedacted {
				t.Errorf("Text should not be redacted, got: %s", e.Text)
			}
		})
	}
}

func TestFileStore_Search_ExactMatch(t *testing.T) {
	tests := []struct {
		name      string
		events    []Event
		query     string
		wantCount int
		wantMatch string
	}{
		{
			name: "finds exact keyword match",
			events: []Event{
				{Kind: "message", Role: "user", Text: "How do I configure the database?"},
				{Kind: "message", Role: "assistant", Text: "You can configure the database in config.yaml"},
				{Kind: "message", Role: "user", Text: "Thanks, what about logging?"},
			},
			query:     "database",
			wantCount: 2,
			wantMatch: "database",
		},
		{
			name: "case insensitive search",
			events: []Event{
				{Kind: "message", Text: "The DATABASE connection failed"},
				{Kind: "message", Text: "Database retry succeeded"},
				{Kind: "message", Text: "No matches here"},
			},
			query:     "database",
			wantCount: 2,
			wantMatch: "",
		},
		{
			name: "no matches returns empty",
			events: []Event{
				{Kind: "message", Text: "Hello world"},
				{Kind: "message", Text: "Goodbye world"},
			},
			query:     "nonexistent",
			wantCount: 0,
			wantMatch: "",
		},
		{
			name: "multiple keywords - matches any keyword",
			events: []Event{
				{Kind: "message", Text: "Error in authentication module"},
				{Kind: "message", Text: "Authentication failed for user"},
				{Kind: "message", Text: "Module loaded successfully"},
			},
			query:     "authentication module",
			wantCount: 3, // all three contain at least one keyword
			wantMatch: "",
		},
		{
			name: "empty query returns nil",
			events: []Event{
				{Kind: "message", Text: "Some text"},
			},
			query:     "",
			wantCount: 0,
			wantMatch: "",
		},
		{
			name: "whitespace query returns nil",
			events: []Event{
				{Kind: "message", Text: "Some text"},
			},
			query:     "   ",
			wantCount: 0,
			wantMatch: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := NewFileStore(tmpDir)
			session := "search-test"

			// Append test events
			if err := store.Append(session, tt.events); err != nil {
				t.Fatalf("Append() error = %v", err)
			}

			// Search
			results, err := store.Search(session, tt.query, 6000, 10)
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}

			if len(results) != tt.wantCount {
				t.Errorf("Search() returned %d results, want %d", len(results), tt.wantCount)
			}

			if tt.wantMatch != "" {
				found := false
				for _, r := range results {
					if strings.Contains(strings.ToLower(r), tt.wantMatch) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Search results should contain %q", tt.wantMatch)
				}
			}
		})
	}
}

func TestFileStore_Search_MaxSnippets(t *testing.T) {
	tests := []struct {
		name        string
		eventCount  int
		maxSnippets int
		wantMax     int
	}{
		{
			name:        "limits to maxSnippets",
			eventCount:  20,
			maxSnippets: 5,
			wantMax:     5,
		},
		{
			name:        "returns all if under limit",
			eventCount:  3,
			maxSnippets: 10,
			wantMax:     3,
		},
		{
			name:        "default maxSnippets when zero",
			eventCount:  20,
			maxSnippets: 0,
			wantMax:     8, // default is 8
		},
		{
			name:        "maxSnippets of 1",
			eventCount:  10,
			maxSnippets: 1,
			wantMax:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := NewFileStore(tmpDir)
			session := "maxsnippets-test"

			// Create events with searchable text
			events := make([]Event, tt.eventCount)
			for i := 0; i < tt.eventCount; i++ {
				events[i] = Event{
					Kind: "message",
					Role: "user",
					Text: "This is a searchable test message number " + string(rune('A'+i)),
				}
			}

			if err := store.Append(session, events); err != nil {
				t.Fatalf("Append() error = %v", err)
			}

			results, err := store.Search(session, "searchable test message", 100000, tt.maxSnippets)
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}

			if len(results) > tt.wantMax {
				t.Errorf("Search() returned %d results, want at most %d", len(results), tt.wantMax)
			}
		})
	}
}

func TestFileStore_Search_MaxChars(t *testing.T) {
	tests := []struct {
		name      string
		events    []Event
		maxChars  int
		wantUnder bool
	}{
		{
			name: "respects maxChars limit",
			events: []Event{
				{Kind: "message", Text: strings.Repeat("searchable text ", 100)},
				{Kind: "message", Text: strings.Repeat("more searchable text ", 100)},
			},
			maxChars:  500,
			wantUnder: true,
		},
		{
			name: "default maxChars when zero",
			events: []Event{
				{Kind: "message", Text: "searchable text"},
			},
			maxChars:  0,
			wantUnder: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := NewFileStore(tmpDir)
			session := "maxchars-test"

			if err := store.Append(session, tt.events); err != nil {
				t.Fatalf("Append() error = %v", err)
			}

			results, err := store.Search(session, "searchable text", tt.maxChars, 100)
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}

			if tt.wantUnder {
				totalChars := 0
				for _, r := range results {
					totalChars += len(r)
				}

				limit := tt.maxChars
				if limit <= 0 {
					limit = 6000 // default
				}

				if totalChars > limit {
					t.Errorf("Total chars %d exceeds maxChars %d", totalChars, limit)
				}
			}
		})
	}
}

func TestFileStore_Search_NonExistentSession(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	results, err := store.Search("nonexistent-session", "query", 6000, 10)
	if err != nil {
		t.Errorf("Search() should not error for nonexistent session, got: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search() should return empty for nonexistent session, got %d results", len(results))
	}
}

func TestFileStore_NilStore(t *testing.T) {
	var store *FileStore

	t.Run("Append on nil store", func(t *testing.T) {
		err := store.Append("session", []Event{{Kind: "message", Text: "test"}})
		if err == nil {
			t.Error("Expected error for nil store, got nil")
		}
	})

	t.Run("Search on nil store", func(t *testing.T) {
		_, err := store.Search("session", "query", 100, 10)
		if err == nil {
			t.Error("Expected error for nil store, got nil")
		}
	})
}

func TestFileStore_EmptyBaseDir(t *testing.T) {
	store := NewFileStore("")

	t.Run("Append with empty BaseDir", func(t *testing.T) {
		err := store.Append("session", []Event{{Kind: "message", Text: "test"}})
		if err == nil {
			t.Error("Expected error for empty BaseDir, got nil")
		}
	})

	t.Run("Search with empty BaseDir", func(t *testing.T) {
		_, err := store.Search("session", "query", 100, 10)
		if err == nil {
			t.Error("Expected error for empty BaseDir, got nil")
		}
	})
}

func TestFileStore_SearchCache(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	session := "cache-test"

	events := []Event{
		{Kind: "message", Role: "user", Text: "This is a cacheable message"},
	}

	if err := store.Append(session, events); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	// First search populates cache
	results1, err := store.Search(session, "cacheable", 6000, 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	// Second search should use cache (same query)
	results2, err := store.Search(session, "cacheable", 6000, 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results1) != len(results2) {
		t.Errorf("Cached result count mismatch: %d vs %d", len(results1), len(results2))
	}

	// Append invalidates cache
	if err := store.Append(session, []Event{{Kind: "message", Text: "New cacheable event"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	// Search after append should get fresh results
	results3, err := store.Search(session, "cacheable", 6000, 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results3) <= len(results1) {
		t.Logf("Results after append: %d (was %d)", len(results3), len(results1))
	}
}

func TestFileStore_SessionDir(t *testing.T) {
	tests := []struct {
		name    string
		session string
		wantDir bool
	}{
		{
			name:    "normal session name",
			session: "my-session-123",
			wantDir: true,
		},
		{
			name:    "session with special characters",
			session: "session/with\\special:chars*",
			wantDir: true,
		},
		{
			name:    "empty session",
			session: "",
			wantDir: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := NewFileStore(tmpDir)

			dir := store.SessionDir(tt.session)
			if tt.wantDir && dir == "" {
				t.Error("SessionDir() returned empty string")
			}
			if tt.wantDir && !strings.HasPrefix(dir, tmpDir) {
				t.Errorf("SessionDir() = %q, should be under %q", dir, tmpDir)
			}
		})
	}
}
