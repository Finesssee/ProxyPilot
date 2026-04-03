package startupconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfigPathPrefersExecutableTemplate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	installRoot := filepath.Join(tmpDir, "ProxyPilot")
	exePath := filepath.Join(installRoot, "ProxyPilot.exe")
	workingDir := filepath.Join(tmpDir, "home")

	if err := os.MkdirAll(filepath.Dir(exePath), 0o755); err != nil {
		t.Fatalf("mkdir exe dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installRoot, "config.example.yaml"), []byte("port: 8317\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	if err := os.MkdirAll(workingDir, 0o755); err != nil {
		t.Fatalf("mkdir working dir: %v", err)
	}

	got := ResolveConfigPath("", workingDir, exePath)

	if !got.UsedDefault {
		t.Fatalf("expected implicit default path")
	}
	if want := filepath.Join(installRoot, "config.yaml"); got.ConfigPath != want {
		t.Fatalf("config path = %q, want %q", got.ConfigPath, want)
	}
	if want := filepath.Join(installRoot, "config.example.yaml"); got.TemplatePath != want {
		t.Fatalf("template path = %q, want %q", got.TemplatePath, want)
	}
}

func TestResolveConfigPathFallsBackToWorkingDirectoryTemplate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	installRoot := filepath.Join(tmpDir, "ProxyPilot")
	exePath := filepath.Join(installRoot, "ProxyPilot.exe")
	workingDir := filepath.Join(tmpDir, "repo")

	if err := os.MkdirAll(filepath.Dir(exePath), 0o755); err != nil {
		t.Fatalf("mkdir exe dir: %v", err)
	}
	if err := os.MkdirAll(workingDir, 0o755); err != nil {
		t.Fatalf("mkdir working dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workingDir, "config.example.yaml"), []byte("port: 8317\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	got := ResolveConfigPath("", workingDir, exePath)

	if want := filepath.Join(workingDir, "config.yaml"); got.ConfigPath != want {
		t.Fatalf("config path = %q, want %q", got.ConfigPath, want)
	}
	if want := filepath.Join(workingDir, "config.example.yaml"); got.TemplatePath != want {
		t.Fatalf("template path = %q, want %q", got.TemplatePath, want)
	}
}

func TestResolveConfigPathKeepsExplicitPath(t *testing.T) {
	t.Parallel()

	explicit := filepath.Join(t.TempDir(), "custom.yaml")
	got := ResolveConfigPath("  "+explicit+"  ", "/tmp/work", "/tmp/app/proxypilot")

	if got.UsedDefault {
		t.Fatalf("explicit config path should not be marked as default")
	}
	if got.ConfigPath != explicit {
		t.Fatalf("config path = %q, want %q", got.ConfigPath, explicit)
	}
	if got.TemplatePath != "" {
		t.Fatalf("template path = %q, want empty", got.TemplatePath)
	}
}

func TestEnsureDefaultConfigCopiesTemplate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	resolution := Resolution{
		ConfigPath:   filepath.Join(tmpDir, "config.yaml"),
		TemplatePath: filepath.Join(tmpDir, "config.example.yaml"),
		UsedDefault:  true,
	}
	template := []byte("port: 8317\n")
	if err := os.WriteFile(resolution.TemplatePath, template, 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	created, err := EnsureDefaultConfig(resolution)
	if err != nil {
		t.Fatalf("EnsureDefaultConfig returned error: %v", err)
	}
	if !created {
		t.Fatalf("expected default config to be created")
	}

	got, err := os.ReadFile(resolution.ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(got) != string(template) {
		t.Fatalf("config contents = %q, want %q", got, template)
	}
}
