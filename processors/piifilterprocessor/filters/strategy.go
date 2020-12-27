package filters

// RedactionStrategy describes the redaction strategy to use during the filtering
// of attributes.
type RedactionStrategy int

const (
	Unknown RedactionStrategy = iota
	Redact
	Hash
	Raw
)

const (
	RedactedText = "***"
)
