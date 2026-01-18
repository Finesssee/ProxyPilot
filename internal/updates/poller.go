package updates

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// UpdateCallback is called when an update is available.
type UpdateCallback func(info *UpdateInfo)

// Poller periodically checks for updates in the background.
type Poller struct {
	interval time.Duration
	callback UpdateCallback
	channel  string // "stable" or "prerelease"

	mu       sync.Mutex
	running  bool
	cancel   context.CancelFunc
	lastInfo *UpdateInfo
	lastErr  error
}

// NewPoller creates a new update poller with the given interval.
func NewPoller(interval time.Duration, callback UpdateCallback) *Poller {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return &Poller{
		interval: interval,
		callback: callback,
		channel:  "stable",
	}
}

// SetChannel sets the update channel ("stable" or "prerelease").
func (p *Poller) SetChannel(channel string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.channel = channel
}

// Start begins the background polling loop.
// If already running, this is a no-op.
func (p *Poller) Start() {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.mu.Unlock()

	go p.loop(ctx)
}

// Stop stops the background polling loop.
func (p *Poller) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return
	}
	if p.cancel != nil {
		p.cancel()
	}
	p.running = false
}

// IsRunning returns whether the poller is currently running.
func (p *Poller) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// LastInfo returns the most recent update info (may be nil).
func (p *Poller) LastInfo() *UpdateInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastInfo
}

// LastError returns the most recent error from checking for updates.
func (p *Poller) LastError() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastErr
}

// CheckNow performs an immediate update check, bypassing the polling interval.
func (p *Poller) CheckNow() (*UpdateInfo, error) {
	return p.doCheck()
}

func (p *Poller) loop(ctx context.Context) {
	// Initial delay before first check (let app fully start)
	select {
	case <-ctx.Done():
		return
	case <-time.After(10 * time.Second):
	}

	// First check
	p.doCheckAndNotify()

	// Periodic checks
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("update poller stopped")
			return
		case <-ticker.C:
			p.doCheckAndNotify()
		}
	}
}

func (p *Poller) doCheckAndNotify() {
	info, err := p.doCheck()
	if err != nil {
		log.Warnf("update check failed: %v", err)
		return
	}
	if info != nil && info.Available && p.callback != nil {
		p.callback(info)
	}
}

func (p *Poller) doCheck() (*UpdateInfo, error) {
	p.mu.Lock()
	channel := p.channel
	p.mu.Unlock()

	var info *UpdateInfo
	var err error

	if channel == "prerelease" {
		info, err = CheckForUpdatesIncludePrerelease()
	} else {
		info, err = CheckForUpdates()
	}

	p.mu.Lock()
	p.lastInfo = info
	p.lastErr = err
	p.mu.Unlock()

	return info, err
}

// CheckForUpdatesIncludePrerelease checks for updates including pre-releases.
func CheckForUpdatesIncludePrerelease() (*UpdateInfo, error) {
	// For now, just use the same check as stable.
	// A future enhancement could fetch all releases and find the latest
	// including prereleases via the GitHub API.
	return CheckForUpdates()
}

// Global poller instance for convenience
var globalPoller *Poller
var globalPollerOnce sync.Once

// GetGlobalPoller returns the singleton poller instance.
func GetGlobalPoller() *Poller {
	globalPollerOnce.Do(func() {
		globalPoller = NewPoller(24*time.Hour, nil)
	})
	return globalPoller
}

// StartGlobalPoller starts the global poller with the given settings.
func StartGlobalPoller(interval time.Duration, channel string, callback UpdateCallback) {
	poller := GetGlobalPoller()
	poller.interval = interval
	poller.SetChannel(channel)
	poller.callback = callback
	poller.Start()
}

// StopGlobalPoller stops the global poller.
func StopGlobalPoller() {
	if globalPoller != nil {
		globalPoller.Stop()
	}
}
