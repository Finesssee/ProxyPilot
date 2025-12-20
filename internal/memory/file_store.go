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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type FileStore struct {
	BaseDir   string
	cacheMu   sync.Mutex
	cache     map[string]searchCacheEntry
	cacheKeys []string
}

type searchCacheEntry struct {
	ts       time.Time
	snippets []string
	maxChars int
	maxSnips int
}

func NewFileStore(baseDir string) *FileStore {
	return &FileStore{
		BaseDir: baseDir,
		cache:   make(map[string]searchCacheEntry, 64),
	}
}

var (
	reFilePath = regexp.MustCompile(`(?i)\b[a-z0-9_\-./\\]+?\.(go|js|ts|tsx|jsx|md|yaml|yml|json|ps1|sh|py|toml|txt)\b`)
	reCommand  = regexp.MustCompile(`(?im)^\s*(?:\$|%|>|#)?\s*(go|git|node|npm|pnpm|yarn|python|python3|pip|pip3|deno|cargo)\b[^\n\r]*`)
)

func (s *FileStore) Append(session string, events []Event) error {
	if s == nil || s.BaseDir == "" {
		return errors.New("memory store not configured")
	}
	if session == "" || len(events) == 0 {
		return nil
	}
	s.invalidateSearchCache(session)
	dir := s.sessionDir(session)
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

	cacheKey := s.searchCacheKey(session, query, maxChars, maxSnippets)
	if cached, ok := s.getSearchCache(cacheKey); ok {
		return cached, nil
	}

	dir := s.sessionDir(session)
	path := filepath.Join(dir, "events.jsonl")

	// Anchored summary is always the first snippet when present.
	summary := strings.TrimSpace(s.ReadSummary(session, 12_000))

	data, err := readTailBytes(path, 2*1024*1024)
	if err != nil {
		if os.IsNotExist(err) {
			if summary != "" {
				if len(summary) > maxChars {
					summary = summary[:maxChars] + "\n...[truncated]..."
				}
				return []string{summary}, nil
			}
			return nil, nil
		}
		return nil, err
	}
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) == 0 {
		if summary != "" {
			if len(summary) > maxChars {
				summary = summary[:maxChars] + "\n...[truncated]..."
			}
			return []string{summary}, nil
		}
		return nil, nil
	}

	tokens := queryTokens(query, 10)
	if len(tokens) == 0 {
		if summary != "" {
			if len(summary) > maxChars {
				summary = summary[:maxChars] + "\n...[truncated]..."
			}
			return []string{summary}, nil
		}
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
	chars := 0
	if summary != "" && len(out) < maxSnippets {
		snip := summary
		if len(snip) > 2000 {
			snip = snip[:2000] + "\n...[truncated]..."
		}
		if len(snip) <= maxChars {
			out = append(out, snip)
			chars += len(snip) + 4
		}
	}

	seen := make(map[string]struct{}, maxSnippets*2)
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
	if len(out) > 0 {
		s.setSearchCache(cacheKey, out, maxChars, maxSnippets)
	}
	return out, nil
}

func (s *FileStore) UpsertAnchoredSummary(session string, dropped []Event, pinned string, latestIntent string) error {
	if s == nil || s.BaseDir == "" {
		return errors.New("memory store not configured")
	}
	if session == "" {
		return nil
	}
	dir := s.sessionDir(session)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	pinned = strings.TrimSpace(RedactText(pinned))
	if pinned != "" {
		_ = s.WritePinned(session, pinned, 8000)
	}

	prev := s.readSmallTextFile(filepath.Join(dir, "summary.md"), 60_000)
	next := BuildAnchoredSummary(prev, dropped, latestIntent)
	if strings.TrimSpace(next) == "" {
		return nil
	}
	return s.WriteSummary(session, next, 14_000)
}

func BuildAnchoredSummary(prev string, dropped []Event, latestIntent string) string {
	var b strings.Builder
	prev = strings.TrimSpace(prev)
	if prev != "" {
		// Weight previous anchor by keeping it verbatim at the top.
		b.WriteString(prev)
		b.WriteString("\n\n")
	} else {
		b.WriteString("# Session Anchor Summary\n\n")
	}
	b.WriteString("## Updates\n")
	b.WriteString("- Updated: ")
	b.WriteString(time.Now().Format(time.RFC3339))
	b.WriteString("\n")

	lastUser := ""
	fileHits := make([]string, 0, 16)
	cmdHits := make([]string, 0, 16)
	seenFile := make(map[string]struct{}, 32)
	seenCmd := make(map[string]struct{}, 32)

	for i := range dropped {
		e := dropped[i]
		txt := strings.TrimSpace(e.Text)
		if txt == "" {
			continue
		}
		if strings.EqualFold(e.Role, "user") {
			lastUser = txt
		}
		for _, m := range reFilePath.FindAllString(txt, -1) {
			if _, ok := seenFile[m]; ok {
				continue
			}
			seenFile[m] = struct{}{}
			fileHits = append(fileHits, m)
			if len(fileHits) >= 12 {
				break
			}
		}
		for _, m := range reCommand.FindAllString(txt, -1) {
			m = strings.TrimSpace(m)
			if _, ok := seenCmd[m]; ok {
				continue
			}
			seenCmd[m] = struct{}{}
			cmdHits = append(cmdHits, m)
			if len(cmdHits) >= 8 {
				break
			}
		}
	}

	intent := strings.TrimSpace(latestIntent)
	if intent == "" {
		intent = lastUser
	}
	if intent != "" {
		if len(intent) > 1200 {
			intent = intent[:1200] + "\n...[truncated]..."
		}
		b.WriteString("- Latest user intent:\n\n")
		b.WriteString("```text\n")
		b.WriteString(intent)
		b.WriteString("\n```\n")
	}

	if len(fileHits) > 0 {
		b.WriteString("\n- Referenced files:\n")
		for _, f := range fileHits {
			b.WriteString("  - ")
			b.WriteString(f)
			b.WriteString("\n")
		}
	}
	if len(cmdHits) > 0 {
		b.WriteString("\n- Referenced commands:\n")
		for _, c := range cmdHits {
			b.WriteString("  - ")
			b.WriteString(c)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func (s *FileStore) WriteSummary(session string, summary string, maxChars int) error {
	if s == nil || s.BaseDir == "" {
		return errors.New("memory store not configured")
	}
	if session == "" {
		return nil
	}
	s.invalidateSearchCache(session)
	summary = strings.TrimSpace(RedactText(summary))
	if summary == "" {
		return nil
	}
	if maxChars <= 0 {
		maxChars = 14_000
	}
	if len(summary) > maxChars {
		summary = summary[len(summary)-maxChars:]
		summary = "\n...[truncated]...\n" + summary
	}
	dir := s.sessionDir(session)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "summary.md"), []byte(summary), 0o644)
}

func (s *FileStore) SetAnchorSummary(session string, summary string, maxChars int) error {
	if s == nil || s.BaseDir == "" {
		return errors.New("memory store not configured")
	}
	if session == "" {
		return nil
	}
	s.invalidateSearchCache(session)
	if err := s.WriteSummary(session, summary, maxChars); err != nil {
		return err
	}
	dir := s.sessionDir(session)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	pendingPath := filepath.Join(dir, "anchor_pending.md")
	_ = os.WriteFile(pendingPath, []byte(summary), 0o644)
	_ = s.appendAnchorEvent(dir, summary)
	return nil
}

func (s *FileStore) ReadPendingAnchor(session string, maxChars int) string {
	if s == nil || s.BaseDir == "" {
		return ""
	}
	if session == "" {
		return ""
	}
	if maxChars <= 0 {
		maxChars = 4000
	}
	dir := s.sessionDir(session)
	txt := strings.TrimSpace(s.readSmallTextFile(filepath.Join(dir, "anchor_pending.md"), int64(maxChars*2)))
	if txt == "" {
		return ""
	}
	txt = normalizeEscapedText(txt)
	if len(txt) > maxChars {
		txt = txt[:maxChars] + "\n...[truncated]..."
	}
	return txt
}

func (s *FileStore) IsSemanticDisabled(session string) bool {
	if s == nil || s.BaseDir == "" || session == "" {
		return false
	}
	path := filepath.Join(s.sessionDir(session), "semantic_disabled")
	_, err := os.Stat(path)
	return err == nil
}

func (s *FileStore) SetSemanticDisabled(session string, disabled bool) error {
	if s == nil || s.BaseDir == "" || session == "" {
		return nil
	}
	dir := s.sessionDir(session)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "semantic_disabled")
	if !disabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return os.WriteFile(path, []byte("true\n"), 0o644)
}

func (s *FileStore) ClearPendingAnchor(session string) error {
	if s == nil || s.BaseDir == "" {
		return errors.New("memory store not configured")
	}
	if session == "" {
		return nil
	}
	dir := s.sessionDir(session)
	path := filepath.Join(dir, "anchor_pending.md")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *FileStore) appendAnchorEvent(dir string, summary string) error {
	if strings.TrimSpace(summary) == "" {
		return nil
	}
	path := filepath.Join(dir, "anchors.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	payload := map[string]any{
		"ts":      time.Now().Format(time.RFC3339),
		"summary": summary,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	_, _ = f.Write(b)
	_, _ = f.WriteString("\n")
	return nil
}

func (s *FileStore) searchCacheKey(session string, query string, maxChars int, maxSnips int) string {
	q := strings.ToLower(strings.TrimSpace(query))
	return session + "|" + strconv.Itoa(maxChars) + "|" + strconv.Itoa(maxSnips) + "|" + q
}

func (s *FileStore) getSearchCache(key string) ([]string, bool) {
	if s == nil {
		return nil, false
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	entry, ok := s.cache[key]
	if !ok {
		return nil, false
	}
	if time.Since(entry.ts) > 20*time.Second {
		delete(s.cache, key)
		return nil, false
	}
	out := make([]string, len(entry.snippets))
	copy(out, entry.snippets)
	return out, true
}

func (s *FileStore) setSearchCache(key string, snippets []string, maxChars int, maxSnips int) {
	if s == nil {
		return
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if s.cache == nil {
		s.cache = make(map[string]searchCacheEntry, 64)
	}
	if len(s.cacheKeys) >= 64 {
		oldest := s.cacheKeys[0]
		s.cacheKeys = s.cacheKeys[1:]
		delete(s.cache, oldest)
	}
	s.cache[key] = searchCacheEntry{
		ts:       time.Now(),
		snippets: append([]string(nil), snippets...),
		maxChars: maxChars,
		maxSnips: maxSnips,
	}
	s.cacheKeys = append(s.cacheKeys, key)
}

func (s *FileStore) invalidateSearchCache(session string) {
	if s == nil || session == "" {
		return
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if s.cache == nil {
		return
	}
	prefix := session + "|"
	for k := range s.cache {
		if strings.HasPrefix(k, prefix) {
			delete(s.cache, k)
		}
	}
	if len(s.cacheKeys) > 0 {
		filtered := s.cacheKeys[:0]
		for _, k := range s.cacheKeys {
			if !strings.HasPrefix(k, prefix) {
				filtered = append(filtered, k)
			}
		}
		s.cacheKeys = filtered
	}
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

func (s *FileStore) sessionDir(session string) string {
	return filepath.Join(s.BaseDir, "sessions", sanitizeSessionKey(session))
}

func (s *FileStore) readSmallTextFile(path string, maxBytes int64) string {
	if maxBytes <= 0 {
		maxBytes = 16_000
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, maxBytes))
	if err != nil || len(b) == 0 {
		return ""
	}
	return string(b)
}

func normalizeEscapedText(s string) string {
	// Fix common “double escaped” artifacts from JSON stringification (PowerShell ConvertTo-Json, etc).
	// Example: "\\n" should become "\n", and "\\u2019" should become "’".
	//
	// Best-effort only: on invalid escape sequences, return original.
	if s == "" {
		return s
	}
	looksEscaped := strings.Contains(s, `\n`) || strings.Contains(s, `\u`) || strings.Contains(s, `\r`) || strings.Contains(s, `\t`)
	if !looksEscaped {
		return s
	}

	// If the text already has real newlines, escape them so Unquote can parse a single-line literal.
	if strings.Contains(s, "\n") || strings.Contains(s, "\r") {
		s = strings.ReplaceAll(s, "\r", `\r`)
		s = strings.ReplaceAll(s, "\n", `\n`)
	}

	quoted := `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	decoded, err := strconv.Unquote(quoted)
	if err != nil {
		return s
	}
	// Normalize “smart quotes” to plain ASCII to avoid mojibake in some Windows terminals.
	decoded = strings.NewReplacer(
		"’", "'",
		"‘", "'",
		"“", "\"",
		"”", "\"",
		"–", "-",
		"—", "-",
	).Replace(decoded)
	return decoded
}

func (s *FileStore) ReadTodo(session string, maxChars int) string {
	if s == nil || s.BaseDir == "" {
		return ""
	}
	if session == "" {
		return ""
	}
	if maxChars <= 0 {
		maxChars = 4000
	}
	dir := s.sessionDir(session)
	txt := strings.TrimSpace(s.readSmallTextFile(filepath.Join(dir, "todo.md"), int64(maxChars*2)))
	if txt == "" {
		return ""
	}
	txt = normalizeEscapedText(txt)
	if len(txt) > maxChars {
		txt = txt[:maxChars] + "\n...[truncated]..."
	}
	return txt
}

func (s *FileStore) WriteTodo(session string, todo string, maxChars int) error {
	if s == nil || s.BaseDir == "" {
		return errors.New("memory store not configured")
	}
	if session == "" {
		return nil
	}
	if maxChars <= 0 {
		maxChars = 8000
	}
	todo = strings.TrimSpace(RedactText(todo))
	if todo == "" {
		return nil
	}
	todo = normalizeEscapedText(todo)
	if len(todo) > maxChars {
		todo = todo[:maxChars] + "\n...[truncated]..."
	}
	dir := s.sessionDir(session)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "todo.md"), []byte(todo), 0o644)
}

func (s *FileStore) ReadPinned(session string, maxChars int) string {
	if s == nil || s.BaseDir == "" {
		return ""
	}
	if session == "" {
		return ""
	}
	if maxChars <= 0 {
		maxChars = 6000
	}
	dir := s.sessionDir(session)
	txt := strings.TrimSpace(s.readSmallTextFile(filepath.Join(dir, "pinned.md"), int64(maxChars*2)))
	if txt == "" {
		txt = strings.TrimSpace(s.readSmallTextFile(filepath.Join(dir, "pinned.txt"), int64(maxChars*2)))
	}
	if txt == "" {
		return ""
	}
	txt = normalizeEscapedText(txt)
	if len(txt) > maxChars {
		txt = txt[:maxChars] + "\n...[truncated]..."
	}
	return txt
}

func (s *FileStore) WritePinned(session string, pinned string, maxChars int) error {
	if s == nil || s.BaseDir == "" {
		return errors.New("memory store not configured")
	}
	if session == "" {
		return nil
	}
	if maxChars <= 0 {
		maxChars = 8000
	}
	pinned = strings.TrimSpace(RedactText(pinned))
	if pinned == "" {
		return nil
	}
	pinned = normalizeEscapedText(pinned)
	if len(pinned) > maxChars {
		pinned = pinned[:maxChars] + "\n...[truncated]..."
	}
	dir := s.sessionDir(session)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// Keep legacy pinned.txt for compatibility with earlier builds.
	_ = os.WriteFile(filepath.Join(dir, "pinned.txt"), []byte(pinned), 0o644)
	return os.WriteFile(filepath.Join(dir, "pinned.md"), []byte(pinned), 0o644)
}

func (s *FileStore) ReadSummary(session string, maxChars int) string {
	if s == nil || s.BaseDir == "" {
		return ""
	}
	if session == "" {
		return ""
	}
	if maxChars <= 0 {
		maxChars = 6000
	}
	dir := s.sessionDir(session)
	txt := strings.TrimSpace(s.readSmallTextFile(filepath.Join(dir, "summary.md"), int64(maxChars*2)))
	if txt == "" {
		return ""
	}
	txt = normalizeEscapedText(txt)
	if len(txt) > maxChars {
		txt = txt[:maxChars] + "\n...[truncated]..."
	}
	return txt
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
