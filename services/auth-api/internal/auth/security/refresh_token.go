package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// NewRefreshToken returns the opaque token (sent to the client) and its hash (stored).
func NewRefreshToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashToken(raw), nil
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
