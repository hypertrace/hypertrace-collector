package tenantidprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, defaultTenantIdHeaderName, cfg.TenantIDHeaderName)
	assert.Equal(t, defaultTenantIdAttributeKey, cfg.TenantIDAttributeKey)
}
