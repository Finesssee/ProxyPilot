package translator

import (
	"testing"
)

func TestFallbackRegistry_RegisterChain(t *testing.T) {
	fr := NewFallbackRegistry()

	via := []Format{FormatOpenAI}
	fr.RegisterChain(FormatGemini, FormatClaude, via)

	chain := fr.GetChain(FormatGemini, FormatClaude)
	if chain == nil {
		t.Fatal("GetChain returned nil")
	}
	if len(chain) != 1 {
		t.Errorf("Chain length = %d, want 1", len(chain))
	}
	if chain[0] != FormatOpenAI {
		t.Errorf("Chain[0] = %v, want %v", chain[0], FormatOpenAI)
	}
}

func TestFallbackRegistry_GetChain_NotRegistered(t *testing.T) {
	fr := NewFallbackRegistry()

	chain := fr.GetChain(FormatGemini, FormatClaude)
	if chain != nil {
		t.Error("GetChain should return nil for unregistered chain")
	}
}

func TestFallbackRegistry_GetChain_ReturnsCopy(t *testing.T) {
	fr := NewFallbackRegistry()

	via := []Format{FormatOpenAI, FormatGemini}
	fr.RegisterChain("from", "to", via)

	chain1 := fr.GetChain("from", "to")
	chain2 := fr.GetChain("from", "to")

	// Modify chain1
	chain1[0] = "modified"

	// chain2 should be unaffected
	if chain2[0] == "modified" {
		t.Error("GetChain should return a copy, not the original slice")
	}
}

func TestFallbackRegistry_UnregisterChain(t *testing.T) {
	fr := NewFallbackRegistry()

	fr.RegisterChain(FormatGemini, FormatClaude, []Format{FormatOpenAI})

	// Verify it exists
	if fr.GetChain(FormatGemini, FormatClaude) == nil {
		t.Fatal("Chain should exist before unregister")
	}

	fr.UnregisterChain(FormatGemini, FormatClaude)

	// Verify it's gone
	if fr.GetChain(FormatGemini, FormatClaude) != nil {
		t.Error("Chain should be nil after unregister")
	}
}

func TestFallbackRegistry_UnregisterChain_NonExistent(t *testing.T) {
	fr := NewFallbackRegistry()

	// Should not panic
	fr.UnregisterChain(FormatGemini, FormatClaude)
}

func TestFallbackRegistry_Clone(t *testing.T) {
	fr := NewFallbackRegistry()

	fr.RegisterChain(FormatGemini, FormatClaude, []Format{FormatOpenAI})
	fr.RegisterChain(FormatOpenAI, FormatGemini, []Format{FormatClaude})

	clone := fr.Clone()

	// Verify clone has same chains
	chain1 := clone.GetChain(FormatGemini, FormatClaude)
	if chain1 == nil || len(chain1) != 1 {
		t.Error("Clone should have the same chains")
	}

	chain2 := clone.GetChain(FormatOpenAI, FormatGemini)
	if chain2 == nil || len(chain2) != 1 {
		t.Error("Clone should have all chains")
	}

	// Modify original, clone should be unaffected
	fr.UnregisterChain(FormatGemini, FormatClaude)

	if clone.GetChain(FormatGemini, FormatClaude) == nil {
		t.Error("Clone should be independent of original")
	}
}

func TestBuildFullPath(t *testing.T) {
	tests := []struct {
		name     string
		from     Format
		to       Format
		via      []Format
		expected []Format
	}{
		{
			name:     "Single intermediate",
			from:     FormatGemini,
			to:       FormatClaude,
			via:      []Format{FormatOpenAI},
			expected: []Format{FormatGemini, FormatOpenAI, FormatClaude},
		},
		{
			name:     "Multiple intermediates",
			from:     "A",
			to:       "D",
			via:      []Format{"B", "C"},
			expected: []Format{"A", "B", "C", "D"},
		},
		{
			name:     "No intermediates",
			from:     FormatOpenAI,
			to:       FormatClaude,
			via:      []Format{},
			expected: []Format{FormatOpenAI, FormatClaude},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildFullPath(tt.from, tt.to, tt.via)

			if len(result) != len(tt.expected) {
				t.Fatalf("Path length = %d, want %d", len(result), len(tt.expected))
			}

			for i, f := range result {
				if f != tt.expected[i] {
					t.Errorf("Path[%d] = %v, want %v", i, f, tt.expected[i])
				}
			}
		})
	}
}

func TestTranslateRequestViaChain(t *testing.T) {
	reg := NewRegistry()

	// Register A -> B
	reg.Register("A", "B", func(model string, data []byte, stream bool) []byte {
		return append(data, []byte("-B")...)
	}, ResponseTransform{})

	// Register B -> C
	reg.Register("B", "C", func(model string, data []byte, stream bool) []byte {
		return append(data, []byte("-C")...)
	}, ResponseTransform{})

	// Register C -> D
	reg.Register("C", "D", func(model string, data []byte, stream bool) []byte {
		return append(data, []byte("-D")...)
	}, ResponseTransform{})

	path := []Format{"A", "B", "C", "D"}
	result := reg.TranslateRequestViaChain(path, "model", []byte("start"), false)

	expected := "start-B-C-D"
	if string(result) != expected {
		t.Errorf("Result = %q, want %q", string(result), expected)
	}
}

func TestTranslateRequestViaChain_ShortPath(t *testing.T) {
	reg := NewRegistry()

	// Path with less than 2 elements should return original
	result := reg.TranslateRequestViaChain([]Format{"A"}, "model", []byte("data"), false)
	if string(result) != "data" {
		t.Error("Short path should return original data")
	}

	result = reg.TranslateRequestViaChain([]Format{}, "model", []byte("data"), false)
	if string(result) != "data" {
		t.Error("Empty path should return original data")
	}
}

func TestPackageLevelFallbackFunctions(t *testing.T) {
	// Clear any existing chains
	UnregisterFallbackChain(FormatGemini, FormatClaude)

	RegisterFallbackChain(FormatGemini, FormatClaude, []Format{FormatOpenAI})

	chain := GetFallbackChain(FormatGemini, FormatClaude)
	if chain == nil {
		t.Error("Package-level RegisterFallbackChain should work")
	}
	if len(chain) != 1 || chain[0] != FormatOpenAI {
		t.Error("Chain should contain the registered intermediate")
	}

	UnregisterFallbackChain(FormatGemini, FormatClaude)
	if GetFallbackChain(FormatGemini, FormatClaude) != nil {
		t.Error("Package-level UnregisterFallbackChain should work")
	}
}

func TestDefaultFallbackRegistry(t *testing.T) {
	fr := DefaultFallbackRegistry()
	if fr == nil {
		t.Error("DefaultFallbackRegistry should not return nil")
	}
}

func TestPackageLevelTranslateRequestViaChain(t *testing.T) {
	// This just verifies the package-level function doesn't panic
	result := TranslateRequestViaChain([]Format{FormatOpenAI, FormatClaude}, "model", []byte("test"), false)
	if result == nil {
		t.Error("TranslateRequestViaChain should not return nil")
	}
}
