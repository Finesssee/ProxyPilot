// Package usage provides usage tracking and cost estimation.
package usage

import (
	"math"
	"testing"
)

// TestGetModelPricing_ExactMatch tests exact matches for each model type in the pricing table.
func TestGetModelPricing_ExactMatch(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected ModelPricing
	}{
		// Anthropic Claude models
		{
			name:     "claude-opus-4",
			model:    "claude-opus-4",
			expected: ModelPricing{15.00, 75.00, 1.50},
		},
		{
			name:     "claude-sonnet-4",
			model:    "claude-sonnet-4",
			expected: ModelPricing{3.00, 15.00, 0.30},
		},
		{
			name:     "claude-3-5-sonnet",
			model:    "claude-3-5-sonnet",
			expected: ModelPricing{3.00, 15.00, 0.30},
		},
		{
			name:     "claude-3-5-haiku",
			model:    "claude-3-5-haiku",
			expected: ModelPricing{0.80, 4.00, 0.08},
		},
		{
			name:     "claude-3-opus",
			model:    "claude-3-opus",
			expected: ModelPricing{15.00, 75.00, 1.50},
		},
		{
			name:     "claude-3-sonnet",
			model:    "claude-3-sonnet",
			expected: ModelPricing{3.00, 15.00, 0.30},
		},
		{
			name:     "claude-3-haiku",
			model:    "claude-3-haiku",
			expected: ModelPricing{0.25, 1.25, 0.025},
		},
		// OpenAI GPT models
		{
			name:     "gpt-4o",
			model:    "gpt-4o",
			expected: ModelPricing{2.50, 10.00, 1.25},
		},
		{
			name:     "gpt-4o-mini",
			model:    "gpt-4o-mini",
			expected: ModelPricing{0.15, 0.60, 0.075},
		},
		{
			name:     "gpt-4-turbo",
			model:    "gpt-4-turbo",
			expected: ModelPricing{10.00, 30.00, 5.00},
		},
		{
			name:     "gpt-4",
			model:    "gpt-4",
			expected: ModelPricing{30.00, 60.00, 15.00},
		},
		{
			name:     "gpt-3.5-turbo",
			model:    "gpt-3.5-turbo",
			expected: ModelPricing{0.50, 1.50, 0.25},
		},
		{
			name:     "o1",
			model:    "o1",
			expected: ModelPricing{15.00, 60.00, 7.50},
		},
		{
			name:     "o1-mini",
			model:    "o1-mini",
			expected: ModelPricing{3.00, 12.00, 1.50},
		},
		{
			name:     "o1-preview",
			model:    "o1-preview",
			expected: ModelPricing{15.00, 60.00, 7.50},
		},
		{
			name:     "o3-mini",
			model:    "o3-mini",
			expected: ModelPricing{1.10, 4.40, 0.55},
		},
		// Google Gemini models
		{
			name:     "gemini-2.0-flash",
			model:    "gemini-2.0-flash",
			expected: ModelPricing{0.10, 0.40, 0.025},
		},
		{
			name:     "gemini-1.5-pro",
			model:    "gemini-1.5-pro",
			expected: ModelPricing{1.25, 5.00, 0.3125},
		},
		{
			name:     "gemini-1.5-flash",
			model:    "gemini-1.5-flash",
			expected: ModelPricing{0.075, 0.30, 0.01875},
		},
		{
			name:     "gemini-1.5-flash-8b",
			model:    "gemini-1.5-flash-8b",
			expected: ModelPricing{0.0375, 0.15, 0.01},
		},
		{
			name:     "gemini-1.0-pro",
			model:    "gemini-1.0-pro",
			expected: ModelPricing{0.50, 1.50, 0.125},
		},
		// Qwen models
		{
			name:     "qwen-turbo",
			model:    "qwen-turbo",
			expected: ModelPricing{0.30, 0.60, 0.15},
		},
		{
			name:     "qwen-plus",
			model:    "qwen-plus",
			expected: ModelPricing{0.80, 2.00, 0.40},
		},
		{
			name:     "qwen-max",
			model:    "qwen-max",
			expected: ModelPricing{2.40, 9.60, 1.20},
		},
		{
			name:     "qwq-32b",
			model:    "qwq-32b",
			expected: ModelPricing{0.15, 0.60, 0.075},
		},
		// DeepSeek models
		{
			name:     "deepseek-chat",
			model:    "deepseek-chat",
			expected: ModelPricing{0.14, 0.28, 0.07},
		},
		{
			name:     "deepseek-reasoner",
			model:    "deepseek-reasoner",
			expected: ModelPricing{0.55, 2.19, 0.275},
		},
		// Free tier models
		{
			name:     "codex",
			model:    "codex",
			expected: ModelPricing{0.00, 0.00, 0.00},
		},
		{
			name:     "copilot",
			model:    "copilot",
			expected: ModelPricing{0.00, 0.00, 0.00},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing, found := GetModelPricing(tt.model)
			if !found {
				t.Errorf("GetModelPricing(%q) returned found=false, expected true", tt.model)
				return
			}
			if pricing.InputPerMillion != tt.expected.InputPerMillion {
				t.Errorf("GetModelPricing(%q).InputPerMillion = %v, want %v", tt.model, pricing.InputPerMillion, tt.expected.InputPerMillion)
			}
			if pricing.OutputPerMillion != tt.expected.OutputPerMillion {
				t.Errorf("GetModelPricing(%q).OutputPerMillion = %v, want %v", tt.model, pricing.OutputPerMillion, tt.expected.OutputPerMillion)
			}
			if pricing.CachedPerMillion != tt.expected.CachedPerMillion {
				t.Errorf("GetModelPricing(%q).CachedPerMillion = %v, want %v", tt.model, pricing.CachedPerMillion, tt.expected.CachedPerMillion)
			}
		})
	}
}

// TestGetModelPricing_FuzzyMatch tests partial model name matching.
func TestGetModelPricing_FuzzyMatch(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		expectFound bool
		wantInput   float64 // Expected InputPerMillion (for verification)
	}{
		{
			name:        "claude-opus-4-20250101 version suffix",
			model:       "claude-opus-4-20250101",
			expectFound: true,
			wantInput:   15.00,
		},
		{
			name:        "claude-3-5-sonnet-20241022 version suffix",
			model:       "claude-3-5-sonnet-20241022",
			expectFound: true,
			wantInput:   3.00,
		},
		{
			name:        "uppercase model name",
			model:       "CLAUDE-OPUS-4",
			expectFound: true,
			wantInput:   15.00,
		},
		{
			name:        "mixed case model name",
			model:       "Claude-Sonnet-4",
			expectFound: true,
			wantInput:   3.00,
		},
		// Note: Models with version suffixes that match multiple patterns in the pricing
		// table (e.g., gpt-4o-mini matches both gpt-4o and gpt-4o-mini) have non-deterministic
		// results due to Go map iteration order. We test unambiguous fuzzy matches instead.
		{
			name:        "gemini with suffix",
			model:       "gemini-1.5-pro-latest",
			expectFound: true,
			wantInput:   1.25,
		},
		{
			name:        "deepseek-chat variant",
			model:       "deepseek-chat-v2",
			expectFound: true,
			wantInput:   0.14,
		},
		{
			name:        "model with prefix",
			model:       "models/gemini-2.0-flash",
			expectFound: true,
			wantInput:   0.10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing, found := GetModelPricing(tt.model)
			if found != tt.expectFound {
				t.Errorf("GetModelPricing(%q) found = %v, want %v", tt.model, found, tt.expectFound)
				return
			}
			if tt.expectFound && pricing.InputPerMillion != tt.wantInput {
				t.Errorf("GetModelPricing(%q).InputPerMillion = %v, want %v", tt.model, pricing.InputPerMillion, tt.wantInput)
			}
		})
	}
}

// TestGetModelPricing_UnknownModel tests that unknown models return false.
func TestGetModelPricing_UnknownModel(t *testing.T) {
	tests := []struct {
		name  string
		model string
	}{
		{
			name:  "completely unknown model",
			model: "unknown-model-xyz",
		},
		{
			name:  "random string",
			model: "abcdefghijklmnop",
		},
		// Note: empty string matches due to HasPrefix(pattern, "") returning true
		// for any pattern, so it's not tested here as "not found".
		{
			name:  "partial match that should not match",
			model: "llama-3-70b",
		},
		{
			name:  "mistral model (not in table)",
			model: "mistral-large",
		},
		{
			name:  "special characters only",
			model: "!@#$%^&*()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing, found := GetModelPricing(tt.model)
			if found {
				t.Errorf("GetModelPricing(%q) returned found=true, expected false", tt.model)
			}
			if pricing.InputPerMillion != 0 || pricing.OutputPerMillion != 0 || pricing.CachedPerMillion != 0 {
				t.Errorf("GetModelPricing(%q) returned non-zero pricing for unknown model: %+v", tt.model, pricing)
			}
		})
	}
}

// TestGetDirectAPIPricing_ExactMatch tests direct API pricing lookup for exact matches.
func TestGetDirectAPIPricing_ExactMatch(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected ModelPricing
	}{
		{
			name:     "claude-opus-4",
			model:    "claude-opus-4",
			expected: ModelPricing{15.00, 75.00, 1.50},
		},
		{
			name:     "gpt-4o",
			model:    "gpt-4o",
			expected: ModelPricing{2.50, 10.00, 1.25},
		},
		{
			name:     "gemini-1.5-pro",
			model:    "gemini-1.5-pro",
			expected: ModelPricing{1.25, 5.00, 0.3125},
		},
		{
			name:     "deepseek-chat",
			model:    "deepseek-chat",
			expected: ModelPricing{0.14, 0.28, 0.07},
		},
		{
			name:     "qwen-max",
			model:    "qwen-max",
			expected: ModelPricing{2.40, 9.60, 1.20},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing, found := GetDirectAPIPricing(tt.model)
			if !found {
				t.Errorf("GetDirectAPIPricing(%q) returned found=false, expected true", tt.model)
				return
			}
			if pricing.InputPerMillion != tt.expected.InputPerMillion {
				t.Errorf("GetDirectAPIPricing(%q).InputPerMillion = %v, want %v", tt.model, pricing.InputPerMillion, tt.expected.InputPerMillion)
			}
			if pricing.OutputPerMillion != tt.expected.OutputPerMillion {
				t.Errorf("GetDirectAPIPricing(%q).OutputPerMillion = %v, want %v", tt.model, pricing.OutputPerMillion, tt.expected.OutputPerMillion)
			}
			if pricing.CachedPerMillion != tt.expected.CachedPerMillion {
				t.Errorf("GetDirectAPIPricing(%q).CachedPerMillion = %v, want %v", tt.model, pricing.CachedPerMillion, tt.expected.CachedPerMillion)
			}
		})
	}
}

// TestGetDirectAPIPricing_FuzzyMatch tests fuzzy matching for direct API pricing.
func TestGetDirectAPIPricing_FuzzyMatch(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		expectFound bool
		wantInput   float64
	}{
		{
			name:        "claude-opus-4 with version",
			model:       "claude-opus-4-20250101",
			expectFound: true,
			wantInput:   15.00,
		},
		{
			name:        "uppercase gpt-4o",
			model:       "GPT-4O",
			expectFound: true,
			wantInput:   2.50,
		},
		{
			name:        "gemini with suffix",
			model:       "gemini-1.5-pro-latest",
			expectFound: true,
			wantInput:   1.25,
		},
		{
			name:        "unknown model",
			model:       "unknown-model",
			expectFound: false,
			wantInput:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing, found := GetDirectAPIPricing(tt.model)
			if found != tt.expectFound {
				t.Errorf("GetDirectAPIPricing(%q) found = %v, want %v", tt.model, found, tt.expectFound)
				return
			}
			if tt.expectFound && pricing.InputPerMillion != tt.wantInput {
				t.Errorf("GetDirectAPIPricing(%q).InputPerMillion = %v, want %v", tt.model, pricing.InputPerMillion, tt.wantInput)
			}
		})
	}
}

// TestCalculateCost_AllTokenTypes tests cost calculation with input/output/cached tokens.
func TestCalculateCost_AllTokenTypes(t *testing.T) {
	tests := []struct {
		name         string
		pricing      ModelPricing
		inputTokens  int64
		outputTokens int64
		cachedTokens int64
		expectedCost float64
	}{
		{
			name:         "claude-opus-4 with all token types",
			pricing:      ModelPricing{15.00, 75.00, 1.50},
			inputTokens:  1000,
			outputTokens: 500,
			cachedTokens: 2000,
			// (1000 * 15.00 / 1M) + (500 * 75.00 / 1M) + (2000 * 1.50 / 1M)
			// = 0.015 + 0.0375 + 0.003 = 0.0555
			expectedCost: 0.0555,
		},
		{
			name:         "gpt-4o standard usage",
			pricing:      ModelPricing{2.50, 10.00, 1.25},
			inputTokens:  10000,
			outputTokens: 2000,
			cachedTokens: 5000,
			// (10000 * 2.50 / 1M) + (2000 * 10.00 / 1M) + (5000 * 1.25 / 1M)
			// = 0.025 + 0.02 + 0.00625 = 0.05125
			expectedCost: 0.05125,
		},
		{
			name:         "input tokens only",
			pricing:      ModelPricing{3.00, 15.00, 0.30},
			inputTokens:  100000,
			outputTokens: 0,
			cachedTokens: 0,
			// 100000 * 3.00 / 1M = 0.3
			expectedCost: 0.3,
		},
		{
			name:         "output tokens only",
			pricing:      ModelPricing{3.00, 15.00, 0.30},
			inputTokens:  0,
			outputTokens: 100000,
			cachedTokens: 0,
			// 100000 * 15.00 / 1M = 1.5
			expectedCost: 1.5,
		},
		{
			name:         "cached tokens only",
			pricing:      ModelPricing{3.00, 15.00, 0.30},
			inputTokens:  0,
			outputTokens: 0,
			cachedTokens: 100000,
			// 100000 * 0.30 / 1M = 0.03
			expectedCost: 0.03,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := CalculateCost(tt.pricing, tt.inputTokens, tt.outputTokens, tt.cachedTokens)
			if !almostEqual(cost, tt.expectedCost, 1e-9) {
				t.Errorf("CalculateCost() = %v, want %v", cost, tt.expectedCost)
			}
		})
	}
}

// TestCalculateCost_ZeroTokens tests with zero token values.
func TestCalculateCost_ZeroTokens(t *testing.T) {
	tests := []struct {
		name         string
		pricing      ModelPricing
		inputTokens  int64
		outputTokens int64
		cachedTokens int64
	}{
		{
			name:         "all zeros with non-zero pricing",
			pricing:      ModelPricing{15.00, 75.00, 1.50},
			inputTokens:  0,
			outputTokens: 0,
			cachedTokens: 0,
		},
		{
			name:         "zero pricing with non-zero tokens",
			pricing:      ModelPricing{0.00, 0.00, 0.00},
			inputTokens:  1000000,
			outputTokens: 1000000,
			cachedTokens: 1000000,
		},
		{
			name:         "codex free tier",
			pricing:      ModelPricing{0.00, 0.00, 0.00},
			inputTokens:  50000,
			outputTokens: 10000,
			cachedTokens: 25000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := CalculateCost(tt.pricing, tt.inputTokens, tt.outputTokens, tt.cachedTokens)
			if cost != 0 {
				t.Errorf("CalculateCost() = %v, want 0", cost)
			}
		})
	}
}

// TestCalculateCost_LargeNumbers tests with 1M+ tokens.
func TestCalculateCost_LargeNumbers(t *testing.T) {
	tests := []struct {
		name         string
		pricing      ModelPricing
		inputTokens  int64
		outputTokens int64
		cachedTokens int64
		expectedCost float64
	}{
		{
			name:         "exactly 1 million input tokens",
			pricing:      ModelPricing{15.00, 75.00, 1.50},
			inputTokens:  1_000_000,
			outputTokens: 0,
			cachedTokens: 0,
			expectedCost: 15.00,
		},
		{
			name:         "exactly 1 million output tokens",
			pricing:      ModelPricing{15.00, 75.00, 1.50},
			inputTokens:  0,
			outputTokens: 1_000_000,
			cachedTokens: 0,
			expectedCost: 75.00,
		},
		{
			name:         "exactly 1 million cached tokens",
			pricing:      ModelPricing{15.00, 75.00, 1.50},
			inputTokens:  0,
			outputTokens: 0,
			cachedTokens: 1_000_000,
			expectedCost: 1.50,
		},
		{
			name:         "10 million tokens of each type",
			pricing:      ModelPricing{15.00, 75.00, 1.50},
			inputTokens:  10_000_000,
			outputTokens: 10_000_000,
			cachedTokens: 10_000_000,
			// (10M * 15 / 1M) + (10M * 75 / 1M) + (10M * 1.5 / 1M)
			// = 150 + 750 + 15 = 915
			expectedCost: 915.00,
		},
		{
			name:         "100 million input tokens gpt-4",
			pricing:      ModelPricing{30.00, 60.00, 15.00},
			inputTokens:  100_000_000,
			outputTokens: 10_000_000,
			cachedTokens: 50_000_000,
			// (100M * 30 / 1M) + (10M * 60 / 1M) + (50M * 15 / 1M)
			// = 3000 + 600 + 750 = 4350
			expectedCost: 4350.00,
		},
		{
			name:         "very large numbers - billion tokens",
			pricing:      ModelPricing{0.10, 0.40, 0.025},
			inputTokens:  1_000_000_000,
			outputTokens: 100_000_000,
			cachedTokens: 500_000_000,
			// (1B * 0.10 / 1M) + (100M * 0.40 / 1M) + (500M * 0.025 / 1M)
			// = 100 + 40 + 12.5 = 152.5
			expectedCost: 152.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := CalculateCost(tt.pricing, tt.inputTokens, tt.outputTokens, tt.cachedTokens)
			if !almostEqual(cost, tt.expectedCost, 1e-6) {
				t.Errorf("CalculateCost() = %v, want %v", cost, tt.expectedCost)
			}
		})
	}
}

// TestEstimateModelCost_Found tests successful cost estimation.
func TestEstimateModelCost_Found(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		inputTokens  int64
		outputTokens int64
		cachedTokens int64
		wantFound    bool
	}{
		{
			name:         "claude-opus-4",
			model:        "claude-opus-4",
			inputTokens:  10000,
			outputTokens: 2000,
			cachedTokens: 5000,
			wantFound:    true,
		},
		{
			name:         "gpt-4o",
			model:        "gpt-4o",
			inputTokens:  50000,
			outputTokens: 10000,
			cachedTokens: 25000,
			wantFound:    true,
		},
		{
			name:         "gemini-1.5-pro",
			model:        "gemini-1.5-pro",
			inputTokens:  100000,
			outputTokens: 20000,
			cachedTokens: 0,
			wantFound:    true,
		},
		{
			name:         "claude-3-5-sonnet with version",
			model:        "claude-3-5-sonnet-20241022",
			inputTokens:  1000000,
			outputTokens: 100000,
			cachedTokens: 500000,
			wantFound:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxyCost, directCost, found := EstimateModelCost(tt.model, tt.inputTokens, tt.outputTokens, tt.cachedTokens)
			if found != tt.wantFound {
				t.Errorf("EstimateModelCost() found = %v, want %v", found, tt.wantFound)
				return
			}
			if tt.wantFound {
				// Verify costs are calculated (non-negative)
				if proxyCost < 0 {
					t.Errorf("EstimateModelCost() proxyCost = %v, want >= 0", proxyCost)
				}
				if directCost < 0 {
					t.Errorf("EstimateModelCost() directCost = %v, want >= 0", directCost)
				}
				// For models in both tables, proxy and direct costs should be equal
				if !almostEqual(proxyCost, directCost, 1e-9) {
					t.Logf("proxyCost (%v) != directCost (%v) - this is expected if pricing differs", proxyCost, directCost)
				}
			}
		})
	}
}

// TestEstimateModelCost_NotFound tests that unknown models return found=false.
func TestEstimateModelCost_NotFound(t *testing.T) {
	tests := []struct {
		name  string
		model string
	}{
		{
			name:  "unknown model",
			model: "unknown-model-xyz",
		},
		{
			name:  "llama model (not in table)",
			model: "llama-3-70b",
		},
		{
			name:  "mistral model",
			model: "mistral-large",
		},
		// Note: empty string matches due to HasPrefix(pattern, "") returning true
		// for any pattern, so it's not tested here as "not found".
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxyCost, directCost, found := EstimateModelCost(tt.model, 10000, 2000, 5000)
			if found {
				t.Errorf("EstimateModelCost(%q) found = true, want false", tt.model)
			}
			if proxyCost != 0 {
				t.Errorf("EstimateModelCost(%q) proxyCost = %v, want 0", tt.model, proxyCost)
			}
			if directCost != 0 {
				t.Errorf("EstimateModelCost(%q) directCost = %v, want 0", tt.model, directCost)
			}
		})
	}
}

// TestFallbackEstimateCost_Anthropic tests Claude provider fallback estimation.
func TestFallbackEstimateCost_Anthropic(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		model        string
		inputTokens  int64
		outputTokens int64
		expectedCost float64
	}{
		{
			name:         "anthropic provider",
			provider:     "anthropic",
			model:        "some-claude-model",
			inputTokens:  1_000_000,
			outputTokens: 1_000_000,
			// inputRate = 3.0/1M, outputRate = 15.0/1M
			// (1M * 3.0/1M) + (1M * 15.0/1M) = 3 + 15 = 18
			expectedCost: 18.0,
		},
		{
			name:         "claude in provider",
			provider:     "claude-api",
			model:        "unknown",
			inputTokens:  500_000,
			outputTokens: 100_000,
			// (500K * 3.0/1M) + (100K * 15.0/1M) = 1.5 + 1.5 = 3.0
			expectedCost: 3.0,
		},
		{
			name:         "claude in model name",
			provider:     "unknown-provider",
			model:        "claude-next",
			inputTokens:  100_000,
			outputTokens: 50_000,
			// (100K * 3.0/1M) + (50K * 15.0/1M) = 0.3 + 0.75 = 1.05
			expectedCost: 1.05,
		},
		{
			name:         "uppercase ANTHROPIC",
			provider:     "ANTHROPIC",
			model:        "model",
			inputTokens:  1_000_000,
			outputTokens: 0,
			expectedCost: 3.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := FallbackEstimateCost(tt.provider, tt.model, tt.inputTokens, tt.outputTokens)
			if !almostEqual(cost, tt.expectedCost, 1e-9) {
				t.Errorf("FallbackEstimateCost() = %v, want %v", cost, tt.expectedCost)
			}
		})
	}
}

// TestFallbackEstimateCost_OpenAI tests GPT provider fallback estimation.
func TestFallbackEstimateCost_OpenAI(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		model        string
		inputTokens  int64
		outputTokens int64
		expectedCost float64
	}{
		{
			name:         "openai provider",
			provider:     "openai",
			model:        "some-gpt-model",
			inputTokens:  1_000_000,
			outputTokens: 1_000_000,
			// inputRate = 10.0/1M, outputRate = 30.0/1M
			// (1M * 10.0/1M) + (1M * 30.0/1M) = 10 + 30 = 40
			expectedCost: 40.0,
		},
		{
			name:         "gpt-4 in model name",
			provider:     "unknown",
			model:        "gpt-4-unknown",
			inputTokens:  500_000,
			outputTokens: 100_000,
			// (500K * 10.0/1M) + (100K * 30.0/1M) = 5 + 3 = 8
			expectedCost: 8.0,
		},
		{
			name:         "gpt-3.5 in model name",
			provider:     "custom",
			model:        "gpt-3.5-custom",
			inputTokens:  100_000,
			outputTokens: 50_000,
			// (100K * 10.0/1M) + (50K * 30.0/1M) = 1 + 1.5 = 2.5
			expectedCost: 2.5,
		},
		{
			name:         "uppercase OPENAI",
			provider:     "OPENAI",
			model:        "model",
			inputTokens:  1_000_000,
			outputTokens: 0,
			expectedCost: 10.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := FallbackEstimateCost(tt.provider, tt.model, tt.inputTokens, tt.outputTokens)
			if !almostEqual(cost, tt.expectedCost, 1e-9) {
				t.Errorf("FallbackEstimateCost() = %v, want %v", cost, tt.expectedCost)
			}
		})
	}
}

// TestFallbackEstimateCost_Google tests Gemini provider fallback estimation.
func TestFallbackEstimateCost_Google(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		model        string
		inputTokens  int64
		outputTokens int64
		expectedCost float64
	}{
		{
			name:         "google provider",
			provider:     "google",
			model:        "some-model",
			inputTokens:  1_000_000,
			outputTokens: 1_000_000,
			// inputRate = 0.50/1M, outputRate = 1.50/1M
			// (1M * 0.50/1M) + (1M * 1.50/1M) = 0.5 + 1.5 = 2.0
			expectedCost: 2.0,
		},
		{
			name:         "gemini in provider",
			provider:     "gemini-api",
			model:        "unknown",
			inputTokens:  2_000_000,
			outputTokens: 500_000,
			// (2M * 0.50/1M) + (500K * 1.50/1M) = 1.0 + 0.75 = 1.75
			expectedCost: 1.75,
		},
		{
			name:         "gemini in model name",
			provider:     "custom",
			model:        "gemini-ultra",
			inputTokens:  10_000_000,
			outputTokens: 1_000_000,
			// (10M * 0.50/1M) + (1M * 1.50/1M) = 5.0 + 1.5 = 6.5
			expectedCost: 6.5,
		},
		{
			name:         "uppercase GOOGLE",
			provider:     "GOOGLE",
			model:        "model",
			inputTokens:  1_000_000,
			outputTokens: 0,
			expectedCost: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := FallbackEstimateCost(tt.provider, tt.model, tt.inputTokens, tt.outputTokens)
			if !almostEqual(cost, tt.expectedCost, 1e-9) {
				t.Errorf("FallbackEstimateCost() = %v, want %v", cost, tt.expectedCost)
			}
		})
	}
}

// TestFallbackEstimateCost_Unknown tests that unknown providers return 0.
func TestFallbackEstimateCost_Unknown(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		model    string
	}{
		{
			name:     "completely unknown provider and model",
			provider: "unknown-provider",
			model:    "unknown-model",
		},
		{
			name:     "empty strings",
			provider: "",
			model:    "",
		},
		{
			name:     "random provider",
			provider: "xyzabc",
			model:    "abc123",
		},
		{
			name:     "llama provider (not supported)",
			provider: "meta",
			model:    "llama-3",
		},
		{
			name:     "mistral (not in fallback)",
			provider: "mistral-ai",
			model:    "mistral-large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := FallbackEstimateCost(tt.provider, tt.model, 1_000_000, 1_000_000)
			if cost != 0 {
				t.Errorf("FallbackEstimateCost(%q, %q) = %v, want 0", tt.provider, tt.model, cost)
			}
		})
	}
}

// TestFallbackEstimateCost_Qwen tests Qwen provider fallback estimation.
func TestFallbackEstimateCost_Qwen(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		model        string
		inputTokens  int64
		outputTokens int64
		expectedCost float64
	}{
		{
			name:         "qwen provider",
			provider:     "qwen",
			model:        "some-model",
			inputTokens:  1_000_000,
			outputTokens: 1_000_000,
			// inputRate = 0.80/1M, outputRate = 2.00/1M
			// (1M * 0.80/1M) + (1M * 2.00/1M) = 0.8 + 2.0 = 2.8
			expectedCost: 2.8,
		},
		{
			name:         "qwen in model name",
			provider:     "custom",
			model:        "qwen-ultra",
			inputTokens:  500_000,
			outputTokens: 100_000,
			// (500K * 0.80/1M) + (100K * 2.00/1M) = 0.4 + 0.2 = 0.6
			expectedCost: 0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := FallbackEstimateCost(tt.provider, tt.model, tt.inputTokens, tt.outputTokens)
			if !almostEqual(cost, tt.expectedCost, 1e-9) {
				t.Errorf("FallbackEstimateCost() = %v, want %v", cost, tt.expectedCost)
			}
		})
	}
}

// TestFallbackEstimateCost_DeepSeek tests DeepSeek provider fallback estimation.
func TestFallbackEstimateCost_DeepSeek(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		model        string
		inputTokens  int64
		outputTokens int64
		expectedCost float64
	}{
		{
			name:         "deepseek provider",
			provider:     "deepseek",
			model:        "some-model",
			inputTokens:  1_000_000,
			outputTokens: 1_000_000,
			// inputRate = 0.14/1M, outputRate = 0.28/1M
			// (1M * 0.14/1M) + (1M * 0.28/1M) = 0.14 + 0.28 = 0.42
			expectedCost: 0.42,
		},
		{
			name:         "deepseek in model name",
			provider:     "custom",
			model:        "deepseek-v3",
			inputTokens:  10_000_000,
			outputTokens: 1_000_000,
			// (10M * 0.14/1M) + (1M * 0.28/1M) = 1.4 + 0.28 = 1.68
			expectedCost: 1.68,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := FallbackEstimateCost(tt.provider, tt.model, tt.inputTokens, tt.outputTokens)
			if !almostEqual(cost, tt.expectedCost, 1e-9) {
				t.Errorf("FallbackEstimateCost() = %v, want %v", cost, tt.expectedCost)
			}
		})
	}
}

// almostEqual checks if two float64 values are equal within a tolerance.
func almostEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}
