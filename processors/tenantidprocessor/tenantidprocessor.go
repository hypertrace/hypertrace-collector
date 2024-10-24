package tenantidprocessor

import (
	"context"
	"fmt"
	"strings"

	internalmetadata "github.com/hypertrace/collector/processors/tenantidprocessor/internal/metadata"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

const tagTenantID string = "tenant-id"

type tenantIdProcessor struct {
	tenantIDHeaderName   string
	tenantIDAttributeKey string
	logger               *zap.Logger
	telemetryBuilder     *internalmetadata.TelemetryBuilder
}

// ProcessMetrics implements processorhelper.ProcessMetricsFunc
func (p *tenantIdProcessor) ProcessMetrics(ctx context.Context, metrics pmetric.Metrics) (pmetric.Metrics, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return metrics, fmt.Errorf("could not extract headers from context. Number of metrics: %d", metrics.MetricCount())
	}

	tenantIDHeaders := md.Get(p.tenantIDHeaderName)
	if len(tenantIDHeaders) == 0 {
		return metrics, fmt.Errorf("missing header: %s", p.tenantIDHeaderName)
	} else if len(tenantIDHeaders) > 1 {
		return metrics, fmt.Errorf("multiple tenant ID headers were provided, %s: %s", p.tenantIDHeaderName, strings.Join(tenantIDHeaders, ", "))
	}

	tenantID := tenantIDHeaders[0]
	p.addTenantIdToMetrics(metrics, tenantID)

	tenantAttr := metric.WithAttributes(attribute.KeyValue{
		Key:   attribute.Key(tagTenantID),
		Value: attribute.StringValue(tenantID),
	})
	p.telemetryBuilder.ProcessorMetricsPerTenant.Add(ctx, int64(metrics.MetricCount()), tenantAttr)

	return metrics, nil

}

// ProcessTraces implements processorhelper.ProcessTracesFunc
func (p *tenantIdProcessor) ProcessTraces(ctx context.Context, traces ptrace.Traces) (ptrace.Traces, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return traces, fmt.Errorf("could not extract headers from context. Number of spans: %d", traces.SpanCount())
	}

	tenantIDHeaders := md.Get(p.tenantIDHeaderName)
	if len(tenantIDHeaders) == 0 {
		return traces, fmt.Errorf("missing header: %s", p.tenantIDHeaderName)
	} else if len(tenantIDHeaders) > 1 {
		return traces, fmt.Errorf("multiple tenant ID headers were provided, %s: %s", p.tenantIDHeaderName, strings.Join(tenantIDHeaders, ", "))
	}

	tenantID := tenantIDHeaders[0]
	p.addTenantIdToSpans(traces, tenantID)

	tenantAttr := metric.WithAttributes(attribute.KeyValue{
		Key:   attribute.Key(tagTenantID),
		Value: attribute.StringValue(tenantID),
	})
	p.telemetryBuilder.ProcessorSpansPerTenant.Add(ctx, int64(traces.SpanCount()), tenantAttr)

	return traces, nil
}

func (p *tenantIdProcessor) addTenantIdToSpans(traces ptrace.Traces, tenantIDHeaderValue string) {
	rss := traces.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		rs.Resource().Attributes().PutStr(p.tenantIDAttributeKey, tenantIDHeaderValue)
	}
}

func (p *tenantIdProcessor) addTenantIdToMetrics(metrics pmetric.Metrics, tenantIDHeaderValue string) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		rm.Resource().Attributes().PutStr(p.tenantIDAttributeKey, tenantIDHeaderValue)
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			metrics := sm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				metricDataType := metric.Type()
				switch metricDataType {
				case pmetric.MetricTypeEmpty:
					p.logger.Error("Cannot add tenantId to metric. Metric Data type not present for metric: " + metric.Name())
				case pmetric.MetricTypeGauge:
					metricData := metric.Gauge().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).Attributes().PutStr(p.tenantIDAttributeKey, tenantIDHeaderValue)
					}
				case pmetric.MetricTypeSum:
					metricData := metric.Sum().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).Attributes().PutStr(p.tenantIDAttributeKey, tenantIDHeaderValue)
					}
				case pmetric.MetricTypeHistogram:
					metricData := metric.Histogram().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).Attributes().PutStr(p.tenantIDAttributeKey, tenantIDHeaderValue)
					}
				case pmetric.MetricTypeExponentialHistogram:
					metricData := metric.ExponentialHistogram().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).Attributes().PutStr(p.tenantIDAttributeKey, tenantIDHeaderValue)
					}
				case pmetric.MetricTypeSummary:
					metricData := metric.Summary().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).Attributes().PutStr(p.tenantIDAttributeKey, tenantIDHeaderValue)
					}
				}
			}
		}
	}
}
