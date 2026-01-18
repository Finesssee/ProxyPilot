//go:build windows

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"github.com/jchv/go-webview2"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/api/middleware"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/cmd"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/desktopctl"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/integrations"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/logging"
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/translator"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/trayicon"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/updates"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/winutil"
	configaccess "github.com/router-for-me/CLIProxyAPI/v6/sdk/access/config"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	log "github.com/sirupsen/logrus"
)

const autostartAppName = "ProxyPilot"
const thinkingProxyPort = 8317

var assetServerURL string
var dashboardMu sync.Mutex
var dashboardOpen bool

func main() {
	// Single-instance check - prevent multiple tray apps
	instance, err := winutil.AcquireSingleInstance("ProxyPilotTray")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check for existing instance: %v\n", err)
		os.Exit(1)
	}
	if instance == nil {
		// Another instance is already running
		_ = winutil.ShowToast("ProxyPilot", "Another instance is already running")
		os.Exit(0)
	}
	defer instance.Release()

	var repoRoot string
	var configPath string
	flag.StringVar(&repoRoot, "repo", "", "Repo root (used to locate logs/)")
	flag.StringVar(&configPath, "config", "", "Path to config.yaml (defaults to <repo>/config.yaml)")
	flag.Parse()

	repoRoot, configPath = applyDefaults(repoRoot, configPath)
	run(repoRoot, configPath)
}

func run(repoRoot, configPath string) {
	// Initialize logging
	logging.SetupBaseLogger()

	// Check startup health - auto-rollback on crash loops
	if health, err := updates.RecordStartup(); err == nil {
		if health.ShouldRollback {
			fmt.Fprintf(os.Stderr, "Detected crash loop (%d rapid restarts), triggering auto-rollback\n", health.StartupCount)
			if result, err := updates.Rollback(); err == nil && result.Success {
				_ = winutil.ShowToast("ProxyPilot", "Auto-rollback triggered due to startup failures. Restarting with previous version...")
				// On Windows, execute the rollback script and exit
				if result.NeedsRestart {
					os.Exit(0) // Let the rollback script restart us
				}
			}
		}
	}

	// Load config
	cfg, err := config.LoadConfigOptional(configPath, false)
	if err == nil && cfg != nil {
		logging.ConfigureLogOutput(cfg)
		util.SetLogLevel(cfg)
	}
	if cfg == nil {
		cfg = &config.Config{Port: 8318} // Default config if load fails
	}

	// Register token store
	sdkAuth.RegisterTokenStore(sdkAuth.NewFileTokenStore())

	// Register access providers
	configaccess.Register()

	// Create embedded engine (will be used instead of desktopctl)
	engine := NewEmbeddedEngine()

	// Get or create management password
	password, _ := desktopctl.GetManagementPassword()

	systray.Run(func() {
		thinkingProxy := startThinkingProxy(engine)
		defer thinkingProxy.Close()

		if ico := trayicon.ProxyPilotICO(); len(ico) > 0 {
			systray.SetIcon(ico)
		}
		systray.SetTitle("ProxyPilot")
		systray.SetTooltip("ProxyPilot")

		// Header
		systray.AddMenuItem("ProxyPilot", "ProxyPilot").Disable()
		systray.AddSeparator()

		// Status display item (disabled, updated dynamically)
		statusItem := systray.AddMenuItem("○ Stopped", "Current proxy status")
		statusItem.Disable()
		systray.AddSeparator()

		// Main actions
		openDashboard := systray.AddMenuItem("Open Dashboard", "Open ProxyPilot Dashboard")
		toggleItem := systray.AddMenuItem("Start Proxy", "Start/Stop proxy")
		refreshTokensItem := systray.AddMenuItem("Refresh Tokens", "Refresh all auth tokens")
		copyURLItem := systray.AddMenuItem("Copy API URL", "Copy http://127.0.0.1:8317/v1")
		systray.AddSeparator()

		// Providers submenu - click to login
		providersMenu := systray.AddMenuItem("Providers", "Login to providers")
		claudeLoginItem := providersMenu.AddSubMenuItem("Login Claude", "Login to Claude using OAuth")
		geminiLoginItem := providersMenu.AddSubMenuItem("Login Gemini", "Login to Gemini using OAuth")
		codexLoginItem := providersMenu.AddSubMenuItem("Login Codex", "Login to OpenAI Codex using OAuth")
		qwenLoginItem := providersMenu.AddSubMenuItem("Login Qwen", "Login to Qwen using OAuth")
		antigravityLoginItem := providersMenu.AddSubMenuItem("Login Antigravity", "Login to Antigravity using OAuth")
		kiroLoginItem := providersMenu.AddSubMenuItem("Login Kiro", "Login to Kiro using OAuth")
		minimaxLoginItem := providersMenu.AddSubMenuItem("Login MiniMax", "Add MiniMax API key")
		zhipuLoginItem := providersMenu.AddSubMenuItem("Login Zhipu", "Add Zhipu AI API key")

		// Accounts submenu
		accountsMenu := systray.AddMenuItem("Accounts", "Account management")
		copyAccountListItem := accountsMenu.AddSubMenuItem("Copy Account List", "Copy detailed account list to clipboard")
		cleanupExpiredItem := accountsMenu.AddSubMenuItem("Cleanup Expired", "Remove expired auth tokens")
		exportAccountsItem := accountsMenu.AddSubMenuItem("Export Accounts...", "Export accounts to file")
		importAccountsItem := accountsMenu.AddSubMenuItem("Import Accounts...", "Import accounts from file")

		// Diagnostics submenu
		diagMenu := systray.AddMenuItem("Diagnostics", "Diagnostic tools")
		copyDiagItem := diagMenu.AddSubMenuItem("Copy Diagnostics", "Copy diagnostics to clipboard")
		copyStatusItem := diagMenu.AddSubMenuItem("Copy Account Status", "Copy account health summary to clipboard")
		copyRateLimitsItem := diagMenu.AddSubMenuItem("Copy Rate Limits", "Copy rate limit status to clipboard")
		copyUsageItem := diagMenu.AddSubMenuItem("Copy Usage Stats", "Copy usage statistics to clipboard")
		copyModelsItem := diagMenu.AddSubMenuItem("Copy Model List", "Copy available models to clipboard")
		copyLogsItem := diagMenu.AddSubMenuItem("Copy Recent Logs", "Copy recent log entries to clipboard")
		openLogsItem := diagMenu.AddSubMenuItem("Open Logs Folder", "Open logs folder in explorer")
		openAuthItem := diagMenu.AddSubMenuItem("Open Auth Folder", "Open auth folder in explorer")

		// Settings submenu
		settingsMenu := systray.AddMenuItem("Settings", "Application settings")
		startupItem := settingsMenu.AddSubMenuItem("Start with Windows", "Launch ProxyPilot on Windows startup")
		autoProxyItem := settingsMenu.AddSubMenuItem("Auto-start Proxy", "Automatically start proxy when tray launches")
		checkUpdateItem := settingsMenu.AddSubMenuItem("Check for Updates", "Check for new versions")
		installUpdateItem := settingsMenu.AddSubMenuItem("Download & Install Update", "Download and install the latest version")
		installUpdateItem.Hide() // Hidden until an update is available
		rollbackItem := settingsMenu.AddSubMenuItem("Rollback to Previous Version", "Restore the previous version")
		if !updates.CanRollback() {
			rollbackItem.Disable()
		}
		defenderExclusionItem := settingsMenu.AddSubMenuItem("Add to Windows Defender Exclusions", "Prevent Defender from scanning ProxyPilot (requires admin)")

		// Set initial checkbox states
		if enabled, _, _ := desktopctl.IsWindowsRunAutostartEnabled(autostartAppName); enabled {
			startupItem.Check()
		}
		if autoProxy, _ := desktopctl.GetAutoStartProxy(); autoProxy {
			autoProxyItem.Check()
		}

		systray.AddSeparator()
		quitItem := systray.AddMenuItem("Quit", "Quit ProxyPilot")

		// Auto-start proxy on launch if enabled
		autoProxyOn, _ := desktopctl.GetAutoStartProxy()
		if autoProxyOn {
			go func() {
				if !engine.IsRunning() {
					engine.Start(cfg, configPath, password)
				}
			}()
		}

		// Check for updates on startup (silently, in background) with configurable polling
		var latestUpdateInfo *updates.UpdateInfo
		var updateMu sync.Mutex
		if cfg.Updates.IsAutoCheckEnabled() {
			updates.StartGlobalPoller(
				cfg.Updates.GetCheckInterval(),
				cfg.Updates.GetChannel(),
				func(info *updates.UpdateInfo) {
					if info != nil && info.Available {
						updateMu.Lock()
						latestUpdateInfo = info
						updateMu.Unlock()
						installUpdateItem.SetTitle(fmt.Sprintf("Download & Install v%s", info.Version))
						installUpdateItem.Show()
						if cfg.Updates.IsNotifyOnUpdateEnabled() {
							_ = winutil.ShowToast("ProxyPilot Update", fmt.Sprintf("v%s is available! Click 'Check for Updates' to download.", info.Version))
						}
					}
				},
			)
		}

		// Mark as healthy after 30 seconds of successful operation
		// This clears the startup failure counter
		go func() {
			time.Sleep(30 * time.Second)
			_ = updates.MarkHealthy()
		}()

		// Update UI based on status
		refresh := func() {
			st := engine.Status()
			
			// Get account count
			accountCount := 0
			if store := sdkAuth.GetTokenStore(); store != nil {
				if auths, err := store.List(context.Background()); err == nil {
					accountCount = len(auths)
				}
			}
			
			if st.Running {
				port := st.Port
				if port <= 0 {
					port = 8318
				}
				statusItem.SetTitle(fmt.Sprintf("● Running on :%d", port))
				if accountCount > 0 {
					systray.SetTooltip(fmt.Sprintf("ProxyPilot - Running (:%d) - %d accounts", port, accountCount))
				} else {
					systray.SetTooltip(fmt.Sprintf("ProxyPilot - Running (:%d)", port))
				}
				toggleItem.SetTitle("Stop Proxy")
				toggleItem.SetTooltip("Stop the proxy")
			} else {
				statusItem.SetTitle("○ Stopped")
				if accountCount > 0 {
					systray.SetTooltip(fmt.Sprintf("ProxyPilot - Stopped - %d accounts", accountCount))
				} else {
					systray.SetTooltip("ProxyPilot - Stopped")
				}
				toggleItem.SetTitle("Start Proxy")
				toggleItem.SetTooltip("Start the proxy")
			}
		}
		refresh()

		// Refresh status periodically
		go func() {
			t := time.NewTicker(2 * time.Second)
			defer t.Stop()
			for range t.C {
				refresh()
			}
		}()

		// Handle clicks
		go func() {
			for {
				select {
				case <-openDashboard.ClickedCh:
					openProxyUIWithAutostart(engine, cfg, configPath, password)
				case <-toggleItem.ClickedCh:
					if engine.IsRunning() {
						engine.Stop()
					} else {
						engine.Start(cfg, configPath, password)
					}
					refresh()
				case <-refreshTokensItem.ClickedCh:
					go func() {
						_ = cmd.RefreshTokens(cfg, "", false) // Refresh all, no JSON output
					}()
				case <-copyURLItem.ClickedCh:
					copyToClipboard(fmt.Sprintf("http://127.0.0.1:%d/v1", thinkingProxyPort))
				case <-claudeLoginItem.ClickedCh:
					go startOAuthFlow(engine, getOAuthEndpoint("claude"))
				case <-geminiLoginItem.ClickedCh:
					go startOAuthFlow(engine, getOAuthEndpoint("gemini-cli"))
				case <-codexLoginItem.ClickedCh:
					go startOAuthFlow(engine, getOAuthEndpoint("codex"))
				case <-qwenLoginItem.ClickedCh:
					go startOAuthFlow(engine, getOAuthEndpoint("qwen"))
				case <-antigravityLoginItem.ClickedCh:
					go startOAuthFlow(engine, getOAuthEndpoint("antigravity"))
				case <-kiroLoginItem.ClickedCh:
					go startOAuthFlow(engine, getOAuthEndpoint("kiro"))
				case <-minimaxLoginItem.ClickedCh:
					go runCLI("-minimax-login")
				case <-zhipuLoginItem.ClickedCh:
					go runCLI("-zhipu-login")
				case <-copyDiagItem.ClickedCh:
					copyDiagnosticsToClipboard(engine)
				case <-copyStatusItem.ClickedCh:
					copyAccountStatusToClipboard(engine)
				case <-copyRateLimitsItem.ClickedCh:
					go func() {
						if text := captureRateLimits(engine); text != "" {
							copyToClipboard(text)
						}
					}()
				case <-copyUsageItem.ClickedCh:
					go func() {
						if text := captureUsageStats(); text != "" {
							copyToClipboard(text)
						}
					}()
				case <-copyModelsItem.ClickedCh:
					go func() {
						if text := captureModelList(); text != "" {
							copyToClipboard(text)
						}
					}()
				case <-copyLogsItem.ClickedCh:
					go func() {
						if text := captureRecentLogs(); text != "" {
							copyToClipboard(text)
						}
					}()
				case <-copyAccountListItem.ClickedCh:
					go func() {
						if text := captureAccountList(); text != "" {
							copyToClipboard(text)
						}
					}()
				case <-cleanupExpiredItem.ClickedCh:
					go func() {
						runCLI("-cleanup-expired")
					}()
				case <-exportAccountsItem.ClickedCh:
					go exportAccountsDialog()
				case <-importAccountsItem.ClickedCh:
					go importAccountsDialog()
				case <-openLogsItem.ClickedCh:
					desktopctl.OpenLogsFolder(repoRoot, configPath)
				case <-openAuthItem.ClickedCh:
					if dir, err := desktopctl.AuthDirFor(configPath); err == nil {
						desktopctl.OpenFolder(dir)
					}
				case <-startupItem.ClickedCh:
					go func() {
						if startupItem.Checked() {
							// Disable startup
							if err := desktopctl.DisableWindowsRunAutostart(autostartAppName); err == nil {
								startupItem.Uncheck()
								_ = winutil.ShowToast("ProxyPilot", "Removed from Windows startup")
							}
						} else {
							// Enable startup
							cmd, err := autostartCommand(repoRoot, configPath)
							if err == nil {
								if err := desktopctl.EnableWindowsRunAutostart(autostartAppName, cmd); err == nil {
									startupItem.Check()
									_ = winutil.ShowToast("ProxyPilot", "Added to Windows startup")
								}
							}
						}
					}()
				case <-autoProxyItem.ClickedCh:
					go func() {
						if autoProxyItem.Checked() {
							_ = desktopctl.SetAutoStartProxy(false)
							autoProxyItem.Uncheck()
						} else {
							_ = desktopctl.SetAutoStartProxy(true)
							autoProxyItem.Check()
						}
					}()
				case <-checkUpdateItem.ClickedCh:
					go checkForUpdatesWithToast()
				case <-installUpdateItem.ClickedCh:
					go func() {
						updateMu.Lock()
						info := latestUpdateInfo
						updateMu.Unlock()
						if info == nil || !info.Available {
							_ = winutil.ShowToast("ProxyPilot", "No update available")
							return
						}
						performOneClickUpdate(info)
					}()
				case <-rollbackItem.ClickedCh:
					go func() {
						if !updates.CanRollback() {
							_ = winutil.ShowToast("ProxyPilot", "No previous version available for rollback")
							return
						}
						result, err := updates.Rollback()
						if err != nil {
							_ = winutil.ShowToast("ProxyPilot", fmt.Sprintf("Rollback failed: %v", err))
							return
						}
						if result.Success {
							_ = winutil.ShowToast("ProxyPilot", fmt.Sprintf("Rolling back to v%s...", result.RestoredVersion))
							if result.NeedsRestart {
								// Execute the rollback script
								systray.Quit()
							}
						}
					}()
				case <-defenderExclusionItem.ClickedCh:
					go func() {
						authDir, _ := desktopctl.AuthDirFor(configPath)
						if err := winutil.PromptDefenderExclusion(authDir); err != nil {
							_ = winutil.ShowToast("ProxyPilot", fmt.Sprintf("Failed: %v", err))
						} else {
							_ = winutil.ShowToast("ProxyPilot", "Defender exclusion prompt shown (requires admin approval)")
						}
					}()
				case <-quitItem.ClickedCh:
					systray.Quit()
					return
				}
			}
		}()
	}, func() {
		// onExit: Graceful cleanup when systray exits
		log.Info("systray exiting, cleaning up...")
		if engine.IsRunning() {
			if err := engine.Stop(); err != nil {
				log.Warnf("error stopping engine on exit: %v", err)
			}
		}
		// Stop the update poller
		updates.StopGlobalPoller()
		log.Info("cleanup complete")
	})
}

type closeFn func() error

func (c closeFn) Close() error { return c() }

func startThinkingProxy(engine *EmbeddedEngine) ioCloser {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", thinkingProxyPort))
	if err != nil {
		// Best effort: don't crash the tray app if the port is already taken.
		return closeFn(func() error { return nil })
	}

	var (
		mu         sync.Mutex
		lastPort   int
		lastProxy  *httputil.ReverseProxy
		lastTarget *url.URL
	)

	getProxy := func() (*httputil.ReverseProxy, *url.URL) {
		st := engine.Status()
		port := st.Port
		if port <= 0 {
			port = 8318
		}
		target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))

		mu.Lock()
		defer mu.Unlock()
		if lastProxy != nil && lastTarget != nil && lastPort == port {
			return lastProxy, lastTarget
		}
		rp := httputil.NewSingleHostReverseProxy(target)
		rp.FlushInterval = 50 * time.Millisecond
		origDirector := rp.Director
		rp.Director = func(r *http.Request) {
			origDirector(r)
			// Preserve original Host header behavior for local forwarding.
			r.Host = target.Host
		}
		rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"message":"engine unavailable","type":"server_error"}}`))
		}
		lastPort = port
		lastProxy = rp
		lastTarget = target
		return rp, target
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Only allow localhost clients to use the thinking proxy.
		host, _, _ := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
		if host != "127.0.0.1" && host != "::1" && !strings.EqualFold(host, "localhost") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		rp, _ := getProxy()
		rp.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() { _ = srv.Serve(ln) }()

	return closeFn(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		return ln.Close()
	})
}

type ioCloser interface {
	Close() error
}

func openProxyUIWithAutostart(engine *EmbeddedEngine, cfg *config.Config, configPath, password string) error {
	// Start proxy if not running
	if !engine.IsRunning() {
		if err := engine.Start(cfg, configPath, password); err != nil {
			// Continue anyway to show UI
		}
	}

	// Open embedded WebView2 dashboard
	go func() {
		openEmbeddedDashboard(engine, cfg, configPath, password)
	}()
	return nil
}

func openEmbeddedDashboard(engine *EmbeddedEngine, cfg *config.Config, configPath, password string) {
	// Lock this goroutine to an OS thread - required for Windows COM/GUI operations
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Prevent opening multiple dashboard windows
	dashboardMu.Lock()
	if dashboardOpen {
		dashboardMu.Unlock()
		return
	}
	dashboardOpen = true
	dashboardMu.Unlock()

	defer func() {
		dashboardMu.Lock()
		dashboardOpen = false
		dashboardMu.Unlock()
	}()

	// Recover from any panics to prevent the tray app from crashing
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "WebView2 panic: %v\n", r)
			// Try to open in browser as fallback
			st := engine.Status()
			if st.Running && strings.TrimSpace(st.BaseURL) != "" {
				_ = desktopctl.OpenBrowser(st.BaseURL + "/proxypilot.html")
			}
		}
	}()

	target := assetServerURL + "/index.html"
	if assetServerURL == "" {
		// Fallback to browser if asset server failed
		st := engine.Status()
		if st.Running && strings.TrimSpace(st.BaseURL) != "" {
			_ = desktopctl.OpenBrowser(st.BaseURL + "/proxypilot.html")
		}
		return
	}

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     true,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "ProxyPilot",
			Width:  1200,
			Height: 850,
			Center: true,
			IconId: 1, // Use embedded icon from resource.syso
		},
	})
	if w == nil {
		// Fallback to browser - WebView2 runtime may not be installed
		fmt.Fprintf(os.Stderr, "WebView2 failed to initialize, falling back to browser\n")
		_ = desktopctl.OpenBrowser(target)
		return
	}
	defer w.Destroy()

	// Bind desktop functions for the React UI
	_ = w.Bind("pp_status", func() (map[string]any, error) {
		cur := engine.Status()
		return map[string]any{
			"running":    cur.Running,
			"port":       cur.Port,
			"base_url":   cur.BaseURL,
			"last_error": cur.LastError,
		}, nil
	})
	_ = w.Bind("pp_start", func() error {
		return engine.Start(cfg, configPath, password)
	})
	_ = w.Bind("pp_stop", func() error { return engine.Stop() })
	_ = w.Bind("pp_restart", func() error {
		return engine.Restart(cfg, configPath, password)
	})
	_ = w.Bind("pp_open_logs", func() error { return desktopctl.OpenLogsFolder("", configPath) })
	_ = w.Bind("pp_open_auth_folder", func() error {
		dir, err := desktopctl.AuthDirFor(configPath)
		if err != nil {
			return err
		}
		return desktopctl.OpenFolder(dir)
	})
	_ = w.Bind("pp_get_oauth_private", func() (bool, error) { return desktopctl.GetOAuthPrivate() })
	_ = w.Bind("pp_set_oauth_private", func(enabled bool) error { return desktopctl.SetOAuthPrivate(enabled) })
	_ = w.Bind("pp_oauth", func(provider string) error { return startOAuthFlow(engine, getOAuthEndpoint(provider)) })
	_ = w.Bind("pp_copy_diagnostics", func() error { return copyDiagnosticsToClipboard(engine) })
	_ = w.Bind("pp_get_management_key", func() (string, error) { return desktopctl.GetManagementPassword() })
	_ = w.Bind("pp_open_diagnostics", func() error {
		cur := engine.Status()
		if !cur.Running {
			return fmt.Errorf("proxy not running")
		}
		return desktopctl.OpenBrowser(cur.BaseURL + "/proxypilot.html")
	})
	_ = w.Bind("pp_get_requests", func() (any, error) {
		return middleware.GetRequestMonitor(), nil
	})
	_ = w.Bind("pp_get_usage", func() (any, error) {
		stats := usage.GetRequestStatistics()
		if stats == nil {
			return nil, fmt.Errorf("usage statistics not available")
		}
		return usage.ComputeUsageStats(stats.Snapshot()), nil
	})
	_ = w.Bind("pp_detect_agents", func() ([]integrations.Agent, error) {
		st := engine.Status()
		proxyURL := st.BaseURL
		if proxyURL == "" {
			proxyURL = fmt.Sprintf("http://127.0.0.1:%d", st.Port)
		}
		return integrations.DetectCLIAgents(proxyURL), nil
	})
	_ = w.Bind("pp_configure_agent", func(agentID string) error {
		st := engine.Status()
		proxyURL := st.BaseURL
		if proxyURL == "" {
			proxyURL = fmt.Sprintf("http://127.0.0.1:%d", st.Port)
		}
		return integrations.ConfigureCLIAgent(agentID, proxyURL)
	})
	_ = w.Bind("pp_unconfigure_agent", func(agentID string) error {
		return integrations.UnconfigureCLIAgent(agentID)
	})
	_ = w.Bind("pp_check_updates", func() (*updates.UpdateInfo, error) {
		return updates.CheckForUpdates()
	})
	_ = w.Bind("pp_download_update", func(url string) error {
		return desktopctl.OpenBrowser(url)
	})

	w.Navigate(target)
	w.Run()
}

func getOAuthEndpoint(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "antigravity":
		return "/v0/management/antigravity-auth-url"
	case "gemini-cli", "geminicli", "gemini":
		return "/v0/management/gemini-cli-auth-url"
	case "codex", "openai":
		return "/v0/management/codex-auth-url"
	case "claude", "anthropic":
		return "/v0/management/anthropic-auth-url"
	case "qwen":
		return "/v0/management/qwen-auth-url"
	case "kiro":
		return "/v0/management/kiro-auth-url"
	default:
		return ""
	}
}

func shorten(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func sanitizeFileName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	repl := strings.NewReplacer(
		"\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_", " ", "_",
	)
	return repl.Replace(s)
}

func launchUpdate(path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".exe":
		// Inno installer (silent, per-user).
		return exec.Command(path, "/SILENT", "/NORESTART", "/CLOSEAPPLICATIONS", "/RESTARTAPPLICATIONS").Start()
	default:
		// Fallback: let Windows handle the file (zip, etc.)
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
	}
}

type authURLResponse struct {
	Status string `json:"status"`
	URL    string `json:"url"`
	State  string `json:"state"`
	Error  string `json:"error"`
}

func startOAuthFlow(engine *EmbeddedEngine, endpointPath string) error {
	st := engine.Status()
	if !st.Running || strings.TrimSpace(st.BaseURL) == "" {
		return fmt.Errorf("proxy is not running")
	}
	key, err := desktopctl.GetManagementPassword()
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, st.BaseURL+endpointPath, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Management-Key", key)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var out authURLResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if strings.TrimSpace(out.Error) != "" {
			return fmt.Errorf("%s", out.Error)
		}
		return fmt.Errorf("auth url request failed: %s", resp.Status)
	}
	if strings.TrimSpace(out.URL) == "" {
		return fmt.Errorf("missing auth url")
	}

	private, _ := desktopctl.GetOAuthPrivate()
	return openOAuthURL(out.URL, private)
}

func copyDiagnosticsToClipboard(engine *EmbeddedEngine) error {
	st := engine.Status()
	if !st.Running || strings.TrimSpace(st.BaseURL) == "" {
		return fmt.Errorf("proxy is not running")
	}
	key, err := desktopctl.GetManagementPassword()
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, st.BaseURL+"/v0/management/proxypilot/diagnostics?lines=200", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Management-Key", key)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var payload struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("diagnostics failed: %s", resp.Status)
	}
	if strings.TrimSpace(payload.Text) == "" {
		return fmt.Errorf("empty diagnostics")
	}
	return copyToClipboard(payload.Text)
}

func copyToClipboard(text string) error {
	cmd := exec.Command("cmd", "/c", "clip")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func openOAuthURL(url string, private bool) error {
	if !private {
		return desktopctl.OpenBrowser(url)
	}
	edge, err := findEdge()
	if err != nil {
		return desktopctl.OpenBrowser(url)
	}
	return exec.Command(edge, "--inprivate", url).Start()
}

func findEdge() (string, error) {
	if p, err := exec.LookPath("msedge.exe"); err == nil && strings.TrimSpace(p) != "" {
		return p, nil
	}
	if p, err := exec.LookPath("msedge"); err == nil && strings.TrimSpace(p) != "" {
		return p, nil
	}
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
	}
	for _, c := range candidates {
		if strings.TrimSpace(c) == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("msedge.exe not found")
}

func autostartCommand(repoRoot, configPath string) (string, error) {
	exe, err := os.Executable()
	if err == nil && strings.TrimSpace(exe) != "" {
		exe = filepath.Clean(exe)
	} else {
		exe = ""
	}
	if exe == "" {
		return "", fmt.Errorf("unable to resolve tray executable path")
	}
	args := make([]string, 0, 4)
	if strings.TrimSpace(repoRoot) != "" {
		args = append(args, "-repo", repoRoot)
	}
	if strings.TrimSpace(configPath) != "" {
		args = append(args, "-config", configPath)
	}
	return quoteWindowsCommand(exe, args), nil
}

func applyDefaults(repoRoot, configPath string) (string, string) {
	repoRoot = strings.TrimSpace(repoRoot)
	configPath = strings.TrimSpace(configPath)

	exe, _ := os.Executable()
	exeDir := ""
	if strings.TrimSpace(exe) != "" {
		exeDir = filepath.Dir(filepath.Clean(exe))
	}

	// If launched from a repo/app "bin" directory, treat its parent as the root.
	defaultRoot := exeDir
	if strings.EqualFold(filepath.Base(defaultRoot), "bin") {
		defaultRoot = filepath.Dir(defaultRoot)
	}

	if repoRoot == "" && defaultRoot != "" {
		repoRoot = defaultRoot
	}

	if configPath == "" && repoRoot != "" {
		configPath = filepath.Join(repoRoot, "config.yaml")
	}

	if configPath != "" {
		ensureConfig(configPath)
	}

	return repoRoot, configPath
}

func ensureConfig(configPath string) {
	if _, err := os.Stat(configPath); err == nil {
		return
	}
	dir := filepath.Dir(configPath)
	example := filepath.Join(dir, "config.example.yaml")
	if _, err := os.Stat(example); err != nil {
		return
	}
	b, err := os.ReadFile(example)
	if err != nil {
		return
	}
	b = bootstrapLocalConfig(b)
	_ = os.WriteFile(configPath, b, 0o644)
}

func bootstrapLocalConfig(b []byte) []byte {
	// Best-effort: make the packaged default usable without editing.
	// Keep it simple (string-based) so we don't need YAML parsing in the tray binary.
	s := string(b)
	s = strings.ReplaceAll(s, "- \"your-api-key-1\"", "- \"local-dev-key\"")
	s = strings.ReplaceAll(s, "- \"your-api-key-2\"\r\n", "")
	s = strings.ReplaceAll(s, "- \"your-api-key-2\"\n", "")
	s = strings.ReplaceAll(s, "secret-key: \"\"\r\n", "secret-key: \"local-dev-key\"\r\n")
	s = strings.ReplaceAll(s, "secret-key: \"\"\n", "secret-key: \"local-dev-key\"\n")
	return []byte(s)
}

func quoteWindowsCommand(exe string, args []string) string {
	quoted := make([]string, 0, 1+len(args))
	quoted = append(quoted, `"`+strings.ReplaceAll(exe, `"`, `\"`)+`"`)
	for _, a := range args {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if strings.ContainsAny(a, " \t") {
			quoted = append(quoted, `"`+strings.ReplaceAll(a, `"`, `\"`)+`"`)
		} else {
			quoted = append(quoted, a)
		}
	}
	return strings.Join(quoted, " ")
}

// captureUsageStats captures usage statistics output by running CLI
func captureUsageStats() string {
	return runCLI("-usage")
}

// captureModelList captures model list output by running CLI
func captureModelList() string {
	return runCLI("-list-models")
}

// captureRecentLogs captures recent log entries by running CLI
func captureRecentLogs() string {
	return runCLI("-logs", "100")
}

// captureAccountStatus captures account status output by running CLI
func captureAccountStatus() string {
	return runCLI("-status")
}

// captureAccountList captures detailed account list by running CLI
func captureAccountList() string {
	return runCLI("-list-accounts")
}

// captureRateLimits fetches rate limit status from the API
func captureRateLimits(engine *EmbeddedEngine) string {
	st := engine.Status()
	if !st.Running {
		return "Rate Limits: Proxy not running"
	}

	port := st.Port
	if port <= 0 {
		port = 8318
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/v0/management/rate-limits/summary", port)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Sprintf("Failed to fetch rate limits: %v", err)
	}
	defer resp.Body.Close()

	var summary struct {
		Total        int    `json:"total"`
		Available    int    `json:"available"`
		CoolingDown  int    `json:"cooling_down"`
		Disabled     int    `json:"disabled"`
		NextRecoveryIn string `json:"next_recovery_in,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return fmt.Sprintf("Failed to parse rate limits: %v", err)
	}

	var sb strings.Builder
	sb.WriteString("=== Rate Limit Status ===\n\n")
	sb.WriteString(fmt.Sprintf("Total Credentials: %d\n", summary.Total))
	sb.WriteString(fmt.Sprintf("Available: %d\n", summary.Available))
	sb.WriteString(fmt.Sprintf("Cooling Down: %d\n", summary.CoolingDown))
	sb.WriteString(fmt.Sprintf("Disabled: %d\n", summary.Disabled))
	if summary.NextRecoveryIn != "" {
		sb.WriteString(fmt.Sprintf("\nNext Recovery In: %s\n", summary.NextRecoveryIn))
	}

	// Also get detailed info
	detailURL := fmt.Sprintf("http://127.0.0.1:%d/v0/management/rate-limits", port)
	detailResp, err := http.Get(detailURL)
	if err == nil {
		defer detailResp.Body.Close()
		var detail struct {
			Credentials []struct {
				AuthID        string `json:"auth_id"`
				Provider      string `json:"provider"`
				Email         string `json:"email,omitempty"`
				QuotaExceeded bool   `json:"quota_exceeded"`
				RecoverIn     string `json:"recover_in,omitempty"`
			} `json:"credentials"`
		}
		if err := json.NewDecoder(detailResp.Body).Decode(&detail); err == nil && len(detail.Credentials) > 0 {
			sb.WriteString("\n=== Per-Credential Status ===\n")
			for _, cred := range detail.Credentials {
				status := "✓ OK"
				if cred.QuotaExceeded {
					status = fmt.Sprintf("⏳ Cooling (%s)", cred.RecoverIn)
				}
				name := cred.Email
				if name == "" {
					name = cred.AuthID[:8]
				}
				sb.WriteString(fmt.Sprintf("\n[%s] %s: %s", cred.Provider, name, status))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// copyAccountStatusToClipboard copies account status summary to clipboard
func copyAccountStatusToClipboard(engine *EmbeddedEngine) error {
	text := captureAccountStatus()
	if text == "" {
		return fmt.Errorf("no account status available")
	}
	return copyToClipboard(text)
}

// exportAccountsDialog opens a file save dialog and exports accounts
func exportAccountsDialog() {
	// Use a default filename in the user's home directory
	home, _ := os.UserHomeDir()
	defaultPath := filepath.Join(home, "proxypilot-accounts.json")
	
	// Run export command
	output := runCLI("-export-accounts", defaultPath)
	if output != "" {
		copyToClipboard(fmt.Sprintf("Accounts exported to: %s\n\n%s", defaultPath, output))
	}
}

// importAccountsDialog opens a file picker and imports accounts
func importAccountsDialog() {
	// Check for default export location
	home, _ := os.UserHomeDir()
	defaultPath := filepath.Join(home, "proxypilot-accounts.json")
	
	// Check if default file exists
	if _, err := os.Stat(defaultPath); err == nil {
		output := runCLI("-import-accounts", defaultPath, "-force")
		if output != "" {
			copyToClipboard(fmt.Sprintf("Accounts imported from: %s\n\n%s", defaultPath, output))
		}
	}
}

// runCLI executes the ProxyPilot CLI with given args and returns output
func runCLI(args ...string) string {
	// Find CLI executable next to tray executable
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exe)
	
	// Try common CLI binary names
	candidates := []string{
		filepath.Join(dir, "ProxyPilot.exe"),
		filepath.Join(dir, "proxypilot.exe"),
		filepath.Join(dir, "server.exe"),
	}
	
	var cliPath string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			cliPath = c
			break
		}
	}
	if cliPath == "" {
		return ""
	}
	
	cmd := exec.Command(cliPath, args...)
	cmd.Env = os.Environ()
	// Hide console window on Windows
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// checkForUpdatesWithToast checks for updates and shows a toast notification
func checkForUpdatesWithToast() {
	info, err := updates.CheckForUpdates()
	if err != nil {
		_ = winutil.ShowToast("ProxyPilot", "Failed to check for updates")
		return
	}
	if info.Available {
		_ = winutil.ShowToast("ProxyPilot", fmt.Sprintf("Update available: v%s", info.Version))
		// Open release page
		_ = desktopctl.OpenBrowser(info.DownloadURL)
	} else {
		_ = winutil.ShowToast("ProxyPilot", "You're running the latest version")
	}
}

// checkForUpdatesOnStartup silently checks for updates on startup
func checkForUpdatesOnStartup() {
	time.Sleep(5 * time.Second) // Wait for tray to fully load
	info, err := updates.CheckForUpdates()
	if err != nil || !info.Available {
		return
	}
	_ = winutil.ShowToast("ProxyPilot Update", fmt.Sprintf("v%s is available! Click 'Check for Updates' to download.", info.Version))
}

// performOneClickUpdate downloads, verifies, and installs the update
func performOneClickUpdate(info *updates.UpdateInfo) {
	if info == nil || !info.Available {
		_ = winutil.ShowToast("ProxyPilot", "No update available")
		return
	}

	_ = winutil.ShowToast("ProxyPilot", fmt.Sprintf("Downloading v%s...", info.Version))

	// Download with progress
	result, err := updates.DownloadUpdate(info.Version, func(progress updates.DownloadProgress) {
		// Could update a progress indicator here in the future
	})
	if err != nil {
		_ = winutil.ShowToast("ProxyPilot", fmt.Sprintf("Download failed: %v", err))
		return
	}

	// Verify the download
	verifyResult, err := updates.VerifyDownload(result)
	if err != nil {
		_ = winutil.ShowToast("ProxyPilot", fmt.Sprintf("Verification failed: %v", err))
		return
	}
	if !verifyResult.Valid {
		_ = winutil.ShowToast("ProxyPilot", fmt.Sprintf("Verification failed: %s", verifyResult.Message))
		return
	}

	_ = winutil.ShowToast("ProxyPilot", "Preparing to install update...")

	// Prepare the update (extract if needed)
	executablePath, err := updates.PrepareInstall(result)
	if err != nil {
		_ = winutil.ShowToast("ProxyPilot", fmt.Sprintf("Preparation failed: %v", err))
		return
	}

	// Install
	installResult, err := updates.InstallUpdate(executablePath)
	if err != nil {
		_ = winutil.ShowToast("ProxyPilot", fmt.Sprintf("Installation failed: %v", err))
		return
	}

	if installResult.Success {
		if installResult.NeedsRestart {
			_ = winutil.ShowToast("ProxyPilot", "Update installed! Restarting...")
			// The install script will restart the app
		} else {
			_ = winutil.ShowToast("ProxyPilot", "Update installed successfully!")
		}
	} else {
		_ = winutil.ShowToast("ProxyPilot", fmt.Sprintf("Installation failed: %s", installResult.Message))
	}
}
