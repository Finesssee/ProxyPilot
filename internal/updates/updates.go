package updates

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo"
)

type UpdateInfo struct {
	Available    bool    `json:"available"`
	Version      string  `json:"version"`
	DownloadURL  string  `json:"download_url"`
	ReleaseNotes string  `json:"release_notes,omitempty"`
	Assets       []Asset `json:"assets,omitempty"`
}

type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
}

type DownloadProgress struct {
	TotalBytes      int64   `json:"total_bytes"`
	DownloadedBytes int64   `json:"downloaded_bytes"`
	Percent         float64 `json:"percent"`
}

type DownloadResult struct {
	FilePath      string `json:"file_path"`
	Version       string `json:"version"`
	SignaturePath string `json:"signature_path,omitempty"`
}

// GitHubRepo is the GitHub repository for update checks.
const GitHubRepo = "Finesssee/ProxyPilot"

func CheckForUpdates() (*UpdateInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	// Use a User-Agent as required by GitHub API
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", GitHubRepo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ProxyPilot-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Handle 404 - no releases found
	if resp.StatusCode == http.StatusNotFound {
		return &UpdateInfo{
			Available:    false,
			Version:      "",
			DownloadURL:  "",
			ReleaseNotes: "No releases published yet",
			Assets:       nil,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to check for updates: %s", resp.Status)
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	currentVersion := strings.TrimPrefix(buildinfo.Version, "v")
	latestVersion := strings.TrimPrefix(release.TagName, "v")

	available := false
	if currentVersion != "dev" && latestVersion != "" && latestVersion != currentVersion {
		// Simple comparison: if versions are different and not dev, an update is available.
		available = true
	}

	assets := make([]Asset, 0, len(release.Assets))
	for _, a := range release.Assets {
		assets = append(assets, Asset{
			Name:        a.Name,
			DownloadURL: a.BrowserDownloadURL,
			Size:        a.Size,
		})
	}

	return &UpdateInfo{
		Available:    available,
		Version:      latestVersion,
		DownloadURL:  release.HTMLURL,
		ReleaseNotes: release.Body,
		Assets:       assets,
	}, nil
}

// GetAssetForPlatform returns the appropriate asset for the current OS/arch.
func GetAssetForPlatform(assets []Asset) (*Asset, *Asset) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Map Go arch to common naming conventions
	archName := goarch
	if goarch == "amd64" {
		archName = "x86_64"
	} else if goarch == "386" {
		archName = "i386"
	}

	var binaryAsset, sigAsset *Asset
	for i := range assets {
		name := strings.ToLower(assets[i].Name)

		// Check for matching platform binary
		if strings.Contains(name, goos) && (strings.Contains(name, archName) || strings.Contains(name, goarch)) {
			if strings.HasSuffix(name, ".sig") || strings.HasSuffix(name, ".asc") {
				sigAsset = &assets[i]
			} else if strings.HasSuffix(name, ".exe") || strings.HasSuffix(name, ".zip") ||
				strings.HasSuffix(name, ".tar.gz") || !strings.Contains(name, ".") ||
				strings.HasSuffix(name, ".dmg") || strings.HasSuffix(name, ".msi") {
				binaryAsset = &assets[i]
			}
		}
	}

	return binaryAsset, sigAsset
}

// DownloadUpdate downloads the update to a temporary directory.
func DownloadUpdate(version string, progressCb func(DownloadProgress)) (*DownloadResult, error) {
	// Get update info to find assets
	info, err := CheckForUpdates()
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}

	if !info.Available || info.Version != version {
		return nil, fmt.Errorf("version %s is not available", version)
	}

	binaryAsset, sigAsset := GetAssetForPlatform(info.Assets)
	if binaryAsset == nil {
		return nil, fmt.Errorf("no suitable binary found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Create temp directory for downloads
	tempDir, err := os.MkdirTemp("", "proxypilot-update-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Download binary
	binaryPath := filepath.Join(tempDir, binaryAsset.Name)
	if err := downloadFile(binaryAsset.DownloadURL, binaryPath, binaryAsset.Size, progressCb); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to download binary: %w", err)
	}

	result := &DownloadResult{
		FilePath: binaryPath,
		Version:  version,
	}

	// Download signature if available
	if sigAsset != nil {
		sigPath := filepath.Join(tempDir, sigAsset.Name)
		if err := downloadFile(sigAsset.DownloadURL, sigPath, sigAsset.Size, nil); err == nil {
			result.SignaturePath = sigPath
		}
	}

	return result, nil
}

func downloadFile(url, destPath string, expectedSize int64, progressCb func(DownloadProgress)) error {
	client := &http.Client{Timeout: 30 * time.Minute}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "ProxyPilot-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Use expected size from API if content-length is not set
	totalSize := resp.ContentLength
	if totalSize <= 0 {
		totalSize = expectedSize
	}

	if progressCb != nil && totalSize > 0 {
		// Wrap reader with progress tracking
		pr := &progressReader{
			reader:   resp.Body,
			total:    totalSize,
			callback: progressCb,
		}
		_, err = io.Copy(out, pr)
	} else {
		_, err = io.Copy(out, resp.Body)
	}

	return err
}

type progressReader struct {
	reader   io.Reader
	total    int64
	current  int64
	callback func(DownloadProgress)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.current += int64(n)
	if pr.callback != nil {
		pr.callback(DownloadProgress{
			TotalBytes:      pr.total,
			DownloadedBytes: pr.current,
			Percent:         float64(pr.current) / float64(pr.total) * 100,
		})
	}
	return n, err
}
