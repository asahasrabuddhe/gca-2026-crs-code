package container

import (
	"crypto/rand"
	"encoding/hex"
)

// newID is twelve hex characters, the shape of a short Docker ID, and
// from 04-uts the container's hostname. The 2025 version hashed the time;
// crypto/rand cannot collide when two containers start in the same tick.
func newID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return hex.EncodeToString(b)
}
