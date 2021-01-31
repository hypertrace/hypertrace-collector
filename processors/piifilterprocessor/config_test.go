package piifilterprocessor

import (
	"path"
	"testing"

	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	assert.NoError(t, err)

	factories.Processors[typeStr] = NewFactory()

	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestTransportConfigToConfig(t *testing.T) {
	tCfg := &TransportConfig{
		RedactStrategyName: "hash",
		KeyRegExs: []TransportPiiElement{{
			RegexPattern:       "[a-z]",
			RedactStrategyName: "redact",
		}},
		ValueRegExs: []TransportPiiElement{{
			RegexPattern:       "[a-z]+",
			RedactStrategyName: "raw",
		}},
		ComplexData: []TransportPiiComplexData{{
			Key:  "query",
			Type: "sql",
		}},
	}

	cfg, err := tCfg.toConfig()
	assert.NoError(t, err)
	assert.Equal(t, redaction.Hash, cfg.RedactStrategy)
	assert.Equal(t, "[a-z]", cfg.KeyRegExs[0].Regex.String())
	assert.Equal(t, redaction.Redact, cfg.KeyRegExs[0].RedactStrategy)
	assert.Equal(t, "[a-z]+", cfg.ValueRegExs[0].Regex.String())
	assert.Equal(t, redaction.Raw, cfg.ValueRegExs[0].RedactStrategy)
	assert.Equal(t, "query", cfg.ComplexData[0].Key)
	assert.Equal(t, "sql", cfg.ComplexData[0].Type)
}
