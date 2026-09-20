// Package idgen generates short, prefixed, random IDs for demo entities
// (quotes, policies, claims).
package idgen

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// New returns an id of the form "<prefix>-<8 random hex characters>".
func New(prefix string) string {
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(buf))
}
