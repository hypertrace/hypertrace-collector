package tenantidprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, defaultHeaderName, cfg.TenantIDHeaderName)
	assert.Equal(t, defaultAttributeKey, cfg.TenantIDAttributeKey)
}
