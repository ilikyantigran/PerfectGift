// Package token mints opaque link tokens and hashes them for storage.
//
// The raw token is what appears in the shared URL and is returned to the poll
// owner exactly once. Only its SHA-256 hash is ever persisted, so a database leak
// never exposes a live link.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// rawBytes is the entropy of a link token (256 bits).
const rawBytes = 32

// New returns a fresh opaque token: the raw value (base64url, share in the URL)
// and its hash (hex SHA-256, safe to store).
func New() (raw string, hash string, err error) {
	b := make([]byte, rawBytes)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, Hash(raw), nil
}

// Hash is the one-way transform applied before storage and before lookup.
func Hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
