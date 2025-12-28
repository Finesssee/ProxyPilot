package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ValidYAML(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantPort int
		wantHost string
		wantErr  bool
	}{
		{
			name: "minimal valid config",
			yaml: `
port: 8080
`,
			wantPort: 8080,
			wantHost: "",
			wantErr:  false,
		},
		{
			name: "config with host and port",
			yaml: `
host: 127.0.0.1
port: 9000
`,
			wantPort: 9000,
			wantHost: "127.0.0.1",
			wantErr:  false,
		},
		{
			name: "config with debug enabled",
			yaml: `
port: 8080
debug: true
`,
			wantPort: 8080,
			wantHost: "",
			wantErr:  false,
		},
		{
			name: "config with gemini keys",
			yaml: `
port: 8080
gemini-api-key:
  - api-key: "test-key-1"
  - api-key: "test-key-2"
`,
			wantPort: 8080,
			wantHost: "",
			wantErr:  false,
		},
		{
			name: "config with tls settings",
			yaml: `
port: 443
tls:
  enable: true
  cert: /path/to/cert.pem
  key: /path/to/key.pem
`,
			wantPort: 443,
			wantHost: "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			cfg, err := LoadConfig(configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if cfg.Port != tt.wantPort {
				t.Errorf("LoadConfig() Port = %v, want %v", cfg.Port, tt.wantPort)
			}
			if cfg.Host != tt.wantHost {
				t.Errorf("LoadConfig() Host = %v, want %v", cfg.Host, tt.wantHost)
			}
		})
	}
}

func TestLoadConfig_EmptyFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		optional bool
		wantErr  bool
	}{
		{
			name:     "empty file with optional false",
			content:  "",
			optional: false,
			wantErr:  false, // Empty file parses to zero-value Config
		},
		{
			name:     "empty file with optional true",
			content:  "",
			optional: true,
			wantErr:  false,
		},
		{
			name:     "whitespace only with optional false",
			content:  "   \n \n   ",
			optional: false,
			wantErr:  false,
		},
		{
			name:     "whitespace only with optional true",
			content:  "   \n \n   ",
			optional: true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			cfg, err := LoadConfigOptional(configPath, tt.optional)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfigOptional() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && cfg == nil {
				t.Error("LoadConfigOptional() returned nil config without error")
			}
		})
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		optional bool
		wantErr  bool
	}{
		{
			name: "invalid yaml syntax",
			content: `
port: 8080
  invalid indentation
`,
			optional: false,
			wantErr:  true,
		},
		{
			name: "invalid yaml with optional true",
			content: `
port: 8080
  invalid indentation
`,
			optional: true,
			wantErr:  false, // Optional mode returns empty config on parse error
		},
		{
			name: "malformed yaml structure",
			content: `
port: [8080
`,
			optional: false,
			wantErr:  true,
		},
		{
			name:     "duplicate keys at same level",
			content:  "port: 8080\nport: 9090\n  - invalid",
			optional: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			cfg, err := LoadConfigOptional(configPath, tt.optional)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfigOptional() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.optional && err == nil && cfg == nil {
				t.Error("LoadConfigOptional() with optional=true returned nil config")
			}
		})
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	tests := []struct {
		name     string
		optional bool
		wantErr  bool
	}{
		{
			name:     "missing file with optional false",
			optional: false,
			wantErr:  true,
		},
		{
			name:     "missing file with optional true",
			optional: true,
			wantErr:  false, // Optional mode returns empty config for missing file
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "nonexistent.yaml")

			cfg, err := LoadConfigOptional(configPath, tt.optional)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfigOptional() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.optional && cfg == nil {
				t.Error("LoadConfigOptional() with optional=true returned nil config for missing file")
			}
		})
	}
}

func TestValidateConfig_ValidPort(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{
			name:    "minimum valid port",
			port:    1,
			wantErr: false,
		},
		{
			name:    "maximum valid port",
			port:    65535,
			wantErr: false,
		},
		{
			name:    "common port 80",
			port:    80,
			wantErr: false,
		},
		{
			name:    "common port 443",
			port:    443,
			wantErr: false,
		},
		{
			name:    "common port 8080",
			port:    8080,
			wantErr: false,
		},
		{
			name:    "high ephemeral port",
			port:    49152,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Port: tt.port}
			_, err := ValidateConfig(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateConfig_InvalidPort(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{
			name:    "zero port",
			port:    0,
			wantErr: true,
		},
		{
			name:    "negative port",
			port:    -1,
			wantErr: true,
		},
		{
			name:    "port exceeds maximum",
			port:    65536,
			wantErr: true,
		},
		{
			name:    "large negative port",
			port:    -65536,
			wantErr: true,
		},
		{
			name:    "very large port",
			port:    100000,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Port: tt.port}
			_, err := ValidateConfig(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeGeminiKeys_RemovesDuplicates(t *testing.T) {
	tests := []struct {
		name      string
		keys      []GeminiKey
		wantCount int
	}{
		{
			name: "duplicate api keys",
			keys: []GeminiKey{
				{APIKey: "key-1"},
				{APIKey: "key-1"},
				{APIKey: "key-2"},
			},
			wantCount: 2,
		},
		{
			name: "all unique keys",
			keys: []GeminiKey{
				{APIKey: "key-1"},
				{APIKey: "key-2"},
				{APIKey: "key-3"},
			},
			wantCount: 3,
		},
		{
			name: "all same keys",
			keys: []GeminiKey{
				{APIKey: "same-key"},
				{APIKey: "same-key"},
				{APIKey: "same-key"},
			},
			wantCount: 1,
		},
		{
			name: "empty and duplicate",
			keys: []GeminiKey{
				{APIKey: ""},
				{APIKey: "key-1"},
				{APIKey: ""},
				{APIKey: "key-1"},
			},
			wantCount: 1,
		},
		{
			name:      "empty slice",
			keys:      []GeminiKey{},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{GeminiKey: tt.keys}
			cfg.SanitizeGeminiKeys()
			if len(cfg.GeminiKey) != tt.wantCount {
				t.Errorf("SanitizeGeminiKeys() count = %v, want %v", len(cfg.GeminiKey), tt.wantCount)
			}
		})
	}
}

func TestSanitizeGeminiKeys_TrimsWhitespace(t *testing.T) {
	tests := []struct {
		name        string
		keys        []GeminiKey
		wantAPIKeys []string
	}{
		{
			name: "leading and trailing spaces",
			keys: []GeminiKey{
				{APIKey: "  key-1  "},
				{APIKey: "\tkey-2\t"},
			},
			wantAPIKeys: []string{"key-1", "key-2"},
		},
		{
			name: "mixed whitespace",
			keys: []GeminiKey{
				{APIKey: " \t key-1 \n "},
			},
			wantAPIKeys: []string{"key-1"},
		},
		{
			name: "spaces in base-url",
			keys: []GeminiKey{
				{APIKey: "key-1", BaseURL: "  https://api.example.com  "},
			},
			wantAPIKeys: []string{"key-1"},
		},
		{
			name: "whitespace only key gets removed",
			keys: []GeminiKey{
				{APIKey: "   "},
				{APIKey: "valid-key"},
			},
			wantAPIKeys: []string{"valid-key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{GeminiKey: tt.keys}
			cfg.SanitizeGeminiKeys()
			if len(cfg.GeminiKey) != len(tt.wantAPIKeys) {
				t.Errorf("SanitizeGeminiKeys() count = %v, want %v", len(cfg.GeminiKey), len(tt.wantAPIKeys))
				return
			}
			for i, want := range tt.wantAPIKeys {
				if cfg.GeminiKey[i].APIKey != want {
					t.Errorf("SanitizeGeminiKeys() key[%d] = %q, want %q", i, cfg.GeminiKey[i].APIKey, want)
				}
			}
		})
	}
}

func TestSanitizeClaudeKeys_NormalizesHeaders(t *testing.T) {
	tests := []struct {
		name        string
		keys        []ClaudeKey
		wantHeaders []map[string]string
	}{
		{
			name: "trims header keys and values",
			keys: []ClaudeKey{
				{
					APIKey: "key-1",
					Headers: map[string]string{
						"  X-Custom-Header  ": "  value1  ",
						"Authorization":       "Bearer token",
					},
				},
			},
			wantHeaders: []map[string]string{
				{
					"X-Custom-Header": "value1",
					"Authorization":   "Bearer token",
				},
			},
		},
		{
			name: "removes empty key headers",
			keys: []ClaudeKey{
				{
					APIKey: "key-1",
					Headers: map[string]string{
						"":         "value",
						"  ":       "value2",
						"X-Header": "value3",
					},
				},
			},
			wantHeaders: []map[string]string{
				{
					"X-Header": "value3",
				},
			},
		},
		{
			name: "removes empty value headers",
			keys: []ClaudeKey{
				{
					APIKey: "key-1",
					Headers: map[string]string{
						"X-Header": "",
						"X-Other":  "   ",
						"X-Valid":  "value",
					},
				},
			},
			wantHeaders: []map[string]string{
				{
					"X-Valid": "value",
				},
			},
		},
		{
			name: "nil headers stays nil",
			keys: []ClaudeKey{
				{APIKey: "key-1", Headers: nil},
			},
			wantHeaders: []map[string]string{nil},
		},
		{
			name: "all empty headers becomes nil",
			keys: []ClaudeKey{
				{
					APIKey: "key-1",
					Headers: map[string]string{
						"":   "",
						"  ": "  ",
					},
				},
			},
			wantHeaders: []map[string]string{nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{ClaudeKey: tt.keys}
			cfg.SanitizeClaudeKeys()

			if len(cfg.ClaudeKey) != len(tt.wantHeaders) {
				t.Fatalf("SanitizeClaudeKeys() count = %v, want %v", len(cfg.ClaudeKey), len(tt.wantHeaders))
			}

			for i, wantHeader := range tt.wantHeaders {
				gotHeader := cfg.ClaudeKey[i].Headers
				if wantHeader == nil {
					if gotHeader != nil {
						t.Errorf("SanitizeClaudeKeys() headers[%d] = %v, want nil", i, gotHeader)
					}
					continue
				}
				if len(gotHeader) != len(wantHeader) {
					t.Errorf("SanitizeClaudeKeys() headers[%d] len = %v, want %v", i, len(gotHeader), len(wantHeader))
					continue
				}
				for k, v := range wantHeader {
					if gotHeader[k] != v {
						t.Errorf("SanitizeClaudeKeys() headers[%d][%q] = %q, want %q", i, k, gotHeader[k], v)
					}
				}
			}
		})
	}
}

func TestNormalizeHeaders_TrimsEmptyKeys(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    map[string]string
	}{
		{
			name: "trims all keys and values",
			headers: map[string]string{
				"  Key1  ": "  Value1  ",
				"\tKey2\t": "\tValue2\t",
			},
			want: map[string]string{
				"Key1": "Value1",
				"Key2": "Value2",
			},
		},
		{
			name: "removes empty keys",
			headers: map[string]string{
				"":     "value",
				"key":  "value",
				"   ":  "value",
				"\t\n": "value",
			},
			want: map[string]string{
				"key": "value",
			},
		},
		{
			name: "removes empty values",
			headers: map[string]string{
				"key1": "",
				"key2": "value",
				"key3": "   ",
			},
			want: map[string]string{
				"key2": "value",
			},
		},
		{
			name:    "nil input returns nil",
			headers: nil,
			want:    nil,
		},
		{
			name:    "empty map returns nil",
			headers: map[string]string{},
			want:    nil,
		},
		{
			name: "all empty entries returns nil",
			headers: map[string]string{
				"":    "",
				"  ":  "  ",
				"\t":  "\n",
				"   ": "",
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeHeaders(tt.headers)
			if tt.want == nil {
				if got != nil {
					t.Errorf("NormalizeHeaders() = %v, want nil", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("NormalizeHeaders() len = %v, want %v", len(got), len(tt.want))
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("NormalizeHeaders()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestNormalizeExcludedModels_Deduplicates(t *testing.T) {
	tests := []struct {
		name   string
		models []string
		want   []string
	}{
		{
			name:   "removes duplicates",
			models: []string{"model-a", "model-b", "model-a", "model-c", "model-b"},
			want:   []string{"model-a", "model-b", "model-c"},
		},
		{
			name:   "case insensitive deduplication",
			models: []string{"Model-A", "model-a", "MODEL-A"},
			want:   []string{"model-a"},
		},
		{
			name:   "trims whitespace before deduplication",
			models: []string{"  model-a  ", "model-a", " model-a"},
			want:   []string{"model-a"},
		},
		{
			name:   "removes empty entries",
			models: []string{"", "model-a", "   ", "model-b", "\t"},
			want:   []string{"model-a", "model-b"},
		},
		{
			name:   "preserves order of first occurrence",
			models: []string{"c-model", "a-model", "b-model", "a-model"},
			want:   []string{"c-model", "a-model", "b-model"},
		},
		{
			name:   "nil input returns nil",
			models: nil,
			want:   nil,
		},
		{
			name:   "empty slice returns nil",
			models: []string{},
			want:   nil,
		},
		{
			name:   "all empty entries returns nil",
			models: []string{"", "  ", "\t"},
			want:   nil,
		},
		{
			name:   "lowercases all models",
			models: []string{"GPT-4", "Claude-3", "GEMINI-PRO"},
			want:   []string{"gpt-4", "claude-3", "gemini-pro"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeExcludedModels(tt.models)
			if tt.want == nil {
				if got != nil {
					t.Errorf("NormalizeExcludedModels() = %v, want nil", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("NormalizeExcludedModels() len = %v, want %v; got = %v", len(got), len(tt.want), got)
				return
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("NormalizeExcludedModels()[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

func TestLookupGlobalModelMapping_ExactMatch(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name     string
		mappings []GlobalModelMapping
		model    string
		provider string
		want     string
	}{
		{
			name: "exact match found",
			mappings: []GlobalModelMapping{
				{From: "smart", To: "claude-opus-4-5-20251101"},
				{From: "fast", To: "claude-sonnet-4-5-20251101"},
			},
			model:    "smart",
			provider: "",
			want:     "claude-opus-4-5-20251101",
		},
		{
			name: "case insensitive match",
			mappings: []GlobalModelMapping{
				{From: "SMART", To: "claude-opus-4-5-20251101"},
			},
			model:    "smart",
			provider: "",
			want:     "claude-opus-4-5-20251101",
		},
		{
			name: "no match returns empty",
			mappings: []GlobalModelMapping{
				{From: "smart", To: "claude-opus-4-5-20251101"},
			},
			model:    "unknown",
			provider: "",
			want:     "",
		},
		{
			name:     "empty mappings returns empty",
			mappings: []GlobalModelMapping{},
			model:    "smart",
			provider: "",
			want:     "",
		},
		{
			name: "provider restriction matches",
			mappings: []GlobalModelMapping{
				{From: "smart", To: "claude-opus-4-5-20251101", Provider: "claude"},
			},
			model:    "smart",
			provider: "claude",
			want:     "claude-opus-4-5-20251101",
		},
		{
			name: "provider restriction does not match",
			mappings: []GlobalModelMapping{
				{From: "smart", To: "claude-opus-4-5-20251101", Provider: "claude"},
			},
			model:    "smart",
			provider: "gemini",
			want:     "",
		},
		{
			name: "disabled mapping skipped",
			mappings: []GlobalModelMapping{
				{From: "smart", To: "claude-opus-4-5-20251101", Enabled: boolPtr(false)},
				{From: "smart", To: "claude-sonnet-4-5-20251101", Enabled: boolPtr(true)},
			},
			model:    "smart",
			provider: "",
			want:     "claude-sonnet-4-5-20251101",
		},
		{
			name: "nil enabled defaults to true",
			mappings: []GlobalModelMapping{
				{From: "smart", To: "claude-opus-4-5-20251101", Enabled: nil},
			},
			model:    "smart",
			provider: "",
			want:     "claude-opus-4-5-20251101",
		},
		{
			name: "first matching mapping wins",
			mappings: []GlobalModelMapping{
				{From: "smart", To: "first-model"},
				{From: "smart", To: "second-model"},
			},
			model:    "smart",
			provider: "",
			want:     "first-model",
		},
		{
			name: "provider case insensitive",
			mappings: []GlobalModelMapping{
				{From: "smart", To: "claude-opus-4-5-20251101", Provider: "CLAUDE"},
			},
			model:    "smart",
			provider: "claude",
			want:     "claude-opus-4-5-20251101",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				GlobalModelMappings: tt.mappings,
			}
			got := cfg.LookupGlobalModelMapping(tt.model, tt.provider)
			if got != tt.want {
				t.Errorf("LookupGlobalModelMapping() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLookupGlobalModelMapping_NilConfig(t *testing.T) {
	var cfg *Config
	got := cfg.LookupGlobalModelMapping("smart", "claude")
	if got != "" {
		t.Errorf("LookupGlobalModelMapping() on nil config = %q, want empty string", got)
	}
}

func TestValidateConfig_NilConfig(t *testing.T) {
	_, err := ValidateConfig(nil)
	if err == nil {
		t.Error("ValidateConfig(nil) should return error")
	}
}

func TestGlobalModelMapping_IsEnabled(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name    string
		mapping GlobalModelMapping
		want    bool
	}{
		{
			name:    "nil enabled defaults to true",
			mapping: GlobalModelMapping{From: "a", To: "b", Enabled: nil},
			want:    true,
		},
		{
			name:    "explicitly enabled",
			mapping: GlobalModelMapping{From: "a", To: "b", Enabled: boolPtr(true)},
			want:    true,
		},
		{
			name:    "explicitly disabled",
			mapping: GlobalModelMapping{From: "a", To: "b", Enabled: boolPtr(false)},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mapping.IsEnabled()
			if got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
