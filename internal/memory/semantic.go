package memory

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type SemanticRecord struct {
	TS      time.Time `json:"ts"`
	Role    string    `json:"role,omitempty"`
	Text    string    `json:"text"`
	Vec     []float32 `json:"vec"`
	Norm    float32   `json:"norm"`
	Source  string    `json:"source,omitempty"`
	Session string    `json:"session,omitempty"`
	Repo    string    `json:"repo,omitempty"`
}

func (s *FileStore) AppendSemantic(namespace string, records []SemanticRecord) error {
	if s == nil || s.BaseDir == "" {
		return errors.New("memory store not configured")
	}
	if namespace == "" || len(records) == 0 {
		return nil
	}
	dir := s.semanticDir(namespace)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	_ = s.writeSemanticNamespace(dir, namespace)
	path := filepath.Join(dir, "items.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	for i := range records {
		r := records[i]
		if r.TS.IsZero() {
			r.TS = time.Now()
		}
		r.Text = strings.TrimSpace(RedactText(r.Text))
		if r.Text == "" || len(r.Vec) == 0 {
			continue
		}
		if r.Norm <= 0 {
			r.Norm = vectorNorm(r.Vec)
		}
		if r.Norm <= 0 {
			continue
		}
		b, err := json.Marshal(r)
		if err != nil {
			continue
		}
		_, _ = f.Write(b)
		_, _ = f.WriteString("\n")
	}
	return nil
}

func (s *FileStore) SearchSemantic(namespace string, query []float32, maxChars int, maxSnippets int) ([]string, error) {
	if s == nil || s.BaseDir == "" {
		return nil, errors.New("memory store not configured")
	}
	if namespace == "" || len(query) == 0 {
		return nil, nil
	}
	if maxChars <= 0 {
		maxChars = 3000
	}
	if maxSnippets <= 0 {
		maxSnippets = 4
	}

	dir := s.semanticDir(namespace)
	path := filepath.Join(dir, "items.jsonl")
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

	qn := vectorNorm(query)
	if qn <= 0 {
		return nil, nil
	}

	type scored struct {
		score float32
		text  string
		ts    time.Time
	}
	scoredSnips := make([]scored, 0, len(lines))
	for i := range lines {
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}
		var r SemanticRecord
		if err := json.Unmarshal(line, &r); err != nil {
			continue
		}
		if len(r.Vec) == 0 || r.Norm <= 0 {
			continue
		}
		score := cosineSim(query, qn, r.Vec, r.Norm)
		if score <= 0 {
			continue
		}
		txt := strings.TrimSpace(r.Text)
		if txt == "" {
			continue
		}
		scoredSnips = append(scoredSnips, scored{score: score, text: txt, ts: r.TS})
	}

	if len(scoredSnips) == 0 {
		return nil, nil
	}
	sort.Slice(scoredSnips, func(i, j int) bool { return scoredSnips[i].score > scoredSnips[j].score })

	out := make([]string, 0, maxSnippets)
	chars := 0
	seen := make(map[string]struct{}, maxSnippets*2)
	for _, s := range scoredSnips {
		if len(out) >= maxSnippets {
			break
		}
		key := s.text
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

func (s *FileStore) ReadSemanticTail(namespace string, limit int) ([]SemanticRecord, error) {
	if s == nil || s.BaseDir == "" {
		return nil, errors.New("memory store not configured")
	}
	if namespace == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	dir := s.semanticDir(namespace)
	path := filepath.Join(dir, "items.jsonl")
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

	out := make([]SemanticRecord, 0, limit)
	for i := len(lines) - 1; i >= 0; i-- {
		if len(out) >= limit {
			break
		}
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}
		var r SemanticRecord
		if err := json.Unmarshal(line, &r); err != nil {
			continue
		}
		if strings.TrimSpace(r.Text) == "" {
			continue
		}
		out = append(out, r)
	}
	// Reverse to chronological order
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (s *FileStore) semanticDir(namespace string) string {
	key := namespaceKey(namespace)
	return filepath.Join(s.BaseDir, "semantic", key)
}

func (s *FileStore) writeSemanticNamespace(dir string, namespace string) error {
	if strings.TrimSpace(namespace) == "" {
		return nil
	}
	path := filepath.Join(dir, "namespace.txt")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(namespace), 0o644)
}

func namespaceKey(namespace string) string {
	h := sha256Hash(namespace)
	return h[:16]
}

func sha256Hash(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func vectorNorm(vec []float32) float32 {
	var sum float64
	for i := range vec {
		v := float64(vec[i])
		sum += v * v
	}
	if sum <= 0 {
		return 0
	}
	return float32(math.Sqrt(sum))
}

func cosineSim(a []float32, aNorm float32, b []float32, bNorm float32) float32 {
	if aNorm <= 0 || bNorm <= 0 {
		return 0
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 0
	}
	var dot float64
	for i := 0; i < n; i++ {
		dot += float64(a[i]) * float64(b[i])
	}
	return float32(dot) / (aNorm * bNorm)
}
