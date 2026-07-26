package vwp

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// SHA256Hex returns the lowercase hex SHA-256 digest of data, the digest form
// used everywhere in VWP manifests and evidence records.
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// VerifyPartHash recomputes the digest of data and compares it to the
// declared lowercase hex digest. A nil error means the digests match.
func VerifyPartHash(data []byte, declaredSHA256 string) error {
	actual := SHA256Hex(data)
	if actual != declaredSHA256 {
		return fmt.Errorf("vwp: hash mismatch: declared %s, actual %s", declaredSHA256, actual)
	}
	return nil
}
