package filters

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/consumer/pdata"
)

type wrappedError struct {
	reason string
	err    error
}

func (e *wrappedError) Unwrap() error { return e.err }

func (e *wrappedError) Error() string { return fmt.Sprintf("%s: %v", e.reason, e.err) }

func WrapError(delegate error, reason string) error {
	return &wrappedError{reason: reason, err: delegate}
}

var (
	// ErrUnprocessableValue represents a recoverable error where a certain value
	// cannot be processed as expected e.g. a malformed cookie or an invalid JSON
	// in an attribute. This kind of errors should be treated as debug rather than
	// an error when it comes to logging.
	ErrUnprocessableValue = errors.New("filter cannot process value")
)

// Filter redacts attributes from a span
type Filter interface {
	// RedactAttribute decided to redact and attribute and returns true if the value has
	// been redacted or false otherwise. It also returns and error when something went
	// went wrong by redacting the value.
	RedactAttribute(key string, value pdata.AttributeValue) (isRedacted bool, err error)
}
