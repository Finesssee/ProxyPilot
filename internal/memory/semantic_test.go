package memory

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSemanticStore_Append(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		records   []SemanticRecord
		wantErr   bool
		wantCount int
	}{
		{
			name:      "appends single record",
			namespace: "test-namespace",
			records: []SemanticRecord{
				{
					TS:   time.Now(),
					Role: "user",
					Text: "Hello, world!",
					Vec:  []float32{0.1, 0.2, 0.3},
				},
			},
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:      "appends multiple records",
			namespace: "multi-record-ns",
			records: []SemanticRecord{
				{Text: "First message", Vec: []float32{0.1, 0.2, 0.3}},
				{Text: "Second message", Vec: []float32{0.4, 0.5, 0.6}},
				{Text: "Third message", Vec: []float32{0.7, 0.8, 0.9}},
			},
			wantErr:   false,
			wantCount: 3,
		},
		{
			name:      "skips records with empty text",
			namespace: "skip-empty-ns",
			records: []SemanticRecord{
				{Text: "Valid text", Vec: []float32{0.1, 0.2, 0.3}},
				{Text: "", Vec: []float32{0.4, 0.5, 0.6}},
				{Text: "   ", Vec: []float32{0.7, 0.8, 0.9}},
			},
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:      "skips records with empty vector",
			namespace: "skip-empty-vec-ns",
			records: []SemanticRecord{
				{Text: "Valid", Vec: []float32{0.1, 0.2, 0.3}},
				{Text: "No vector", Vec: []float32{}},
				{Text: "Nil vector", Vec: nil},
			},
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:      "skips duplicate texts",
			namespace: "skip-dupe-ns",
			records: []SemanticRecord{
				{Text: "Same text", Vec: []float32{0.1, 0.2, 0.3}},
				{Text: "Same text", Vec: []float32{0.4, 0.5, 0.6}},
				{Text: "Different text", Vec: []float32{0.7, 0.8, 0.9}},
			},
			wantErr:   false,
			wantCount: 2,
		},
		{
			name:      "empty namespace returns nil",
			namespace: "",
			records: []SemanticRecord{
				{Text: "Test", Vec: []float32{0.1, 0.2, 0.3}},
			},
			wantErr:   false,
			wantCount: 0,
		},
		{
			name:      "empty records returns nil",
			namespace: "empty-records-ns",
			records:   []SemanticRecord{},
			wantErr:   false,
			wantCount: 0,
		},
		{
			name:      "computes norm if not provided",
			namespace: "compute-norm-ns",
			records: []SemanticRecord{
				{Text: "Norm will be computed", Vec: []float32{3, 4}, Norm: 0},
			},
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:      "uses provided norm",
			namespace: "provided-norm-ns",
			records: []SemanticRecord{
				{Text: "Norm is provided", Vec: []float32{3, 4}, Norm: 5.0},
			},
			wantErr:   false,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := NewFileStore(tmpDir)

			err := store.AppendSemantic(tt.namespace, tt.records)
			if (err != nil) != tt.wantErr {
				t.Errorf("AppendSemantic() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.namespace == "" || len(tt.records) == 0 {
				return
			}

			// Verify file content
			dir := filepath.Join(tmpDir, "semantic")
			entries, err := os.ReadDir(dir)
			if err != nil {
				if tt.wantCount == 0 {
					return // No file expected
				}
				t.Fatalf("Failed to read semantic dir: %v", err)
			}

			if len(entries) == 0 && tt.wantCount > 0 {
				t.Errorf("Expected semantic namespace directory to be created")
				return
			}

			// Find the items.jsonl file
			var itemsPath string
			for _, e := range entries {
				if e.IsDir() {
					itemsPath = filepath.Join(dir, e.Name(), "items.jsonl")
					break
				}
			}

			if itemsPath == "" {
				if tt.wantCount > 0 {
					t.Error("items.jsonl not found")
				}
				return
			}

			data, err := os.ReadFile(itemsPath)
			if err != nil {
				t.Fatalf("Failed to read items.jsonl: %v", err)
			}

			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			actualCount := 0
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					actualCount++
				}
			}

			if actualCount != tt.wantCount {
				t.Errorf("Expected %d records, got %d", tt.wantCount, actualCount)
			}
		})
	}
}

func TestSemanticStore_Search(t *testing.T) {
	tests := []struct {
		name        string
		records     []SemanticRecord
		queryVec    []float32
		maxChars    int
		maxSnippets int
		wantCount   int
		wantFirst   string
	}{
		{
			name: "finds similar vectors",
			records: []SemanticRecord{
				{Text: "Apple is a fruit", Vec: []float32{0.9, 0.1, 0.0}},
				{Text: "Car is a vehicle", Vec: []float32{0.1, 0.9, 0.0}},
				{Text: "Orange is a fruit", Vec: []float32{0.85, 0.15, 0.0}},
			},
			queryVec:    []float32{0.9, 0.1, 0.0},
			maxChars:    6000,
			maxSnippets: 10,
			wantCount:   3,
			wantFirst:   "Apple is a fruit",
		},
		{
			name: "returns empty for no matches",
			records: []SemanticRecord{
				{Text: "Text 1", Vec: []float32{1, 0, 0}},
				{Text: "Text 2", Vec: []float32{0, 1, 0}},
			},
			queryVec:    []float32{0, 0, 1},
			maxChars:    6000,
			maxSnippets: 10,
			wantCount:   0,
			wantFirst:   "",
		},
		{
			name: "respects maxSnippets",
			records: []SemanticRecord{
				{Text: "Text 1", Vec: []float32{0.9, 0.1, 0.0}},
				{Text: "Text 2", Vec: []float32{0.85, 0.15, 0.0}},
				{Text: "Text 3", Vec: []float32{0.8, 0.2, 0.0}},
				{Text: "Text 4", Vec: []float32{0.75, 0.25, 0.0}},
			},
			queryVec:    []float32{0.9, 0.1, 0.0},
			maxChars:    6000,
			maxSnippets: 2,
			wantCount:   2,
			wantFirst:   "Text 1",
		},
		{
			name: "empty query vector returns nil",
			records: []SemanticRecord{
				{Text: "Some text", Vec: []float32{0.1, 0.2, 0.3}},
			},
			queryVec:    []float32{},
			maxChars:    6000,
			maxSnippets: 10,
			wantCount:   0,
			wantFirst:   "",
		},
		{
			name: "orders by similarity score",
			records: []SemanticRecord{
				{Text: "Low similarity", Vec: []float32{0.1, 0.9, 0.0}},
				{Text: "High similarity", Vec: []float32{0.95, 0.05, 0.0}},
				{Text: "Medium similarity", Vec: []float32{0.5, 0.5, 0.0}},
			},
			queryVec:    []float32{1.0, 0.0, 0.0},
			maxChars:    6000,
			maxSnippets: 10,
			wantCount:   3,
			wantFirst:   "High similarity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := NewFileStore(tmpDir)
			namespace := "search-test-ns"

			// Append records
			if len(tt.records) > 0 {
				if err := store.AppendSemantic(namespace, tt.records); err != nil {
					t.Fatalf("AppendSemantic() error = %v", err)
				}
			}

			// Search
			results, err := store.SearchSemantic(namespace, tt.queryVec, tt.maxChars, tt.maxSnippets)
			if err != nil {
				t.Fatalf("SearchSemantic() error = %v", err)
			}

			if len(results) != tt.wantCount {
				t.Errorf("SearchSemantic() returned %d results, want %d", len(results), tt.wantCount)
			}

			if tt.wantFirst != "" && len(results) > 0 {
				if results[0] != tt.wantFirst {
					t.Errorf("First result = %q, want %q", results[0], tt.wantFirst)
				}
			}
		})
	}
}

func TestSemanticStore_SearchWithText(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	namespace := "text-search-ns"

	records := []SemanticRecord{
		{Text: "Python programming tutorial", Vec: []float32{0.9, 0.1, 0.0}},
		{Text: "Java development guide", Vec: []float32{0.85, 0.15, 0.0}},
		{Text: "Python data science course", Vec: []float32{0.88, 0.12, 0.0}},
	}

	if err := store.AppendSemantic(namespace, records); err != nil {
		t.Fatalf("AppendSemantic() error = %v", err)
	}

	// Search with both vector and text (keyword boost)
	results, err := store.SearchSemanticWithText(namespace, []float32{0.9, 0.1, 0.0}, "python", 6000, 10)
	if err != nil {
		t.Fatalf("SearchSemanticWithText() error = %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected results from SearchSemanticWithText")
	}

	// Python results should be boosted
	pythonCount := 0
	for _, r := range results {
		if strings.Contains(strings.ToLower(r), "python") {
			pythonCount++
		}
	}

	if pythonCount == 0 {
		t.Error("Expected Python-related results to be present")
	}
}

func TestEmbedding_Normalize(t *testing.T) {
	tests := []struct {
		name     string
		vec      []float32
		wantNorm float32
		wantZero bool
	}{
		{
			name:     "unit vector",
			vec:      []float32{1, 0, 0},
			wantNorm: 1.0,
			wantZero: false,
		},
		{
			name:     "3-4-5 right triangle",
			vec:      []float32{3, 4},
			wantNorm: 5.0,
			wantZero: false,
		},
		{
			name:     "all zeros",
			vec:      []float32{0, 0, 0},
			wantNorm: 0,
			wantZero: true,
		},
		{
			name:     "empty vector",
			vec:      []float32{},
			wantNorm: 0,
			wantZero: true,
		},
		{
			name:     "negative values",
			vec:      []float32{-3, 4},
			wantNorm: 5.0,
			wantZero: false,
		},
		{
			name:     "high dimensional",
			vec:      []float32{1, 1, 1, 1},
			wantNorm: 2.0, // sqrt(4) = 2
			wantZero: false,
		},
		{
			name:     "small values",
			vec:      []float32{0.001, 0.001, 0.001},
			wantNorm: float32(math.Sqrt(0.000003)),
			wantZero: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			norm := vectorNorm(tt.vec)

			if tt.wantZero && norm != 0 {
				t.Errorf("vectorNorm() = %v, want 0", norm)
				return
			}

			if !tt.wantZero {
				tolerance := float32(0.0001)
				if diff := math.Abs(float64(norm - tt.wantNorm)); diff > float64(tolerance) {
					t.Errorf("vectorNorm() = %v, want %v (diff: %v)", norm, tt.wantNorm, diff)
				}
			}
		})
	}
}

func TestEmbedding_CosineSimilarity(t *testing.T) {
	tests := []struct {
		name       string
		vecA       []float32
		normA      float32
		vecB       []float32
		normB      float32
		wantSim    float32
		tolerance  float32
	}{
		{
			name:      "identical vectors",
			vecA:      []float32{1, 0, 0},
			normA:     1.0,
			vecB:      []float32{1, 0, 0},
			normB:     1.0,
			wantSim:   1.0,
			tolerance: 0.0001,
		},
		{
			name:      "orthogonal vectors",
			vecA:      []float32{1, 0, 0},
			normA:     1.0,
			vecB:      []float32{0, 1, 0},
			normB:     1.0,
			wantSim:   0.0,
			tolerance: 0.0001,
		},
		{
			name:      "opposite vectors",
			vecA:      []float32{1, 0, 0},
			normA:     1.0,
			vecB:      []float32{-1, 0, 0},
			normB:     1.0,
			wantSim:   -1.0,
			tolerance: 0.0001,
		},
		{
			name:      "45 degree angle",
			vecA:      []float32{1, 0},
			normA:     1.0,
			vecB:      []float32{1, 1},
			normB:     float32(math.Sqrt(2)),
			wantSim:   float32(1.0 / math.Sqrt(2)), // cos(45) = 1/sqrt(2)
			tolerance: 0.0001,
		},
		{
			name:      "non-unit vectors",
			vecA:      []float32{3, 4},
			normA:     5.0,
			vecB:      []float32{6, 8},
			normB:     10.0,
			wantSim:   1.0, // Same direction
			tolerance: 0.0001,
		},
		{
			name:      "zero norm A returns 0",
			vecA:      []float32{0, 0, 0},
			normA:     0,
			vecB:      []float32{1, 0, 0},
			normB:     1.0,
			wantSim:   0,
			tolerance: 0,
		},
		{
			name:      "zero norm B returns 0",
			vecA:      []float32{1, 0, 0},
			normA:     1.0,
			vecB:      []float32{0, 0, 0},
			normB:     0,
			wantSim:   0,
			tolerance: 0,
		},
		{
			name:      "different length vectors",
			vecA:      []float32{1, 2, 3, 4, 5},
			normA:     vectorNorm([]float32{1, 2, 3, 4, 5}),
			vecB:      []float32{1, 2, 3},
			normB:     vectorNorm([]float32{1, 2, 3}),
			wantSim:   float32(14.0) / (vectorNorm([]float32{1, 2, 3, 4, 5}) * vectorNorm([]float32{1, 2, 3})),
			tolerance: 0.0001,
		},
		{
			name:      "high dimensional similar",
			vecA:      []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8},
			normA:     vectorNorm([]float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8}),
			vecB:      []float32{0.15, 0.25, 0.35, 0.45, 0.55, 0.65, 0.75, 0.85},
			normB:     vectorNorm([]float32{0.15, 0.25, 0.35, 0.45, 0.55, 0.65, 0.75, 0.85}),
			wantSim:   0.999, // Very similar
			tolerance: 0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sim := cosineSim(tt.vecA, tt.normA, tt.vecB, tt.normB)

			diff := math.Abs(float64(sim - tt.wantSim))
			if diff > float64(tt.tolerance) {
				t.Errorf("cosineSim() = %v, want %v (diff: %v, tolerance: %v)", sim, tt.wantSim, diff, tt.tolerance)
			}
		})
	}
}

func TestSemanticRecord_Serialization(t *testing.T) {
	tests := []struct {
		name   string
		record SemanticRecord
	}{
		{
			name: "full record",
			record: SemanticRecord{
				TS:      time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Role:    "user",
				Text:    "Test message",
				Vec:     []float32{0.1, 0.2, 0.3},
				Norm:    0.374,
				Source:  "chat",
				Session: "session-123",
				Repo:    "org/repo",
			},
		},
		{
			name: "minimal record",
			record: SemanticRecord{
				Text: "Minimal",
				Vec:  []float32{1.0},
				Norm: 1.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.record)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			var decoded SemanticRecord
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			if decoded.Text != tt.record.Text {
				t.Errorf("Text mismatch: got %q, want %q", decoded.Text, tt.record.Text)
			}
			if len(decoded.Vec) != len(tt.record.Vec) {
				t.Errorf("Vec length mismatch: got %d, want %d", len(decoded.Vec), len(tt.record.Vec))
			}
			if decoded.Norm != tt.record.Norm {
				t.Errorf("Norm mismatch: got %v, want %v", decoded.Norm, tt.record.Norm)
			}
		})
	}
}

func TestSemanticStore_ReadSemanticTail(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	namespace := "tail-test-ns"

	// Append multiple records
	records := make([]SemanticRecord, 10)
	for i := 0; i < 10; i++ {
		records[i] = SemanticRecord{
			TS:   time.Now().Add(time.Duration(i) * time.Minute),
			Text: "Message " + string(rune('A'+i)),
			Vec:  []float32{float32(i), float32(i + 1), float32(i + 2)},
		}
	}

	if err := store.AppendSemantic(namespace, records); err != nil {
		t.Fatalf("AppendSemantic() error = %v", err)
	}

	// Read tail with limit
	tail, err := store.ReadSemanticTail(namespace, 5)
	if err != nil {
		t.Fatalf("ReadSemanticTail() error = %v", err)
	}

	if len(tail) != 5 {
		t.Errorf("ReadSemanticTail() returned %d records, want 5", len(tail))
	}

	// Verify order (should be chronological, last 5 records)
	if len(tail) > 0 {
		// First in tail should be 6th record (index 5)
		if !strings.Contains(tail[0].Text, "Message F") {
			t.Errorf("First record = %q, expected Message F", tail[0].Text)
		}
	}
}

func TestSemanticStore_ListNamespaces(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	namespaces := []string{"ns-alpha", "ns-beta", "ns-gamma"}
	for _, ns := range namespaces {
		records := []SemanticRecord{
			{Text: "Record in " + ns, Vec: []float32{0.1, 0.2, 0.3}},
		}
		if err := store.AppendSemantic(ns, records); err != nil {
			t.Fatalf("AppendSemantic() error = %v", err)
		}
	}

	// List all namespaces
	list, err := store.ListSemanticNamespaces(0)
	if err != nil {
		t.Fatalf("ListSemanticNamespaces() error = %v", err)
	}

	if len(list) != len(namespaces) {
		t.Errorf("ListSemanticNamespaces() returned %d, want %d", len(list), len(namespaces))
	}

	// List with limit
	limited, err := store.ListSemanticNamespaces(2)
	if err != nil {
		t.Fatalf("ListSemanticNamespaces() error = %v", err)
	}

	if len(limited) != 2 {
		t.Errorf("ListSemanticNamespaces(2) returned %d, want 2", len(limited))
	}
}

func TestTokenizeSemanticQuery(t *testing.T) {
	tests := []struct {
		name   string
		query  string
		max    int
		want   []string
		wantN  int
	}{
		{
			name:  "basic tokenization",
			query: "Hello World",
			max:   10,
			want:  []string{"hello", "world"},
			wantN: 2,
		},
		{
			name:  "removes stop words",
			query: "the quick brown fox and the lazy dog",
			max:   10,
			want:  []string{"quick", "brown", "fox", "lazy", "dog"},
			wantN: 5,
		},
		{
			name:  "respects max limit",
			query: "one two three four five six seven",
			max:   3,
			want:  []string{"one", "two", "three"},
			wantN: 3,
		},
		{
			name:  "removes short words",
			query: "I am a test is it ok",
			max:   10,
			want:  []string{"test"},
			wantN: 1,
		},
		{
			name:  "handles special characters",
			query: "hello-world test_case foo.bar",
			max:   10,
			want:  []string{"hello-world", "test_case", "foo", "bar"},
			wantN: 4,
		},
		{
			name:  "empty query",
			query: "",
			max:   10,
			want:  nil,
			wantN: 0,
		},
		{
			name:  "only stop words",
			query: "the and for with",
			max:   10,
			want:  nil,
			wantN: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tokenizeSemanticQuery(tt.query, tt.max)

			if len(tokens) != tt.wantN {
				t.Errorf("tokenizeSemanticQuery() returned %d tokens, want %d", len(tokens), tt.wantN)
			}

			if tt.want != nil {
				for i, want := range tt.want {
					if i >= len(tokens) {
						t.Errorf("Missing token at index %d: want %q", i, want)
						continue
					}
					if tokens[i] != want {
						t.Errorf("Token %d = %q, want %q", i, tokens[i], want)
					}
				}
			}
		})
	}
}

func TestSemanticTokenOverlap(t *testing.T) {
	tests := []struct {
		name   string
		tokens []string
		text   string
		want   int
	}{
		{
			name:   "all tokens match",
			tokens: []string{"hello", "world"},
			text:   "Hello World Test",
			want:   2,
		},
		{
			name:   "some tokens match",
			tokens: []string{"hello", "world", "foo"},
			text:   "Hello bar baz",
			want:   1,
		},
		{
			name:   "no tokens match",
			tokens: []string{"foo", "bar"},
			text:   "Hello World",
			want:   0,
		},
		{
			name:   "empty tokens",
			tokens: []string{},
			text:   "Hello World",
			want:   0,
		},
		{
			name:   "empty text",
			tokens: []string{"hello"},
			text:   "",
			want:   0,
		},
		{
			name:   "case insensitive",
			tokens: []string{"hello"},
			text:   "HELLO World",
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overlap := semanticTokenOverlap(tt.tokens, tt.text)
			if overlap != tt.want {
				t.Errorf("semanticTokenOverlap() = %d, want %d", overlap, tt.want)
			}
		})
	}
}

func TestSemanticRecencyScore(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		ts         time.Time
		windowDays int
		wantMin    float32
		wantMax    float32
	}{
		{
			name:       "current time",
			ts:         now,
			windowDays: 30,
			wantMin:    0.99,
			wantMax:    1.0,
		},
		{
			name:       "half window age",
			ts:         now.Add(-15 * 24 * time.Hour),
			windowDays: 30,
			wantMin:    0.4,
			wantMax:    0.6,
		},
		{
			name:       "at window edge",
			ts:         now.Add(-30 * 24 * time.Hour),
			windowDays: 30,
			wantMin:    0.0,
			wantMax:    0.01,
		},
		{
			name:       "beyond window",
			ts:         now.Add(-60 * 24 * time.Hour),
			windowDays: 30,
			wantMin:    0.0,
			wantMax:    0.0,
		},
		{
			name:       "zero time",
			ts:         time.Time{},
			windowDays: 30,
			wantMin:    0.0,
			wantMax:    0.0,
		},
		{
			name:       "zero window",
			ts:         now,
			windowDays: 0,
			wantMin:    0.0,
			wantMax:    0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := semanticRecencyScore(now, tt.ts, tt.windowDays)
			if score < tt.wantMin || score > tt.wantMax {
				t.Errorf("semanticRecencyScore() = %v, want between %v and %v", score, tt.wantMin, tt.wantMax)
			}
		})
	}
}
