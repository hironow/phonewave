package domain

import (
	"crypto/sha256"
	"encoding/hex"
)

// ContentIdempotencyKey computes a SHA256 hex-encoded idempotency key
// from raw D-Mail content bytes. This provides exact-match dedup
// without needing to parse the D-Mail frontmatter.
func ContentIdempotencyKey(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
