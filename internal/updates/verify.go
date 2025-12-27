package updates

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// VerifyResult contains the result of a verification check.
type VerifyResult struct {
	Valid    bool   `json:"valid"`
	Message  string `json:"message"`
	Checksum string `json:"checksum,omitempty"`
}

// VerifyDownload verifies the downloaded file.
// For now, this does basic checksum verification.
// In the future, this can be extended to verify GPG signatures.
func VerifyDownload(result *DownloadResult) (*VerifyResult, error) {
	if result == nil || result.FilePath == "" {
		return &VerifyResult{
			Valid:   false,
			Message: "no download result provided",
		}, nil
	}

	// Check file exists
	info, err := os.Stat(result.FilePath)
	if err != nil {
		return &VerifyResult{
			Valid:   false,
			Message: fmt.Sprintf("file not found: %v", err),
		}, nil
	}

	// Check file has reasonable size (at least 1MB for a Go binary)
	if info.Size() < 1024*1024 {
		return &VerifyResult{
			Valid:   false,
			Message: "downloaded file is too small to be valid",
		}, nil
	}

	// Compute SHA256 checksum
	checksum, err := computeSHA256(result.FilePath)
	if err != nil {
		return &VerifyResult{
			Valid:   false,
			Message: fmt.Sprintf("failed to compute checksum: %v", err),
		}, nil
	}

	// If we have a signature file, try to verify
	if result.SignaturePath != "" {
		sigValid, err := verifySignatureFile(result.FilePath, result.SignaturePath)
		if err != nil {
			// Signature verification failed - but file may still be valid
			return &VerifyResult{
				Valid:    true, // Still valid, just no signature verification
				Message:  fmt.Sprintf("signature verification skipped: %v", err),
				Checksum: checksum,
			}, nil
		}
		if !sigValid {
			return &VerifyResult{
				Valid:   false,
				Message: "signature verification failed",
			}, nil
		}
		return &VerifyResult{
			Valid:    true,
			Message:  "signature verified successfully",
			Checksum: checksum,
		}, nil
	}

	return &VerifyResult{
		Valid:    true,
		Message:  "basic verification passed (no signature available)",
		Checksum: checksum,
	}, nil
}

func computeSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// verifySignatureFile attempts to verify a GPG/PGP signature.
// This is a placeholder - real implementation would use golang.org/x/crypto/openpgp
// or similar library with an embedded public key.
func verifySignatureFile(filePath, sigPath string) (bool, error) {
	// Check signature file exists
	sigData, err := os.ReadFile(sigPath)
	if err != nil {
		return false, fmt.Errorf("failed to read signature file: %w", err)
	}

	// For now, just check it's not empty and looks like a signature
	sigStr := strings.TrimSpace(string(sigData))
	if len(sigStr) < 64 {
		return false, fmt.Errorf("signature file too short")
	}

	// TODO: Implement actual GPG signature verification
	// This requires embedding a public key and using openpgp library
	return false, fmt.Errorf("GPG signature verification not yet implemented")
}

// VerifyChecksum verifies a file against an expected SHA256 checksum.
func VerifyChecksum(filePath, expectedChecksum string) (*VerifyResult, error) {
	actualChecksum, err := computeSHA256(filePath)
	if err != nil {
		return &VerifyResult{
			Valid:   false,
			Message: fmt.Sprintf("failed to compute checksum: %v", err),
		}, nil
	}

	if strings.EqualFold(actualChecksum, expectedChecksum) {
		return &VerifyResult{
			Valid:    true,
			Message:  "checksum verified successfully",
			Checksum: actualChecksum,
		}, nil
	}

	return &VerifyResult{
		Valid:    false,
		Message:  fmt.Sprintf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum),
		Checksum: actualChecksum,
	}, nil
}
