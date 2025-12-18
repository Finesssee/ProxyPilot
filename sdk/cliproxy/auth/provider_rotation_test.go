package auth

import "testing"

func TestRotateProviders_Gemini3ProKeepsOrder(t *testing.T) {
	m := NewManager(nil, nil, nil)

	providers := []string{"antigravity", "gemini-cli"}
	model := "gemini-3-pro-preview"

	first := m.rotateProviders(model, providers)
	if len(first) != 2 || first[0] != "antigravity" || first[1] != "gemini-cli" {
		t.Fatalf("expected stable provider order, got %#v", first)
	}

	// Even after advancing, order should remain stable.
	m.advanceProviderCursor(model, providers)
	second := m.rotateProviders(model, providers)
	if len(second) != 2 || second[0] != "antigravity" || second[1] != "gemini-cli" {
		t.Fatalf("expected stable provider order after advance, got %#v", second)
	}
}

func TestRotateProviders_Gemini3FlashKeepsOrder(t *testing.T) {
	m := NewManager(nil, nil, nil)

	providers := []string{"antigravity", "gemini-cli"}
	model := "gemini-3-flash-preview"

	first := m.rotateProviders(model, providers)
	if len(first) != 2 || first[0] != "antigravity" || first[1] != "gemini-cli" {
		t.Fatalf("expected stable provider order, got %#v", first)
	}

	// Even after advancing, order should remain stable.
	m.advanceProviderCursor(model, providers)
	second := m.rotateProviders(model, providers)
	if len(second) != 2 || second[0] != "antigravity" || second[1] != "gemini-cli" {
		t.Fatalf("expected stable provider order after advance, got %#v", second)
	}
}

func TestRotateProviders_OtherModelsRotate(t *testing.T) {
	m := NewManager(nil, nil, nil)

	providers := []string{"a", "b", "c"}
	model := "gpt-5.2"

	first := m.rotateProviders(model, providers)
	if first[0] != "a" {
		t.Fatalf("expected initial rotation to be identity, got %#v", first)
	}
	m.advanceProviderCursor(model, providers)

	second := m.rotateProviders(model, providers)
	if second[0] != "b" || second[1] != "c" || second[2] != "a" {
		t.Fatalf("expected rotation after advance, got %#v", second)
	}
}
