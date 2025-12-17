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
