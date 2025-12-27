package updates

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo"
)

type UpdateInfo struct {
	Available   bool   `json:"available"`
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
}

func CheckForUpdates() (*UpdateInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	// Use a User-Agent as required by GitHub API
	req, err := http.NewRequest("GET", "https://api.github.com/repos/router-for-me/ProxyPilot/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ProxyPilot-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to check for updates: %s", resp.Status)
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	currentVersion := strings.TrimPrefix(buildinfo.Version, "v")
	latestVersion := strings.TrimPrefix(release.TagName, "v")

	available := false
	if currentVersion != "dev" && latestVersion != "" && latestVersion != currentVersion {
		// Simple comparison: if versions are different and not dev, an update is available.
		// In a real scenario, we might want to use a semver library to check if latest > current.
		available = true
	}

	return &UpdateInfo{
		Available:   available,
		Version:     latestVersion,
		DownloadURL: release.HTMLURL,
	}, nil
}
