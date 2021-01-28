package redaction

import (
	"crypto/sha1"
	"fmt"
)

// HashRedacter returns a hashed value which can't be reversed but still identified
func HashRedacter(val string) string {
	h := sha1.New()
	h.Write([]byte(val))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// RedactRedacter returns a obfuscated value can't be reversed nor identified
func RedactRedacter(_ string) string { return "***" }

// RawRedacter returns the raw value
func RawRedacter(val string) string { return val }

// Redacter redacts a string based on redaction strategy
type Redacter func(string) string

var (
	// DefaultRedacter is the default redacter which masks the text
	DefaultRedacter = RedactRedacter
)

// Redacters is a map of redacters identified by the strategy
var Redacters = map[Strategy]Redacter{
	Redact: RedactRedacter,
	Hash:   HashRedacter,
	Raw:    RawRedacter,
}
