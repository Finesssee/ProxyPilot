package util

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
)

func TestGetProviderName_Gemini3ProPrefersAntigravityThenGeminiCLI(t *testing.T) {
	modelID := "gemini-3-pro-test-routing"

	reg := registry.GetGlobalRegistry()
	reg.RegisterClient("client-antigravity-test-routing", "antigravity", []*registry.ModelInfo{
		{ID: modelID, Object: "model", OwnedBy: "test"},
	})
	reg.RegisterClient("client-gemini-cli-test-routing", "gemini-cli", []*registry.ModelInfo{
		{ID: modelID, Object: "model", OwnedBy: "test"},
	})

	providers := GetProviderName(modelID)
	if len(providers) < 2 {
		t.Fatalf("expected at least two providers, got %#v", providers)
	}
	if providers[0] != "antigravity" || providers[1] != "gemini-cli" {
		t.Fatalf("expected antigravity then gemini-cli, got %#v", providers)
	}
}

func TestGetProviderName_Gemini3FlashPrefersAntigravityThenGeminiCLI(t *testing.T) {
	modelID := "gemini-3-flash-preview"

	reg := registry.GetGlobalRegistry()
	reg.RegisterClient("client-antigravity-test-routing-flash", "antigravity", []*registry.ModelInfo{
		{ID: modelID, Object: "model", OwnedBy: "test"},
	})
	reg.RegisterClient("client-gemini-cli-test-routing-flash", "gemini-cli", []*registry.ModelInfo{
		{ID: modelID, Object: "model", OwnedBy: "test"},
	})

	providers := GetProviderName(modelID)
	if len(providers) < 2 {
		t.Fatalf("expected at least two providers, got %#v", providers)
	}
	if providers[0] != "antigravity" || providers[1] != "gemini-cli" {
		t.Fatalf("expected antigravity then gemini-cli, got %#v", providers)
	}
}

func TestGetProviderName_Gemini3FlashStablePrefersAntigravityThenGeminiCLI(t *testing.T) {
	modelID := "gemini-3-flash"
	reg := registry.GetGlobalRegistry()
	reg.RegisterClient("client-antigravity-test-routing-flash-stable", "antigravity", []*registry.ModelInfo{{ID: modelID}})
	reg.RegisterClient("client-gemini-cli-test-routing-flash-stable", "gemini-cli", []*registry.ModelInfo{{ID: modelID}})

	providers := GetProviderName(modelID)
	if len(providers) < 2 {
		t.Fatalf("expected at least two providers, got %#v", providers)
	}
	if providers[0] != "antigravity" || providers[1] != "gemini-cli" {
		t.Fatalf("expected antigravity then gemini-cli, got %#v", providers)
	}
}
