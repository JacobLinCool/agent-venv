// Package slug derives a stable filesystem-safe identifier from a name.
// The algorithm matches the other-language implementations: SHA-256 of the
// UTF-8 name, hex-encoded, truncated to 16 characters.
package slug

import (
	"crypto/sha256"
	"encoding/hex"
)

func Of(name string) string {
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:8])
}
