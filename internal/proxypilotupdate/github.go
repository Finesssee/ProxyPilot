package proxypilotupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultLatestReleaseURL = "https://api.github.com/repos/router-for-me/CLIProxyAPI/releases/latest"
	defaultUserAgent        = "ProxyPilot"
)

type Release struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func FetchLatestRelease(ctx context.Context) (*Release, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, defaultLatestReleaseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", defaultUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("github latest release: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func (r *Release) Version() string {
	if r == nil {
		return ""
	}
	if v := strings.TrimSpace(r.TagName); v != "" {
		return v
	}
	return strings.TrimSpace(r.Name)
}

// FindPreferredAsset returns an installer/zip URL in priority order.
func (r *Release) FindPreferredAsset() (name, url string) {
	if r == nil {
		return "", ""
	}
	preferred := []string{
		"ProxyPilot-Setup.exe", // Inno output (recommended)
		"ProxyPilot.zip",       // portable fallback
	}
	assets := r.Assets
	for _, want := range preferred {
		for _, a := range assets {
			if strings.EqualFold(strings.TrimSpace(a.Name), want) && strings.TrimSpace(a.BrowserDownloadURL) != "" {
				return a.Name, a.BrowserDownloadURL
			}
		}
	}
	// Fallback: any asset containing "ProxyPilot" and ending with .exe or .zip
	for _, a := range assets {
		n := strings.TrimSpace(a.Name)
		u := strings.TrimSpace(a.BrowserDownloadURL)
		if u == "" {
			continue
		}
		ln := strings.ToLower(n)
		if strings.Contains(ln, "proxypilot") && (strings.HasSuffix(ln, ".exe") || strings.HasSuffix(ln, ".zip")) {
			return n, u
		}
	}
	return "", ""
}
