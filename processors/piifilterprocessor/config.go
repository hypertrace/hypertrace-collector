package piifilterprocessor

import (
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"go.opentelemetry.io/collector/config/configmodels"
)

type Config struct {
	configmodels.ProcessorSettings `mapstructure:",squash"`

	// Global redaction strategy. Defaults to Redact
	RedactStrategy filters.RedactionStrategy `mapstructure:"redaction-strategy"`

	// Prefixes attribute name prefix to match the keyword against
	Prefixes []string `mapstructure:"prefixes"`

	// Regexs are the attribute name of which the value will be filtered
	// when the regex matches the name
	KeyRegExs []PiiElement `mapstructure:"key-regexs"`

	// Regexs are the attribute value which will be filtered when
	// the regex matches
	ValueRegExs []PiiElement `mapstructure:"value-regexs"`
}

// PiiElement identifies configuration for PII filtering
type PiiElement struct {
	Regex             string                    `mapstructure:"regex"`
	Category          string                    `mapstructure:"category"`
	RedactStrategy    filters.RedactionStrategy `mapstructure:"redaction-strategy"`
	Fqn               *bool                     `mapstructure:"fqn,omitempty"`
	SessionIdentifier bool                      `mapstructure:"session-identifier"`
	SessionIndexes    []int                     `mapstructure:"session-indexes"`
	SessionSeparator  string                    `mapstructure:"session-separator"`
}
