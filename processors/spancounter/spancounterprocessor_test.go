package spancounter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func TestNewProcessor(t *testing.T) {
	logger := zap.NewNop()
	c := &Config{
		TenantConfigs: []TenantConfig{
			{
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

	p := newProcessor(logger, c)
	assert.Equal(t, defaultTenantIDAttributeKey, p.tenantIDAttributeKey)

	c.TenantIDAttributeKey = "custom-tenant-id"
	p = newProcessor(logger, c)
	assert.Equal(t, "custom-tenant-id", p.tenantIDAttributeKey)
}

func TestSpanMatchesConfig(t *testing.T) {
	span := ptrace.NewSpan()
	span.SetName("span1")
	span.Attributes().PutString("a1", "v1")
	span.Attributes().PutString("a2", "v2")

	sc := SpanConfig{SpanName: "span1"}
	assert.True(t, spanMatchesConfig(span, sc))

	sc = SpanConfig{SpanName: "span2"}
	assert.False(t, spanMatchesConfig(span, sc))

	sc = SpanConfig{
		SpanName: "span1",
		SpanAttributes: []SpanAttribute{
			{Key: "a1"},
		},
	}
	assert.True(t, spanMatchesConfig(span, sc))

	sc = SpanConfig{
		SpanName: "span1",
		SpanAttributes: []SpanAttribute{
			{Key: "a1", Value: "v1"},
		},
	}
	assert.True(t, spanMatchesConfig(span, sc))

	sc = SpanConfig{
		SpanName: "span1",
		SpanAttributes: []SpanAttribute{
			{Key: "a1", Value: "v3"},
		},
	}
	assert.False(t, spanMatchesConfig(span, sc))
}
