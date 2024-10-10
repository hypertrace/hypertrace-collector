package spancounter

import (
	"context"

	"github.com/hypertrace/collector/processors/spancounter/internal/metadata"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const (
	defaultTenantIDAttributeKey string = "tenant-id"
	tagSpanCriteriaLabel        string = "span-criteria-label"
)

type spanCounterProcessor struct {
	logger *zap.Logger
	// The levels are tenant > service > span config. So this is a map of tenant ids
	// to maps of service names to span configs
	tenantIDAttributeKey string
	tenantsMap           map[string]map[string][]SpanConfig
	telemetryBuilder     *metadata.TelemetryBuilder
}

func newProcessor(logger *zap.Logger, cfg *Config, telemetryBuilder *metadata.TelemetryBuilder) *spanCounterProcessor {
	tm := createTenantsMap(cfg)
	tenantIDAttributeKey := defaultTenantIDAttributeKey
	if len(cfg.TenantIDAttributeKey) != 0 {
		tenantIDAttributeKey = cfg.TenantIDAttributeKey
	}
	return &spanCounterProcessor{
		logger:               logger,
		tenantIDAttributeKey: tenantIDAttributeKey,
		tenantsMap:           tm,
		telemetryBuilder:     telemetryBuilder,
	}
}

func createTenantsMap(cfg *Config) map[string]map[string][]SpanConfig {
	m := make(map[string]map[string][]SpanConfig, len(cfg.TenantConfigs))
	for _, tc := range cfg.TenantConfigs {
		if len(tc.TenantId) == 0 { // skip empty tenant id
			continue
		}
		sm := make(map[string][]SpanConfig, len(tc.ServiceConfigs))
		for _, sc := range tc.ServiceConfigs {
			if len(sc.ServiceName) == 0 { // skip empty service name
				continue
			}

			sm[sc.ServiceName] = sc.SpanConfigs
		}

		m[tc.TenantId] = sm
	}
	return m
}

func (p *spanCounterProcessor) ProcessTraces(ctx context.Context, traces ptrace.Traces) (ptrace.Traces, error) {
	if len(p.tenantsMap) == 0 {
		return traces, nil
	}

	rss := traces.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		tenantIdVal, found := rs.Resource().Attributes().Get(p.tenantIDAttributeKey)
		tenantId := tenantIdVal.Str()
		if !found || len(tenantId) == 0 {
			continue
		}
		servicesMap, found := p.tenantsMap[tenantId]
		if !found || len(servicesMap) == 0 {
			continue
		}

		serviceNameVal, found := rs.Resource().Attributes().Get(conventions.AttributeServiceName)
		serviceName := serviceNameVal.Str()
		if !found || len(serviceName) == 0 {
			continue
		}

		spanConfigs, found := servicesMap[serviceName]
		if !found || len(spanConfigs) == 0 {
			continue
		}

		for _, sc := range spanConfigs {
			spanCount := 0
			for j := 0; j < rs.ScopeSpans().Len(); j++ {
				scss := rs.ScopeSpans().At(j)
				for k := 0; k < scss.Spans().Len(); k++ {
					span := scss.Spans().At(k)
					if spanMatchesConfig(span, sc) {
						spanCount++
					}
				}
			}

			if spanCount > 0 {
				p.telemetryBuilder.ProcessorCriteriaBasedSpanCount.Add(ctx, int64(spanCount), metric.WithAttributes(attribute.KeyValue{
					Key:   attribute.Key(tagSpanCriteriaLabel),
					Value: attribute.StringValue(sc.Label),
				}))
			}
		}
	}

	return traces, nil
}

func spanMatchesConfig(span ptrace.Span, spanConfig SpanConfig) bool {
	// If span name is configured, it needs to match. If not configured, skip this check.
	if len(spanConfig.SpanName) != 0 && span.Name() != spanConfig.SpanName {
		return false
	}

	for _, attr := range spanConfig.SpanAttributes {
		v, ok := span.Attributes().Get(attr.Key)
		if !ok {
			return false
		}

		// empty value means we are just checking for the presence of attribute.
		if len(attr.Value) == 0 || attr.Value == v.Str() {
			continue
		} else {
			return false
		}
	}

	return true
}
