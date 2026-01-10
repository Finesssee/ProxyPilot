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

	// With atomic rotation, calling rotateProviders again should still keep order
	// for gemini-3 models with both antigravity and gemini-cli.
	second := m.rotateProviders(model, providers)
	if len(second) != 2 || second[0] != "antigravity" || second[1] != "gemini-cli" {
		t.Fatalf("expected stable provider order on second call, got %#v", second)
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

	// With atomic rotation, calling rotateProviders again should still keep order
	// for gemini-3 models with both antigravity and gemini-cli.
	second := m.rotateProviders(model, providers)
	if len(second) != 2 || second[0] != "antigravity" || second[1] != "gemini-cli" {
		t.Fatalf("expected stable provider order on second call, got %#v", second)
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

	// With atomic rotation, the cursor is advanced on each call.
	second := m.rotateProviders(model, providers)
	if second[0] != "b" || second[1] != "c" || second[2] != "a" {
		t.Fatalf("expected rotation after second call, got %#v", second)
	}
}
