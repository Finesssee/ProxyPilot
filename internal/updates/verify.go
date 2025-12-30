package updates

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// VerifyResult contains the result of a verification check.
type VerifyResult struct {
	Valid    bool   `json:"valid"`
	Message  string `json:"message"`
	Checksum string `json:"checksum,omitempty"`
}

// VerifyDownload verifies the downloaded file using GPG signature verification.
// The binary must be signed with the embedded public key to be considered valid.
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

	// If we have a signature file, verify the GPG signature
	if result.SignaturePath != "" {
		sigValid, err := verifySignatureFile(result.FilePath, result.SignaturePath)
		if err != nil {
			// Signature verification failed - file is NOT valid
			return &VerifyResult{
				Valid:    false,
				Message:  fmt.Sprintf("signature verification failed: %v", err),
				Checksum: checksum,
			}, nil
		}
		if !sigValid {
			return &VerifyResult{
				Valid:   false,
				Message: "signature verification failed: invalid signature",
			}, nil
		}
		return &VerifyResult{
			Valid:    true,
			Message:  "GPG signature verified successfully",
			Checksum: checksum,
		}, nil
	}

	// No signature file provided - fail secure
	return &VerifyResult{
		Valid:    false,
		Message:  "no signature file available for verification",
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

// verifySignatureFile verifies a GPG detached signature against the binary file.
// It uses the embedded public key to verify the signature.
func verifySignatureFile(filePath, sigPath string) (bool, error) {
	// Check if public key is configured
	if !IsKeyConfigured() {
		return false, errors.New("GPG public key not configured: please replace the placeholder key with your actual signing key")
	}

	// Read the public key
	keyring, err := openpgp.ReadArmoredKeyRing(strings.NewReader(GetPublicKey()))
	if err != nil {
		return false, fmt.Errorf("failed to read public key: %w", err)
	}

	// Open the binary file for verification
	binaryFile, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to open binary file: %w", err)
	}
	defer binaryFile.Close()

	// Read the signature file
	sigData, err := os.ReadFile(sigPath)
	if err != nil {
		return false, fmt.Errorf("failed to read signature file: %w", err)
	}

	sigStr := strings.TrimSpace(string(sigData))
	if len(sigStr) < 64 {
		return false, errors.New("signature file is too short to be valid")
	}

	// Determine if signature is armored (ASCII) or binary
	if strings.HasPrefix(sigStr, "-----BEGIN PGP SIGNATURE-----") {
		// Armored signature - use CheckArmoredDetachedSignature
		signer, err := openpgp.CheckArmoredDetachedSignature(keyring, binaryFile, strings.NewReader(sigStr), nil)
		if err != nil {
			return false, fmt.Errorf("armored signature verification failed: %w", err)
		}
		if signer == nil {
			return false, errors.New("signature is valid but signer not found in keyring")
		}
		return true, nil
	}

	// Binary signature - use CheckDetachedSignature
	sigReader, err := os.Open(sigPath)
	if err != nil {
		return false, fmt.Errorf("failed to open signature file: %w", err)
	}
	defer sigReader.Close()

	// Reset binary file position after the armored check read it
	if _, err := binaryFile.Seek(0, io.SeekStart); err != nil {
		return false, fmt.Errorf("failed to reset binary file position: %w", err)
	}

	signer, err := openpgp.CheckDetachedSignature(keyring, binaryFile, sigReader, nil)
	if err != nil {
		return false, fmt.Errorf("binary signature verification failed: %w", err)
	}
	if signer == nil {
		return false, errors.New("signature is valid but signer not found in keyring")
	}

	return true, nil
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
