package redaction

import (
	"crypto/sha1"
	"fmt"
)

func hashRedacter(val string) string {
	h := sha1.New()
	h.Write([]byte(val))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func redactRedacter(_ string) string { return "***" }

func rawRedacter(val string) string { return val }

// Redacter redacts a string based on redaction strategy
type Redacter func(string) string

var (
	// DefaultRedacter is the default redacter which masks the text
	DefaultRedacter = redactRedacter
)

// Redacters is a map of redacters identified by the strategy
var Redacters = map[Strategy]Redacter{
	Unknown: rawRedacter,
	Redact:  redactRedacter,
	Hash:    hashRedacter,
	Raw:     rawRedacter,
}
