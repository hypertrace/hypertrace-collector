package piifilterprocessor

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
	"go.opentelemetry.io/collector/config/configmodels"
)

// TransportConfig is the config coming directly from the user input.
// The processor first matches span attribute key with attribute keys in complex data where the complex
// data defines type of the attribute.
// If there is no match the attribute key is checked against prefixes to remove prefix (e.g. http.request.header) and then
// the final the key value is matched against key_regexs.
type TransportConfig struct {
	configmodels.ProcessorSettings `mapstructure:",squash"`

	// Global redaction strategy. Available redact, hash, Defaults to Redact
	RedactStrategyName string `mapstructure:"redaction_strategy"`

	// Prefixes attribute name prefix to match the keyword against
	Prefixes []string `mapstructure:"prefixes"`

	// Regexs are the attribute name of which the value will be filtered
	// when the regex matches the name
	KeyRegExs []TransportPiiElement `mapstructure:"key_regexs"`

	// Regexs are the attribute value which will be filtered when
	// the regex matches
	ValueRegExs []TransportPiiElement `mapstructure:"value_regexs"`

	// ComplexData contains all complex data types to filter, such
	// as json, sql etc
	ComplexData []TransportPiiComplexData `mapstructure:"complex_data"`
}

type TransportPiiElement struct {
	RegexPattern string `mapstructure:"regex"`
	// Redaction strategy e.g. raw (keep the attribute), redact (***), hash (SHA-1)
	RedactStrategyName string `mapstructure:"redaction_strategy"`
	// Use fully qualified name of flattened complex data
	FQN bool `mapstructure:"fqn,omitempty"`
	// Whether the element represents a user session.
	// The redacted element is added as session.id attribute to span.
	SessionIdentifier bool `mapstructure:"session_identifier"`
}

type TransportPiiComplexData struct {
	Key string `mapstructure:"key"`
	// Type of the attribute - json, urlencoded, keyvalue, cookie, sql
	Type string `mapstructure:"type"`
	// Attribute key with type.
	TypeKey string `mapstructure:"type_key"`
}

func (tpe *TransportPiiElement) toPiiElement() (*PiiElement, error) {
	rp, err := regexp.Compile(tpe.RegexPattern)
	if err != nil {
		return nil, fmt.Errorf("error compiling key regex %s already specified", tpe.RegexPattern)
	}

	return &PiiElement{
		Regex:             rp,
		RedactStrategy:    mapToRedactionStrategy(tpe.RedactStrategyName),
		FQN:               tpe.FQN,
		SessionIdentifier: tpe.SessionIdentifier,
	}, nil
}

func (tc *TransportConfig) toConfig() (*Config, error) {
	c := &Config{
		ProcessorSettings: tc.ProcessorSettings,
		RedactStrategy:    mapToRedactionStrategy(tc.RedactStrategyName),
	}

	for _, prefix := range tc.Prefixes {
		if strings.Trim(prefix, " ") == "" {
			return nil, fmt.Errorf("invalid prefix, ")
		}
	}

	c.KeyRegExs = make([]PiiElement, len(tc.KeyRegExs))
	for i, tpe := range tc.KeyRegExs {
		if pe, err := tpe.toPiiElement(); err == nil {
			c.KeyRegExs[i] = *pe
		} else {
			return nil, err
		}
	}

	c.ValueRegExs = make([]PiiElement, len(tc.ValueRegExs))
	for i, tpe := range tc.ValueRegExs {
		if pe, err := tpe.toPiiElement(); err == nil {
			c.ValueRegExs[i] = *pe
		} else {
			return nil, err
		}
	}

	for _, tpe := range tc.ComplexData {
		if tpe.Key == "" {
			return nil, errors.New("key for complex data entry is empty")
		}

		c.ComplexData = append(c.ComplexData, PiiComplexData(tpe))
	}

	return c, nil
}

func mapToRedactionStrategy(name string) redaction.Strategy {
	switch name {
	case "hash":
		return redaction.Hash
	case "raw":
		return redaction.Raw
	case "redact":
		return redaction.Redact
	default:
		return redaction.Unknown
	}
}

type Config struct {
	configmodels.ProcessorSettings
	RedactStrategy redaction.Strategy
	Prefixes       []string
	KeyRegExs      []PiiElement
	ValueRegExs    []PiiElement
	ComplexData    []PiiComplexData
}

type PiiElement struct {
	Regex             *regexp.Regexp
	RedactStrategy    redaction.Strategy
	FQN               bool
	SessionIdentifier bool
}

type PiiComplexData struct {
	Key     string
	Type    string
	TypeKey string
}
