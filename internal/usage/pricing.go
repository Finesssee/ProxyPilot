// Package usage provides usage tracking and cost estimation.
package usage

import "strings"

// ModelPricing defines the cost per million tokens for a model.
type ModelPricing struct {
	InputPerMillion  float64 // Cost per 1M input tokens
	OutputPerMillion float64 // Cost per 1M output tokens
	CachedPerMillion float64 // Cost per 1M cached input tokens (usually discounted)
}

// PricingTable maps model name patterns to their pricing.
// Prices are in USD per million tokens (as of Jan 2026).
var PricingTable = map[string]ModelPricing{
	// Anthropic Claude models
	"claude-opus-4-5":   {15.00, 75.00, 1.50},
	"claude-opus-4":     {15.00, 75.00, 1.50},
	"claude-sonnet-4-5": {3.00, 15.00, 0.30},
	"claude-sonnet-4":   {3.00, 15.00, 0.30},
	"claude-haiku-4-5":  {0.80, 4.00, 0.08},
	"claude-3-5-sonnet": {3.00, 15.00, 0.30},
	"claude-3-5-haiku":  {0.80, 4.00, 0.08},
	"claude-3-opus":     {15.00, 75.00, 1.50},
	"claude-3-sonnet":   {3.00, 15.00, 0.30},
	"claude-3-haiku":    {0.25, 1.25, 0.025},

	// OpenAI GPT/Codex models
	"gpt-5.2":       {10.00, 30.00, 5.00},
	"gpt-5.2-codex": {10.00, 30.00, 5.00},
	"gpt-5-codex":   {8.00, 24.00, 4.00},
	"gpt-4o":        {2.50, 10.00, 1.25},
	"gpt-4o-mini":   {0.15, 0.60, 0.075},
	"gpt-4-turbo":   {10.00, 30.00, 5.00},
	"gpt-4":         {30.00, 60.00, 15.00},
	"gpt-3.5-turbo": {0.50, 1.50, 0.25},
	"o1":            {15.00, 60.00, 7.50},
	"o1-mini":       {3.00, 12.00, 1.50},
	"o1-preview":    {15.00, 60.00, 7.50},
	"o3-mini":       {1.10, 4.40, 0.55},

	// Google Gemini models
	"gemini-3-pro":        {1.25, 5.00, 0.3125},
	"gemini-3-flash":      {0.15, 0.60, 0.0375},
	"gemini-2.5-pro":      {1.25, 5.00, 0.3125},
	"gemini-2.5-flash":    {0.15, 0.60, 0.0375},
	"gemini-2.0-flash":    {0.10, 0.40, 0.025},
	"gemini-1.5-pro":      {1.25, 5.00, 0.3125},
	"gemini-1.5-flash":    {0.075, 0.30, 0.01875},
	"gemini-1.5-flash-8b": {0.0375, 0.15, 0.01},
	"gemini-1.0-pro":      {0.50, 1.50, 0.125},

	// Kiro (AWS CodeWhisperer) - subscription-based, estimate as Claude pricing
	"kiro-claude-sonnet": {3.00, 15.00, 0.30},
	"kiro-claude-haiku":  {0.80, 4.00, 0.08},
	"kiro":               {3.00, 15.00, 0.30},

	// Qwen models
	"qwen-turbo": {0.30, 0.60, 0.15},
	"qwen-plus":  {0.80, 2.00, 0.40},
	"qwen-max":   {2.40, 9.60, 1.20},
	"qwq-32b":    {0.15, 0.60, 0.075},

	// DeepSeek models
	"deepseek-chat":     {0.14, 0.28, 0.07},
	"deepseek-reasoner": {0.55, 2.19, 0.275},

	// MiniMax models
	"minimax-m2":   {0.50, 1.50, 0.25},
	"minimax-m2.1": {0.60, 1.80, 0.30},

	// Zhipu GLM models
	"glm-4.5": {1.00, 3.00, 0.50},
	"glm-4.6": {1.20, 3.60, 0.60},
	"glm-4.7": {1.50, 4.50, 0.75},
	"glm-4":   {1.00, 3.00, 0.50},

	// Free tier proxy (subscription-based)
	"codex":   {0.00, 0.00, 0.00},
	"copilot": {0.00, 0.00, 0.00},
}

// DirectAPIPricing stores the "official" direct API pricing for comparison.
// This is used to calculate savings when using ProxyPilot vs direct API.
var DirectAPIPricing = map[string]ModelPricing{
	// Anthropic Claude models
	"claude-opus-4-5":   {15.00, 75.00, 1.50},
	"claude-opus-4":     {15.00, 75.00, 1.50},
	"claude-sonnet-4-5": {3.00, 15.00, 0.30},
	"claude-sonnet-4":   {3.00, 15.00, 0.30},
	"claude-haiku-4-5":  {0.80, 4.00, 0.08},
	"claude-3-5-sonnet": {3.00, 15.00, 0.30},
	"claude-3-5-haiku":  {0.80, 4.00, 0.08},
	"claude-3-opus":     {15.00, 75.00, 1.50},
	"claude-3-sonnet":   {3.00, 15.00, 0.30},
	"claude-3-haiku":    {0.25, 1.25, 0.025},

	// OpenAI GPT/Codex models
	"gpt-5.2":       {10.00, 30.00, 5.00},
	"gpt-5.2-codex": {10.00, 30.00, 5.00},
	"gpt-5-codex":   {8.00, 24.00, 4.00},
	"gpt-4o":        {2.50, 10.00, 1.25},
	"gpt-4o-mini":   {0.15, 0.60, 0.075},
	"gpt-4-turbo":   {10.00, 30.00, 5.00},
	"gpt-4":         {30.00, 60.00, 15.00},
	"gpt-3.5-turbo": {0.50, 1.50, 0.25},
	"o1":            {15.00, 60.00, 7.50},
	"o1-mini":       {3.00, 12.00, 1.50},
	"o1-preview":    {15.00, 60.00, 7.50},
	"o3-mini":       {1.10, 4.40, 0.55},

	// Google Gemini models
	"gemini-3-pro":        {1.25, 5.00, 0.3125},
	"gemini-3-flash":      {0.15, 0.60, 0.0375},
	"gemini-2.5-pro":      {1.25, 5.00, 0.3125},
	"gemini-2.5-flash":    {0.15, 0.60, 0.0375},
	"gemini-2.0-flash":    {0.10, 0.40, 0.025},
	"gemini-1.5-pro":      {1.25, 5.00, 0.3125},
	"gemini-1.5-flash":    {0.075, 0.30, 0.01875},
	"gemini-1.5-flash-8b": {0.0375, 0.15, 0.01},
	"gemini-1.0-pro":      {0.50, 1.50, 0.125},

	// Kiro - subscription-based so "direct" would be the subscription cost
	"kiro-claude-sonnet": {3.00, 15.00, 0.30},
	"kiro-claude-haiku":  {0.80, 4.00, 0.08},
	"kiro":               {3.00, 15.00, 0.30},

	// Qwen models
	"qwen-turbo": {0.30, 0.60, 0.15},
	"qwen-plus":  {0.80, 2.00, 0.40},
	"qwen-max":   {2.40, 9.60, 1.20},
	"qwq-32b":    {0.15, 0.60, 0.075},

	// DeepSeek models
	"deepseek-chat":     {0.14, 0.28, 0.07},
	"deepseek-reasoner": {0.55, 2.19, 0.275},

	// MiniMax models
	"minimax-m2":   {0.50, 1.50, 0.25},
	"minimax-m2.1": {0.60, 1.80, 0.30},

	// Zhipu GLM models
	"glm-4.5": {1.00, 3.00, 0.50},
	"glm-4.6": {1.20, 3.60, 0.60},
	"glm-4.7": {1.50, 4.50, 0.75},
	"glm-4":   {1.00, 3.00, 0.50},
}

// GetModelPricing returns the pricing for a given model.
// It attempts exact match first, then fuzzy matching on model name patterns.
func GetModelPricing(model string) (ModelPricing, bool) {
	m := strings.ToLower(model)

	// Exact match first
	if pricing, ok := PricingTable[m]; ok {
		return pricing, true
	}

	// Fuzzy match - check if model contains known pattern
	for pattern, pricing := range PricingTable {
		if strings.Contains(m, pattern) {
			return pricing, true
		}
	}

	// Try to match by prefix
	for pattern, pricing := range PricingTable {
		if strings.HasPrefix(m, pattern) || strings.HasPrefix(pattern, m) {
			return pricing, true
		}
	}

	return ModelPricing{}, false
}

// GetDirectAPIPricing returns the direct API pricing for comparison.
func GetDirectAPIPricing(model string) (ModelPricing, bool) {
	m := strings.ToLower(model)

	if pricing, ok := DirectAPIPricing[m]; ok {
		return pricing, true
	}

	for pattern, pricing := range DirectAPIPricing {
		if strings.Contains(m, pattern) {
			return pricing, true
		}
	}

	return ModelPricing{}, false
}

// CalculateCost calculates the cost for given token usage.
func CalculateCost(pricing ModelPricing, inputTokens, outputTokens, cachedTokens int64) float64 {
	inputCost := float64(inputTokens) * pricing.InputPerMillion / 1_000_000
	outputCost := float64(outputTokens) * pricing.OutputPerMillion / 1_000_000
	cachedCost := float64(cachedTokens) * pricing.CachedPerMillion / 1_000_000
	return inputCost + outputCost + cachedCost
}

// EstimateModelCost estimates the cost for a specific model and token usage.
// Returns (proxyCost, directCost, found).
func EstimateModelCost(model string, inputTokens, outputTokens, cachedTokens int64) (proxyCost, directCost float64, found bool) {
	pricing, ok := GetModelPricing(model)
	if !ok {
		return 0, 0, false
	}

	directPricing, directOk := GetDirectAPIPricing(model)
	if !directOk {
		directPricing = pricing // Fallback to same pricing
	}

	proxyCost = CalculateCost(pricing, inputTokens, outputTokens, cachedTokens)
	directCost = CalculateCost(directPricing, inputTokens, outputTokens, cachedTokens)

	return proxyCost, directCost, true
}

// FallbackEstimateCost provides a fallback cost estimation based on provider.
// Used when model-specific pricing is not found.
func FallbackEstimateCost(provider, model string, inputTokens, outputTokens int64) float64 {
	p := strings.ToLower(provider)
	m := strings.ToLower(model)

	var inputRate, outputRate float64

	if strings.Contains(p, "anthropic") || strings.Contains(p, "claude") || strings.Contains(m, "claude") {
		inputRate = 3.0 / 1_000_000
		outputRate = 15.0 / 1_000_000
	} else if strings.Contains(p, "openai") || strings.Contains(m, "gpt-4") || strings.Contains(m, "gpt-3.5") {
		inputRate = 10.0 / 1_000_000
		outputRate = 30.0 / 1_000_000
	} else if strings.Contains(p, "google") || strings.Contains(p, "gemini") || strings.Contains(m, "gemini") {
		inputRate = 0.50 / 1_000_000
		outputRate = 1.50 / 1_000_000
	} else if strings.Contains(p, "qwen") || strings.Contains(m, "qwen") {
		inputRate = 0.80 / 1_000_000
		outputRate = 2.00 / 1_000_000
	} else if strings.Contains(p, "deepseek") || strings.Contains(m, "deepseek") {
		inputRate = 0.14 / 1_000_000
		outputRate = 0.28 / 1_000_000
	} else {
		return 0
	}

	return (float64(inputTokens) * inputRate) + (float64(outputTokens) * outputRate)
}
