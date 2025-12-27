// Package usage provides usage tracking and logging functionality for the CLI Proxy API server.
// It includes plugins for monitoring API usage, token consumption, and other metrics
// to help with observability and billing purposes.
package usage

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

var statisticsEnabled atomic.Bool

func init() {
	statisticsEnabled.Store(true)
	coreusage.RegisterPlugin(NewLoggerPlugin())
}

// LoggerPlugin collects in-memory request statistics for usage analysis.
// It implements coreusage.Plugin to receive usage records emitted by the runtime.
type LoggerPlugin struct {
	stats *RequestStatistics
}

// NewLoggerPlugin constructs a new logger plugin instance.
//
// Returns:
//   - *LoggerPlugin: A new logger plugin instance wired to the shared statistics store.
func NewLoggerPlugin() *LoggerPlugin { return &LoggerPlugin{stats: defaultRequestStatistics} }

// HandleUsage implements coreusage.Plugin.
// It updates the in-memory statistics store whenever a usage record is received.
//
// Parameters:
//   - ctx: The context for the usage record
//   - record: The usage record to aggregate
func (p *LoggerPlugin) HandleUsage(ctx context.Context, record coreusage.Record) {
	if !statisticsEnabled.Load() {
		return
	}
	if p == nil || p.stats == nil {
		return
	}
	p.stats.Record(ctx, record)
}

// SetStatisticsEnabled toggles whether in-memory statistics are recorded.
func SetStatisticsEnabled(enabled bool) { statisticsEnabled.Store(enabled) }

// StatisticsEnabled reports the current recording state.
func StatisticsEnabled() bool { return statisticsEnabled.Load() }

// RequestStatistics maintains aggregated request metrics in memory.
type RequestStatistics struct {
	mu sync.RWMutex

	totalRequests     int64
	successCount      int64
	failureCount      int64
	totalTokens       int64
	totalInputTokens  int64
	totalOutputTokens int64

	apis map[string]*apiStats

	requestsByDay     map[string]int64
	requestsByHour    map[int]int64
	tokensByDay       map[string]int64
	inputTokensByDay  map[string]int64
	outputTokensByDay map[string]int64
	tokensByHour      map[int]int64
}

// apiStats holds aggregated metrics for a single API key.
type apiStats struct {
	TotalRequests int64
	TotalTokens   int64
	Models        map[string]*modelStats
}

// modelStats holds aggregated metrics for a specific model within an API.
type modelStats struct {
	TotalRequests int64
	TotalTokens   int64
	Details       []RequestDetail
}

// RequestDetail stores the timestamp and token usage for a single request.
type RequestDetail struct {
	Timestamp time.Time  `json:"timestamp"`
	Source    string     `json:"source"`
	AuthIndex uint64     `json:"auth_index"`
	Tokens    TokenStats `json:"tokens"`
	Failed    bool       `json:"failed"`
}

// TokenStats captures the token usage breakdown for a request.
type TokenStats struct {
	InputTokens     int64 `json:"input_tokens"`
	OutputTokens    int64 `json:"output_tokens"`
	ReasoningTokens int64 `json:"reasoning_tokens"`
	CachedTokens    int64 `json:"cached_tokens"`
	TotalTokens     int64 `json:"total_tokens"`
}

// StatisticsSnapshot represents an immutable view of the aggregated metrics.
type StatisticsSnapshot struct {
	TotalRequests     int64 `json:"total_requests"`
	SuccessCount      int64 `json:"success_count"`
	FailureCount      int64 `json:"failure_count"`
	TotalTokens       int64 `json:"total_tokens"`
	TotalInputTokens  int64 `json:"total_input_tokens"`
	TotalOutputTokens int64 `json:"total_output_tokens"`

	APIs map[string]APISnapshot `json:"apis"`

	RequestsByDay     map[string]int64 `json:"requests_by_day"`
	RequestsByHour    map[string]int64 `json:"requests_by_hour"`
	TokensByDay       map[string]int64 `json:"tokens_by_day"`
	InputTokensByDay  map[string]int64 `json:"input_tokens_by_day"`
	OutputTokensByDay map[string]int64 `json:"output_tokens_by_day"`
	TokensByHour      map[string]int64 `json:"tokens_by_hour"`
}

// APISnapshot summarises metrics for a single API key.
type APISnapshot struct {
	TotalRequests int64                    `json:"total_requests"`
	TotalTokens   int64                    `json:"total_tokens"`
	Models        map[string]ModelSnapshot `json:"models"`
}

// ModelSnapshot summarises metrics for a specific model.
type ModelSnapshot struct {
	TotalRequests int64           `json:"total_requests"`
	TotalTokens   int64           `json:"total_tokens"`
	Details       []RequestDetail `json:"details"`
}

var defaultRequestStatistics = NewRequestStatistics()

// GetRequestStatistics returns the shared statistics store.
func GetRequestStatistics() *RequestStatistics { return defaultRequestStatistics }

// NewRequestStatistics constructs an empty statistics store.
func NewRequestStatistics() *RequestStatistics {
	return &RequestStatistics{
		apis:              make(map[string]*apiStats),
		requestsByDay:     make(map[string]int64),
		requestsByHour:    make(map[int]int64),
		tokensByDay:       make(map[string]int64),
		inputTokensByDay:  make(map[string]int64),
		outputTokensByDay: make(map[string]int64),
		tokensByHour:      make(map[int]int64),
	}
}

// Record ingests a new usage record and updates the aggregates.
func (s *RequestStatistics) Record(ctx context.Context, record coreusage.Record) {
	if s == nil {
		return
	}
	if !statisticsEnabled.Load() {
		return
	}
	timestamp := record.RequestedAt
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	detail := normaliseDetail(record.Detail)
	totalTokens := detail.TotalTokens
	inputTokens := detail.InputTokens
	outputTokens := detail.OutputTokens
	statsKey := record.APIKey
	if statsKey == "" {
		statsKey = resolveAPIIdentifier(ctx, record)
	}
	failed := record.Failed
	if !failed {
		failed = !resolveSuccess(ctx)
	}
	success := !failed
	modelName := record.Model
	if modelName == "" {
		modelName = "unknown"
	}
	dayKey := timestamp.Format("2006-01-02")
	hourKey := timestamp.Hour()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.totalRequests++
	if success {
		s.successCount++
	} else {
		s.failureCount++
	}
	s.totalTokens += totalTokens
	s.totalInputTokens += inputTokens
	s.totalOutputTokens += outputTokens

	stats, ok := s.apis[statsKey]
	if !ok {
		stats = &apiStats{Models: make(map[string]*modelStats)}
		s.apis[statsKey] = stats
	}
	s.updateAPIStats(stats, modelName, RequestDetail{
		Timestamp: timestamp,
		Source:    record.Source,
		AuthIndex: record.AuthIndex,
		Tokens:    detail,
		Failed:    failed,
	})

	s.requestsByDay[dayKey]++
	s.requestsByHour[hourKey]++
	s.tokensByDay[dayKey] += totalTokens
	s.inputTokensByDay[dayKey] += inputTokens
	s.outputTokensByDay[dayKey] += outputTokens
	s.tokensByHour[hourKey] += totalTokens
}

func (s *RequestStatistics) updateAPIStats(stats *apiStats, model string, detail RequestDetail) {
	stats.TotalRequests++
	stats.TotalTokens += detail.Tokens.TotalTokens
	modelStatsValue, ok := stats.Models[model]
	if !ok {
		modelStatsValue = &modelStats{}
		stats.Models[model] = modelStatsValue
	}
	modelStatsValue.TotalRequests++
	modelStatsValue.TotalTokens += detail.Tokens.TotalTokens
	modelStatsValue.Details = append(modelStatsValue.Details, detail)
}

// Snapshot returns a copy of the aggregated metrics for external consumption.
func (s *RequestStatistics) Snapshot() StatisticsSnapshot {
	result := StatisticsSnapshot{}
	if s == nil {
		return result
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result.TotalRequests = s.totalRequests
	result.SuccessCount = s.successCount
	result.FailureCount = s.failureCount
	result.TotalTokens = s.totalTokens
	result.TotalInputTokens = s.totalInputTokens
	result.TotalOutputTokens = s.totalOutputTokens

	result.APIs = make(map[string]APISnapshot, len(s.apis))
	for apiName, stats := range s.apis {
		apiSnapshot := APISnapshot{
			TotalRequests: stats.TotalRequests,
			TotalTokens:   stats.TotalTokens,
			Models:        make(map[string]ModelSnapshot, len(stats.Models)),
		}
		for modelName, modelStatsValue := range stats.Models {
			requestDetails := make([]RequestDetail, len(modelStatsValue.Details))
			copy(requestDetails, modelStatsValue.Details)
			apiSnapshot.Models[modelName] = ModelSnapshot{
				TotalRequests: modelStatsValue.TotalRequests,
				TotalTokens:   modelStatsValue.TotalTokens,
				Details:       requestDetails,
			}
		}
		result.APIs[apiName] = apiSnapshot
	}

	result.RequestsByDay = make(map[string]int64, len(s.requestsByDay))
	for k, v := range s.requestsByDay {
		result.RequestsByDay[k] = v
	}

	result.RequestsByHour = make(map[string]int64, len(s.requestsByHour))
	for hour, v := range s.requestsByHour {
		key := formatHour(hour)
		result.RequestsByHour[key] = v
	}

	result.TokensByDay = make(map[string]int64, len(s.tokensByDay))
	for k, v := range s.tokensByDay {
		result.TokensByDay[k] = v
	}

	result.InputTokensByDay = make(map[string]int64, len(s.inputTokensByDay))
	for k, v := range s.inputTokensByDay {
		result.InputTokensByDay[k] = v
	}

	result.OutputTokensByDay = make(map[string]int64, len(s.outputTokensByDay))
	for k, v := range s.outputTokensByDay {
		result.OutputTokensByDay[k] = v
	}

	result.TokensByHour = make(map[string]int64, len(s.tokensByHour))
	for hour, v := range s.tokensByHour {
		key := formatHour(hour)
		result.TokensByHour[key] = v
	}

	return result
}

func resolveAPIIdentifier(ctx context.Context, record coreusage.Record) string {
	if ctx != nil {
		if ginCtx, ok := ctx.Value("gin").(*gin.Context); ok && ginCtx != nil {
			path := ginCtx.FullPath()
			if path == "" && ginCtx.Request != nil {
				path = ginCtx.Request.URL.Path
			}
			method := ""
			if ginCtx.Request != nil {
				method = ginCtx.Request.Method
			}
			if path != "" {
				if method != "" {
					return method + " " + path
				}
				return path
			}
		}
	}
	if record.Provider != "" {
		return record.Provider
	}
	return "unknown"
}

func resolveSuccess(ctx context.Context) bool {
	if ctx == nil {
		return true
	}
	ginCtx, ok := ctx.Value("gin").(*gin.Context)
	if !ok || ginCtx == nil {
		return true
	}
	status := ginCtx.Writer.Status()
	if status == 0 {
		return true
	}
	return status < httpStatusBadRequest
}

const httpStatusBadRequest = 400

func normaliseDetail(detail coreusage.Detail) TokenStats {
	tokens := TokenStats{
		InputTokens:     detail.InputTokens,
		OutputTokens:    detail.OutputTokens,
		ReasoningTokens: detail.ReasoningTokens,
		CachedTokens:    detail.CachedTokens,
		TotalTokens:     detail.TotalTokens,
	}
	if tokens.TotalTokens == 0 {
		tokens.TotalTokens = detail.InputTokens + detail.OutputTokens + detail.ReasoningTokens
	}
	if tokens.TotalTokens == 0 {
		tokens.TotalTokens = detail.InputTokens + detail.OutputTokens + detail.ReasoningTokens + detail.CachedTokens
	}
	return tokens
}

func formatHour(hour int) string {
	if hour < 0 {
		hour = 0
	}
	hour = hour % 24
	return fmt.Sprintf("%02d", hour)
}

// ComputeUsageStats converts a StatisticsSnapshot into a structured UsageStats object.
func ComputeUsageStats(snapshot StatisticsSnapshot) interfaces.UsageStats {
	stats := interfaces.UsageStats{
		TotalRequests:      snapshot.TotalRequests,
		SuccessCount:       snapshot.SuccessCount,
		FailureCount:       snapshot.FailureCount,
		TotalInputTokens:   snapshot.TotalInputTokens,
		TotalOutputTokens:  snapshot.TotalOutputTokens,
		EstimatedCostSaved: 0,
		ActualCost:         0,
		DirectAPICost:      0,
		Savings:            0,
		SavingsPercent:     0,
		ByModel:            make(map[string]int64),
		ByProvider:         make(map[string]int64),
		CostByModel:        make(map[string]float64),
		CostByProvider:     make(map[string]float64),
		Daily:              make([]interfaces.DailyUsage, 0),
	}

	var totalProxyCost float64
	var totalDirectCost float64

	for apiName, apiSnapshot := range snapshot.APIs {
		// Try to identify provider from apiName
		provider := apiName
		// If apiName is "METHOD PATH", it's not a provider name.
		if strings.Contains(apiName, " ") {
			provider = "unknown"
		}

		for modelName, modelSnapshot := range apiSnapshot.Models {
			stats.ByModel[modelName] += modelSnapshot.TotalRequests

			// If provider is unknown, try to infer from model name
			actualProvider := provider
			if actualProvider == "unknown" {
				actualProvider = inferProviderFromModel(modelName)
			}
			stats.ByProvider[actualProvider] += modelSnapshot.TotalRequests

			for _, detail := range modelSnapshot.Details {
				proxyCost, directCost := estimateCostWithSavings(
					actualProvider,
					modelName,
					detail.Tokens.InputTokens,
					detail.Tokens.OutputTokens,
					detail.Tokens.CachedTokens,
				)
				totalProxyCost += proxyCost
				totalDirectCost += directCost
				stats.CostByModel[modelName] += proxyCost
				stats.CostByProvider[actualProvider] += proxyCost
			}
		}
	}

	stats.ActualCost = totalProxyCost
	stats.DirectAPICost = totalDirectCost
	stats.Savings = totalDirectCost - totalProxyCost
	if totalDirectCost > 0 {
		stats.SavingsPercent = (stats.Savings / totalDirectCost) * 100
	}
	// Keep backward compatibility
	stats.EstimatedCostSaved = totalProxyCost

	// Last 7 days
	now := time.Now()
	for i := 6; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		daily := interfaces.DailyUsage{
			Date:         date,
			Requests:     snapshot.RequestsByDay[date],
			Tokens:       snapshot.TokensByDay[date],
			InputTokens:  snapshot.InputTokensByDay[date],
			OutputTokens: snapshot.OutputTokensByDay[date],
		}
		stats.Daily = append(stats.Daily, daily)
	}

	return stats
}

func inferProviderFromModel(model string) string {
	m := strings.ToLower(model)
	if strings.Contains(m, "claude") {
		return "anthropic"
	}
	if strings.Contains(m, "gpt") {
		return "openai"
	}
	if strings.Contains(m, "gemini") {
		return "google"
	}
	return "unknown"
}

func estimateCost(provider string, model string, inputTokens, outputTokens int64) float64 {
	// Try model-specific pricing first
	proxyCost, _, found := EstimateModelCost(model, inputTokens, outputTokens, 0)
	if found {
		return proxyCost
	}
	// Fallback to provider-based estimation
	return FallbackEstimateCost(provider, model, inputTokens, outputTokens)
}

// estimateCostWithSavings calculates both proxy cost and direct API cost.
// Returns (proxyCost, directCost).
func estimateCostWithSavings(provider string, model string, inputTokens, outputTokens, cachedTokens int64) (float64, float64) {
	proxyCost, directCost, found := EstimateModelCost(model, inputTokens, outputTokens, cachedTokens)
	if found {
		return proxyCost, directCost
	}
	// Fallback - same cost for both
	fallback := FallbackEstimateCost(provider, model, inputTokens, outputTokens)
	return fallback, fallback
}
