package piifilterprocessor

import (
	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
	"go.opentelemetry.io/collector/config/configmodels"
)

type Config struct {
	configmodels.ProcessorSettings `mapstructure:",squash"`

	// Global redaction strategy. Defaults to Redact
	RedactStrategy redaction.Strategy `mapstructure:"redaction_strategy"`

	// Regexs are the attribute name of which the value will be filtered
	// when the regex matches the name
	KeyRegExs []PiiElement `mapstructure:"key_regexs"`

	// Regexs are the attribute value which will be filtered when
	// the regex matches
	ValueRegExs []PiiElement `mapstructure:"value_regexs"`

	// ComplexData contains all complex data types to filter, such
	// as json, sql etc
	ComplexData []PiiComplexData `mapstructure:"complex_data"`
}

// PiiElement identifies configuration for PII filtering
type PiiElement struct {
	Regex          string             `mapstructure:"regex"`
	RedactStrategy redaction.Strategy `mapstructure:"redaction_strategy"`
	FQN            bool               `mapstructure:"fqn,omitempty"`
}

// PiiComplexData identifes the attribute names which define
// where the content is and where the content type or
// the type itself
type PiiComplexData struct {
	Key     string `mapstructure:"key"`
	Type    string `mapstructure:"type"`
	TypeKey string `mapstructure:"type_key"`
}
