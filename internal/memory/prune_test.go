package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrune_ByTokenCount(t *testing.T) {
	tests := []struct {
		name                  string
		sessionCount          int
		eventsPerSession      int
		maxBytesPerSession    int64
		wantSessionsTrimmed   int
	}{
		{
			name:                  "trims large sessions",
			sessionCount:          2,
			eventsPerSession:      100,
			maxBytesPerSession:    500,
			wantSessionsTrimmed:   2,
		},
		{
			name:                  "no trimming needed for small sessions",
			sessionCount:          2,
			eventsPerSession:      2,
			maxBytesPerSession:    100000,
			wantSessionsTrimmed:   0,
		},
		{
			name:                  "zero maxBytes skips trimming",
			sessionCount:          2,
			eventsPerSession:      100,
			maxBytesPerSession:    0,
			wantSessionsTrimmed:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := NewFileStore(tmpDir)

			// Create sessions with events
			for i := 0; i < tt.sessionCount; i++ {
				session := "session-" + string(rune('A'+i))
				events := make([]Event, tt.eventsPerSession)
				for j := 0; j < tt.eventsPerSession; j++ {
					events[j] = Event{
						TS:   time.Now(),
						Kind: "message",
						Role: "user",
						Text: strings.Repeat("This is a test message with some content. ", 10),
					}
				}
				if err := store.Append(session, events); err != nil {
					t.Fatalf("Append() error = %v", err)
				}
			}

			// Prune sessions
			result, err := store.PruneSessions(0, 0, tt.maxBytesPerSession)
			if err != nil {
				t.Fatalf("PruneSessions() error = %v", err)
			}

			if result.SessionsTrimmed != tt.wantSessionsTrimmed {
				t.Errorf("SessionsTrimmed = %d, want %d", result.SessionsTrimmed, tt.wantSessionsTrimmed)
			}

			if result.SessionsTrimmed > 0 && result.BytesFreed <= 0 {
				t.Error("Expected BytesFreed > 0 when sessions are trimmed")
			}
		})
	}
}

func TestPrune_PreservesRecent(t *testing.T) {
	tests := []struct {
		name              string
		sessionAges       []int // days ago
		maxAgeDays        int
		maxSessions       int
		wantRemoved       int
		wantKept          []string
	}{
		{
			name:              "preserves recent sessions by age",
			sessionAges:       []int{1, 5, 10, 20, 40}, // days ago
			maxAgeDays:        30,
			maxSessions:       0,
			wantRemoved:       1, // only 40 days old is removed
			wantKept:          []string{"session-0", "session-1", "session-2", "session-3"},
		},
		{
			name:              "preserves by max sessions count",
			sessionAges:       []int{1, 2, 3, 4, 5},
			maxAgeDays:        0,
			maxSessions:       3,
			wantRemoved:       2, // keeps 3 most recent
			wantKept:          []string{"session-0", "session-1", "session-2"},
		},
		{
			name:              "combines age and count limits",
			sessionAges:       []int{1, 5, 35, 40},
			maxAgeDays:        30,
			maxSessions:       2,
			wantRemoved:       2, // 35 and 40 days old are removed
			wantKept:          []string{"session-0", "session-1"},
		},
		{
			name:              "no pruning when all within limits",
			sessionAges:       []int{1, 2, 3},
			maxAgeDays:        30,
			maxSessions:       10,
			wantRemoved:       0,
			wantKept:          []string{"session-0", "session-1", "session-2"},
		},
		{
			name:              "removes all old sessions",
			sessionAges:       []int{100, 200, 300},
			maxAgeDays:        30,
			maxSessions:       0,
			wantRemoved:       3,
			wantKept:          []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := NewFileStore(tmpDir)

			// Create sessions with different ages
			for i, daysAgo := range tt.sessionAges {
				session := "session-" + string(rune('0'+i))
				sessionDir := filepath.Join(tmpDir, "sessions", session)
				if err := os.MkdirAll(sessionDir, 0o755); err != nil {
					t.Fatalf("Failed to create session dir: %v", err)
				}

				// Create events file
				eventsPath := filepath.Join(sessionDir, "events.jsonl")
				if err := os.WriteFile(eventsPath, []byte(`{"kind":"message","text":"test"}`+"\n"), 0o644); err != nil {
					t.Fatalf("Failed to create events file: %v", err)
				}

				// Set modification time
				modTime := time.Now().Add(-time.Duration(daysAgo) * 24 * time.Hour)
				if err := os.Chtimes(eventsPath, modTime, modTime); err != nil {
					t.Fatalf("Failed to set file times: %v", err)
				}
			}

			// Prune sessions
			result, err := store.PruneSessions(tt.maxAgeDays, tt.maxSessions, 0)
			if err != nil {
				t.Fatalf("PruneSessions() error = %v", err)
			}

			if result.SessionsRemoved != tt.wantRemoved {
				t.Errorf("SessionsRemoved = %d, want %d", result.SessionsRemoved, tt.wantRemoved)
			}

			// Verify kept sessions exist
			for _, session := range tt.wantKept {
				sessionDir := filepath.Join(tmpDir, "sessions", session)
				if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
					t.Errorf("Session %q should have been preserved but was removed", session)
				}
			}

			// Verify removed sessions don't exist
			sessions, _ := store.ListSessions(0)
			if len(sessions) != len(tt.wantKept) {
				t.Errorf("Expected %d sessions remaining, got %d", len(tt.wantKept), len(sessions))
			}
		})
	}
}

func TestPrune_SemanticNamespaces(t *testing.T) {
	tests := []struct {
		name                string
		namespaceCount      int
		recordsPerNs        int
		maxAgeDays          int
		maxNamespaces       int
		maxBytesPerNs       int64
		wantRemoved         int
		wantTrimmed         int
	}{
		{
			name:                "removes old namespaces",
			namespaceCount:      3,
			recordsPerNs:        5,
			maxAgeDays:          30,
			maxNamespaces:       0,
			maxBytesPerNs:       0,
			wantRemoved:         0, // all are recent
			wantTrimmed:         0,
		},
		{
			name:                "limits namespace count",
			namespaceCount:      5,
			recordsPerNs:        5,
			maxAgeDays:          0,
			maxNamespaces:       3,
			maxBytesPerNs:       0,
			wantRemoved:         2,
			wantTrimmed:         0,
		},
		{
			name:                "trims large namespaces",
			namespaceCount:      2,
			recordsPerNs:        500,
			maxAgeDays:          0,
			maxNamespaces:       0,
			maxBytesPerNs:       2000,
			wantRemoved:         0,
			wantTrimmed:         2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := NewFileStore(tmpDir)

			// Create namespaces with records
			for i := 0; i < tt.namespaceCount; i++ {
				namespace := "namespace-" + string(rune('A'+i))
				records := make([]SemanticRecord, tt.recordsPerNs)
				for j := 0; j < tt.recordsPerNs; j++ {
					// Make each text unique to avoid deduplication
					records[j] = SemanticRecord{
						TS:   time.Now(),
						Text: strings.Repeat("Test record content ", 20) + " " + string(rune('A'+i)) + string(rune('0'+j%10)),
						Vec:  []float32{float32(i), float32(j), 0.5},
					}
				}
				if err := store.AppendSemantic(namespace, records); err != nil {
					t.Fatalf("AppendSemantic() error = %v", err)
				}
				// Add small delay to ensure different modification times
				time.Sleep(10 * time.Millisecond)
			}

			// Prune semantic namespaces
			result, err := store.PruneSemantic(tt.maxAgeDays, tt.maxNamespaces, tt.maxBytesPerNs)
			if err != nil {
				t.Fatalf("PruneSemantic() error = %v", err)
			}

			if result.SemanticNamespacesRemoved != tt.wantRemoved {
				t.Errorf("SemanticNamespacesRemoved = %d, want %d", result.SemanticNamespacesRemoved, tt.wantRemoved)
			}

			if result.SemanticNamespacesTrimmed != tt.wantTrimmed {
				t.Errorf("SemanticNamespacesTrimmed = %d, want %d", result.SemanticNamespacesTrimmed, tt.wantTrimmed)
			}
		})
	}
}

func TestTrimJSONLFile(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		maxBytes  int64
		wantTrim  bool
		wantFreed bool
	}{
		{
			name:      "trims file exceeding maxBytes",
			content:   `{"a":1}` + "\n" + `{"b":2}` + "\n" + `{"c":3}` + "\n" + `{"d":4}` + "\n",
			maxBytes:  20,
			wantTrim:  true,
			wantFreed: true,
		},
		{
			name:      "no trim needed for small file",
			content:   `{"a":1}` + "\n",
			maxBytes:  1000,
			wantTrim:  false,
			wantFreed: false,
		},
		{
			name:      "zero maxBytes skips trimming",
			content:   `{"a":1}` + "\n" + `{"b":2}` + "\n",
			maxBytes:  0,
			wantTrim:  false,
			wantFreed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "test.jsonl")

			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			originalSize := int64(len(tt.content))
			trimmed, freed := trimJSONLFile(path, tt.maxBytes)

			if trimmed != tt.wantTrim {
				t.Errorf("trimJSONLFile() trimmed = %v, want %v", trimmed, tt.wantTrim)
			}

			if tt.wantFreed && freed <= 0 {
				t.Errorf("Expected freed > 0, got %d", freed)
			}

			if !tt.wantFreed && freed != 0 {
				t.Errorf("Expected freed = 0, got %d", freed)
			}

			// Verify file was actually trimmed
			if tt.wantTrim {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("Failed to read trimmed file: %v", err)
				}
				if int64(len(data)) >= originalSize {
					t.Errorf("File was not actually trimmed: %d >= %d", len(data), originalSize)
				}
			}
		})
	}
}

func TestPruneResult_Fields(t *testing.T) {
	result := PruneResult{
		SessionsRemoved:           5,
		SessionsTrimmed:           3,
		SemanticNamespacesRemoved: 2,
		SemanticNamespacesTrimmed: 1,
		BytesFreed:                1024 * 1024,
	}

	if result.SessionsRemoved != 5 {
		t.Errorf("SessionsRemoved = %d, want 5", result.SessionsRemoved)
	}
	if result.SessionsTrimmed != 3 {
		t.Errorf("SessionsTrimmed = %d, want 3", result.SessionsTrimmed)
	}
	if result.SemanticNamespacesRemoved != 2 {
		t.Errorf("SemanticNamespacesRemoved = %d, want 2", result.SemanticNamespacesRemoved)
	}
	if result.SemanticNamespacesTrimmed != 1 {
		t.Errorf("SemanticNamespacesTrimmed = %d, want 1", result.SemanticNamespacesTrimmed)
	}
	if result.BytesFreed != 1024*1024 {
		t.Errorf("BytesFreed = %d, want %d", result.BytesFreed, 1024*1024)
	}
}

func TestSessionInfo_Fields(t *testing.T) {
	now := time.Now()
	info := SessionInfo{
		Key:              "test-session",
		Path:             "/path/to/session",
		UpdatedAt:        now,
		SizeBytes:        2048,
		EventsBytes:      1024,
		HasSummary:       true,
		HasTodo:          true,
		HasPinned:        false,
		HasAnchorPending: true,
		SemanticDisabled: false,
	}

	if info.Key != "test-session" {
		t.Errorf("Key = %q, want %q", info.Key, "test-session")
	}
	if info.Path != "/path/to/session" {
		t.Errorf("Path = %q, want %q", info.Path, "/path/to/session")
	}
	if !info.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt mismatch")
	}
	if info.SizeBytes != 2048 {
		t.Errorf("SizeBytes = %d, want 2048", info.SizeBytes)
	}
	if info.EventsBytes != 1024 {
		t.Errorf("EventsBytes = %d, want 1024", info.EventsBytes)
	}
	if !info.HasSummary {
		t.Error("HasSummary should be true")
	}
	if !info.HasTodo {
		t.Error("HasTodo should be true")
	}
	if info.HasPinned {
		t.Error("HasPinned should be false")
	}
	if !info.HasAnchorPending {
		t.Error("HasAnchorPending should be true")
	}
	if info.SemanticDisabled {
		t.Error("SemanticDisabled should be false")
	}
}

func TestListSessions(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	// Create some sessions
	sessions := []string{"alpha", "beta", "gamma"}
	for _, session := range sessions {
		events := []Event{{Kind: "message", Text: "test"}}
		if err := store.Append(session, events); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
		// Small delay to ensure different mod times
		time.Sleep(10 * time.Millisecond)
	}

	// List all sessions
	list, err := store.ListSessions(0)
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}

	if len(list) != len(sessions) {
		t.Errorf("ListSessions() returned %d, want %d", len(list), len(sessions))
	}

	// List with limit
	limited, err := store.ListSessions(2)
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}

	if len(limited) != 2 {
		t.Errorf("ListSessions(2) returned %d, want 2", len(limited))
	}

	// Verify most recent is first (gamma was created last)
	if len(limited) > 0 && limited[0].Key != "gamma" {
		t.Errorf("Most recent session should be 'gamma', got %q", limited[0].Key)
	}
}

func TestGetSessionInfo(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	session := "info-test"

	// Create session with various files
	events := []Event{{Kind: "message", Text: "test event"}}
	if err := store.Append(session, events); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	if err := store.WriteSummary(session, "Test summary", 0); err != nil {
		t.Fatalf("WriteSummary() error = %v", err)
	}

	if err := store.WriteTodo(session, "Test todo", 0); err != nil {
		t.Fatalf("WriteTodo() error = %v", err)
	}

	// Get session info
	info, err := store.GetSessionInfo(session)
	if err != nil {
		t.Fatalf("GetSessionInfo() error = %v", err)
	}

	if info.Key != session {
		t.Errorf("Key = %q, want %q", info.Key, session)
	}
	if !info.HasSummary {
		t.Error("HasSummary should be true")
	}
	if !info.HasTodo {
		t.Error("HasTodo should be true")
	}
	if info.SizeBytes <= 0 {
		t.Error("SizeBytes should be > 0")
	}
	if info.EventsBytes <= 0 {
		t.Error("EventsBytes should be > 0")
	}
}

func TestReadEventTail(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	session := "tail-test"

	// Create many events
	events := make([]Event, 20)
	for i := 0; i < 20; i++ {
		events[i] = Event{
			Kind: "message",
			Role: "user",
			Text: "Message " + string(rune('A'+i)),
		}
	}

	if err := store.Append(session, events); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	// Read tail with limit
	tail, err := store.ReadEventTail(session, 5)
	if err != nil {
		t.Fatalf("ReadEventTail() error = %v", err)
	}

	if len(tail) != 5 {
		t.Errorf("ReadEventTail() returned %d events, want 5", len(tail))
	}

	// Verify chronological order (oldest first in result)
	if len(tail) > 0 {
		// Should be last 5 events: P, Q, R, S, T
		expectedFirst := "Message P"
		if tail[0].Text != expectedFirst {
			t.Errorf("First event text = %q, want %q", tail[0].Text, expectedFirst)
		}
	}
}

func TestExportSessionZip(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	session := "export-test"

	// Create session with content
	events := []Event{{Kind: "message", Text: "test event"}}
	if err := store.Append(session, events); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	if err := store.WriteSummary(session, "Test summary", 0); err != nil {
		t.Fatalf("WriteSummary() error = %v", err)
	}

	// Export as zip
	sessionDir := store.SessionDir(session)
	zipData, err := ExportSessionZip(sessionDir)
	if err != nil {
		t.Fatalf("ExportSessionZip() error = %v", err)
	}

	if len(zipData) == 0 {
		t.Error("Expected non-empty zip data")
	}

	// Verify it's a valid zip (starts with PK)
	if len(zipData) < 2 || zipData[0] != 'P' || zipData[1] != 'K' {
		t.Error("Expected valid zip file header")
	}
}

func TestExportSessionZip_EmptyDir(t *testing.T) {
	_, err := ExportSessionZip("")
	if err == nil {
		t.Error("Expected error for empty directory")
	}
}

func TestExportAllZip(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	// Create multiple sessions
	for _, session := range []string{"session-1", "session-2"} {
		events := []Event{{Kind: "message", Text: "test in " + session}}
		if err := store.Append(session, events); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	// Export all
	zipData, err := ExportAllZip(tmpDir, 10*1024*1024)
	if err != nil {
		t.Fatalf("ExportAllZip() error = %v", err)
	}

	if len(zipData) == 0 {
		t.Error("Expected non-empty zip data")
	}
}

func TestExportAllZip_SizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	// Create session with large content
	events := []Event{{Kind: "message", Text: strings.Repeat("x", 10000)}}
	if err := store.Append("large-session", events); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	// Try to export with very small limit
	_, err := ExportAllZip(tmpDir, 100)
	if err == nil {
		t.Error("Expected error when size exceeds limit")
	}
}
