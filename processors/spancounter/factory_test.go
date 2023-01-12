package spancounter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, defaultTenantIDAttributeKey, cfg.TenantIDAttributeKey)
}

func TestAddUniqueLabelsToSpanConfigs(t *testing.T) {
	c := &Config{
		TenantConfigs: []TenantConfig{
			{
				TenantId: "example-tenant",
				ServiceConfigs: []ServiceConfig{
					{
						ServiceName: "example-service",
						SpanConfigs: []SpanConfig{
							{
								Label:    "example-label",
								SpanName: "span-1",
							},
							{
								SpanName: "span-2",
							},
						},
					},
					{
						ServiceName: "example-service-2",
						SpanConfigs: []SpanConfig{
							{
								SpanName: "span-20",
							},
						},
					},
				},
			},
		},
	}

	addUniqueLabelsToSpanConfigs(c)
	assert.Equal(t, "example-label", c.TenantConfigs[0].ServiceConfigs[0].SpanConfigs[0].Label)
	assert.Greater(t, len(c.TenantConfigs[0].ServiceConfigs[0].SpanConfigs[1].Label), 0)
	assert.Greater(t, len(c.TenantConfigs[0].ServiceConfigs[1].SpanConfigs[0].Label), 0)
}
