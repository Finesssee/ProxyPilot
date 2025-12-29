package desktopctl

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/embedded"
)

var (
	embeddedMode   bool
	embeddedMu     sync.RWMutex
	embeddedConfig string
)

// SetEmbeddedMode enables or disables embedded server mode.
// When enabled, Start/Stop/Restart will use the in-process server
// instead of spawning a separate engine process.
func SetEmbeddedMode(enabled bool) {
	embeddedMu.Lock()
	defer embeddedMu.Unlock()
	embeddedMode = enabled
}

// IsEmbeddedMode returns whether embedded mode is enabled.
func IsEmbeddedMode() bool {
	embeddedMu.RLock()
	defer embeddedMu.RUnlock()
	return embeddedMode
}

func StatusFor(configPath string) (Status, error) {
	statePath := defaultStatePath()
	s, _ := loadState(statePath)

	resolvedConfig := strings.TrimSpace(configPath)
	if resolvedConfig == "" && s != nil {
		resolvedConfig = s.ConfigPath
	}
	if resolvedConfig == "" {
		return Status{Running: false, Managed: false}, nil
	}

	port, err := loadPort(resolvedConfig)
	if err != nil {
		return Status{Running: false, Managed: s != nil, PID: pidOrZero(s), ConfigPath: resolvedConfig, ExePath: exeOrEmpty(s), StartedAt: startedAtOrZero(s), LastError: err.Error()}, nil
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	healthErr := checkHealth(baseURL)

	running := healthErr == nil

	// Check if we're in embedded mode
	isEmbedded := IsEmbeddedMode() || embedded.GlobalIsRunning()

	managed := s != nil && (s.PID > 0 || isEmbedded)
	if managed && !isEmbedded {
		if alive, _ := isProcessAlive(s.PID); !alive {
			managed = false
		}
	}
	if s != nil && !isEmbedded {
		repairStateIfStale(statePath, s, running, managed)
	}

	out := Status{
		Running:        running,
		Version:        buildinfo.Version,
		Managed:        managed,
		AutoStartProxy: s != nil && s.AutoStartProxy,
		PID:            pidOrZero(s),
		Port:           port,
		ThinkingPort:   8317,
		BaseURL:        baseURL,
		ConfigPath:     resolvedConfig,
		ExePath:        exeOrEmpty(s),
		StartedAt:      startedAtOrZero(s),
	}
	if inUse, _ := isLocalPortInUse(8317); inUse {
		out.ThinkingRunning = true
	}
	if healthErr != nil {
		out.LastError = healthErr.Error()
	}
	return out, nil
}

func Start(opts StartOptions) (Status, error) {
	// Use embedded mode if enabled or explicitly requested
	if opts.Embedded || IsEmbeddedMode() {
		return startEmbedded(opts)
	}
	return startSubprocess(opts)
}

// startEmbedded starts the proxy server in-process using the embedded package.
func startEmbedded(opts StartOptions) (Status, error) {
	statePath := defaultStatePath()
	prev, _ := loadState(statePath)
	configPath, err := resolveConfigPath(opts.RepoRoot, opts.ConfigPath)
	if err != nil {
		return Status{}, err
	}
	if _, err := os.Stat(configPath); err != nil {
		return Status{}, fmt.Errorf("config not found: %s", configPath)
	}

	port, err := loadPort(configPath)
	if err != nil {
		return Status{}, err
	}

	if inUse, _ := isLocalPortInUse(port); inUse {
		return StatusFor(configPath)
	}

	pw, errPw := getOrCreateManagementPassword()
	if errPw != nil {
		return Status{}, errPw
	}

	// Set management password env var for the embedded server
	os.Setenv("MANAGEMENT_PASSWORD", pw)

	// Start embedded server
	if err := embedded.StartGlobal(configPath, pw); err != nil {
		return Status{}, err
	}

	// Store config path for embedded mode
	embeddedMu.Lock()
	embeddedConfig = configPath
	embeddedMu.Unlock()

	// Save state (with PID 0 to indicate embedded mode)
	s := &state{
		PID:        0, // 0 indicates embedded mode
		ConfigPath: configPath,
		ExePath:    "", // No separate exe in embedded mode
		StartedAt:  embedded.GlobalServer().StartedAt(),
	}
	if prev != nil {
		s.AutoStartProxy = prev.AutoStartProxy
		s.OAuthPrivate = prev.OAuthPrivate
		s.ManagementPassword = prev.ManagementPassword
	}
	_ = saveState(statePath, s)

	return StatusFor(configPath)
}

// startSubprocess starts the proxy server as a separate process (legacy mode).
func startSubprocess(opts StartOptions) (Status, error) {
	statePath := defaultStatePath()
	prev, _ := loadState(statePath)
	configPath, err := resolveConfigPath(opts.RepoRoot, opts.ConfigPath)
	if err != nil {
		return Status{}, err
	}
	exePath := resolveExePath(opts.RepoRoot, opts.ExePath)
	if exePath == "" {
		return Status{}, fmt.Errorf("exe path is required")
	}
	if _, err := os.Stat(exePath); err != nil {
		return Status{}, fmt.Errorf("binary not found: %s", exePath)
	}
	if _, err := os.Stat(configPath); err != nil {
		return Status{}, fmt.Errorf("config not found: %s", configPath)
	}

	port, err := loadPort(configPath)
	if err != nil {
		return Status{}, err
	}

	if inUse, _ := isLocalPortInUse(port); inUse {
		// If something is already listening, treat it as running and don't stomp on it.
		return StatusFor(configPath)
	}

	logDir := strings.TrimSpace(opts.LogDir)
	if logDir == "" {
		if strings.TrimSpace(opts.RepoRoot) != "" {
			logDir = filepath.Join(opts.RepoRoot, "logs")
		} else {
			logDir = filepath.Join(filepath.Dir(configPath), "logs")
		}
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return Status{}, err
	}

	stdoutLog := filepath.Join(logDir, "proxypilot-engine.out.log")
	stderrLog := filepath.Join(logDir, "proxypilot-engine.err.log")

	stdoutFile, err := os.OpenFile(stdoutLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return Status{}, err
	}
	defer stdoutFile.Close()

	stderrFile, err := os.OpenFile(stderrLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return Status{}, err
	}
	defer stderrFile.Close()

	cmd := exec.Command(exePath, "-config", configPath)
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile
	if strings.TrimSpace(opts.RepoRoot) != "" {
		cmd.Dir = opts.RepoRoot
	}
	setSysProcAttr(cmd)
	// Enable management endpoints for the local ProxyPilot UX by setting a per-user secret.
	// This is used to unlock /v0/management routes while keeping remote management disabled by default.
	pw, errPw := getOrCreateManagementPassword()
	if errPw != nil {
		return Status{}, errPw
	}
	cmd.Env = append(os.Environ(), "MANAGEMENT_PASSWORD="+pw)

	if err := cmd.Start(); err != nil {
		return Status{}, err
	}

	s := &state{
		PID:        cmd.Process.Pid,
		ConfigPath: configPath,
		ExePath:    exePath,
		StartedAt:  time.Now(),
	}
	if prev != nil {
		s.AutoStartProxy = prev.AutoStartProxy
		s.OAuthPrivate = prev.OAuthPrivate
		s.ManagementPassword = prev.ManagementPassword
	}
	_ = saveState(statePath, s)

	// Give it a moment to bind before status/health checks.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st, _ := StatusFor(configPath)
		if st.Running {
			return st, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return StatusFor(configPath)
}

func Stop(opts StopOptions) error {
	statePath := defaultStatePath()
	s, _ := loadState(statePath)
	pid := opts.PID
	if pid == 0 && s != nil {
		pid = s.PID
	}

	// Check if embedded server is running
	if IsEmbeddedMode() || embedded.GlobalIsRunning() {
		if err := embedded.StopGlobal(); err != nil {
			return err
		}
		// Clear embedded config
		embeddedMu.Lock()
		embeddedConfig = ""
		embeddedMu.Unlock()
		// Update state
		if s == nil {
			s = &state{}
		}
		s.PID = 0
		s.StartedAt = time.Time{}
		_ = saveState(statePath, s)
		return nil
	}

	// Legacy subprocess mode
	if pid <= 0 {
		return nil
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		_ = deleteState(statePath)
		return nil
	}

	// Best-effort graceful: attempt to interrupt on non-Windows.
	if runtime.GOOS != "windows" {
		_ = p.Signal(os.Interrupt)
		time.Sleep(200 * time.Millisecond)
	}

	_ = p.Kill()
	// Preserve preferences (e.g. auto-start proxy) by keeping state but clearing runtime fields.
	if s == nil {
		s = &state{}
	}
	s.PID = 0
	s.StartedAt = time.Time{}
	_ = saveState(statePath, s)
	return nil
}

func Restart(opts StartOptions) (Status, error) {
	_ = Stop(StopOptions{})
	return Start(opts)
}

func OpenManagementUI(configPath string) error {
	st, err := StatusFor(configPath)
	if err != nil {
		return err
	}
	if st.BaseURL == "" {
		return errors.New("proxy base URL not available")
	}
	return OpenBrowser(managementURL(st.BaseURL))
}

func OpenLogsFolder(repoRoot, configPath string) error {
	logDir := ""
	if strings.TrimSpace(repoRoot) != "" {
		logDir = filepath.Join(repoRoot, "logs")
	} else if strings.TrimSpace(configPath) != "" {
		logDir = filepath.Join(filepath.Dir(configPath), "logs")
	} else {
		logDir = filepath.Join(".", "logs")
	}
	_ = os.MkdirAll(logDir, 0o755)
	return OpenFolder(logDir)
}

func checkHealth(baseURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("health check: %s", resp.Status)
	}
	return nil
}

func isLocalPortInUse(port int) (bool, error) {
	if port <= 0 {
		return false, nil
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return true, nil
	}
	var ne *net.OpError
	if errors.As(err, &ne) {
		return false, nil
	}
	return false, nil
}

func pidOrZero(s *state) int {
	if s == nil {
		return 0
	}
	return s.PID
}

func repairStateIfStale(statePath string, s *state, running bool, managed bool) {
	if s == nil || strings.TrimSpace(statePath) == "" {
		return
	}
	// If we recorded a PID but it's no longer alive, clear runtime fields so we don't keep showing
	// a managed PID that doesn't exist after crashes/reboots.
	if s.PID > 0 && !managed {
		if !running {
			s.PID = 0
			s.StartedAt = time.Time{}
			_ = saveState(statePath, s)
		}
	}
}

func exeOrEmpty(s *state) string {
	if s == nil {
		return ""
	}
	return s.ExePath
}

func startedAtOrZero(s *state) time.Time {
	if s == nil {
		return time.Time{}
	}
	return s.StartedAt
}
