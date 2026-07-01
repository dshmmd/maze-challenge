// Package idgen provides a simple, dependency-free unique ID generator.
package idgen

import (
	"crypto/rand"
	"encoding/hex"
)

// Hex generates random 128-bit hex IDs. It satisfies ports.IDGenerator.
type Hex struct{}

// NewID returns a new random 32-char hex string.
func (Hex) NewID() string {
	var b [16]byte
	// crypto/rand.Read never returns an error on supported platforms.
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
