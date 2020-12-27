package filters

import "go.opentelemetry.io/collector/consumer/pdata"

// Filter redacts attributes from a span
type Filter interface {
	// RedactAttribute decided to redact and attribute and returns true if the value has
	// been redacted or false otherwise. It also returns and error when something went
	// went wrong by redacting the value.
	RedactAttribute(key string, value pdata.AttributeValue) (isRedacted bool, err error)
}
