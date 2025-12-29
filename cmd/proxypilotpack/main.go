package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/trayicon"
)

func main() {
	var (
		outDir  string
		repoDir string
	)
	flag.StringVar(&repoDir, "repo", "", "Repository root (defaults to current directory)")
	flag.StringVar(&outDir, "out", "", "Output directory (defaults to <repo>/dist)")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		die("usage: proxypilotpack <build|package-zip|package-setup|package-inno> [--repo <path>] [--out <path>]")
	}
	cmd := strings.ToLower(strings.TrimSpace(args[0]))

	repoRoot, err := resolveRepoRoot(repoDir)
	if err != nil {
		die(err.Error())
	}
	distRoot := outDir
	if strings.TrimSpace(distRoot) == "" {
		distRoot = filepath.Join(repoRoot, "dist")
	}
	binRoot := filepath.Join(repoRoot, "bin")
	if err := os.MkdirAll(binRoot, 0o755); err != nil {
		die(err.Error())
	}
	if err := os.MkdirAll(distRoot, 0o755); err != nil {
		die(err.Error())
	}

	switch cmd {
	case "build":
		if err := buildBinaries(repoRoot, binRoot); err != nil {
			die(err.Error())
		}
	case "package-zip":
		if err := buildBinaries(repoRoot, binRoot); err != nil {
			die(err.Error())
		}
		if err := packageZip(repoRoot, binRoot, distRoot); err != nil {
			die(err.Error())
		}
	case "package-setup":
		// Alias to the recommended Windows installer.
		if runtime.GOOS != "windows" {
			die("package-setup is only supported on Windows")
		}
		if err := buildBinaries(repoRoot, binRoot); err != nil {
			die(err.Error())
		}
		if err := packageInno(repoRoot, distRoot); err != nil {
			die(err.Error())
		}
	case "package-inno":
		if runtime.GOOS != "windows" {
			die("package-inno is only supported on Windows")
		}
		if err := buildBinaries(repoRoot, binRoot); err != nil {
			die(err.Error())
		}
		if err := packageInno(repoRoot, distRoot); err != nil {
			die(err.Error())
		}
	case "package-iexpress":
		if runtime.GOOS != "windows" {
			die("package-iexpress is only supported on Windows")
		}
		if err := buildBinaries(repoRoot, binRoot); err != nil {
			die(err.Error())
		}
		if err := packageSetup(repoRoot, binRoot, distRoot); err != nil {
			die(err.Error())
		}
	default:
		die(fmt.Sprintf("unknown command: %s", cmd))
	}
}

func resolveRepoRoot(repoDir string) (string, error) {
	root := strings.TrimSpace(repoDir)
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		root = wd
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	// Very small validation: go.mod should exist.
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return "", fmt.Errorf("not a repo root (missing go.mod): %s", root)
	}
	return root, nil
}

func buildBinaries(repoRoot, binRoot string) error {
	proxyExe := filepath.Join(binRoot, "proxypilot-engine.exe")
	if runtime.GOOS != "windows" {
		proxyExe = filepath.Join(binRoot, "proxypilot-engine")
	}
	trayExe := filepath.Join(binRoot, "ProxyPilot.exe")
	if runtime.GOOS != "windows" {
		trayExe = filepath.Join(binRoot, "ProxyPilot")
	}

	if err := run(repoRoot, "go", "build", "-o", proxyExe, "./cmd/server"); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		if err := run(repoRoot, "go", "build", "-ldflags", "-H=windowsgui", "-o", trayExe, "./cmd/cliproxytray"); err != nil {
			return err
		}
	} else {
		if err := run(repoRoot, "go", "build", "-o", trayExe, "./cmd/cliproxytray"); err != nil {
			return err
		}
	}

	if runtime.GOOS == "windows" {
		ico := trayicon.ProxyPilotICO()
		if len(ico) > 0 {
			_ = os.WriteFile(filepath.Join(binRoot, "ProxyPilot.ico"), ico, 0o644)
		}
	}
	return nil
}

func packageZip(repoRoot, binRoot, distRoot string) error {
	outZip := filepath.Join(distRoot, "ProxyPilot.zip")
	_ = os.Remove(outZip)

	files := []struct {
		src string
		dst string
	}{
		{src: filepath.Join(binRoot, "ProxyPilot.exe"), dst: "ProxyPilot.exe"},
		{src: filepath.Join(binRoot, "ProxyPilot.ico"), dst: "ProxyPilot.ico"},
		{src: filepath.Join(binRoot, "proxypilot-engine.exe"), dst: "proxypilot-engine.exe"},
		// Back-compat: keep a copy under the legacy name for older launchers/scripts.
		{src: filepath.Join(binRoot, "proxypilot-engine.exe"), dst: "cliproxyapi-latest.exe"},
	}
	cfg := filepath.Join(repoRoot, "config.example.yaml")
	if _, err := os.Stat(cfg); err == nil {
		files = append(files, struct {
			src string
			dst string
		}{src: cfg, dst: "config.example.yaml"})
	}

	f, err := os.Create(outZip)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)
	for _, it := range files {
		b, err := os.ReadFile(it.src)
		if err != nil {
			return fmt.Errorf("missing %s (build first): %w", it.src, err)
		}
		w, err := zw.Create(it.dst)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, bytes.NewReader(b)); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return nil
}

func packageSetup(repoRoot, binRoot, distRoot string) error {
	iexpress := filepath.Join(os.Getenv("WINDIR"), "System32", "iexpress.exe")
	if _, err := os.Stat(iexpress); err != nil {
		return fmt.Errorf("IExpress not found at %s (use package-zip instead)", iexpress)
	}

	staging := filepath.Join(distRoot, "ProxyPilot-Staging")
	_ = os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return err
	}

	// Payload files
	mgrExe := filepath.Join(binRoot, "ProxyPilot.exe")
	srvExe := filepath.Join(binRoot, "proxypilot-engine.exe")
	if err := copyFile(mgrExe, filepath.Join(staging, "ProxyPilot.exe")); err != nil {
		return err
	}
	if err := copyFile(srvExe, filepath.Join(staging, "proxypilot-engine.exe")); err != nil {
		return err
	}
	_ = copyFile(srvExe, filepath.Join(staging, "cliproxyapi-latest.exe"))
	cfgSrc := filepath.Join(repoRoot, "config.example.yaml")
	if _, err := os.Stat(cfgSrc); err == nil {
		if err := copyFile(cfgSrc, filepath.Join(staging, "config.example.yaml")); err != nil {
			return err
		}
	}

	installCmd := filepath.Join(staging, "install.cmd")
	installBody := "" +
		"@echo off\r\n" +
		"setlocal\r\n" +
		"set \"SRC=%~dp0\"\r\n" +
		"set \"DEST=%LOCALAPPDATA%\\ProxyPilot\"\r\n" +
		"if not exist \"%DEST%\" mkdir \"%DEST%\" >nul 2>&1\r\n" +
		"copy /Y \"%SRC%ProxyPilot.exe\" \"%DEST%\\ProxyPilot.exe\" >nul\r\n" +
		"copy /Y \"%SRC%proxypilot-engine.exe\" \"%DEST%\\proxypilot-engine.exe\" >nul\r\n" +
		"copy /Y \"%SRC%proxypilot-engine.exe\" \"%DEST%\\cliproxyapi-latest.exe\" >nul\r\n" +
		"if exist \"%SRC%config.example.yaml\" copy /Y \"%SRC%config.example.yaml\" \"%DEST%\\config.example.yaml\" >nul\r\n" +
		"if not exist \"%DEST%\\config.yaml\" if exist \"%DEST%\\config.example.yaml\" copy /Y \"%DEST%\\config.example.yaml\" \"%DEST%\\config.yaml\" >nul\r\n" +
		"\r\n" +
		"REM Create Start Menu + Desktop shortcuts\r\n" +
		"powershell -NoProfile -ExecutionPolicy Bypass -Command \"$dest=Join-Path $env:LOCALAPPDATA 'ProxyPilot'; $sm=Join-Path $env:APPDATA 'Microsoft\\\\Windows\\\\Start Menu\\\\Programs\\\\ProxyPilot'; New-Item -ItemType Directory -Force -Path $sm | Out-Null; $ws=New-Object -ComObject WScript.Shell; $lnk=$ws.CreateShortcut((Join-Path $sm 'ProxyPilot.lnk')); $lnk.TargetPath=(Join-Path $dest 'ProxyPilot.exe'); $lnk.WorkingDirectory=$dest; $lnk.IconLocation=$lnk.TargetPath+',0'; $lnk.Save(); $desk=[Environment]::GetFolderPath('Desktop'); $dlnk=$ws.CreateShortcut((Join-Path $desk 'ProxyPilot.lnk')); $dlnk.TargetPath=(Join-Path $dest 'ProxyPilot.exe'); $dlnk.WorkingDirectory=$dest; $dlnk.IconLocation=$dlnk.TargetPath+',0'; $dlnk.Save()\" >nul 2>&1\r\n" +
		"\r\n" +
		"start \"\" \"%DEST%\\ProxyPilot.exe\"\r\n"
	if err := os.WriteFile(installCmd, []byte(installBody), 0o644); err != nil {
		return err
	}

	outExe := filepath.Join(distRoot, "ProxyPilot-Setup.exe")
	if err := os.Remove(outExe); err != nil && !errors.Is(err, os.ErrNotExist) {
		// If the target is locked (common if the user just ran the installer), build to a unique name instead.
		ext := filepath.Ext(outExe)
		base := strings.TrimSuffix(filepath.Base(outExe), ext)
		outExe = filepath.Join(distRoot, fmt.Sprintf("%s-%s%s", base, time.Now().Format("20060102-150405"), ext))
	}

	sedPath := filepath.Join(staging, "package.sed")
	sed, err := buildIExpressSed(staging, outExe)
	if err != nil {
		return err
	}
	// If config.example.yaml is absent, omit it to avoid IExpress build failure.
	if _, err := os.Stat(filepath.Join(staging, "config.example.yaml")); errors.Is(err, os.ErrNotExist) {
		sed = strings.ReplaceAll(sed, "%FILE2%=config.example.yaml\r\n", "")
		sed = strings.ReplaceAll(sed, "FILE2=config.example.yaml\r\n", "")
	}
	if err := os.WriteFile(sedPath, []byte(sed), 0o644); err != nil {
		return err
	}

	if err := run(repoRoot, iexpress, "/n", "/q", sedPath); err != nil {
		return err
	}
	if _, err := os.Stat(outExe); err != nil {
		return fmt.Errorf("installer build failed; expected: %s", outExe)
	}
	return nil
}

func buildIExpressSed(staging string, outExe string) (string, error) {
	stagingAbs, err := filepath.Abs(staging)
	if err != nil {
		return "", err
	}
	outAbs, err := filepath.Abs(outExe)
	if err != nil {
		return "", err
	}
	// IExpress SED wants backslashes escaped.
	stagingEsc := strings.ReplaceAll(stagingAbs, `\`, `\\`)
	outEsc := strings.ReplaceAll(outAbs, `\`, `\\`)

	return fmt.Sprintf(`[Version]
Class=IExpress
SEDVersion=3
[Options]
PackagePurpose=InstallApp
ShowInstallProgramWindow=0
HideExtractAnimation=1
UseLongFileName=1
InsideCompressed=0
CAB_FixedSize=0
CAB_ResvCodeSigning=0
RebootMode=N
InstallPrompt=
DisplayLicense=
FinishMessage=
TargetName=%s
FriendlyName=ProxyPilot
AppLaunched=cmd.exe /c install.cmd
PostInstallCmd=<None>
AdminQuietInstCmd=
UserQuietInstCmd=
SourceFiles=SourceFiles
[SourceFiles]
SourceFiles0=%s
[SourceFiles0]
%%FILE0%%=ProxyPilot.exe
%%FILE1%%=proxypilot-engine.exe
%%FILE2%%=config.example.yaml
%%FILE3%%=install.cmd
[Strings]
FILE0=ProxyPilot.exe
FILE1=proxypilot-engine.exe
FILE2=config.example.yaml
FILE3=install.cmd
`, outEsc, stagingEsc), nil
}

func packageInno(repoRoot, distRoot string) error {
	iscc, err := findISCC()
	if err != nil {
		return err
	}
	iss := filepath.Join(repoRoot, "installer", "proxypilot.iss")
	if _, err := os.Stat(iss); err != nil {
		return fmt.Errorf("missing inno script: %s", iss)
	}

	appVersion := resolveAppVersion(repoRoot)
	repoAbs, _ := filepath.Abs(repoRoot)
	outAbs, _ := filepath.Abs(distRoot)

	args := []string{
		"/Q",
		"/DAppVersion=" + appVersion,
		"/DRepoRoot=" + repoAbs,
		"/DOutDir=" + outAbs,
		iss,
	}
	return run(repoRoot, iscc, args...)
}

func findISCC() (string, error) {
	if p, err := exec.LookPath("ISCC.exe"); err == nil && strings.TrimSpace(p) != "" {
		return p, nil
	}
	if p, err := exec.LookPath("ISCC"); err == nil && strings.TrimSpace(p) != "" {
		return p, nil
	}
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Inno Setup 6", "ISCC.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Inno Setup 6", "ISCC.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Inno Setup 6", "ISCC.exe"),
	}
	if reg := findInnoInstallLocationFromRegistry(); reg != "" {
		candidates = append(candidates, filepath.Join(reg, "ISCC.exe"))
	}
	for _, c := range candidates {
		if strings.TrimSpace(c) == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("Inno Setup compiler (ISCC.exe) not found; install Inno Setup 6 or add ISCC.exe to PATH")
}

func findInnoInstallLocationFromRegistry() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	// Query uninstall entries for "Inno Setup" and read InstallLocation.
	out, err := exec.Command("reg", "query", `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall`, "/s", "/v", "InstallLocation").Output()
	if err == nil {
		if loc := parseRegInstallLocation(string(out)); loc != "" {
			return loc
		}
	}
	out, err = exec.Command("reg", "query", `HKLM\Software\Microsoft\Windows\CurrentVersion\Uninstall`, "/s", "/v", "InstallLocation").Output()
	if err == nil {
		if loc := parseRegInstallLocation(string(out)); loc != "" {
			return loc
		}
	}
	out, err = exec.Command("reg", "query", `HKLM\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`, "/s", "/v", "InstallLocation").Output()
	if err == nil {
		if loc := parseRegInstallLocation(string(out)); loc != "" {
			return loc
		}
	}
	return ""
}

func parseRegInstallLocation(out string) string {
	// reg.exe output lines look like:
	// InstallLocation    REG_SZ    C:\Users\...\Inno Setup 6\
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(line), "installlocation") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		loc := strings.TrimSpace(strings.Join(fields[2:], " "))
		loc = strings.Trim(loc, `"`)
		loc = strings.TrimSpace(loc)
		if loc == "" {
			continue
		}
		if _, err := os.Stat(loc); err == nil {
			return strings.TrimRight(loc, `\`)
		}
	}
	return ""
}

func resolveAppVersion(repoRoot string) string {
	// Prefer git sha if available. Keep it short and filesystem-safe.
	out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "--short", "HEAD").Output()
	if err == nil {
		sha := strings.TrimSpace(string(out))
		if sha != "" {
			return "dev-" + sha
		}
	}
	return "dev-" + time.Now().Format("20060102-150405")
}

func run(dir string, name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func die(msg string) {
	_, _ = fmt.Fprintln(os.Stderr, msg)
	os.Exit(2)
}
