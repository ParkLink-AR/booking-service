package entity

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// GenerateVerificationCode returns a random 4-digit numeric code ("0000"-"9999"),
// zero-padded, for a driver to relay to a parking spot's owner in person. Uses
// crypto/rand rather than math/rand -- this project's own security rule -- even though
// the code itself is a short-lived, low-stakes PIN meant to be read aloud, not a
// cryptographic secret.
func GenerateVerificationCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%04d", n.Int64()), nil
}
