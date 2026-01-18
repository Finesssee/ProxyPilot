package updates

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPollerCreation(t *testing.T) {
	callback := func(info *UpdateInfo) {}

	// Test with default interval
	p := NewPoller(0, callback)
	if p.interval != 24*time.Hour {
		t.Errorf("expected default interval of 24h, got %v", p.interval)
	}

	// Test with custom interval
	p2 := NewPoller(1*time.Hour, callback)
	if p2.interval != 1*time.Hour {
		t.Errorf("expected interval of 1h, got %v", p2.interval)
	}

	if p2.channel != "stable" {
		t.Errorf("expected default channel 'stable', got %v", p2.channel)
	}
}

func TestPollerSetChannel(t *testing.T) {
	p := NewPoller(1*time.Hour, nil)

	p.SetChannel("prerelease")
	if p.channel != "prerelease" {
		t.Errorf("expected channel 'prerelease', got %v", p.channel)
	}
}

func TestPollerStartStop(t *testing.T) {
	callCount := 0
	callback := func(info *UpdateInfo) {
		callCount++
	}

	p := NewPoller(100*time.Millisecond, callback)

	// Should not be running initially
	if p.IsRunning() {
		t.Error("poller should not be running initially")
	}

	p.Start()

	// Should be running now
	if !p.IsRunning() {
		t.Error("poller should be running after Start()")
	}

	// Starting again should be a no-op
	p.Start()
	if !p.IsRunning() {
		t.Error("poller should still be running after second Start()")
	}

	p.Stop()

	// Should not be running after Stop
	if p.IsRunning() {
		t.Error("poller should not be running after Stop()")
	}
}

func TestRollbackDataDir(t *testing.T) {
	dir, err := getDataDir()
	if err != nil {
		t.Fatalf("getDataDir() failed: %v", err)
	}

	if dir == "" {
		t.Error("getDataDir() returned empty string")
	}

	// Should be under user config dir
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("getDataDir() returned non-existent directory")
	}
}

func TestRollbackInfoSaveLoad(t *testing.T) {
	// Create a temporary backup file
	tempDir := t.TempDir()
	fakeExe := filepath.Join(tempDir, "test.exe")
	fakeBackup := fakeExe + ".old"

	// Create fake files
	if err := os.WriteFile(fakeExe, []byte("current"), 0755); err != nil {
		t.Fatalf("failed to create fake exe: %v", err)
	}
	if err := os.WriteFile(fakeBackup, []byte("backup"), 0755); err != nil {
		t.Fatalf("failed to create fake backup: %v", err)
	}

	// Note: This test would require mocking os.Executable()
	// For now, just test that GetRollbackInfo handles missing file gracefully
	info, err := GetRollbackInfo()
	if err != nil {
		t.Fatalf("GetRollbackInfo() should not fail on missing file: %v", err)
	}
	// info may be nil if no rollback is saved, which is fine
	_ = info
}

func TestCanRollback(t *testing.T) {
	// Without any saved rollback info, should return false
	// Note: This might return true if a previous test left state
	// In a clean environment, it should be false
	result := CanRollback()
	// Just verify the function doesn't panic
	_ = result
}

func TestRecordStartup(t *testing.T) {
	result, err := RecordStartup()
	if err != nil {
		t.Fatalf("RecordStartup() failed: %v", err)
	}

	if result == nil {
		t.Fatal("RecordStartup() returned nil result")
	}

	if result.StartupCount < 1 {
		t.Errorf("StartupCount should be at least 1, got %d", result.StartupCount)
	}

	if result.LastStartup.IsZero() {
		t.Error("LastStartup should not be zero")
	}
}

func TestMarkHealthy(t *testing.T) {
	// Should not error
	err := MarkHealthy()
	if err != nil {
		t.Fatalf("MarkHealthy() failed: %v", err)
	}

	// After marking healthy, startup count should reset
	result, err := RecordStartup()
	if err != nil {
		t.Fatalf("RecordStartup() after MarkHealthy failed: %v", err)
	}

	// Should start fresh
	if result.StartupCount != 1 {
		t.Errorf("StartupCount should be 1 after MarkHealthy, got %d", result.StartupCount)
	}
}

func TestClearRollbackInfo(t *testing.T) {
	// Should not error even if no rollback info exists
	err := ClearRollbackInfo()
	if err != nil {
		t.Fatalf("ClearRollbackInfo() failed: %v", err)
	}
}

func TestGetAssetForPlatform(t *testing.T) {
	assets := []Asset{
		{Name: "proxypilot-windows-x86_64.exe", DownloadURL: "https://example.com/win64.exe", Size: 1000},
		{Name: "proxypilot-windows-x86_64.exe.sig", DownloadURL: "https://example.com/win64.sig", Size: 100},
		{Name: "proxypilot-linux-x86_64", DownloadURL: "https://example.com/linux64", Size: 1200},
		{Name: "proxypilot-darwin-arm64", DownloadURL: "https://example.com/darwin-arm", Size: 1100},
	}

	binary, sig := GetAssetForPlatform(assets)

	// The result depends on runtime.GOOS/GOARCH
	// Just verify the function doesn't panic and returns something reasonable
	// On Windows x64, both should be non-nil
	_ = binary
	_ = sig
}

func TestDownloadProgressReader(t *testing.T) {
	// Test the progress reader
	data := []byte("hello world")
	var lastProgress DownloadProgress

	pr := &progressReader{
		reader: &mockReader{data: data},
		total:  int64(len(data)),
		callback: func(p DownloadProgress) {
			lastProgress = p
		},
	}

	buf := make([]byte, 5)
	n, _ := pr.Read(buf)

	if n != 5 {
		t.Errorf("expected to read 5 bytes, got %d", n)
	}

	expectedPercent := float64(5) / float64(11) * 100
	if lastProgress.Percent < expectedPercent-0.1 || lastProgress.Percent > expectedPercent+0.1 {
		t.Errorf("expected progress around %.1f%%, got %.1f%%", expectedPercent, lastProgress.Percent)
	}
}

type mockReader struct {
	data   []byte
	offset int
}

func (m *mockReader) Read(p []byte) (n int, err error) {
	if m.offset >= len(m.data) {
		return 0, nil
	}
	n = copy(p, m.data[m.offset:])
	m.offset += n
	return n, nil
}
