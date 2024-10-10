package spancounter

import (
	"context"
	"testing"

	"github.com/hypertrace/collector/processors/spancounter/internal/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
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
	telemetryBuilder, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	p := newProcessor(logger, c, telemetryBuilder)
	assert.Equal(t, defaultTenantIDAttributeKey, p.tenantIDAttributeKey)

	c.TenantIDAttributeKey = "custom-tenant-id"
	p = newProcessor(logger, c, telemetryBuilder)
	assert.Equal(t, "custom-tenant-id", p.tenantIDAttributeKey)
}

func TestCreateTenantsMap(t *testing.T) {
	c := &Config{
		TenantConfigs: []TenantConfig{
			{
				// No tenant id. Will skip this whole config.
				ServiceConfigs: []ServiceConfig{
					{
						ServiceName: "example-service",
						SpanConfigs: []SpanConfig{
							{
								Label:    "example-label",
								SpanName: "span-1",
							},
						},
					},
				},
			},
			{
				TenantId: "example-tenant",
				ServiceConfigs: []ServiceConfig{
					{
						ServiceName: "example-service-1",
						SpanConfigs: []SpanConfig{
							{
								Label:    "example-label-1",
								SpanName: "span-1",
							},
						},
					},
					{
						// No service name. Will skip
						SpanConfigs: []SpanConfig{
							{
								Label:    "example-label-2",
								SpanName: "span-2",
							},
						},
					},
				},
			},
			// well formed config
			{
				TenantId: "example-tenant-2",
				ServiceConfigs: []ServiceConfig{
					{
						ServiceName: "example-service-10",
						SpanConfigs: []SpanConfig{
							{
								Label:    "example-label-10",
								SpanName: "span-10",
							},
						},
					},
					{
						ServiceName: "example-service-11",
						SpanConfigs: []SpanConfig{
							{
								Label:    "label-11",
								SpanName: "span-11",
								SpanAttributes: []SpanAttribute{
									{Key: "k1"},
									{Key: "k2", Value: "v2"},
								},
							},
						},
					},
				},
			},
		},
	}

	expectedMap := map[string]map[string][]SpanConfig{
		"example-tenant": {
			"example-service-1": {
				{
					Label:    "example-label-1",
					SpanName: "span-1",
				},
			},
		},
		"example-tenant-2": {
			"example-service-10": {
				{
					Label:    "example-label-10",
					SpanName: "span-10",
				},
			},
			"example-service-11": {
				{
					Label:    "label-11",
					SpanName: "span-11",
					SpanAttributes: []SpanAttribute{
						{Key: "k1"},
						{Key: "k2", Value: "v2"},
					},
				},
			},
		},
	}

	assert.Equal(t, expectedMap, createTenantsMap(c))
}

func TestSpanMatchesConfig(t *testing.T) {
	span := ptrace.NewSpan()
	span.SetName("span1")
	span.Attributes().PutStr("a1", "v1")
	span.Attributes().PutStr("a2", "v2")

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
			{Key: "a1", Value: "v1"},
			{Key: "a2", Value: "v2"},
		},
	}
	assert.True(t, spanMatchesConfig(span, sc))

	sc = SpanConfig{
		SpanName: "span1",
		SpanAttributes: []SpanAttribute{
			{Key: "a1"},
			{Key: "a2", Value: "v2"},
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

	sc = SpanConfig{
		SpanAttributes: []SpanAttribute{
			{Key: "a1", Value: "v3"},
		},
	}
	assert.False(t, spanMatchesConfig(span, sc))

	sc = SpanConfig{
		SpanAttributes: []SpanAttribute{
			{Key: "a3"},
		},
	}
	assert.False(t, spanMatchesConfig(span, sc))
}

func TestProcessTraces(t *testing.T) {
	// Create test traces
	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty()
	rs0 := td.ResourceSpans().At(0)
	rs0.Resource().Attributes().PutStr(defaultTenantIDAttributeKey, "example-tenant-1")
	rs0.Resource().Attributes().PutStr(conventions.AttributeServiceName, "example-service-1")
	rs0.ScopeSpans().AppendEmpty()
	rs0scopespan0 := td.ResourceSpans().At(0).ScopeSpans().At(0)
	rs0scopespan0.Spans().AppendEmpty()
	rs0scopespan0.Spans().At(0).SetName("span-1")
	rs0scopespan0.Spans().At(0).Attributes().PutStr("k1", "v1")
	rs0scopespan0.Spans().At(0).Attributes().PutStr("k2", "v2")
	rs0scopespan0.Spans().AppendEmpty()
	rs0scopespan0.Spans().At(1).SetName("span-2")
	rs0scopespan0.Spans().At(1).Attributes().PutStr("k1", "v10")
	rs0scopespan0.Spans().At(1).Attributes().PutStr("k2", "v20")
	rs0scopespan0.Spans().AppendEmpty()
	rs0scopespan0.Spans().At(2).SetName("span-1")
	rs0scopespan0.Spans().At(2).Attributes().PutStr("k1", "v1")
	rs0scopespan0.Spans().At(2).Attributes().PutStr("k2", "v23")

	logger := zap.NewNop()
	c := &Config{
		TenantConfigs: []TenantConfig{
			{
				TenantId: "example-tenant-1",
				ServiceConfigs: []ServiceConfig{
					{
						ServiceName: "example-service-1",
						SpanConfigs: []SpanConfig{
							{
								SpanName: "span-1",
								SpanAttributes: []SpanAttribute{
									{Key: "k1", Value: "v1"},
									{Key: "k2", Value: "v23"},
								},
							},
						},
					},
				},
			},
		},
	}
	telemetryBuilder, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	p := newProcessor(logger, c, telemetryBuilder)

	// We cannot verify metrics :( We will verify no errors and no change in traces
	processedTd, err := p.ProcessTraces(context.Background(), td)
	assert.NoError(t, err)
	assert.Equal(t, td, processedTd)

	// Non matching tenant should also not throw an error
	c.TenantConfigs[0].TenantId = "example-tenant-2"
	p = newProcessor(logger, c, telemetryBuilder)

	processedTd, err = p.ProcessTraces(context.Background(), td)
	assert.NoError(t, err)
	assert.Equal(t, td, processedTd)

	// Empty config
	c = &Config{}
	p = newProcessor(logger, c, telemetryBuilder)

	processedTd, err = p.ProcessTraces(context.Background(), td)
	assert.NoError(t, err)
	assert.Equal(t, td, processedTd)
}
