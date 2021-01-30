package redaction

import (
	"crypto/sha1"
	"fmt"
)

// HashRedactor returns a hashed value which can't be reversed but still identified
func HashRedactor(val string) string {
	h := sha1.New()
	h.Write([]byte(val))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// RedactRedactor returns a obfuscated value can't be reversed nor identified
func RedactRedactor(_ string) string { return "***" }

// RawRedactor returns the raw value
func RawRedactor(val string) string { return val }

// Redactor redacts a string based on redaction strategy
type Redactor func(string) string

var (
	// DefaultRedactor is the default redactor which masks the text
	DefaultRedactor = RedactRedactor
)

// Redactors is a map of redactors identified by the strategy
var Redactors = map[Strategy]Redactor{
	Redact: RedactRedactor,
	Hash:   HashRedactor,
	Raw:    RawRedactor,
}
