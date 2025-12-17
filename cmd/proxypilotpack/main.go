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
		die("usage: proxypilotpack <build|package-zip|package-setup> [--repo <path>] [--out <path>]")
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
		if runtime.GOOS != "windows" {
			die("package-setup is only supported on Windows")
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
	proxyExe := filepath.Join(binRoot, "cliproxyapi-latest.exe")
	if runtime.GOOS != "windows" {
		proxyExe = filepath.Join(binRoot, "cliproxyapi-latest")
	}
	trayExe := filepath.Join(binRoot, "ProxyPilot.exe")
	if runtime.GOOS != "windows" {
		trayExe = filepath.Join(binRoot, "ProxyPilot")
	}

	if err := run(repoRoot, "go", "build", "-o", proxyExe, "./cmd/server"); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		if err := run(repoRoot, "go", "build", "-ldflags", "-H windowsgui", "-o", trayExe, "./cmd/cliproxytray"); err != nil {
			return err
		}
	} else {
		if err := run(repoRoot, "go", "build", "-o", trayExe, "./cmd/cliproxytray"); err != nil {
			return err
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
		{src: filepath.Join(binRoot, "cliproxyapi-latest.exe"), dst: "cliproxyapi-latest.exe"},
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
	srvExe := filepath.Join(binRoot, "cliproxyapi-latest.exe")
	if err := copyFile(mgrExe, filepath.Join(staging, "ProxyPilot.exe")); err != nil {
		return err
	}
	if err := copyFile(srvExe, filepath.Join(staging, "cliproxyapi-latest.exe")); err != nil {
		return err
	}
	cfgSrc := filepath.Join(repoRoot, "config.example.yaml")
	if _, err := os.Stat(cfgSrc); err == nil {
		if err := copyFile(cfgSrc, filepath.Join(staging, "config.example.yaml")); err != nil {
			return err
		}
	}

	runCmd := filepath.Join(staging, "run-manager.cmd")
	if err := os.WriteFile(runCmd, []byte("@echo off\r\nsetlocal\r\nstart \"\" \"%~dp0ProxyPilot.exe\"\r\n"), 0o644); err != nil {
		return err
	}

	outExe := filepath.Join(distRoot, "ProxyPilot-Setup.exe")
	_ = os.Remove(outExe)

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
AppLaunched=run-manager.cmd
PostInstallCmd=
AdminQuietInstCmd=
UserQuietInstCmd=
SourceFiles=SourceFiles
[SourceFiles]
SourceFiles0=%s
[SourceFiles0]
%%FILE0%%=ProxyPilot.exe
%%FILE1%%=cliproxyapi-latest.exe
%%FILE2%%=config.example.yaml
%%FILE3%%=run-manager.cmd
[Strings]
FILE0=ProxyPilot.exe
FILE1=cliproxyapi-latest.exe
FILE2=config.example.yaml
FILE3=run-manager.cmd
`, outEsc, stagingEsc), nil
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
