// Package config provides configuration management for the CLI Proxy API server.
// It handles loading and parsing YAML configuration files, and provides structured
// access to application settings including server port, authentication directory,
// debug settings, proxy configuration, and API keys.
package config

// GlobalModelMapperFunc is a function type for looking up global model mappings.
// It takes a model name and provider hint, and returns the mapped model name (or empty string if no mapping).
type GlobalModelMapperFunc func(model string, provider string) string

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

	// Access holds request authentication provider configuration.
	Access AccessConfig `yaml:"auth,omitempty" json:"auth,omitempty"`

	// Streaming configures server-side streaming behavior (keep-alives and safe bootstrap retries).
	Streaming StreamingConfig `yaml:"streaming" json:"streaming"`

	// Compression configures context compression behavior for long conversations.
	Compression CompressionConfig `yaml:"compression,omitempty" json:"compression,omitempty"`

	// GlobalModelMapper is an optional hook for looking up global model mappings.
	// This is set by the parent Config to enable cross-provider model aliasing.
	GlobalModelMapper GlobalModelMapperFunc `yaml:"-" json:"-"`
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

	// MaxChunkSize limits the maximum size (in bytes) of individual response chunks forwarded to clients.
	// This can reduce latency for large responses by preventing buffer accumulation.
	// nil means default (65536 = 64KB). 0 disables chunk size limiting.
	MaxChunkSize *int `yaml:"max-chunk-size,omitempty" json:"max-chunk-size,omitempty"`
}

// CompressionConfig holds context compression behavior configuration.
type CompressionConfig struct {
	// Enabled toggles LLM-based structured summarization.
	// nil means default (true).
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	// ThresholdPercent triggers compression when context usage exceeds this fraction.
	// nil means default (0.75 = 75%).
	ThresholdPercent *float64 `yaml:"threshold-percent,omitempty" json:"threshold-percent,omitempty"`

	// MaxSummaryTokens limits the output size of generated summaries.
	// nil means default (2000).
	MaxSummaryTokens *int `yaml:"max-summary-tokens,omitempty" json:"max-summary-tokens,omitempty"`

	// SummarizationTimeoutSeconds is the timeout for LLM summarization calls.
	// nil means default (30).
	SummarizationTimeoutSeconds *int `yaml:"summarization-timeout-seconds,omitempty" json:"summarization-timeout-seconds,omitempty"`

	// FallbackToRegex uses regex-based summarization when LLM fails.
	// nil means default (true).
	FallbackToRegex *bool `yaml:"fallback-to-regex,omitempty" json:"fallback-to-regex,omitempty"`
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

// IsEnabled returns whether LLM compression is enabled, defaulting to true.
func (c *CompressionConfig) IsEnabled() bool {
	if c == nil || c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

// GetThresholdPercent returns the compression threshold, defaulting to 0.75.
func (c *CompressionConfig) GetThresholdPercent() float64 {
	if c == nil || c.ThresholdPercent == nil {
		return 0.75
	}
	return *c.ThresholdPercent
}

// GetMaxSummaryTokens returns the max summary tokens, defaulting to 2000.
func (c *CompressionConfig) GetMaxSummaryTokens() int {
	if c == nil || c.MaxSummaryTokens == nil {
		return 2000
	}
	return *c.MaxSummaryTokens
}

// GetSummarizationTimeout returns the summarization timeout, defaulting to 30 seconds.
func (c *CompressionConfig) GetSummarizationTimeout() int {
	if c == nil || c.SummarizationTimeoutSeconds == nil {
		return 30
	}
	return *c.SummarizationTimeoutSeconds
}

// ShouldFallbackToRegex returns whether to fall back to regex, defaulting to true.
func (c *CompressionConfig) ShouldFallbackToRegex() bool {
	if c == nil || c.FallbackToRegex == nil {
		return true
	}
	return *c.FallbackToRegex
}

// LookupGlobalModelMapping returns the mapped model name if a global mapping exists.
// It delegates to the GlobalModelMapper hook if set.
// Returns empty string if no mapping found or hook not set.
func (cfg *SDKConfig) LookupGlobalModelMapping(model string, provider string) string {
	if cfg == nil || cfg.GlobalModelMapper == nil {
		return ""
	}
	return cfg.GlobalModelMapper(model, provider)
}

