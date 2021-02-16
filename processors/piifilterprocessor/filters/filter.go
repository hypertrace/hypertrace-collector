package filters

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/hypertrace/collector/processors"
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
	Name() string

	// RedactAttribute redacts and attribute based on the filter configuration.
	// It returns reduction result as ParsedAttribute and optionally a new attribute
	// that should be added to a span.
	RedactAttribute(key string, value pdata.AttributeValue) (parsedAttribute *processors.ParsedAttribute, newAttribute *Attribute, err error)
}

// Attribute holds key and attribute value.
type Attribute struct {
	Key   string
	Value pdata.AttributeValue
}
