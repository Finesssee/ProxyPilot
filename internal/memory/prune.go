package memory

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type SessionInfo struct {
	Key              string    `json:"key"`
	Path             string    `json:"-"`
	UpdatedAt        time.Time `json:"updated_at"`
	SizeBytes        int64     `json:"size_bytes"`
	EventsBytes      int64     `json:"events_bytes"`
	HasSummary       bool      `json:"has_summary"`
	HasTodo          bool      `json:"has_todo"`
	HasPinned        bool      `json:"has_pinned"`
	HasAnchorPending bool      `json:"has_anchor_pending"`
}

type AnchorEvent struct {
	TS      time.Time `json:"ts"`
	Summary string    `json:"summary"`
}

type PruneResult struct {
	SessionsRemoved           int   `json:"sessions_removed"`
	SessionsTrimmed           int   `json:"sessions_trimmed"`
	SemanticNamespacesRemoved int   `json:"semantic_namespaces_removed"`
	SemanticNamespacesTrimmed int   `json:"semantic_namespaces_trimmed"`
	BytesFreed                int64 `json:"bytes_freed"`
}

func (s *FileStore) ListSessions(limit int) ([]SessionInfo, error) {
	if s == nil || s.BaseDir == "" {
		return nil, errors.New("memory store not configured")
	}
	root := filepath.Join(s.BaseDir, "sessions")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []SessionInfo{}, nil
		}
		return nil, err
	}

	out := make([]SessionInfo, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		key := e.Name()
		dir := filepath.Join(root, key)
		info, err := s.getSessionInfoFromDir(key, dir)
		if err != nil {
			continue
		}
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

func (s *FileStore) GetSessionInfo(session string) (SessionInfo, error) {
	if s == nil || s.BaseDir == "" {
		return SessionInfo{}, errors.New("memory store not configured")
	}
	if session == "" {
		return SessionInfo{}, errors.New("session required")
	}
	dir := s.sessionDir(session)
	return s.getSessionInfoFromDir(session, dir)
}

func (s *FileStore) getSessionInfoFromDir(session string, dir string) (SessionInfo, error) {
	info := SessionInfo{Key: session, Path: dir}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return info, err
	}
	var latest time.Time
	var total int64
	var eventsBytes int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		p := filepath.Join(dir, name)
		fi, err := e.Info()
		if err != nil {
			continue
		}
		total += fi.Size()
		if fi.ModTime().After(latest) {
			latest = fi.ModTime()
		}
		switch name {
		case "summary.md":
			info.HasSummary = true
		case "todo.md":
			info.HasTodo = true
		case "pinned.md", "pinned.txt":
			info.HasPinned = true
		case "anchor_pending.md":
			info.HasAnchorPending = true
		case "events.jsonl":
			eventsBytes = fi.Size()
			_ = p
		}
	}
	info.UpdatedAt = latest
	info.SizeBytes = total
	info.EventsBytes = eventsBytes
	return info, nil
}

func (s *FileStore) ReadEventTail(session string, limit int) ([]Event, error) {
	if s == nil || s.BaseDir == "" {
		return nil, errors.New("memory store not configured")
	}
	if session == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	path := filepath.Join(s.sessionDir(session), "events.jsonl")
	data, err := readTailBytes(path, 2*1024*1024)
	if err != nil {
		if os.IsNotExist(err) {
			return []Event{}, nil
		}
		return nil, err
	}
	lines := bytes.Split(data, []byte("\n"))
	out := make([]Event, 0, limit)
	for i := len(lines) - 1; i >= 0; i-- {
		if len(out) >= limit {
			break
		}
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		if strings.TrimSpace(e.Text) == "" {
			continue
		}
		out = append(out, e)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (s *FileStore) ReadAnchorTail(session string, limit int) ([]AnchorEvent, error) {
	if s == nil || s.BaseDir == "" {
		return nil, errors.New("memory store not configured")
	}
	if session == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	path := filepath.Join(s.sessionDir(session), "anchors.jsonl")
	data, err := readTailBytes(path, 2*1024*1024)
	if err != nil {
		if os.IsNotExist(err) {
			return []AnchorEvent{}, nil
		}
		return nil, err
	}
	lines := bytes.Split(data, []byte("\n"))
	out := make([]AnchorEvent, 0, limit)
	for i := len(lines) - 1; i >= 0; i-- {
		if len(out) >= limit {
			break
		}
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}
		var raw struct {
			TS      string `json:"ts"`
			Summary string `json:"summary"`
		}
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}
		if strings.TrimSpace(raw.Summary) == "" {
			continue
		}
		var ts time.Time
		if raw.TS != "" {
			if parsed, err := time.Parse(time.RFC3339, raw.TS); err == nil {
				ts = parsed
			}
		}
		out = append(out, AnchorEvent{TS: ts, Summary: raw.Summary})
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (s *FileStore) PruneSessions(maxAgeDays int, maxSessions int, maxBytesPerSession int64) (PruneResult, error) {
	var res PruneResult
	if s == nil || s.BaseDir == "" {
		return res, errors.New("memory store not configured")
	}
	sessions, err := s.ListSessions(0)
	if err != nil {
		return res, err
	}
	if len(sessions) == 0 {
		return res, nil
	}
	now := time.Now()
	toRemove := make(map[string]struct{})
	if maxAgeDays > 0 {
		cutoff := now.Add(-time.Duration(maxAgeDays) * 24 * time.Hour)
		for _, info := range sessions {
			if !info.UpdatedAt.IsZero() && info.UpdatedAt.Before(cutoff) {
				toRemove[info.Key] = struct{}{}
			}
		}
	}
	if maxSessions > 0 && len(sessions) > maxSessions {
		for i := maxSessions; i < len(sessions); i++ {
			toRemove[sessions[i].Key] = struct{}{}
		}
	}

	remaining := make([]SessionInfo, 0, len(sessions))
	for _, info := range sessions {
		if _, ok := toRemove[info.Key]; ok {
			if err := os.RemoveAll(info.Path); err == nil {
				res.SessionsRemoved++
			}
			continue
		}
		remaining = append(remaining, info)
	}

	if maxBytesPerSession > 0 {
		for _, info := range remaining {
			eventsPath := filepath.Join(info.Path, "events.jsonl")
			if trimmed, freed := trimJSONLFile(eventsPath, maxBytesPerSession); trimmed {
				res.SessionsTrimmed++
				res.BytesFreed += freed
			}
		}
	}

	return res, nil
}

func (s *FileStore) PruneSemantic(maxAgeDays int, maxNamespaces int, maxBytesPerNamespace int64) (PruneResult, error) {
	var res PruneResult
	if s == nil || s.BaseDir == "" {
		return res, errors.New("memory store not configured")
	}
	semanticDir := filepath.Join(s.BaseDir, "semantic")
	entries, err := os.ReadDir(semanticDir)
	if err != nil {
		if os.IsNotExist(err) {
			return res, nil
		}
		return res, err
	}
	type nsInfo struct {
		key   string
		path  string
		mtime time.Time
	}
	namespaces := make([]nsInfo, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(semanticDir, e.Name())
		fi, err := os.Stat(filepath.Join(path, "items.jsonl"))
		if err != nil {
			continue
		}
		namespaces = append(namespaces, nsInfo{key: e.Name(), path: path, mtime: fi.ModTime()})
	}
	sort.Slice(namespaces, func(i, j int) bool {
		return namespaces[i].mtime.After(namespaces[j].mtime)
	})

	now := time.Now()
	toRemove := make(map[string]struct{})
	if maxAgeDays > 0 {
		cutoff := now.Add(-time.Duration(maxAgeDays) * 24 * time.Hour)
		for _, ns := range namespaces {
			if ns.mtime.Before(cutoff) {
				toRemove[ns.key] = struct{}{}
			}
		}
	}
	if maxNamespaces > 0 && len(namespaces) > maxNamespaces {
		for i := maxNamespaces; i < len(namespaces); i++ {
			toRemove[namespaces[i].key] = struct{}{}
		}
	}

	for _, ns := range namespaces {
		if _, ok := toRemove[ns.key]; ok {
			if err := os.RemoveAll(ns.path); err == nil {
				res.SemanticNamespacesRemoved++
			}
			continue
		}
		if maxBytesPerNamespace > 0 {
			itemsPath := filepath.Join(ns.path, "items.jsonl")
			if trimmed, freed := trimJSONLFile(itemsPath, maxBytesPerNamespace); trimmed {
				res.SemanticNamespacesTrimmed++
				res.BytesFreed += freed
			}
		}
	}
	return res, nil
}

func trimJSONLFile(path string, maxBytes int64) (bool, int64) {
	if maxBytes <= 0 {
		return false, 0
	}
	fi, err := os.Stat(path)
	if err != nil {
		return false, 0
	}
	if fi.Size() <= maxBytes {
		return false, 0
	}
	data, err := readTailBytes(path, maxBytes)
	if err != nil {
		return false, 0
	}
	if len(data) == 0 {
		return false, 0
	}
	// Ensure we start on a line boundary
	if i := bytes.IndexByte(data, '\n'); i >= 0 && i+1 < len(data) {
		data = data[i+1:]
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return false, 0
	}
	return true, fi.Size() - int64(len(data))
}

func ExportSessionZip(sessionDir string) ([]byte, error) {
	if strings.TrimSpace(sessionDir) == "" {
		return nil, errors.New("session directory required")
	}
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	err := filepath.WalkDir(sessionDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(sessionDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." || rel == "" {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, f); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
