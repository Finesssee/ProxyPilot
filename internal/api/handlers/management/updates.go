package management

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/updates"
)

// updateState tracks the current update operation state.
type updateState struct {
	mu             sync.RWMutex
	downloading    bool
	progress       updates.DownloadProgress
	downloadResult *updates.DownloadResult
	lastError      string
}

var currentUpdate = &updateState{}

func (h *Handler) GetUpdateInfo(c *gin.Context) {
	info, err := updates.CheckForUpdates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, info)
}

// GetUpdateStatus returns the current update download status.
func (h *Handler) GetUpdateStatus(c *gin.Context) {
	currentUpdate.mu.RLock()
	defer currentUpdate.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"downloading": currentUpdate.downloading,
		"progress":    currentUpdate.progress,
		"ready":       currentUpdate.downloadResult != nil,
		"error":       currentUpdate.lastError,
	})
}

// DownloadUpdate starts downloading an update.
func (h *Handler) DownloadUpdate(c *gin.Context) {
	var req struct {
		Version string `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "version is required"})
		return
	}

	currentUpdate.mu.Lock()
	if currentUpdate.downloading {
		currentUpdate.mu.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": "download already in progress"})
		return
	}
	currentUpdate.downloading = true
	currentUpdate.progress = updates.DownloadProgress{}
	currentUpdate.downloadResult = nil
	currentUpdate.lastError = ""
	currentUpdate.mu.Unlock()

	// Start download in background
	go func() {
		result, err := updates.DownloadUpdate(req.Version, func(progress updates.DownloadProgress) {
			currentUpdate.mu.Lock()
			currentUpdate.progress = progress
			currentUpdate.mu.Unlock()
		})

		currentUpdate.mu.Lock()
		currentUpdate.downloading = false
		if err != nil {
			currentUpdate.lastError = err.Error()
		} else {
			currentUpdate.downloadResult = result
		}
		currentUpdate.mu.Unlock()
	}()

	c.JSON(http.StatusAccepted, gin.H{"message": "download started"})
}

// VerifyUpdate verifies the downloaded update.
func (h *Handler) VerifyUpdate(c *gin.Context) {
	currentUpdate.mu.RLock()
	result := currentUpdate.downloadResult
	currentUpdate.mu.RUnlock()

	if result == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no download available to verify"})
		return
	}

	verifyResult, err := updates.VerifyDownload(result)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, verifyResult)
}

// InstallUpdate installs the downloaded update.
func (h *Handler) InstallUpdate(c *gin.Context) {
	currentUpdate.mu.RLock()
	downloadResult := currentUpdate.downloadResult
	currentUpdate.mu.RUnlock()

	if downloadResult == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no download available to install"})
		return
	}

	// Verify before install
	verifyResult, err := updates.VerifyDownload(downloadResult)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !verifyResult.Valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "verification failed: " + verifyResult.Message})
		return
	}

	// Prepare the update (extract if needed)
	executablePath, err := updates.PrepareInstall(downloadResult)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Install
	installResult, err := updates.InstallUpdate(executablePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, installResult)
}
