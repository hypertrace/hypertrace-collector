package redaction

// Strategy describes the redaction strategy to use during the filtering
// of attributes.
type Strategy int

const (
	Unknown Strategy = iota
	Redact
	Hash
	Raw
)
