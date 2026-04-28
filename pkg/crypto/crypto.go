package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// NewID returns a cryptographically random hex string of the given byte length.
// Use length=16 for a 32-character ID.
func NewID(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto rand read: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Hash returns the SHA-256 hex digest of the input string.
func Hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
