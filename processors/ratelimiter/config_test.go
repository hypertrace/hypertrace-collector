package ratelimiter

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.NoError(t, err)

	factories.Processors[typeStr] = NewFactory()

	cfg, err := otelcoltest.LoadConfig(path.Join(".", "testdata", "config.yml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	id := component.ID{}
	id.UnmarshalText([]byte(typeStr))
	tIDcfg := cfg.Processors[id].(*Config)
	assert.Equal(t, "header-tenant", tIDcfg.TenantIDHeaderName)
	assert.Equal(t, "app", tIDcfg.Domain)
	assert.Equal(t, "localhost", tIDcfg.ServiceHost)
	assert.Equal(t, uint16(8081), tIDcfg.ServicePort)
	assert.Equal(t, uint32(10), tIDcfg.DomainSoftRateLimitThreshold)
	assert.Equal(t, uint32(10), tIDcfg.TimeoutMillis)
}
