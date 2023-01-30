package ratelimiter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, defaultHeaderName, cfg.TenantIDHeaderName)
	assert.Equal(t, defaultServiceHost, cfg.ServiceHost)
	assert.Equal(t, defaultServicePort, cfg.ServicePort)
	assert.Equal(t, defaultDomainSoftLimitThreshold, cfg.DomainSoftRateLimitThreshold)
	assert.Equal(t, defaultDomain, cfg.Domain)
	assert.Equal(t, defaultTimeoutMillis, cfg.TimeoutMillis)
}
