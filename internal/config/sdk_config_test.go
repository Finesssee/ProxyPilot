package config

import "testing"

func TestCompressionConfig_IsEnabled(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name string
		cfg  *CompressionConfig
		want bool
	}{
		{
			name: "nil config defaults to true",
			cfg:  nil,
			want: true,
		},
		{
			name: "nil Enabled defaults to true",
			cfg:  &CompressionConfig{Enabled: nil},
			want: true,
		},
		{
			name: "explicitly enabled",
			cfg:  &CompressionConfig{Enabled: boolPtr(true)},
			want: true,
		},
		{
			name: "explicitly disabled",
			cfg:  &CompressionConfig{Enabled: boolPtr(false)},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.IsEnabled()
			if got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompressionConfig_GetThresholdPercent(t *testing.T) {
	floatPtr := func(f float64) *float64 { return &f }

	tests := []struct {
		name string
		cfg  *CompressionConfig
		want float64
	}{
		{
			name: "nil config defaults to 0.75",
			cfg:  nil,
			want: 0.75,
		},
		{
			name: "nil ThresholdPercent defaults to 0.75",
			cfg:  &CompressionConfig{ThresholdPercent: nil},
			want: 0.75,
		},
		{
			name: "custom threshold",
			cfg:  &CompressionConfig{ThresholdPercent: floatPtr(0.9)},
			want: 0.9,
		},
		{
			name: "low threshold",
			cfg:  &CompressionConfig{ThresholdPercent: floatPtr(0.5)},
			want: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.GetThresholdPercent()
			if got != tt.want {
				t.Errorf("GetThresholdPercent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompressionConfig_GetMaxSummaryTokens(t *testing.T) {
	intPtr := func(i int) *int { return &i }

	tests := []struct {
		name string
		cfg  *CompressionConfig
		want int
	}{
		{
			name: "nil config defaults to 2000",
			cfg:  nil,
			want: 2000,
		},
		{
			name: "nil MaxSummaryTokens defaults to 2000",
			cfg:  &CompressionConfig{MaxSummaryTokens: nil},
			want: 2000,
		},
		{
			name: "custom value",
			cfg:  &CompressionConfig{MaxSummaryTokens: intPtr(5000)},
			want: 5000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.GetMaxSummaryTokens()
			if got != tt.want {
				t.Errorf("GetMaxSummaryTokens() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompressionConfig_GetSummarizationTimeout(t *testing.T) {
	intPtr := func(i int) *int { return &i }

	tests := []struct {
		name string
		cfg  *CompressionConfig
		want int
	}{
		{
			name: "nil config defaults to 30",
			cfg:  nil,
			want: 30,
		},
		{
			name: "nil timeout defaults to 30",
			cfg:  &CompressionConfig{SummarizationTimeoutSeconds: nil},
			want: 30,
		},
		{
			name: "custom timeout",
			cfg:  &CompressionConfig{SummarizationTimeoutSeconds: intPtr(60)},
			want: 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.GetSummarizationTimeout()
			if got != tt.want {
				t.Errorf("GetSummarizationTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompressionConfig_ShouldFallbackToRegex(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name string
		cfg  *CompressionConfig
		want bool
	}{
		{
			name: "nil config defaults to true",
			cfg:  nil,
			want: true,
		},
		{
			name: "nil FallbackToRegex defaults to true",
			cfg:  &CompressionConfig{FallbackToRegex: nil},
			want: true,
		},
		{
			name: "explicitly enabled",
			cfg:  &CompressionConfig{FallbackToRegex: boolPtr(true)},
			want: true,
		},
		{
			name: "explicitly disabled",
			cfg:  &CompressionConfig{FallbackToRegex: boolPtr(false)},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ShouldFallbackToRegex()
			if got != tt.want {
				t.Errorf("ShouldFallbackToRegex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSDKConfig_GetAutoRefreshBuffer(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *SDKConfig
		wantMs int64 // milliseconds for easier comparison
	}{
		{
			name:   "nil config defaults to 5m",
			cfg:    nil,
			wantMs: 5 * 60 * 1000,
		},
		{
			name:   "empty string defaults to 5m",
			cfg:    &SDKConfig{AutoRefreshBuffer: ""},
			wantMs: 5 * 60 * 1000,
		},
		{
			name:   "valid 10m",
			cfg:    &SDKConfig{AutoRefreshBuffer: "10m"},
			wantMs: 10 * 60 * 1000,
		},
		{
			name:   "valid 1h",
			cfg:    &SDKConfig{AutoRefreshBuffer: "1h"},
			wantMs: 60 * 60 * 1000,
		},
		{
			name:   "invalid duration defaults to 5m",
			cfg:    &SDKConfig{AutoRefreshBuffer: "invalid"},
			wantMs: 5 * 60 * 1000,
		},
		{
			name:   "negative duration defaults to 5m",
			cfg:    &SDKConfig{AutoRefreshBuffer: "-5m"},
			wantMs: 5 * 60 * 1000,
		},
		{
			name:   "zero duration defaults to 5m",
			cfg:    &SDKConfig{AutoRefreshBuffer: "0s"},
			wantMs: 5 * 60 * 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.GetAutoRefreshBuffer()
			gotMs := got.Milliseconds()
			if gotMs != tt.wantMs {
				t.Errorf("GetAutoRefreshBuffer() = %v ms, want %v ms", gotMs, tt.wantMs)
			}
		})
	}
}

func TestSDKConfig_GetDailyResetHour(t *testing.T) {
	intPtr := func(i int) *int { return &i }

	tests := []struct {
		name string
		cfg  *SDKConfig
		want int
	}{
		{
			name: "nil config defaults to 0",
			cfg:  nil,
			want: 0,
		},
		{
			name: "nil DailyResetHour defaults to 0",
			cfg:  &SDKConfig{DailyResetHour: nil},
			want: 0,
		},
		{
			name: "valid hour 12",
			cfg:  &SDKConfig{DailyResetHour: intPtr(12)},
			want: 12,
		},
		{
			name: "valid hour 23",
			cfg:  &SDKConfig{DailyResetHour: intPtr(23)},
			want: 23,
		},
		{
			name: "valid hour 0",
			cfg:  &SDKConfig{DailyResetHour: intPtr(0)},
			want: 0,
		},
		{
			name: "negative hour defaults to 0",
			cfg:  &SDKConfig{DailyResetHour: intPtr(-1)},
			want: 0,
		},
		{
			name: "hour > 23 defaults to 0",
			cfg:  &SDKConfig{DailyResetHour: intPtr(24)},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.GetDailyResetHour()
			if got != tt.want {
				t.Errorf("GetDailyResetHour() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMakeInlineAPIKeyProvider(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want bool // whether result should be non-nil
	}{
		{
			name: "empty keys returns nil",
			keys: []string{},
			want: false,
		},
		{
			name: "nil keys returns nil",
			keys: nil,
			want: false,
		},
		{
			name: "single key returns provider",
			keys: []string{"key1"},
			want: true,
		},
		{
			name: "multiple keys returns provider",
			keys: []string{"key1", "key2", "key3"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MakeInlineAPIKeyProvider(tt.keys)
			if (got != nil) != tt.want {
				t.Errorf("MakeInlineAPIKeyProvider() != nil = %v, want %v", got != nil, tt.want)
			}
			if got != nil {
				if got.Type != AccessProviderTypeConfigAPIKey {
					t.Errorf("Type = %s, want %s", got.Type, AccessProviderTypeConfigAPIKey)
				}
				if got.Name != DefaultAccessProviderName {
					t.Errorf("Name = %s, want %s", got.Name, DefaultAccessProviderName)
				}
				if len(got.APIKeys) != len(tt.keys) {
					t.Errorf("len(APIKeys) = %d, want %d", len(got.APIKeys), len(tt.keys))
				}
			}
		})
	}
}

func TestSDKConfig_ConfigAPIKeyProvider(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *SDKConfig
		wantNil  bool
		wantName string
	}{
		{
			name:    "nil config returns nil",
			cfg:     nil,
			wantNil: true,
		},
		{
			name:    "empty providers returns nil",
			cfg:     &SDKConfig{},
			wantNil: true,
		},
		{
			name: "no matching type returns nil",
			cfg: &SDKConfig{
				Access: AccessConfig{
					Providers: []AccessProvider{
						{Type: "other-type", Name: "other"},
					},
				},
			},
			wantNil: true,
		},
		{
			name: "finds config-api-key provider",
			cfg: &SDKConfig{
				Access: AccessConfig{
					Providers: []AccessProvider{
						{Type: AccessProviderTypeConfigAPIKey, Name: "my-provider"},
					},
				},
			},
			wantNil:  false,
			wantName: "my-provider",
		},
		{
			name: "sets default name if empty",
			cfg: &SDKConfig{
				Access: AccessConfig{
					Providers: []AccessProvider{
						{Type: AccessProviderTypeConfigAPIKey, Name: ""},
					},
				},
			},
			wantNil:  false,
			wantName: DefaultAccessProviderName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ConfigAPIKeyProvider()
			if (got == nil) != tt.wantNil {
				t.Errorf("ConfigAPIKeyProvider() nil = %v, wantNil %v", got == nil, tt.wantNil)
			}
			if got != nil && got.Name != tt.wantName {
				t.Errorf("Name = %s, want %s", got.Name, tt.wantName)
			}
		})
	}
}

func TestSDKConfig_LookupGlobalModelMapping(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *SDKConfig
		model    string
		provider string
		want     string
	}{
		{
			name:     "nil config returns empty",
			cfg:      nil,
			model:    "test",
			provider: "claude",
			want:     "",
		},
		{
			name:     "nil mapper returns empty",
			cfg:      &SDKConfig{GlobalModelMapper: nil},
			model:    "test",
			provider: "claude",
			want:     "",
		},
		{
			name: "mapper returns mapping",
			cfg: &SDKConfig{
				GlobalModelMapper: func(model, provider string) string {
					if model == "alias" {
						return "actual-model"
					}
					return ""
				},
			},
			model:    "alias",
			provider: "claude",
			want:     "actual-model",
		},
		{
			name: "mapper receives provider",
			cfg: &SDKConfig{
				GlobalModelMapper: func(model, provider string) string {
					if provider == "gemini" {
						return "gemini-mapped"
					}
					return ""
				},
			},
			model:    "test",
			provider: "gemini",
			want:     "gemini-mapped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.LookupGlobalModelMapping(tt.model, tt.provider)
			if got != tt.want {
				t.Errorf("LookupGlobalModelMapping() = %q, want %q", got, tt.want)
			}
		})
	}
}
