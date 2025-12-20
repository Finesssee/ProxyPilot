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
	"strconv"
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

type SemanticNamespaceInfo struct {
	Key        string    `json:"key"`
	Namespace  string    `json:"namespace"`
	UpdatedAt  time.Time `json:"updated_at"`
	SizeBytes  int64     `json:"size_bytes"`
	ItemsBytes int64     `json:"items_bytes"`
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

	seen := make(map[string]struct{}, len(records))
	for i := range records {
		r := records[i]
		if r.TS.IsZero() {
			r.TS = time.Now()
		}
		r.Text = strings.TrimSpace(RedactText(r.Text))
		if r.Text == "" || len(r.Vec) == 0 {
			continue
		}
		if _, ok := seen[r.Text]; ok {
			continue
		}
		seen[r.Text] = struct{}{}
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
	return s.SearchSemanticWithText(namespace, query, "", maxChars, maxSnippets)
}

func (s *FileStore) SearchSemanticWithText(namespace string, query []float32, queryText string, maxChars int, maxSnippets int) ([]string, error) {
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

	tokens := []string{}
	if semanticRerankEnabled() && strings.TrimSpace(queryText) != "" {
		tokens = tokenizeSemanticQuery(queryText, 12)
	}
	now := time.Now()
	maxAgeDays := semanticMaxAgeDays()
	recencyWindow := semanticRecencyWindowDays(maxAgeDays)
	keywordBoost := semanticKeywordBoost()
	recencyBoost := semanticRecencyBoost()

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
		if maxAgeDays > 0 && !r.TS.IsZero() {
			if now.Sub(r.TS) > time.Duration(maxAgeDays)*24*time.Hour {
				continue
			}
		}
		score := cosineSim(query, qn, r.Vec, r.Norm)
		if score <= 0 {
			continue
		}
		if len(tokens) > 0 {
			overlap := semanticTokenOverlap(tokens, r.Text)
			if overlap > 0 {
				score *= 1 + keywordBoost*float32(overlap)/float32(len(tokens))
			}
		}
		if recencyBoost > 0 {
			ageBoost := semanticRecencyScore(now, r.TS, recencyWindow)
			if ageBoost > 0 {
				score *= 1 + recencyBoost*ageBoost
			}
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

func (s *FileStore) ListSemanticNamespaces(limit int) ([]SemanticNamespaceInfo, error) {
	if s == nil || s.BaseDir == "" {
		return nil, errors.New("memory store not configured")
	}
	root := filepath.Join(s.BaseDir, "semantic")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []SemanticNamespaceInfo{}, nil
		}
		return nil, err
	}
	out := make([]SemanticNamespaceInfo, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		itemsPath := filepath.Join(dir, "items.jsonl")
		fi, err := os.Stat(itemsPath)
		if err != nil {
			continue
		}
		ns := strings.TrimSpace(s.readSmallTextFile(filepath.Join(dir, "namespace.txt"), 2048))
		info := SemanticNamespaceInfo{
			Key:        e.Name(),
			Namespace:  ns,
			UpdatedAt:  fi.ModTime(),
			ItemsBytes: fi.Size(),
		}
		var total int64
		if entries2, err := os.ReadDir(dir); err == nil {
			for _, e2 := range entries2 {
				if e2.IsDir() {
					continue
				}
				if fi2, err := e2.Info(); err == nil {
					total += fi2.Size()
				}
			}
		}
		info.SizeBytes = total
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
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

func semanticMaxAgeDays() int {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SEMANTIC_MAX_AGE_DAYS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

func semanticRecencyWindowDays(maxAgeDays int) int {
	if maxAgeDays > 0 {
		return maxAgeDays
	}
	return 30
}

func semanticKeywordBoost() float32 {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SEMANTIC_KEYWORD_BOOST")); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil {
			return float32(f)
		}
	}
	return 0.25
}

func semanticRecencyBoost() float32 {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SEMANTIC_RECENCY_BOOST")); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil {
			return float32(f)
		}
	}
	return 0.15
}

func semanticRerankEnabled() bool {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SEMANTIC_RERANK")); v != "" {
		if strings.EqualFold(v, "0") || strings.EqualFold(v, "false") || strings.EqualFold(v, "off") || strings.EqualFold(v, "no") {
			return false
		}
	}
	return true
}

func tokenizeSemanticQuery(q string, max int) []string {
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
		"you": {}, "your": {}, "are": {}, "was": {}, "were": {}, "can": {}, "could": {}, "should": {}, "would": {}, "will": {},
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

func semanticTokenOverlap(tokens []string, text string) int {
	if len(tokens) == 0 || strings.TrimSpace(text) == "" {
		return 0
	}
	low := strings.ToLower(text)
	count := 0
	for _, t := range tokens {
		if t == "" {
			continue
		}
		if strings.Contains(low, t) {
			count++
		}
	}
	return count
}

func semanticRecencyScore(now time.Time, ts time.Time, windowDays int) float32 {
	if ts.IsZero() || windowDays <= 0 {
		return 0
	}
	age := now.Sub(ts)
	if age <= 0 {
		return 1
	}
	window := time.Duration(windowDays) * 24 * time.Hour
	if age >= window {
		return 0
	}
	return float32(1 - float64(age)/float64(window))
}
