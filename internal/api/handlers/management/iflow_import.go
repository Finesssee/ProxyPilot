package management

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	iflowauth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/iflow"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/errors"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

// ImportIFlowCredential handles uploading a text file with iFlow cookies (one per line).
// Each cookie is authenticated and saved as a separate auth record.
func (h *Handler) ImportIFlowCredential(c *gin.Context) {
	if h == nil || h.cfg == nil {
		e := errors.New(http.StatusServiceUnavailable, "service_unavailable", "config unavailable", nil)
		c.JSON(e.HTTPStatusCode, e)
		return
	}
	if h.cfg.AuthDir == "" {
		e := errors.New(http.StatusServiceUnavailable, "config_error", "auth directory not configured", nil)
		c.JSON(e.HTTPStatusCode, e)
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		e := errors.New(http.StatusBadRequest, "invalid_request", "file required", err)
		c.JSON(e.HTTPStatusCode, e)
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		e := errors.New(http.StatusBadRequest, "file_error", fmt.Sprintf("failed to read file: %v", err), err)
		c.JSON(e.HTTPStatusCode, e)
		return
	}
	defer file.Close()

	ctx := context.Background()
	if reqCtx := c.Request.Context(); reqCtx != nil {
		ctx = reqCtx
	}

	cookies, err := parseCookieLines(file)
	if err != nil {
		e := errors.New(http.StatusBadRequest, "parse_error", "failed to parse file", err)
		c.JSON(e.HTTPStatusCode, e)
		return
	}

	if len(cookies) == 0 {
		e := errors.New(http.StatusBadRequest, "invalid_file", "no valid cookies found in file", nil)
		c.JSON(e.HTTPStatusCode, e)
		return
	}

	authSvc := iflowauth.NewIFlowAuth(h.cfg)
	var results []importResult
	successCount := 0

	for i, raw := range cookies {
		lineNum := i + 1
		result := h.processIFlowCookie(ctx, authSvc, raw, lineNum)
		if result.Success {
			successCount++
		}
		results = append(results, result)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "ok",
		"total":         len(cookies),
		"success_count": successCount,
		"failed_count":  len(cookies) - successCount,
		"results":       results,
	})
}

type importResult struct {
	Line      int    `json:"line"`
	Success   bool   `json:"success"`
	Email     string `json:"email,omitempty"`
	SavedPath string `json:"saved_path,omitempty"`
	Error     string `json:"error,omitempty"`
}

func parseCookieLines(r io.Reader) ([]string, error) {
	var cookies []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cookies = append(cookies, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cookies, nil
}

func (h *Handler) processIFlowCookie(ctx context.Context, authSvc *iflowauth.IFlowAuth, raw string, lineNum int) importResult {
	cookieValue, errNormalize := iflowauth.NormalizeCookie(raw)
	if errNormalize != nil {
		return importResult{Line: lineNum, Success: false, Error: errNormalize.Error()}
	}

	// Check for duplicate BXAuth before authentication
	bxAuth := iflowauth.ExtractBXAuth(cookieValue)
	if existingFile, err := iflowauth.CheckDuplicateBXAuth(h.cfg.AuthDir, bxAuth); err != nil {
		return importResult{Line: lineNum, Success: false, Error: "failed to check duplicate"}
	} else if existingFile != "" {
		existingFileName := filepath.Base(existingFile)
		return importResult{Line: lineNum, Success: false, Error: fmt.Sprintf("duplicate BXAuth found: %s", existingFileName)}
	}

	tokenData, errAuth := authSvc.AuthenticateWithCookie(ctx, cookieValue)
	if errAuth != nil {
		return importResult{Line: lineNum, Success: false, Error: errAuth.Error()}
	}

	tokenData.Cookie = cookieValue

	tokenStorage := authSvc.CreateCookieTokenStorage(tokenData)
	email := strings.TrimSpace(tokenStorage.Email)
	if email == "" {
		return importResult{Line: lineNum, Success: false, Error: "failed to extract email from token"}
	}

	fileName := iflowauth.SanitizeIFlowFileName(email)
	if fileName == "" {
		fileName = fmt.Sprintf("iflow-%d", time.Now().UnixMilli())
	}

	tokenStorage.Email = email
	timestamp := time.Now().Unix()

	record := &coreauth.Auth{
		ID:       fmt.Sprintf("iflow-%s-%d.json", fileName, timestamp),
		Provider: "iflow",
		FileName: fmt.Sprintf("iflow-%s-%d.json", fileName, timestamp),
		Storage:  tokenStorage,
		Metadata: map[string]any{
			"email":        email,
			"api_key":      tokenStorage.APIKey,
			"expired":      tokenStorage.Expire,
			"cookie":       tokenStorage.Cookie,
			"type":         tokenStorage.Type,
			"last_refresh": tokenStorage.LastRefresh,
		},
		Attributes: map[string]string{
			"api_key": tokenStorage.APIKey,
		},
	}

	savedPath, errSave := h.saveTokenRecord(ctx, record)
	if errSave != nil {
		return importResult{Line: lineNum, Success: false, Error: "failed to save authentication tokens"}
	}

	return importResult{Line: lineNum, Success: true, Email: email, SavedPath: savedPath}
}
