// Package main provides the entry point for the ProxyPilot engine.
// This server acts as a proxy that provides OpenAI/Gemini/Claude compatible API interfaces
// for CLI models, allowing CLI models to be used with tools and libraries designed for standard AI APIs.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	configaccess "github.com/router-for-me/CLIProxyAPI/v6/internal/access/config_access"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/cmd"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/desktopctl"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/logging"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/managementasset"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/misc"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/store"
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/translator"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/tui"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

var (
	Version           = "dev"
	Commit            = "none"
	BuildDate         = "unknown"
	DefaultConfigPath = ""
)

// init initializes the shared logger setup.
func init() {
	logging.SetupBaseLogger()
	buildinfo.Version = Version
	buildinfo.Commit = Commit
	buildinfo.BuildDate = BuildDate
}

// setKiroIncognitoMode sets the incognito browser mode for Kiro authentication.
// Kiro defaults to incognito mode for multi-account support.
// Users can explicitly override with --incognito or --no-incognito flags.
func setKiroIncognitoMode(cfg *config.Config, useIncognito, noIncognito bool) {
	if useIncognito {
		cfg.IncognitoBrowser = true
	} else if noIncognito {
		cfg.IncognitoBrowser = false
	} else {
		cfg.IncognitoBrowser = true // Kiro default
	}
}

// main is the entry point of the application.
// It parses command-line flags, loads configuration, and starts the appropriate
// service based on the provided flags (login, codex-login, or server mode).
func main() {
	fmt.Printf("ProxyPilot Engine Version: %s, Commit: %s, BuiltAt: %s\n", buildinfo.Version, buildinfo.Commit, buildinfo.BuildDate)

	// Command-line flags to control the application's behavior.
	var login bool
	var codexLogin bool
	var claudeLogin bool
	var qwenLogin bool
	var iflowLogin bool
	var iflowCookie bool
	var noBrowser bool
	var oauthCallbackPort int
	var antigravityLogin bool
	var kiroLogin bool
	var kiroGoogleLogin bool
	var kiroAWSLogin bool
	var kiroAWSAuthCode bool
	var kiroImport bool
	var antigravityImport bool
	var minimaxLogin bool
	var zhipuLogin bool
	var kimiLogin bool
	// var githubCopilotLogin bool // REMOVED - GitHub Copilot excluded
	var detectAgents bool
	var setupClaude bool
	var setupCodex bool
	var setupDroid bool
	var setupOpenCode bool
	var setupGemini bool
	var setupCursor bool
	var setupKilo bool
	var setupRooCode bool
	var setupAll bool
	var switchAgent string
	var switchMode string
	var projectID string
	var vertexImport string
	var configPath string
	var password string
	var noIncognito bool
	var useIncognito bool

	// Account management flags
	var showVersion bool
	var showStatus bool
	var listAccounts bool
	var cleanupExpired bool
	var removeAccount string
	var refreshTokens string
	var jsonOutput bool
	var quietMode bool
	var verboseMode bool

	// Usage and logs flags
	var showUsage bool
	var showLogs bool
	var logLines int

	// Model and export flags
	var listModels bool
	var exportAccounts string
	var importAccounts string
	var includeTokens bool
	var forceImport bool

	// Windows service flags
	var runAsService bool
	var serviceCmd string

	// TUI flag
	var launchTUI bool

	// Define command-line flags for different operation modes.
	flag.BoolVar(&login, "login", false, "Login Google Account")
	flag.BoolVar(&codexLogin, "codex-login", false, "Login to Codex using OAuth")
	flag.BoolVar(&claudeLogin, "claude-login", false, "Login to Claude using OAuth")
	flag.BoolVar(&qwenLogin, "qwen-login", false, "Login to Qwen using OAuth")
	flag.BoolVar(&iflowLogin, "iflow-login", false, "Login to iFlow using OAuth")
	flag.BoolVar(&iflowCookie, "iflow-cookie", false, "Login to iFlow using Cookie")
	flag.BoolVar(&noBrowser, "no-browser", false, "Don't open browser automatically for OAuth")
	flag.BoolVar(&useIncognito, "incognito", false, "Open browser in incognito/private mode for OAuth (useful for multiple accounts)")
	flag.BoolVar(&noIncognito, "no-incognito", false, "Force disable incognito mode (uses existing browser session)")
	flag.IntVar(&oauthCallbackPort, "oauth-callback-port", 0, "Override OAuth callback port (defaults to provider-specific port)")
	flag.BoolVar(&antigravityLogin, "antigravity-login", false, "Login to Antigravity using OAuth")
	flag.BoolVar(&kiroLogin, "kiro-login", false, "Login to Kiro using Google OAuth")
	flag.BoolVar(&kiroGoogleLogin, "kiro-google-login", false, "Login to Kiro using Google OAuth (same as --kiro-login)")
	flag.BoolVar(&kiroAWSLogin, "kiro-aws-login", false, "Login to Kiro using AWS Builder ID (device code flow)")
	flag.BoolVar(&kiroAWSAuthCode, "kiro-aws-authcode", false, "Login to Kiro using AWS Builder ID (authorization code flow, better UX)")
	flag.BoolVar(&kiroImport, "kiro-import", false, "Import Kiro token from Kiro IDE (~/.aws/sso/cache/kiro-auth-token.json)")
	flag.BoolVar(&antigravityImport, "antigravity-import", false, "Import Antigravity token from Antigravity IDE")
	flag.BoolVar(&minimaxLogin, "minimax-login", false, "Add MiniMax API key")
	flag.BoolVar(&zhipuLogin, "zhipu-login", false, "Add Zhipu AI API key")
	flag.BoolVar(&kimiLogin, "kimi-login", false, "Login to Kimi using OAuth")
	// GitHub Copilot login removed
	flag.BoolVar(&detectAgents, "detect-agents", false, "Detect installed CLI agents")
	flag.BoolVar(&setupClaude, "setup-claude", false, "Configure Claude Code to use ProxyPilot")
	flag.BoolVar(&setupCodex, "setup-codex", false, "Configure Codex CLI to use ProxyPilot")
	flag.BoolVar(&setupDroid, "setup-droid", false, "Configure Factory Droid to use ProxyPilot")
	flag.BoolVar(&setupOpenCode, "setup-opencode", false, "Configure OpenCode to use ProxyPilot")
	flag.BoolVar(&setupGemini, "setup-gemini", false, "Configure Gemini CLI to use ProxyPilot")
	flag.BoolVar(&setupCursor, "setup-cursor", false, "Configure Cursor to use ProxyPilot")
	flag.BoolVar(&setupKilo, "setup-kilo", false, "Configure Kilo Code CLI to use ProxyPilot")
	flag.BoolVar(&setupRooCode, "setup-roocode", false, "Configure RooCode (VS Code) to use ProxyPilot")
	flag.BoolVar(&setupAll, "setup-all", false, "Configure all detected CLI agents (with backup)")
	flag.StringVar(&switchAgent, "switch", "", "Switch agent config mode (e.g., --switch claude)")
	flag.StringVar(&switchMode, "mode", "", "Switch mode: proxy, native, or status (default: status)")
	flag.StringVar(&projectID, "project_id", "", "Project ID (Gemini only, not required)")
	flag.StringVar(&configPath, "config", DefaultConfigPath, "Configure File Path")
	flag.StringVar(&vertexImport, "vertex-import", "", "Import Vertex service account key JSON file")
	flag.StringVar(&password, "password", "", "")

	flag.BoolVar(&showVersion, "version", false, "Show ProxyPilot version and exit")
	flag.BoolVar(&showStatus, "status", false, "Show ProxyPilot status and exit")
	flag.BoolVar(&listAccounts, "list-accounts", false, "List all configured accounts and exit")
	flag.BoolVar(&cleanupExpired, "cleanup-expired", false, "Remove expired tokens and exit")
	flag.StringVar(&removeAccount, "remove-account", "", "Remove a specific account by name and exit")
	flag.StringVar(&refreshTokens, "refresh", "", "Force token refresh (all, or email/id to refresh specific)")
	flag.BoolVar(&jsonOutput, "json", false, "Output in JSON format (overrides --quiet)")
	flag.BoolVar(&quietMode, "quiet", false, "Run in quiet mode (overrides --verbose)")
	flag.BoolVar(&verboseMode, "verbose", false, "Run in verbose mode")

	flag.BoolVar(&listModels, "list-models", false, "List available models per provider and exit")
	flag.StringVar(&exportAccounts, "export-accounts", "", "Export accounts to JSON file (use - for stdout)")
	flag.StringVar(&importAccounts, "import-accounts", "", "Import accounts from JSON file")
	flag.BoolVar(&includeTokens, "include-tokens", false, "Include sensitive tokens in export (use with -export-accounts)")
	flag.BoolVar(&forceImport, "force", false, "Force overwrite existing accounts on import")
	flag.BoolVar(&showUsage, "usage", false, "Show token usage statistics and exit")
	flag.BoolVar(&showLogs, "logs", false, "View recent proxy logs and exit")
	flag.IntVar(&logLines, "n", 50, "Number of log lines to show (used with -logs)")

	// Windows service flags
	flag.BoolVar(&runAsService, "service", false, "Run as Windows service (internal)")
	flag.StringVar(&serviceCmd, "service-cmd", "", "Service command: install, uninstall, start, stop, status")

	// TUI flag
	flag.BoolVar(&launchTUI, "tui", false, "Launch interactive terminal dashboard")

	flag.CommandLine.Usage = func() {
		out := flag.CommandLine.Output()
		_, _ = fmt.Fprintf(out, "Usage of %s\n", os.Args[0])
		flag.CommandLine.VisitAll(func(f *flag.Flag) {
			if f.Name == "password" {
				return
			}
			s := fmt.Sprintf("  -%s", f.Name)
			name, unquoteUsage := flag.UnquoteUsage(f)
			if name != "" {
				s += " " + name
			}
			if len(s) <= 4 {
				s += "	"
			} else {
				s += "\n    "
			}
			if unquoteUsage != "" {
				s += unquoteUsage
			}
			if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" {
				s += fmt.Sprintf(" (default %s)", f.DefValue)
			}
			_, _ = fmt.Fprint(out, s+"\n")
		})
	}

	// Check for subcommand-style switch command before flag.Parse()
	// Supports: proxypilot switch [agent] [mode]
	// agent can be: claude, gemini, codex, opencode, droid, cursor
	// mode can be: proxy, native, status (default if omitted)
	var subcommandSwitch bool
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "switch" {
		subcommandSwitch = true
		switchArgs := args[1:]
		// Filter out any flags like --status from positional args
		var positionalArgs []string
		for _, arg := range switchArgs {
			if arg == "--status" {
				switchMode = "status"
			} else if !strings.HasPrefix(arg, "-") {
				positionalArgs = append(positionalArgs, arg)
			}
		}
		// Parse positional arguments: [agent] [mode]
		if len(positionalArgs) >= 1 {
			switchAgent = positionalArgs[0]
		}
		if len(positionalArgs) >= 2 {
			switchMode = positionalArgs[1]
		}
		// Default mode to "status" if not specified
		if switchMode == "" {
			switchMode = "status"
		}
	}

	// Pre-process -refresh flag: if -refresh is present without a value, treat as -refresh=all
	for i, arg := range os.Args[1:] {
		if arg == "-refresh" || arg == "--refresh" {
			// Check if next arg exists and is not another flag
			nextIdx := i + 2 // +1 for 1-based slice, +1 for next
			if nextIdx >= len(os.Args) || strings.HasPrefix(os.Args[nextIdx], "-") {
				os.Args[i+1] = "-refresh=all"
			}
			break
		}
	}

	// Parse the command-line flags.
	flag.Parse()

	// Handle Windows service commands early (before config loading)
	if serviceCmd != "" {
		if handleServiceCommand([]string{serviceCmd, configPath}) {
			return
		}
	}
	if runAsService {
		if err := runService(configPath); err != nil {
			log.Errorf("service error: %v", err)
			os.Exit(1)
		}
		return
	}

	// Core application variables.
	var err error
	var cfg *config.Config
	var isCloudDeploy bool
	var (
		usePostgresStore     bool
		pgStoreDSN           string
		pgStoreSchema        string
		pgStoreLocalPath     string
		pgStoreInst          *store.PostgresStore
		useGitStore          bool
		gitStoreRemoteURL    string
		gitStoreUser         string
		gitStorePassword     string
		gitStoreLocalPath    string
		gitStoreInst         *store.GitTokenStore
		gitStoreRoot         string
		useObjectStore       bool
		objectStoreEndpoint  string
		objectStoreAccess    string
		objectStoreSecret    string
		objectStoreBucket    string
		objectStoreLocalPath string
		objectStoreInst      *store.ObjectTokenStore
	)

	wd, err := os.Getwd()
	if err != nil {
		log.Errorf("failed to get working directory: %v", err)
		return
	}

	// Load environment variables from .env if present.
	if errLoad := godotenv.Load(filepath.Join(wd, ".env")); errLoad != nil {
		if !errors.Is(errLoad, os.ErrNotExist) {
			log.WithError(errLoad).Warn("failed to load .env file")
		}
	}

	lookupEnv := func(keys ...string) (string, bool) {
		for _, key := range keys {
			if value, ok := os.LookupEnv(key); ok {
				if trimmed := strings.TrimSpace(value); trimmed != "" {
					return trimmed, true
				}
			}
		}
		return "", false
	}
	writableBase := util.WritablePath()
	if value, ok := lookupEnv("PGSTORE_DSN", "pgstore_dsn"); ok {
		usePostgresStore = true
		pgStoreDSN = value
	}
	if usePostgresStore {
		if value, ok := lookupEnv("PGSTORE_SCHEMA", "pgstore_schema"); ok {
			pgStoreSchema = value
		}
		if value, ok := lookupEnv("PGSTORE_LOCAL_PATH", "pgstore_local_path"); ok {
			pgStoreLocalPath = value
		}
		if pgStoreLocalPath == "" {
			if writableBase != "" {
				pgStoreLocalPath = writableBase
			} else {
				pgStoreLocalPath = wd
			}
		}
		useGitStore = false
	}
	if value, ok := lookupEnv("GITSTORE_GIT_URL", "gitstore_git_url"); ok {
		useGitStore = true
		gitStoreRemoteURL = value
	}
	if value, ok := lookupEnv("GITSTORE_GIT_USERNAME", "gitstore_git_username"); ok {
		gitStoreUser = value
	}
	if value, ok := lookupEnv("GITSTORE_GIT_TOKEN", "gitstore_git_token"); ok {
		gitStorePassword = value
	}
	if value, ok := lookupEnv("GITSTORE_LOCAL_PATH", "gitstore_local_path"); ok {
		gitStoreLocalPath = value
	}
	if value, ok := lookupEnv("OBJECTSTORE_ENDPOINT", "objectstore_endpoint"); ok {
		useObjectStore = true
		objectStoreEndpoint = value
	}
	if value, ok := lookupEnv("OBJECTSTORE_ACCESS_KEY", "objectstore_access_key"); ok {
		objectStoreAccess = value
	}
	if value, ok := lookupEnv("OBJECTSTORE_SECRET_KEY", "objectstore_secret_key"); ok {
		objectStoreSecret = value
	}
	if value, ok := lookupEnv("OBJECTSTORE_BUCKET", "objectstore_bucket"); ok {
		objectStoreBucket = value
	}
	if value, ok := lookupEnv("OBJECTSTORE_LOCAL_PATH", "objectstore_local_path"); ok {
		objectStoreLocalPath = value
	}

	// Check for cloud deploy mode only on first execution
	// Read env var name in uppercase: DEPLOY
	deployEnv := os.Getenv("DEPLOY")
	if deployEnv == "cloud" {
		isCloudDeploy = true
	}

	// Determine and load the configuration file.
	// Prefer the Postgres store when configured, otherwise fallback to git or local files.
	var configFilePath string
	if usePostgresStore {
		if pgStoreLocalPath == "" {
			pgStoreLocalPath = wd
		}
		pgStoreLocalPath = filepath.Join(pgStoreLocalPath, "pgstore")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		pgStoreInst, err = store.NewPostgresStore(ctx, store.PostgresStoreConfig{
			DSN:      pgStoreDSN,
			Schema:   pgStoreSchema,
			SpoolDir: pgStoreLocalPath,
		})
		cancel()
		if err != nil {
			log.Errorf("failed to initialize postgres token store: %v", err)
			return
		}
		examplePath := filepath.Join(wd, "config.example.yaml")
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		if errBootstrap := pgStoreInst.Bootstrap(ctx, examplePath); errBootstrap != nil {
			cancel()
			log.Errorf("failed to bootstrap postgres-backed config: %v", errBootstrap)
			return
		}
		cancel()
		configFilePath = pgStoreInst.ConfigPath()
		cfg, err = config.LoadConfigOptional(configFilePath, isCloudDeploy)
		if err == nil {
			cfg.AuthDir = pgStoreInst.AuthDir()
			log.Infof("postgres-backed token store enabled, workspace path: %s", pgStoreInst.WorkDir())
		}
	} else if useObjectStore {
		if objectStoreLocalPath == "" {
			if writableBase != "" {
				objectStoreLocalPath = writableBase
			} else {
				objectStoreLocalPath = wd
			}
		}
		objectStoreRoot := filepath.Join(objectStoreLocalPath, "objectstore")
		resolvedEndpoint := strings.TrimSpace(objectStoreEndpoint)
		useSSL := true
		if strings.Contains(resolvedEndpoint, "://") {
			parsed, errParse := url.Parse(resolvedEndpoint)
			if errParse != nil {
				log.Errorf("failed to parse object store endpoint %q: %v", objectStoreEndpoint, errParse)
				return
			}
			switch strings.ToLower(parsed.Scheme) {
			case "http":
				useSSL = false
			case "https":
				useSSL = true
			default:
				log.Errorf("unsupported object store scheme %q (only http and https are allowed)", parsed.Scheme)
				return
			}
			if parsed.Host == "" {
				log.Errorf("object store endpoint %q is missing host information", objectStoreEndpoint)
				return
			}
			resolvedEndpoint = parsed.Host
			if parsed.Path != "" && parsed.Path != "/" {
				resolvedEndpoint = strings.TrimSuffix(parsed.Host+parsed.Path, "/")
			}
		}
		resolvedEndpoint = strings.TrimRight(resolvedEndpoint, "/")
		objCfg := store.ObjectStoreConfig{
			Endpoint:  resolvedEndpoint,
			Bucket:    objectStoreBucket,
			AccessKey: objectStoreAccess,
			SecretKey: objectStoreSecret,
			LocalRoot: objectStoreRoot,
			UseSSL:    useSSL,
			PathStyle: true,
		}
		objectStoreInst, err = store.NewObjectTokenStore(objCfg)
		if err != nil {
			log.Errorf("failed to initialize object token store: %v", err)
			return
		}
		examplePath := filepath.Join(wd, "config.example.yaml")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if errBootstrap := objectStoreInst.Bootstrap(ctx, examplePath); errBootstrap != nil {
			cancel()
			log.Errorf("failed to bootstrap object-backed config: %v", errBootstrap)
			return
		}
		cancel()
		configFilePath = objectStoreInst.ConfigPath()
		cfg, err = config.LoadConfigOptional(configFilePath, isCloudDeploy)
		if err == nil {
			if cfg == nil {
				cfg = &config.Config{}
			}
			cfg.AuthDir = objectStoreInst.AuthDir()
			log.Infof("object-backed token store enabled, bucket: %s", objectStoreBucket)
		}
	} else if useGitStore {
		if gitStoreLocalPath == "" {
			if writableBase != "" {
				gitStoreLocalPath = writableBase
			} else {
				gitStoreLocalPath = wd
			}
		}
		gitStoreRoot = filepath.Join(gitStoreLocalPath, "gitstore")
		authDir := filepath.Join(gitStoreRoot, "auths")
		gitStoreInst = store.NewGitTokenStore(gitStoreRemoteURL, gitStoreUser, gitStorePassword)
		gitStoreInst.SetBaseDir(authDir)
		if errRepo := gitStoreInst.EnsureRepository(); errRepo != nil {
			log.Errorf("failed to prepare git token store: %v", errRepo)
			return
		}
		configFilePath = gitStoreInst.ConfigPath()
		if configFilePath == "" {
			configFilePath = filepath.Join(gitStoreRoot, "config", "config.yaml")
		}
		if _, statErr := os.Stat(configFilePath); errors.Is(statErr, fs.ErrNotExist) {
			examplePath := filepath.Join(wd, "config.example.yaml")
			if _, errExample := os.Stat(examplePath); errExample != nil {
				log.Errorf("failed to find template config file: %v", errExample)
				return
			}
			if errCopy := misc.CopyConfigTemplate(examplePath, configFilePath); errCopy != nil {
				log.Errorf("failed to bootstrap git-backed config: %v", errCopy)
				return
			}
			if errCommit := gitStoreInst.PersistConfig(context.Background()); errCommit != nil {
				log.Errorf("failed to commit initial git-backed config: %v", errCommit)
				return
			}
			log.Infof("git-backed config initialized from template: %s", configFilePath)
		} else if statErr != nil {
			log.Errorf("failed to inspect git-backed config: %v", statErr)
			return
		}
		cfg, err = config.LoadConfigOptional(configFilePath, isCloudDeploy)
		if err == nil {
			cfg.AuthDir = gitStoreInst.AuthDir()
			log.Infof("git-backed token store enabled, repository path: %s", gitStoreRoot)
		}
	} else if configPath != "" {
		configFilePath = configPath
		cfg, err = config.LoadConfigOptional(configPath, isCloudDeploy)
	} else {
		wd, err = os.Getwd()
		if err != nil {
			log.Errorf("failed to get working directory: %v", err)
			return
		}
		configFilePath = filepath.Join(wd, "config.yaml")
		cfg, err = config.LoadConfigOptional(configFilePath, isCloudDeploy)
	}
	if err != nil {
		// For switch command and TUI, config is optional - use defaults
		if subcommandSwitch || switchAgent != "" || launchTUI {
			cfg = &config.Config{Port: 8318}
		} else {
			log.Errorf("failed to load config: %v", err)
			return
		}
	}
	if cfg == nil {
		cfg = &config.Config{}
	}

	// In cloud deploy mode, check if we have a valid configuration
	var configFileExists bool
	if isCloudDeploy {
		if info, errStat := os.Stat(configFilePath); errStat != nil {
			// Don't mislead: API server will not start until configuration is provided.
			log.Info("Cloud deploy mode: No configuration file detected; standing by for configuration")
			configFileExists = false
		} else if info.IsDir() {
			log.Info("Cloud deploy mode: Config path is a directory; standing by for configuration")
			configFileExists = false
		} else if cfg.Port == 0 {
			// LoadConfigOptional returns empty config when file is empty or invalid.
			// Config file exists but is empty or invalid; treat as missing config
			log.Info("Cloud deploy mode: Configuration file is empty or invalid; standing by for valid configuration")
			configFileExists = false
		} else {
			log.Info("Cloud deploy mode: Configuration file detected; starting service")
			configFileExists = true
		}
	}

	// Perform basic semantic validation of the loaded configuration.
	if warnings, errValidate := config.ValidateConfig(cfg); errValidate != nil {
		log.Errorf("invalid configuration: %v", errValidate)
		return
	} else if len(warnings) > 0 {
		for _, w := range warnings {
			log.Warnf("config warning: %s", w)
		}
	}

	usage.SetStatisticsEnabled(cfg.UsageStatisticsEnabled)
	coreauth.SetQuotaCooldownDisabled(cfg.DisableCooling)
	// AntigravityPrimaryEmail removed - field does not exist

	if err = logging.ConfigureLogOutput(cfg); err != nil {
		log.Errorf("failed to configure log output: %v", err)
		return
	}

	log.Infof("ProxyPilot Engine Version: %s, Commit: %s, BuiltAt: %s", buildinfo.Version, buildinfo.Commit, buildinfo.BuildDate)

	// Set the log level based on the configuration.
	util.SetLogLevel(cfg)

	// CLI flags override config-based log level
	if quietMode {
		logging.SetLogLevel("quiet")
	} else if verboseMode {
		logging.SetLogLevel("verbose")
	}

	if resolvedAuthDir, errResolveAuthDir := util.ResolveAuthDir(cfg.AuthDir); errResolveAuthDir != nil {
		log.Errorf("failed to resolve auth directory: %v", errResolveAuthDir)
		return
	} else {
		cfg.AuthDir = resolvedAuthDir
	}
	managementasset.SetCurrentConfig(cfg)

	// Create login options to be used in authentication flows.
	options := &cmd.LoginOptions{
		NoBrowser:    noBrowser,
		CallbackPort: oauthCallbackPort,
	}

	// Register the shared token store once so all components use the same persistence backend.
	if usePostgresStore {
		sdkAuth.RegisterTokenStore(pgStoreInst)
	} else if useObjectStore {
		sdkAuth.RegisterTokenStore(objectStoreInst)
	} else if useGitStore {
		sdkAuth.RegisterTokenStore(gitStoreInst)
	} else {
		sdkAuth.RegisterTokenStore(sdkAuth.NewFileTokenStore())
	}

	// Register built-in access providers before constructing services.
	configaccess.Register(&cfg.SDKConfig)

	// Handle different command modes based on the provided flags.

	if vertexImport != "" {
		// Handle Vertex service account import
		cmd.DoVertexImport(cfg, vertexImport)
	} else if showVersion {
		// Version already printed at startup, just exit
		return
	} else if showStatus {
		if err := cmd.ShowStatus(jsonOutput); err != nil {
			log.Errorf("status failed: %v", err)
			os.Exit(1)
		}
		return
	} else if launchTUI {
		proxyURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
		mgmtKey, _ := desktopctl.GetManagementPassword()
		if err := tui.Run(proxyURL, mgmtKey); err != nil {
			log.Errorf("tui failed: %v", err)
			os.Exit(1)
		}
		return
	} else if listAccounts {
		if err := cmd.ListAccounts(jsonOutput); err != nil {
			log.Errorf("list-accounts failed: %v", err)
			os.Exit(1)
		}
		return
	} else if listModels {
		if err := cmd.ListModels(jsonOutput); err != nil {
			log.Errorf("list-models failed: %v", err)
			os.Exit(1)
		}
		return
	} else if exportAccounts != "" {
		if err := cmd.ExportAccounts(exportAccounts, includeTokens, jsonOutput); err != nil {
			log.Errorf("export-accounts failed: %v", err)
			os.Exit(1)
		}
		return
	} else if importAccounts != "" {
		if err := cmd.ImportAccounts(importAccounts, forceImport, jsonOutput); err != nil {
			log.Errorf("import-accounts failed: %v", err)
			os.Exit(1)
		}
		return
	} else if cleanupExpired {
		if err := cmd.CleanupExpired(false); err != nil {
			log.Errorf("cleanup-expired failed: %v", err)
			os.Exit(1)
		}
		return
	} else if removeAccount != "" {
		if err := cmd.RemoveAccount(removeAccount); err != nil {
			log.Errorf("remove-account failed: %v", err)
			os.Exit(1)
		}
		return
	} else if refreshTokens != "" {
		identifier := ""
		if refreshTokens != "all" {
			identifier = refreshTokens
		}
		if err := cmd.RefreshTokens(cfg, identifier, jsonOutput); err != nil {
			log.Errorf("refresh failed: %v", err)
			os.Exit(1)
		}
		return
	} else if showUsage {
		if err := cmd.ShowUsage(jsonOutput); err != nil {
			log.Errorf("usage failed: %v", err)
			os.Exit(1)
		}
		return
	} else if showLogs {
		if err := cmd.ShowLogs(logLines, jsonOutput); err != nil {
			log.Errorf("logs failed: %v", err)
			os.Exit(1)
		}
		return
	} else if login {
		// Handle Google/Gemini login
		cmd.DoLogin(cfg, projectID, options)
	} else if antigravityLogin {
		// Handle Antigravity login
		cmd.DoAntigravityLogin(cfg, options)
	} else if codexLogin {
		// Handle Codex login
		cmd.DoCodexLogin(cfg, options)
	} else if claudeLogin {
		// Handle Claude login
		cmd.DoClaudeLogin(cfg, options)
	} else if qwenLogin {
		cmd.DoQwenLogin(cfg, options)
	} else if kiroLogin {
		// For Kiro auth, default to incognito mode for multi-account support
		// Users can explicitly override with --no-incognito
		// Note: This config mutation is safe - auth commands exit after completion
		// and don't share config with StartService (which is in the else branch)
		setKiroIncognitoMode(cfg, useIncognito, noIncognito)
		cmd.DoKiroLogin(cfg, options)
	} else if kiroGoogleLogin {
		// For Kiro auth, default to incognito mode for multi-account support
		// Users can explicitly override with --no-incognito
		// Note: This config mutation is safe - auth commands exit after completion
		setKiroIncognitoMode(cfg, useIncognito, noIncognito)
		cmd.DoKiroGoogleLogin(cfg, options)
	} else if kiroAWSLogin {
		// For Kiro auth, default to incognito mode for multi-account support
		// Users can explicitly override with --no-incognito
		setKiroIncognitoMode(cfg, useIncognito, noIncognito)
		cmd.DoKiroAWSLogin(cfg, options)
	} else if kiroAWSAuthCode {
		// For Kiro auth with authorization code flow (better UX)
		setKiroIncognitoMode(cfg, useIncognito, noIncognito)
		cmd.DoKiroAWSAuthCodeLogin(cfg, options)
	} else if kiroImport {
		cmd.DoKiroImport(cfg, options)
	} else if antigravityImport {
		cmd.DoAntigravityImport(cfg)
	} else if minimaxLogin {
		cmd.DoMiniMaxLogin(cfg, options)
	} else if zhipuLogin {
		cmd.DoZhipuLogin(cfg, options)
	} else if iflowLogin {
		cmd.DoIFlowLogin(cfg, options)
	} else if iflowCookie {
		cmd.DoIFlowCookieAuth(cfg, options)
	} else if kimiLogin {
		cmd.DoKimiLogin(cfg, options)
	} else if detectAgents {
		cmd.DoDetectAgents()
	} else if setupClaude {
		cmd.DoSetupClaude(cfg)
	} else if setupCodex {
		cmd.DoSetupCodex(cfg)
	} else if setupDroid {
		cmd.DoSetupDroid(cfg)
	} else if setupOpenCode {
		cmd.DoSetupOpenCode(cfg)
	} else if setupGemini {
		cmd.DoSetupGeminiCLI(cfg)
	} else if setupCursor {
		cmd.DoSetupCursor(cfg)
	} else if setupKilo {
		cmd.DoSetupKiloCode(cfg)
	} else if setupRooCode {
		cmd.DoSetupRooCode(cfg)
	} else if setupAll {
		cmd.DoSetupAll(cfg)
	} else if subcommandSwitch || switchAgent != "" || switchMode != "" {
		// Handle switch command:
		// - Subcommand style: proxypilot switch claude proxy
		// - Flag style: proxypilot --switch claude --mode proxy
		cmd.DoSwitch(cfg, switchAgent, switchMode)
	} else {
		// In cloud deploy mode without config file, just wait for shutdown signals
		if isCloudDeploy && !configFileExists {
			// No config file available, just wait for shutdown
			cmd.WaitForCloudDeploy()
			return
		}

		// Start the main proxy service
		// Auto-generate management password if not provided
		// This enables browser-based webui access without requiring the -password flag
		autoGenPassword := false
		if password == "" {
			if pw, err := desktopctl.GetManagementPassword(); err == nil {
				password = pw
				autoGenPassword = true
				log.Info("using auto-generated management password for webui access")
			} else {
				log.Warnf("failed to get management password: %v (webui may require manual authentication)", err)
			}
		}
		// Use standalone mode (no keep-alive shutdown) when password was auto-generated
		// Keep-alive shutdown is only needed when the tray app spawns the server as subprocess
		managementasset.StartAutoUpdater(context.Background(), configFilePath)
		if autoGenPassword {
			cmd.StartServiceStandalone(cfg, configFilePath, password)
		} else {
			cmd.StartService(cfg, configFilePath, password)
		}
	}
}
