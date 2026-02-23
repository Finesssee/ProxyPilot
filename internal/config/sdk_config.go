// Package config provides configuration management for the CLI Proxy API server.
// It handles loading and parsing YAML configuration files, and provides structured
// access to application settings including server port, authentication directory,
// debug settings, proxy configuration, and API keys.
package config

import "time"

// SDKConfig represents the application's configuration, loaded from a YAML file.
type SDKConfig struct {
	// ProxyURL is the URL of an optional proxy server to use for outbound requests.
	ProxyURL string `yaml:"proxy-url" json:"proxy-url"`

	// ForceModelPrefix requires explicit model prefixes (e.g., "teamA/gemini-3-pro-preview")
	// to target prefixed credentials. When false, unprefixed model requests may use prefixed
	// credentials as well.
	ForceModelPrefix bool `yaml:"force-model-prefix" json:"force-model-prefix"`

	// RequestLog enables or disables detailed request logging functionality.
	RequestLog bool `yaml:"request-log" json:"request-log"`

	// APIKeys is a list of keys for authenticating clients to this proxy server.
	APIKeys []string `yaml:"api-keys" json:"api-keys"`

	// PassthroughHeaders controls whether upstream response headers are forwarded to downstream clients.
	// Default is false (disabled).
	PassthroughHeaders bool `yaml:"passthrough-headers" json:"passthrough-headers"`

	// Access holds request authentication provider configuration.
	Access AccessConfig `yaml:"auth,omitempty" json:"auth,omitempty"`

	// Streaming configures server-side streaming behavior (keep-alives and safe bootstrap retries).
	Streaming StreamingConfig `yaml:"streaming" json:"streaming"`

	// NonStreamKeepAliveInterval controls how often blank lines are emitted for non-streaming responses.
	// <= 0 disables keep-alives. Value is in seconds.
	NonStreamKeepAliveInterval int `yaml:"nonstream-keepalive-interval,omitempty" json:"nonstream-keepalive-interval,omitempty"`

	// AutoRefreshBuffer specifies the duration before token expiry to trigger a refresh.
	// Defaults to 5m.
	AutoRefreshBuffer string `yaml:"auto-refresh-buffer,omitempty" json:"auto-refresh-buffer,omitempty"`

	// DailyResetHour specifies the hour (0-23) at which daily usage counters reset.
	DailyResetHour *int `yaml:"daily-reset-hour,omitempty" json:"daily-reset-hour,omitempty"`

	// GlobalModelMapper is a function that maps model names globally across providers.
	GlobalModelMapper func(model, provider string) string `yaml:"-" json:"-"`

	// Compression configures context compression behavior.
	Compression *CompressionConfig `yaml:"compression,omitempty" json:"compression,omitempty"`
}

// StreamingConfig holds server streaming behavior configuration.
type StreamingConfig struct {
	// KeepAliveSeconds controls how often the server emits SSE heartbeats (": keep-alive\n\n").
	// <= 0 disables keep-alives. Default is 0.
	KeepAliveSeconds int `yaml:"keepalive-seconds,omitempty" json:"keepalive-seconds,omitempty"`

	// BootstrapRetries controls how many times the server may retry a streaming request before any bytes are sent,
	// to allow auth rotation / transient recovery.
	// <= 0 disables bootstrap retries. Default is 0.
	BootstrapRetries int `yaml:"bootstrap-retries,omitempty" json:"bootstrap-retries,omitempty"`
}

// AccessConfig groups request authentication providers.
type AccessConfig struct {
	// Providers lists configured authentication providers.
	Providers []AccessProvider `yaml:"providers,omitempty" json:"providers,omitempty"`
}

// AccessProvider describes a request authentication provider entry.
type AccessProvider struct {
	// Name is the instance identifier for the provider.
	Name string `yaml:"name" json:"name"`

	// Type selects the provider implementation registered via the SDK.
	Type string `yaml:"type" json:"type"`

	// SDK optionally names a third-party SDK module providing this provider.
	SDK string `yaml:"sdk,omitempty" json:"sdk,omitempty"`

	// APIKeys lists inline keys for providers that require them.
	APIKeys []string `yaml:"api-keys,omitempty" json:"api-keys,omitempty"`

	// Config passes provider-specific options to the implementation.
	Config map[string]any `yaml:"config,omitempty" json:"config,omitempty"`
}

const (
	// AccessProviderTypeConfigAPIKey is the built-in provider validating inline API keys.
	AccessProviderTypeConfigAPIKey = "config-api-key"

	// DefaultAccessProviderName is applied when no provider name is supplied.
	DefaultAccessProviderName = "config-inline"
)

// ConfigAPIKeyProvider returns the first inline API key provider if present.
func (c *SDKConfig) ConfigAPIKeyProvider() *AccessProvider {
	if c == nil {
		return nil
	}
	for i := range c.Access.Providers {
		if c.Access.Providers[i].Type == AccessProviderTypeConfigAPIKey {
			if c.Access.Providers[i].Name == "" {
				c.Access.Providers[i].Name = DefaultAccessProviderName
			}
			return &c.Access.Providers[i]
		}
	}
	return nil
}

// MakeInlineAPIKeyProvider constructs an inline API key provider configuration.
// It returns nil when no keys are supplied.
func MakeInlineAPIKeyProvider(keys []string) *AccessProvider {
	if len(keys) == 0 {
		return nil
	}
	provider := &AccessProvider{
		Name:    DefaultAccessProviderName,
		Type:    AccessProviderTypeConfigAPIKey,
		APIKeys: append([]string(nil), keys...),
	}
	return provider
}

// CompressionConfig configures context compression behavior.
type CompressionConfig struct {
	// Enabled controls whether context compression is active.
	// Defaults to true if nil.
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	// ThresholdPercent specifies the token usage threshold (0.0-1.0) that triggers compression.
	// Defaults to 0.75 if nil.
	ThresholdPercent *float64 `yaml:"threshold-percent,omitempty" json:"threshold-percent,omitempty"`

	// MaxSummaryTokens limits the size of generated summaries.
	// Defaults to 2000 if nil.
	MaxSummaryTokens *int `yaml:"max-summary-tokens,omitempty" json:"max-summary-tokens,omitempty"`

	// SummarizationTimeoutSeconds limits how long to wait for summarization.
	// Defaults to 30 if nil.
	SummarizationTimeoutSeconds *int `yaml:"summarization-timeout-seconds,omitempty" json:"summarization-timeout-seconds,omitempty"`

	// FallbackToRegex enables regex-based compression when LLM summarization fails.
	// Defaults to true if nil.
	FallbackToRegex *bool `yaml:"fallback-to-regex,omitempty" json:"fallback-to-regex,omitempty"`
}

// IsEnabled returns whether compression is enabled. Defaults to true.
func (c *CompressionConfig) IsEnabled() bool {
	if c == nil || c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

// GetThresholdPercent returns the compression threshold. Defaults to 0.75.
func (c *CompressionConfig) GetThresholdPercent() float64 {
	if c == nil || c.ThresholdPercent == nil {
		return 0.75
	}
	return *c.ThresholdPercent
}

// GetMaxSummaryTokens returns the max summary token limit. Defaults to 2000.
func (c *CompressionConfig) GetMaxSummaryTokens() int {
	if c == nil || c.MaxSummaryTokens == nil {
		return 2000
	}
	return *c.MaxSummaryTokens
}

// GetSummarizationTimeout returns the summarization timeout in seconds. Defaults to 30.
func (c *CompressionConfig) GetSummarizationTimeout() int {
	if c == nil || c.SummarizationTimeoutSeconds == nil {
		return 30
	}
	return *c.SummarizationTimeoutSeconds
}

// ShouldFallbackToRegex returns whether to fallback to regex compression. Defaults to true.
func (c *CompressionConfig) ShouldFallbackToRegex() bool {
	if c == nil || c.FallbackToRegex == nil {
		return true
	}
	return *c.FallbackToRegex
}

// GetAutoRefreshBuffer returns the auto-refresh buffer duration. Defaults to 5m.
func (c *SDKConfig) GetAutoRefreshBuffer() time.Duration {
	if c == nil || c.AutoRefreshBuffer == "" {
		return 5 * time.Minute
	}
	d, err := time.ParseDuration(c.AutoRefreshBuffer)
	if err != nil || d <= 0 {
		return 5 * time.Minute
	}
	return d
}

// GetDailyResetHour returns the daily reset hour. Defaults to 0. Returns 0 for invalid values.
func (c *SDKConfig) GetDailyResetHour() int {
	if c == nil || c.DailyResetHour == nil {
		return 0
	}
	h := *c.DailyResetHour
	if h < 0 || h > 23 {
		return 0
	}
	return h
}

// LookupGlobalModelMapping returns a mapped model name if configured, or empty string.
func (c *SDKConfig) LookupGlobalModelMapping(model, provider string) string {
	if c == nil || c.GlobalModelMapper == nil {
		return ""
	}
	return c.GlobalModelMapper(model, provider)
}
