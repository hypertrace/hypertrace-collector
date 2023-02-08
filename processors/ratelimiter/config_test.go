package ratelimiter

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factories.Processors[typeStr] = NewFactory()

	cfg, err := servicetest.LoadConfig(path.Join(".", "testdata", "config.yml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	tIDcfg := cfg.Processors[config.NewComponentID(typeStr)].(*Config)
	assert.Equal(t, "header-tenant", tIDcfg.TenantIDHeaderName)
	assert.Equal(t, "app", tIDcfg.Domain)
	assert.Equal(t, "localhost", tIDcfg.ServiceHost)
	assert.Equal(t, uint16(8081), tIDcfg.ServicePort)
	assert.Equal(t, uint32(10), tIDcfg.TimeoutMillis)
}
