// Package middleware provides HTTP middleware components for the CLI Proxy API server.
// This file implements persistent request history storage with file-based persistence.
package middleware

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
)

const (
	maxHistorySize   = 500 // Maximum number of entries to persist
	historyFileName  = "request-history.json"
	autoSaveInterval = 30 * time.Second
)

// RequestHistory manages persistent request history storage.
type RequestHistory struct {
	mu       sync.RWMutex
	entries  []interfaces.RequestLogEntry
	filePath string
	dirty    bool
	stopSave chan struct{}
	saveWg   sync.WaitGroup
}

// RequestHistorySnapshot represents the persisted state of request history.
type RequestHistorySnapshot struct {
	Version        int                          `json:"version"`
	UpdatedAt      time.Time                    `json:"updated_at"`
	TotalTokensIn  int64                        `json:"total_tokens_in"`
	TotalTokensOut int64                        `json:"total_tokens_out"`
	TotalCostUSD   float64                      `json:"total_cost_usd"`
	Entries        []interfaces.RequestLogEntry `json:"entries"`
}

// RequestHistoryStats provides aggregated statistics from history.
type RequestHistoryStats struct {
	TotalRequests  int64              `json:"total_requests"`
	SuccessCount   int64              `json:"success_count"`
	FailureCount   int64              `json:"failure_count"`
	TotalTokensIn  int64              `json:"total_tokens_in"`
	TotalTokensOut int64              `json:"total_tokens_out"`
	TotalCostUSD   float64            `json:"total_cost_usd"`
	DirectAPICost  float64            `json:"direct_api_cost"`
	Savings        float64            `json:"savings"`
	SavingsPercent float64            `json:"savings_percent"`
	ByModel        map[string]int64   `json:"by_model"`
	ByProvider     map[string]int64   `json:"by_provider"`
	CostByModel    map[string]float64 `json:"cost_by_model"`
}

// RequestHistoryFilter defines filtering options for history queries.
type RequestHistoryFilter struct {
	StartDate  *time.Time `json:"start_date,omitempty"`
	EndDate    *time.Time `json:"end_date,omitempty"`
	Model      string     `json:"model,omitempty"`
	Provider   string     `json:"provider,omitempty"`
	StatusMin  int        `json:"status_min,omitempty"`
	StatusMax  int        `json:"status_max,omitempty"`
	ErrorsOnly bool       `json:"errors_only,omitempty"`
	Limit      int        `json:"limit,omitempty"`
	Offset     int        `json:"offset,omitempty"`
}

var (
	globalHistory         *RequestHistory
	globalHistoryOnce     sync.Once
	historyEnabled        atomic.Bool
	historySamplePermille atomic.Uint32
	historyRandSeed       atomic.Uint64
)

func init() {
	historySamplePermille.Store(1000)
	historyRandSeed.Store(uint64(time.Now().UnixNano()))
}

// SetRequestHistoryEnabled toggles request history persistence.
func SetRequestHistoryEnabled(enabled bool) {
	historyEnabled.Store(enabled)
}

// SetRequestHistorySampleRate sets the sampling rate (0.0-1.0).
func SetRequestHistorySampleRate(rate float64) {
	if rate < 0 {
		rate = 1
	}
	if rate > 1 {
		rate = 1
	}
	historySamplePermille.Store(uint32(rate * 1000))
}

// IsRequestHistoryEnabled returns whether request history is enabled.
func IsRequestHistoryEnabled() bool {
	return historyEnabled.Load()
}

// ShouldSampleRequestHistory returns whether the current request should be sampled.
func ShouldSampleRequestHistory() bool {
	rate := historySamplePermille.Load()
	if rate >= 1000 {
		return true
	}
	if rate == 0 {
		return false
	}
	return fastRandPermille() < rate
}

func fastRandPermille() uint32 {
	x := historyRandSeed.Add(0x9e3779b97f4a7c15)
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return uint32(x % 1000)
}

// GetRequestHistory returns the global request history instance.
func GetRequestHistory() *RequestHistory {
	if !IsRequestHistoryEnabled() {
		return nil
	}
	globalHistoryOnce.Do(func() {
		globalHistory = newRequestHistory()
	})
	return globalHistory
}

// newRequestHistory creates a new RequestHistory instance.
func newRequestHistory() *RequestHistory {
	homeDir, _ := os.UserHomeDir()
	filePath := filepath.Join(homeDir, ".cli-proxy-api", historyFileName)

	h := &RequestHistory{
		entries:  make([]interfaces.RequestLogEntry, 0),
		filePath: filePath,
		stopSave: make(chan struct{}),
	}

	// Load existing history
	h.load()

	// Start auto-save goroutine
	h.saveWg.Add(1)
	go h.autoSave()

	return h
}

// AddEntry adds a new request log entry to history.
func (h *RequestHistory) AddEntry(entry interfaces.RequestLogEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.entries = append(h.entries, entry)

	// Trim if exceeds max size
	if len(h.entries) > maxHistorySize {
		h.entries = h.entries[len(h.entries)-maxHistorySize:]
	}

	h.dirty = true
}

// GetEntries returns all entries, optionally filtered.
func (h *RequestHistory) GetEntries(filter *RequestHistoryFilter) []interfaces.RequestLogEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if filter == nil {
		result := make([]interfaces.RequestLogEntry, len(h.entries))
		copy(result, h.entries)
		// Sort by timestamp descending (newest first)
		sort.Slice(result, func(i, j int) bool {
			return result[i].Timestamp.After(result[j].Timestamp)
		})
		return result
	}

	var filtered []interfaces.RequestLogEntry
	for _, entry := range h.entries {
		if h.matchesFilter(entry, filter) {
			filtered = append(filtered, entry)
		}
	}

	// Sort by timestamp descending
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	// Apply pagination
	if filter.Offset > 0 && filter.Offset < len(filtered) {
		filtered = filtered[filter.Offset:]
	} else if filter.Offset >= len(filtered) {
		return []interfaces.RequestLogEntry{}
	}

	if filter.Limit > 0 && filter.Limit < len(filtered) {
		filtered = filtered[:filter.Limit]
	}

	return filtered
}

// GetStats returns aggregated statistics from history.
func (h *RequestHistory) GetStats() RequestHistoryStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := RequestHistoryStats{
		ByModel:     make(map[string]int64),
		ByProvider:  make(map[string]int64),
		CostByModel: make(map[string]float64),
	}

	for _, entry := range h.entries {
		stats.TotalRequests++

		if entry.Status >= 200 && entry.Status < 300 {
			stats.SuccessCount++
		} else {
			stats.FailureCount++
		}

		stats.TotalTokensIn += int64(entry.InputTokens)
		stats.TotalTokensOut += int64(entry.OutputTokens)

		if entry.Model != "" {
			stats.ByModel[entry.Model]++
		}
		if entry.Provider != "" {
			stats.ByProvider[entry.Provider]++
		}

		// Calculate cost
		proxyCost, directCost, found := usage.EstimateModelCost(
			entry.Model,
			int64(entry.InputTokens),
			int64(entry.OutputTokens),
			0,
		)
		if found {
			stats.TotalCostUSD += proxyCost
			stats.DirectAPICost += directCost
			stats.CostByModel[entry.Model] += proxyCost
		}
	}

	stats.Savings = stats.DirectAPICost - stats.TotalCostUSD
	if stats.DirectAPICost > 0 {
		stats.SavingsPercent = (stats.Savings / stats.DirectAPICost) * 100
	}

	return stats
}

// Clear removes all entries from history.
func (h *RequestHistory) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.entries = make([]interfaces.RequestLogEntry, 0)
	h.dirty = true
	h.saveNow()
}

// Export returns the full history for backup/export.
func (h *RequestHistory) Export() RequestHistorySnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := h.getStatsInternal()

	return RequestHistorySnapshot{
		Version:        1,
		UpdatedAt:      time.Now().UTC(),
		TotalTokensIn:  stats.TotalTokensIn,
		TotalTokensOut: stats.TotalTokensOut,
		TotalCostUSD:   stats.TotalCostUSD,
		Entries:        h.entries,
	}
}

// Import merges imported history with existing.
func (h *RequestHistory) Import(snapshot RequestHistorySnapshot) (added, skipped int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	existingIDs := make(map[string]struct{})
	for _, e := range h.entries {
		existingIDs[e.ID] = struct{}{}
	}

	for _, entry := range snapshot.Entries {
		if _, exists := existingIDs[entry.ID]; exists {
			skipped++
			continue
		}
		h.entries = append(h.entries, entry)
		added++
	}

	// Trim if exceeds max size
	if len(h.entries) > maxHistorySize {
		h.entries = h.entries[len(h.entries)-maxHistorySize:]
	}

	h.dirty = true
	return added, skipped
}

// Count returns the total number of entries in history.
func (h *RequestHistory) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.entries)
}

// Stop stops the auto-save goroutine and performs final save.
func (h *RequestHistory) Stop() {
	close(h.stopSave)
	h.saveWg.Wait()
	h.Save()
}

// Save persists history to disk.
func (h *RequestHistory) Save() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.saveNow()
}

func (h *RequestHistory) saveNow() error {
	if !h.dirty {
		return nil
	}

	snapshot := RequestHistorySnapshot{
		Version:   1,
		UpdatedAt: time.Now().UTC(),
		Entries:   h.entries,
	}

	// Calculate totals
	for _, e := range h.entries {
		snapshot.TotalTokensIn += int64(e.InputTokens)
		snapshot.TotalTokensOut += int64(e.OutputTokens)
		if cost, _, found := usage.EstimateModelCost(e.Model, int64(e.InputTokens), int64(e.OutputTokens), 0); found {
			snapshot.TotalCostUSD += cost
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(h.filePath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(h.filePath, data, 0644); err != nil {
		return err
	}

	h.dirty = false
	return nil
}

func (h *RequestHistory) load() error {
	data, err := os.ReadFile(h.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var snapshot RequestHistorySnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return err
	}

	h.entries = snapshot.Entries
	return nil
}

func (h *RequestHistory) autoSave() {
	defer h.saveWg.Done()

	ticker := time.NewTicker(autoSaveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.mu.Lock()
			if h.dirty {
				_ = h.saveNow()
			}
			h.mu.Unlock()
		case <-h.stopSave:
			return
		}
	}
}

func (h *RequestHistory) matchesFilter(entry interfaces.RequestLogEntry, filter *RequestHistoryFilter) bool {
	if filter.StartDate != nil && entry.Timestamp.Before(*filter.StartDate) {
		return false
	}
	if filter.EndDate != nil && entry.Timestamp.After(*filter.EndDate) {
		return false
	}
	if filter.Model != "" && entry.Model != filter.Model {
		return false
	}
	if filter.Provider != "" && entry.Provider != filter.Provider {
		return false
	}
	if filter.StatusMin > 0 && entry.Status < filter.StatusMin {
		return false
	}
	if filter.StatusMax > 0 && entry.Status > filter.StatusMax {
		return false
	}
	if filter.ErrorsOnly && entry.Error == "" {
		return false
	}
	return true
}

func (h *RequestHistory) getStatsInternal() RequestHistoryStats {
	stats := RequestHistoryStats{
		ByModel:     make(map[string]int64),
		ByProvider:  make(map[string]int64),
		CostByModel: make(map[string]float64),
	}

	for _, entry := range h.entries {
		stats.TotalRequests++
		stats.TotalTokensIn += int64(entry.InputTokens)
		stats.TotalTokensOut += int64(entry.OutputTokens)

		if cost, _, found := usage.EstimateModelCost(entry.Model, int64(entry.InputTokens), int64(entry.OutputTokens), 0); found {
			stats.TotalCostUSD += cost
		}
	}

	return stats
}
