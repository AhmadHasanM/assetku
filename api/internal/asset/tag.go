package asset

import (
	"crypto/rand"
	"math/big"
	"strings"
)

const tagAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GenerateTag returns "AST-" followed by 8 random characters from A-Z0-9.
func GenerateTag() string {
	var b strings.Builder
	b.WriteString("AST-")
	n := big.NewInt(int64(len(tagAlphabet)))
	for range 8 {
		i, err := rand.Int(rand.Reader, n)
		if err != nil {
			panic(err) // crypto/rand never fails on supported platforms
		}
		b.WriteByte(tagAlphabet[i.Int64()])
	}
	return b.String()
}

// NormalizeTag trims surrounding whitespace; case is preserved (uniqueness is
// case-insensitive in the database).
func NormalizeTag(s string) string { return strings.TrimSpace(s) }
