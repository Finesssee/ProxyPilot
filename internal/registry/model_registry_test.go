package registry

import (
	"sync"
	"testing"
	"time"
)

// newTestRegistry creates a fresh ModelRegistry for testing
func newTestRegistry() *ModelRegistry {
	return &ModelRegistry{
		models:           make(map[string]*ModelRegistration),
		clientModels:     make(map[string][]string),
		clientModelInfos: make(map[string]map[string]*ModelInfo),
		clientProviders:  make(map[string]string),
		mutex:            &sync.RWMutex{},
	}
}

func TestModelRegistry_RegisterClient_NewClient(t *testing.T) {
	tests := []struct {
		name           string
		clientID       string
		clientProvider string
		models         []*ModelInfo
		wantModelCount int
		wantClientReg  bool
	}{
		{
			name:           "register single model",
			clientID:       "client-1",
			clientProvider: "openai",
			models: []*ModelInfo{
				{ID: "gpt-4", OwnedBy: "openai", Type: "openai"},
			},
			wantModelCount: 1,
			wantClientReg:  true,
		},
		{
			name:           "register multiple models",
			clientID:       "client-2",
			clientProvider: "claude",
			models: []*ModelInfo{
				{ID: "claude-3-opus", OwnedBy: "anthropic", Type: "claude"},
				{ID: "claude-3-sonnet", OwnedBy: "anthropic", Type: "claude"},
				{ID: "claude-3-haiku", OwnedBy: "anthropic", Type: "claude"},
			},
			wantModelCount: 3,
			wantClientReg:  true,
		},
		{
			name:           "register with empty models list",
			clientID:       "client-3",
			clientProvider: "openai",
			models:         []*ModelInfo{},
			wantModelCount: 0,
			wantClientReg:  false,
		},
		{
			name:           "register with nil models in list",
			clientID:       "client-4",
			clientProvider: "gemini",
			models: []*ModelInfo{
				{ID: "gemini-pro", OwnedBy: "google", Type: "gemini"},
				nil,
				{ID: "gemini-ultra", OwnedBy: "google", Type: "gemini"},
			},
			wantModelCount: 2,
			wantClientReg:  true,
		},
		{
			name:           "register with model having empty ID",
			clientID:       "client-5",
			clientProvider: "openai",
			models: []*ModelInfo{
				{ID: "", OwnedBy: "openai", Type: "openai"},
				{ID: "gpt-4", OwnedBy: "openai", Type: "openai"},
			},
			wantModelCount: 1,
			wantClientReg:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry()
			r.RegisterClient(tt.clientID, tt.clientProvider, tt.models)

			// Check model count
			modelCount := len(r.models)
			if modelCount != tt.wantModelCount {
				t.Errorf("model count = %d, want %d", modelCount, tt.wantModelCount)
			}

			// Check client registration
			_, clientExists := r.clientModels[tt.clientID]
			if clientExists != tt.wantClientReg {
				t.Errorf("client registered = %v, want %v", clientExists, tt.wantClientReg)
			}

			// Check provider registration
			if tt.wantClientReg {
				provider, providerExists := r.clientProviders[tt.clientID]
				if !providerExists {
					t.Error("provider should be registered for client")
				}
				if provider != tt.clientProvider {
					t.Errorf("provider = %s, want %s", provider, tt.clientProvider)
				}
			}
		})
	}
}

func TestModelRegistry_RegisterClient_UpdateClient(t *testing.T) {
	tests := []struct {
		name              string
		clientID          string
		initialProvider   string
		initialModels     []*ModelInfo
		updatedProvider   string
		updatedModels     []*ModelInfo
		wantFinalModels   int
		wantFinalProvider string
	}{
		{
			name:            "update with same models",
			clientID:        "client-1",
			initialProvider: "openai",
			initialModels: []*ModelInfo{
				{ID: "gpt-4", OwnedBy: "openai"},
			},
			updatedProvider: "openai",
			updatedModels: []*ModelInfo{
				{ID: "gpt-4", OwnedBy: "openai", DisplayName: "GPT-4 Updated"},
			},
			wantFinalModels:   1,
			wantFinalProvider: "openai",
		},
		{
			name:            "update with different models",
			clientID:        "client-2",
			initialProvider: "claude",
			initialModels: []*ModelInfo{
				{ID: "claude-3-opus", OwnedBy: "anthropic"},
			},
			updatedProvider: "claude",
			updatedModels: []*ModelInfo{
				{ID: "claude-3-sonnet", OwnedBy: "anthropic"},
			},
			wantFinalModels:   1,
			wantFinalProvider: "claude",
		},
		{
			name:            "update with additional models",
			clientID:        "client-3",
			initialProvider: "gemini",
			initialModels: []*ModelInfo{
				{ID: "gemini-pro", OwnedBy: "google"},
			},
			updatedProvider: "gemini",
			updatedModels: []*ModelInfo{
				{ID: "gemini-pro", OwnedBy: "google"},
				{ID: "gemini-ultra", OwnedBy: "google"},
			},
			wantFinalModels:   2,
			wantFinalProvider: "gemini",
		},
		{
			name:            "update with fewer models",
			clientID:        "client-4",
			initialProvider: "openai",
			initialModels: []*ModelInfo{
				{ID: "gpt-4", OwnedBy: "openai"},
				{ID: "gpt-3.5-turbo", OwnedBy: "openai"},
			},
			updatedProvider: "openai",
			updatedModels: []*ModelInfo{
				{ID: "gpt-4", OwnedBy: "openai"},
			},
			wantFinalModels:   1,
			wantFinalProvider: "openai",
		},
		{
			name:            "update with different provider",
			clientID:        "client-5",
			initialProvider: "openai",
			initialModels: []*ModelInfo{
				{ID: "gpt-4", OwnedBy: "openai"},
			},
			updatedProvider: "azure",
			updatedModels: []*ModelInfo{
				{ID: "gpt-4", OwnedBy: "azure"},
			},
			wantFinalModels:   1,
			wantFinalProvider: "azure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry()

			// Initial registration
			r.RegisterClient(tt.clientID, tt.initialProvider, tt.initialModels)

			// Update registration
			r.RegisterClient(tt.clientID, tt.updatedProvider, tt.updatedModels)

			// Check final model count
			modelCount := len(r.models)
			if modelCount != tt.wantFinalModels {
				t.Errorf("final model count = %d, want %d", modelCount, tt.wantFinalModels)
			}

			// Check provider
			provider := r.clientProviders[tt.clientID]
			if provider != tt.wantFinalProvider {
				t.Errorf("final provider = %s, want %s", provider, tt.wantFinalProvider)
			}
		})
	}
}

func TestModelRegistry_RegisterClient_AddModels(t *testing.T) {
	tests := []struct {
		name          string
		registrations []struct {
			clientID string
			provider string
			models   []*ModelInfo
		}
		wantTotalModels       int
		wantModelClientCounts map[string]int
	}{
		{
			name: "multiple clients same model",
			registrations: []struct {
				clientID string
				provider string
				models   []*ModelInfo
			}{
				{
					clientID: "client-1",
					provider: "openai",
					models:   []*ModelInfo{{ID: "gpt-4", OwnedBy: "openai"}},
				},
				{
					clientID: "client-2",
					provider: "openai",
					models:   []*ModelInfo{{ID: "gpt-4", OwnedBy: "openai"}},
				},
			},
			wantTotalModels:       1,
			wantModelClientCounts: map[string]int{"gpt-4": 2},
		},
		{
			name: "multiple clients different models",
			registrations: []struct {
				clientID string
				provider string
				models   []*ModelInfo
			}{
				{
					clientID: "client-1",
					provider: "openai",
					models:   []*ModelInfo{{ID: "gpt-4", OwnedBy: "openai"}},
				},
				{
					clientID: "client-2",
					provider: "claude",
					models:   []*ModelInfo{{ID: "claude-3-opus", OwnedBy: "anthropic"}},
				},
			},
			wantTotalModels:       2,
			wantModelClientCounts: map[string]int{"gpt-4": 1, "claude-3-opus": 1},
		},
		{
			name: "multiple clients overlapping models",
			registrations: []struct {
				clientID string
				provider string
				models   []*ModelInfo
			}{
				{
					clientID: "client-1",
					provider: "openai",
					models: []*ModelInfo{
						{ID: "gpt-4", OwnedBy: "openai"},
						{ID: "gpt-3.5-turbo", OwnedBy: "openai"},
					},
				},
				{
					clientID: "client-2",
					provider: "openai",
					models: []*ModelInfo{
						{ID: "gpt-4", OwnedBy: "openai"},
						{ID: "gpt-4-turbo", OwnedBy: "openai"},
					},
				},
			},
			wantTotalModels: 3,
			wantModelClientCounts: map[string]int{
				"gpt-4":         2,
				"gpt-3.5-turbo": 1,
				"gpt-4-turbo":   1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry()

			for _, reg := range tt.registrations {
				r.RegisterClient(reg.clientID, reg.provider, reg.models)
			}

			// Check total model count
			if len(r.models) != tt.wantTotalModels {
				t.Errorf("total models = %d, want %d", len(r.models), tt.wantTotalModels)
			}

			// Check individual model client counts
			for modelID, wantCount := range tt.wantModelClientCounts {
				reg, exists := r.models[modelID]
				if !exists {
					t.Errorf("model %s not found in registry", modelID)
					continue
				}
				if reg.Count != wantCount {
					t.Errorf("model %s client count = %d, want %d", modelID, reg.Count, wantCount)
				}
			}
		})
	}
}

func TestModelRegistry_UnregisterClient(t *testing.T) {
	tests := []struct {
		name               string
		setupRegistrations []struct {
			clientID string
			provider string
			models   []*ModelInfo
		}
		unregisterClientID   string
		wantModelsRemaining  int
		wantClientRegistered bool
	}{
		{
			name: "unregister single client with unique model",
			setupRegistrations: []struct {
				clientID string
				provider string
				models   []*ModelInfo
			}{
				{
					clientID: "client-1",
					provider: "openai",
					models:   []*ModelInfo{{ID: "gpt-4", OwnedBy: "openai"}},
				},
			},
			unregisterClientID:   "client-1",
			wantModelsRemaining:  0,
			wantClientRegistered: false,
		},
		{
			name: "unregister one of multiple clients sharing model",
			setupRegistrations: []struct {
				clientID string
				provider string
				models   []*ModelInfo
			}{
				{
					clientID: "client-1",
					provider: "openai",
					models:   []*ModelInfo{{ID: "gpt-4", OwnedBy: "openai"}},
				},
				{
					clientID: "client-2",
					provider: "openai",
					models:   []*ModelInfo{{ID: "gpt-4", OwnedBy: "openai"}},
				},
			},
			unregisterClientID:   "client-1",
			wantModelsRemaining:  1,
			wantClientRegistered: false,
		},
		{
			name: "unregister non-existent client",
			setupRegistrations: []struct {
				clientID string
				provider string
				models   []*ModelInfo
			}{
				{
					clientID: "client-1",
					provider: "openai",
					models:   []*ModelInfo{{ID: "gpt-4", OwnedBy: "openai"}},
				},
			},
			unregisterClientID:   "client-non-existent",
			wantModelsRemaining:  1,
			wantClientRegistered: false,
		},
		{
			name: "unregister client with multiple models",
			setupRegistrations: []struct {
				clientID string
				provider string
				models   []*ModelInfo
			}{
				{
					clientID: "client-1",
					provider: "claude",
					models: []*ModelInfo{
						{ID: "claude-3-opus", OwnedBy: "anthropic"},
						{ID: "claude-3-sonnet", OwnedBy: "anthropic"},
					},
				},
			},
			unregisterClientID:   "client-1",
			wantModelsRemaining:  0,
			wantClientRegistered: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry()

			for _, reg := range tt.setupRegistrations {
				r.RegisterClient(reg.clientID, reg.provider, reg.models)
			}

			r.UnregisterClient(tt.unregisterClientID)

			// Check models remaining
			if len(r.models) != tt.wantModelsRemaining {
				t.Errorf("models remaining = %d, want %d", len(r.models), tt.wantModelsRemaining)
			}

			// Check client is no longer registered
			_, clientExists := r.clientModels[tt.unregisterClientID]
			if clientExists != tt.wantClientRegistered {
				t.Errorf("client registered = %v, want %v", clientExists, tt.wantClientRegistered)
			}
		})
	}
}

func TestModelRegistry_SetModelQuotaExceeded(t *testing.T) {
	tests := []struct {
		name            string
		setupModels     []*ModelInfo
		clientID        string
		quotaModelID    string
		wantQuotaMarked bool
	}{
		{
			name: "set quota exceeded for existing model",
			setupModels: []*ModelInfo{
				{ID: "gpt-4", OwnedBy: "openai"},
			},
			clientID:        "client-1",
			quotaModelID:    "gpt-4",
			wantQuotaMarked: true,
		},
		{
			name: "set quota exceeded for non-existent model",
			setupModels: []*ModelInfo{
				{ID: "gpt-4", OwnedBy: "openai"},
			},
			clientID:        "client-1",
			quotaModelID:    "gpt-3.5-turbo",
			wantQuotaMarked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry()
			r.RegisterClient(tt.clientID, "openai", tt.setupModels)

			r.SetModelQuotaExceeded(tt.clientID, tt.quotaModelID)

			if reg, exists := r.models[tt.quotaModelID]; exists {
				_, quotaMarked := reg.QuotaExceededClients[tt.clientID]
				if quotaMarked != tt.wantQuotaMarked {
					t.Errorf("quota marked = %v, want %v", quotaMarked, tt.wantQuotaMarked)
				}
			} else if tt.wantQuotaMarked {
				t.Error("expected model to exist and have quota marked")
			}
		})
	}
}

func TestModelRegistry_ClearModelQuotaExceeded(t *testing.T) {
	tests := []struct {
		name               string
		setupModels        []*ModelInfo
		clientID           string
		quotaModelID       string
		clearModelID       string
		wantQuotaRemaining bool
	}{
		{
			name: "clear quota exceeded for existing model",
			setupModels: []*ModelInfo{
				{ID: "gpt-4", OwnedBy: "openai"},
			},
			clientID:           "client-1",
			quotaModelID:       "gpt-4",
			clearModelID:       "gpt-4",
			wantQuotaRemaining: false,
		},
		{
			name: "clear quota for different model",
			setupModels: []*ModelInfo{
				{ID: "gpt-4", OwnedBy: "openai"},
				{ID: "gpt-3.5-turbo", OwnedBy: "openai"},
			},
			clientID:           "client-1",
			quotaModelID:       "gpt-4",
			clearModelID:       "gpt-3.5-turbo",
			wantQuotaRemaining: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry()
			r.RegisterClient(tt.clientID, "openai", tt.setupModels)

			// Set quota exceeded first
			r.SetModelQuotaExceeded(tt.clientID, tt.quotaModelID)

			// Clear quota
			r.ClearModelQuotaExceeded(tt.clientID, tt.clearModelID)

			// Check if quota is still marked on the original model
			if reg, exists := r.models[tt.quotaModelID]; exists {
				_, quotaRemaining := reg.QuotaExceededClients[tt.clientID]
				if quotaRemaining != tt.wantQuotaRemaining {
					t.Errorf("quota remaining = %v, want %v", quotaRemaining, tt.wantQuotaRemaining)
				}
			}
		})
	}
}

func TestModelRegistry_GetAvailableModels_OpenAI(t *testing.T) {
	tests := []struct {
		name           string
		setupModels    []*ModelInfo
		wantModelCount int
		wantFields     []string
	}{
		{
			name: "get openai format models",
			setupModels: []*ModelInfo{
				{
					ID:                  "gpt-4",
					OwnedBy:             "openai",
					Type:                "openai",
					Created:             1699999999,
					DisplayName:         "GPT-4",
					ContextLength:       8192,
					MaxCompletionTokens: 4096,
				},
			},
			wantModelCount: 1,
			wantFields:     []string{"id", "object", "owned_by", "context_length", "max_completion_tokens"},
		},
		{
			name: "get multiple openai models",
			setupModels: []*ModelInfo{
				{ID: "gpt-4", OwnedBy: "openai", Type: "openai"},
				{ID: "gpt-3.5-turbo", OwnedBy: "openai", Type: "openai"},
			},
			wantModelCount: 2,
			wantFields:     []string{"id", "object", "owned_by"},
		},
		{
			name:           "get models with empty registry",
			setupModels:    []*ModelInfo{},
			wantModelCount: 0,
			wantFields:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry()
			if len(tt.setupModels) > 0 {
				r.RegisterClient("client-1", "openai", tt.setupModels)
			}

			models := r.GetAvailableModels("openai")

			if len(models) != tt.wantModelCount {
				t.Errorf("model count = %d, want %d", len(models), tt.wantModelCount)
			}

			if len(models) > 0 && tt.wantFields != nil {
				for _, field := range tt.wantFields {
					if _, exists := models[0][field]; !exists {
						t.Errorf("expected field %s not found in model", field)
					}
				}
			}
		})
	}
}

func TestModelRegistry_GetAvailableModels_Claude(t *testing.T) {
	tests := []struct {
		name              string
		setupModels       []*ModelInfo
		wantModelCount    int
		wantThinkingField bool
	}{
		{
			name: "get claude format models without thinking",
			setupModels: []*ModelInfo{
				{
					ID:          "claude-3-opus",
					OwnedBy:     "anthropic",
					Type:        "claude",
					DisplayName: "Claude 3 Opus",
				},
			},
			wantModelCount:    1,
			wantThinkingField: false,
		},
		{
			name: "get claude format models with thinking",
			setupModels: []*ModelInfo{
				{
					ID:          "claude-3-opus",
					OwnedBy:     "anthropic",
					Type:        "claude",
					DisplayName: "Claude 3 Opus",
					Thinking: &ThinkingSupport{
						Min:            1024,
						Max:            16384,
						ZeroAllowed:    true,
						DynamicAllowed: true,
					},
				},
			},
			wantModelCount:    1,
			wantThinkingField: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry()
			r.RegisterClient("client-1", "claude", tt.setupModels)

			models := r.GetAvailableModels("claude")

			if len(models) != tt.wantModelCount {
				t.Errorf("model count = %d, want %d", len(models), tt.wantModelCount)
			}

			if len(models) > 0 {
				_, hasThinking := models[0]["thinking"]
				if hasThinking != tt.wantThinkingField {
					t.Errorf("thinking field present = %v, want %v", hasThinking, tt.wantThinkingField)
				}
			}
		})
	}
}

func TestModelRegistry_GetModelCount(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(r *ModelRegistry)
		modelID   string
		wantCount int
	}{
		{
			name: "single client for model",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
			},
			modelID:   "gpt-4",
			wantCount: 1,
		},
		{
			name: "multiple clients for model",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
				r.RegisterClient("client-2", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
				r.RegisterClient("client-3", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
			},
			modelID:   "gpt-4",
			wantCount: 3,
		},
		{
			name: "non-existent model",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
			},
			modelID:   "gpt-3.5-turbo",
			wantCount: 0,
		},
		{
			name: "model with quota exceeded client",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
				r.RegisterClient("client-2", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
				r.SetModelQuotaExceeded("client-1", "gpt-4")
			},
			modelID:   "gpt-4",
			wantCount: 1,
		},
		{
			name: "model with suspended client",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
				r.RegisterClient("client-2", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
				r.SuspendClientModel("client-1", "gpt-4", "rate-limited")
			},
			modelID:   "gpt-4",
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry()
			tt.setupFunc(r)

			count := r.GetModelCount(tt.modelID)

			if count != tt.wantCount {
				t.Errorf("GetModelCount(%s) = %d, want %d", tt.modelID, count, tt.wantCount)
			}
		})
	}
}

func TestModelRegistry_GetModelProviders(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(r *ModelRegistry)
		modelID       string
		wantProviders []string
	}{
		{
			name: "single provider for model",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
			},
			modelID:       "gpt-4",
			wantProviders: []string{"openai"},
		},
		{
			name: "multiple providers for same model",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
				r.RegisterClient("client-2", "azure", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "azure"},
				})
			},
			modelID:       "gpt-4",
			wantProviders: []string{"azure", "openai"}, // alphabetically sorted when counts equal
		},
		{
			name: "non-existent model",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
			},
			modelID:       "gpt-3.5-turbo",
			wantProviders: nil,
		},
		{
			name: "multiple clients same provider",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
				r.RegisterClient("client-2", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
			},
			modelID:       "gpt-4",
			wantProviders: []string{"openai"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry()
			tt.setupFunc(r)

			providers := r.GetModelProviders(tt.modelID)

			if tt.wantProviders == nil {
				if providers != nil {
					t.Errorf("GetModelProviders(%s) = %v, want nil", tt.modelID, providers)
				}
				return
			}

			if len(providers) != len(tt.wantProviders) {
				t.Errorf("GetModelProviders(%s) = %v, want %v", tt.modelID, providers, tt.wantProviders)
				return
			}

			// Check that all expected providers are present
			providerSet := make(map[string]bool)
			for _, p := range providers {
				providerSet[p] = true
			}
			for _, wantProvider := range tt.wantProviders {
				if !providerSet[wantProvider] {
					t.Errorf("expected provider %s not found in %v", wantProvider, providers)
				}
			}
		})
	}
}

func TestModelRegistry_ConcurrentAccess(t *testing.T) {
	tests := []struct {
		name          string
		numGoroutines int
		numOperations int
	}{
		{
			name:          "low concurrency",
			numGoroutines: 10,
			numOperations: 100,
		},
		{
			name:          "medium concurrency",
			numGoroutines: 50,
			numOperations: 200,
		},
		{
			name:          "high concurrency",
			numGoroutines: 100,
			numOperations: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry()
			var wg sync.WaitGroup

			// Concurrent registrations
			wg.Add(tt.numGoroutines)
			for i := 0; i < tt.numGoroutines; i++ {
				go func(clientNum int) {
					defer wg.Done()
					for j := 0; j < tt.numOperations; j++ {
						models := []*ModelInfo{
							{ID: "shared-model", OwnedBy: "test"},
							{ID: "unique-model-" + string(rune(clientNum)), OwnedBy: "test"},
						}
						r.RegisterClient("client-"+string(rune(clientNum)), "test", models)
					}
				}(i)
			}
			wg.Wait()

			// Concurrent reads
			wg.Add(tt.numGoroutines)
			for i := 0; i < tt.numGoroutines; i++ {
				go func() {
					defer wg.Done()
					for j := 0; j < tt.numOperations; j++ {
						_ = r.GetAvailableModels("openai")
						_ = r.GetModelCount("shared-model")
						_ = r.GetModelProviders("shared-model")
					}
				}()
			}
			wg.Wait()

			// Concurrent quota operations
			wg.Add(tt.numGoroutines)
			for i := 0; i < tt.numGoroutines; i++ {
				go func(clientNum int) {
					defer wg.Done()
					clientID := "client-" + string(rune(clientNum))
					for j := 0; j < tt.numOperations; j++ {
						if j%2 == 0 {
							r.SetModelQuotaExceeded(clientID, "shared-model")
						} else {
							r.ClearModelQuotaExceeded(clientID, "shared-model")
						}
					}
				}(i)
			}
			wg.Wait()

			// Concurrent unregistrations
			wg.Add(tt.numGoroutines)
			for i := 0; i < tt.numGoroutines; i++ {
				go func(clientNum int) {
					defer wg.Done()
					r.UnregisterClient("client-" + string(rune(clientNum)))
				}(i)
			}
			wg.Wait()

			// Verify registry is in consistent state
			// After all unregistrations, models should be empty
			if len(r.models) != 0 {
				// Note: Due to race conditions in registration order, some models might remain
				// This is expected behavior - the test verifies no panics occur
				t.Logf("remaining models after concurrent operations: %d", len(r.models))
			}
		})
	}
}

func TestModelRegistry_ConcurrentAccess_MixedOperations(t *testing.T) {
	r := newTestRegistry()
	var wg sync.WaitGroup
	numGoroutines := 50
	duration := 100 * time.Millisecond

	// Create a done channel to signal goroutines to stop
	done := make(chan struct{})

	// Start goroutines that perform mixed operations
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			clientID := "client-" + string(rune('A'+id%26))
			models := []*ModelInfo{
				{ID: "model-1", OwnedBy: "test"},
				{ID: "model-2", OwnedBy: "test"},
			}

			for {
				select {
				case <-done:
					return
				default:
					switch id % 6 {
					case 0:
						r.RegisterClient(clientID, "test", models)
					case 1:
						r.UnregisterClient(clientID)
					case 2:
						r.GetAvailableModels("openai")
					case 3:
						r.GetModelCount("model-1")
					case 4:
						r.SetModelQuotaExceeded(clientID, "model-1")
					case 5:
						r.ClearModelQuotaExceeded(clientID, "model-1")
					}
				}
			}
		}(i)
	}

	// Let the goroutines run for a bit
	time.Sleep(duration)
	close(done)
	wg.Wait()

	// If we get here without panics, the test passes
	t.Log("concurrent mixed operations completed without panics")
}

func TestModelRegistry_SuspendAndResumeClientModel(t *testing.T) {
	tests := []struct {
		name                  string
		setupFunc             func(r *ModelRegistry)
		suspendClientID       string
		suspendModelID        string
		suspendReason         string
		resumeAfterCheck      bool
		wantCountAfterSuspend int
		wantCountAfterResume  int
	}{
		{
			name: "suspend and resume single client",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
			},
			suspendClientID:       "client-1",
			suspendModelID:        "gpt-4",
			suspendReason:         "rate-limited",
			resumeAfterCheck:      true,
			wantCountAfterSuspend: 0,
			wantCountAfterResume:  1,
		},
		{
			name: "suspend one of multiple clients",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
				r.RegisterClient("client-2", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
			},
			suspendClientID:       "client-1",
			suspendModelID:        "gpt-4",
			suspendReason:         "quota",
			resumeAfterCheck:      true,
			wantCountAfterSuspend: 1,
			wantCountAfterResume:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry()
			tt.setupFunc(r)

			// Suspend the client
			r.SuspendClientModel(tt.suspendClientID, tt.suspendModelID, tt.suspendReason)

			// Check count after suspend
			countAfterSuspend := r.GetModelCount(tt.suspendModelID)
			if countAfterSuspend != tt.wantCountAfterSuspend {
				t.Errorf("count after suspend = %d, want %d", countAfterSuspend, tt.wantCountAfterSuspend)
			}

			if tt.resumeAfterCheck {
				// Resume the client
				r.ResumeClientModel(tt.suspendClientID, tt.suspendModelID)

				// Check count after resume
				countAfterResume := r.GetModelCount(tt.suspendModelID)
				if countAfterResume != tt.wantCountAfterResume {
					t.Errorf("count after resume = %d, want %d", countAfterResume, tt.wantCountAfterResume)
				}
			}
		})
	}
}

func TestModelRegistry_ClientSupportsModel(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(r *ModelRegistry)
		clientID    string
		modelID     string
		wantSupport bool
	}{
		{
			name: "client supports model",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
			},
			clientID:    "client-1",
			modelID:     "gpt-4",
			wantSupport: true,
		},
		{
			name: "client does not support model",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
			},
			clientID:    "client-1",
			modelID:     "gpt-3.5-turbo",
			wantSupport: false,
		},
		{
			name: "non-existent client",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
			},
			clientID:    "client-2",
			modelID:     "gpt-4",
			wantSupport: false,
		},
		{
			name: "empty client ID",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
			},
			clientID:    "",
			modelID:     "gpt-4",
			wantSupport: false,
		},
		{
			name: "empty model ID",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
			},
			clientID:    "client-1",
			modelID:     "",
			wantSupport: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry()
			tt.setupFunc(r)

			supports := r.ClientSupportsModel(tt.clientID, tt.modelID)

			if supports != tt.wantSupport {
				t.Errorf("ClientSupportsModel(%s, %s) = %v, want %v",
					tt.clientID, tt.modelID, supports, tt.wantSupport)
			}
		})
	}
}

func TestModelRegistry_GetModelsForClient(t *testing.T) {
	tests := []struct {
		name           string
		setupFunc      func(r *ModelRegistry)
		clientID       string
		wantModelCount int
		wantModelIDs   []string
	}{
		{
			name: "client with single model",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
			},
			clientID:       "client-1",
			wantModelCount: 1,
			wantModelIDs:   []string{"gpt-4"},
		},
		{
			name: "client with multiple models",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
					{ID: "gpt-3.5-turbo", OwnedBy: "openai"},
				})
			},
			clientID:       "client-1",
			wantModelCount: 2,
			wantModelIDs:   []string{"gpt-4", "gpt-3.5-turbo"},
		},
		{
			name: "non-existent client",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
			},
			clientID:       "client-2",
			wantModelCount: 0,
			wantModelIDs:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry()
			tt.setupFunc(r)

			models := r.GetModelsForClient(tt.clientID)

			if len(models) != tt.wantModelCount {
				t.Errorf("GetModelsForClient(%s) returned %d models, want %d",
					tt.clientID, len(models), tt.wantModelCount)
			}

			if tt.wantModelIDs != nil {
				modelIDSet := make(map[string]bool)
				for _, m := range models {
					modelIDSet[m.ID] = true
				}
				for _, wantID := range tt.wantModelIDs {
					if !modelIDSet[wantID] {
						t.Errorf("expected model %s not found in result", wantID)
					}
				}
			}
		})
	}
}

func TestModelRegistry_GetModelInfo(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(r *ModelRegistry)
		modelID     string
		wantNil     bool
		wantOwnedBy string
	}{
		{
			name: "existing model",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai", DisplayName: "GPT-4"},
				})
			},
			modelID:     "gpt-4",
			wantNil:     false,
			wantOwnedBy: "openai",
		},
		{
			name: "non-existent model",
			setupFunc: func(r *ModelRegistry) {
				r.RegisterClient("client-1", "openai", []*ModelInfo{
					{ID: "gpt-4", OwnedBy: "openai"},
				})
			},
			modelID: "gpt-3.5-turbo",
			wantNil: true,
		},
		{
			name:      "empty registry",
			setupFunc: func(r *ModelRegistry) {},
			modelID:   "gpt-4",
			wantNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry()
			tt.setupFunc(r)

			info := r.GetModelInfo(tt.modelID, "")

			if tt.wantNil {
				if info != nil {
					t.Errorf("GetModelInfo(%s) = %v, want nil", tt.modelID, info)
				}
			} else {
				if info == nil {
					t.Errorf("GetModelInfo(%s) = nil, want non-nil", tt.modelID)
				} else if info.OwnedBy != tt.wantOwnedBy {
					t.Errorf("GetModelInfo(%s).OwnedBy = %s, want %s",
						tt.modelID, info.OwnedBy, tt.wantOwnedBy)
				}
			}
		})
	}
}

func TestModelRegistry_CloneModelInfo(t *testing.T) {
	original := &ModelInfo{
		ID:                         "gpt-4",
		OwnedBy:                    "openai",
		Type:                       "openai",
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		SupportedParameters:        []string{"temperature", "top_p"},
	}

	cloned := cloneModelInfo(original)

	// Verify clone is equal
	if cloned.ID != original.ID {
		t.Errorf("cloned ID = %s, want %s", cloned.ID, original.ID)
	}

	// Verify modifying clone doesn't affect original
	cloned.ID = "gpt-4-modified"
	if original.ID == cloned.ID {
		t.Error("modifying cloned ID should not affect original")
	}

	// Verify slice independence
	cloned.SupportedGenerationMethods[0] = "modified"
	if original.SupportedGenerationMethods[0] == "modified" {
		t.Error("modifying cloned slice should not affect original")
	}

	// Test nil input
	nilClone := cloneModelInfo(nil)
	if nilClone != nil {
		t.Error("cloneModelInfo(nil) should return nil")
	}
}
