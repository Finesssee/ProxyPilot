package memory

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type FileStore struct {
	BaseDir string
}

func NewFileStore(baseDir string) *FileStore {
	return &FileStore{BaseDir: baseDir}
}

func (s *FileStore) Append(session string, events []Event) error {
	if s == nil || s.BaseDir == "" {
		return errors.New("memory store not configured")
	}
	if session == "" || len(events) == 0 {
		return nil
	}
	dir := filepath.Join(s.BaseDir, "sessions", sanitizeSessionKey(session))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "events.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	w := bufio.NewWriterSize(f, 64*1024)
	for i := range events {
		e := events[i]
		if e.TS.IsZero() {
			e.TS = time.Now()
		}
		if e.Text != "" {
			e.Text = RedactText(e.Text)
		}
		b, err := json.Marshal(e)
		if err != nil {
			continue
		}
		_, _ = w.Write(b)
		_, _ = w.WriteString("\n")
	}
	return w.Flush()
}

func (s *FileStore) Search(session string, query string, maxChars int, maxSnippets int) ([]string, error) {
	if s == nil || s.BaseDir == "" {
		return nil, errors.New("memory store not configured")
	}
	if session == "" || strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if maxChars <= 0 {
		maxChars = 6000
	}
	if maxSnippets <= 0 {
		maxSnippets = 8
	}

	dir := filepath.Join(s.BaseDir, "sessions", sanitizeSessionKey(session))
	path := filepath.Join(dir, "events.jsonl")

	data, err := readTailBytes(path, 2*1024*1024)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) == 0 {
		return nil, nil
	}

	tokens := queryTokens(query, 10)
	if len(tokens) == 0 {
		return nil, nil
	}

	type scored struct {
		score int
		text  string
	}
	var scoredSnips []scored
	for i := range lines {
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		txt := strings.TrimSpace(e.Text)
		if txt == "" {
			continue
		}
		txtLower := strings.ToLower(txt)
		score := 0
		for _, t := range tokens {
			if strings.Contains(txtLower, t) {
				score += 3
			}
		}
		if score == 0 {
			continue
		}
		// Recency bonus: prefer newer lines (towards the end of the tail).
		score += i / 200
		scoredSnips = append(scoredSnips, scored{score: score, text: txt})
	}

	if len(scoredSnips) == 0 {
		return nil, nil
	}
	sort.Slice(scoredSnips, func(i, j int) bool { return scoredSnips[i].score > scoredSnips[j].score })

	out := make([]string, 0, maxSnippets)
	seen := make(map[string]struct{}, maxSnippets*2)
	chars := 0
	for _, s := range scoredSnips {
		if len(out) >= maxSnippets {
			break
		}
		h := sha256.Sum256([]byte(s.text))
		key := hex.EncodeToString(h[:8])
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		snip := s.text
		if len(snip) > 1200 {
			snip = snip[:1200] + "\n...[truncated]..."
		}
		if chars+len(snip) > maxChars {
			break
		}
		out = append(out, snip)
		chars += len(snip) + 4
	}
	return out, nil
}

func sanitizeSessionKey(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// Keep filesystem-safe characters.
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if len(out) > 120 {
		out = out[:120]
	}
	return out
}

func readTailBytes(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := st.Size()
	if size <= 0 {
		return nil, io.EOF
	}
	start := int64(0)
	if size > max {
		start = size - max
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(f)
}

func queryTokens(q string, max int) []string {
	q = strings.ToLower(q)
	q = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return ' '
	}, q)
	parts := strings.Fields(q)
	if len(parts) == 0 {
		return nil
	}
	stop := map[string]struct{}{
		"the": {}, "and": {}, "for": {}, "with": {}, "that": {}, "this": {}, "from": {}, "into": {}, "what": {}, "how": {},
		"you": {}, "your": {}, "are": {}, "was": {}, "were": {}, "can": {}, "could": {}, "should": {}, "would": {},
	}
	out := make([]string, 0, max)
	seen := make(map[string]struct{}, max*2)
	for _, p := range parts {
		if len(p) < 3 {
			continue
		}
		if _, ok := stop[p]; ok {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
		if len(out) >= max {
			break
		}
	}
	return out
}
