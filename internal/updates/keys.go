package updates

import (
	_ "embed"
)

// PublicKey is the embedded GPG public key used to verify release signatures.
// This key should be replaced with the actual release signing key before deployment.
//
// To generate a new GPG key pair for signing releases:
//   1. gpg --full-generate-key (select RSA and RSA, 4096 bits)
//   2. gpg --armor --export KEY_ID > release-signing-key.asc
//   3. Replace the content of release-signing-key.asc below
//
// The private key should be kept secure and used only in the CI/CD pipeline
// to sign release binaries.
//
//go:embed release-signing-key.asc
var publicKeyArmored string

// GetPublicKey returns the embedded public key for signature verification.
func GetPublicKey() string {
	return publicKeyArmored
}

// IsKeyConfigured returns true if a real public key has been configured.
// This checks if the key is not the placeholder content.
func IsKeyConfigured() bool {
	// Check if the key contains the actual PGP public key block markers
	return len(publicKeyArmored) > 0 &&
		contains(publicKeyArmored, "-----BEGIN PGP PUBLIC KEY BLOCK-----") &&
		contains(publicKeyArmored, "-----END PGP PUBLIC KEY BLOCK-----")
}

// contains is a simple helper to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
