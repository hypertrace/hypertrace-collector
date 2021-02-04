package hash

import (
	cryptosha1 "crypto/sha1"
	"fmt"

	"golang.org/x/crypto/sha3"
)

type Algorithm func(string) string

func sha1(val string) string {
	h := cryptosha1.New()
	h.Write([]byte(val))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func shake256(val string) string {
	h := make([]byte, 64)
	sha3.ShakeSum256(h, []byte(val))
	return fmt.Sprintf("%x", h)
}

var defaultHasher = sha1

// ResolveHashAlgorithm resolves the hasher based on an algo string. If empty or not found it will
// return the default hasher
func ResolveHashAlgorithm(algo string) (Algorithm, bool) {
	switch algo {
	case "SHA-1":
		return sha1, true
	case "SHAKE256":
		return shake256, true
	default:
		return defaultHasher, false
	}
}
