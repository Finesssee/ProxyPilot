package desktopctl

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func randomPassword(nBytes int) (string, error) {
	if nBytes <= 0 {
		return "", fmt.Errorf("invalid password size")
	}
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// URL-safe, no padding.
	return base64.RawURLEncoding.EncodeToString(b), nil
}
