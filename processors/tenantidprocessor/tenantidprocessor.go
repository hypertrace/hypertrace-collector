package tenantidprocessor

import (
	"context"
	"fmt"
	"strings"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

type processor struct {
	tenantIDHeaderName   string
	tenantIDAttributeKey string
	logger               *zap.Logger
}

// ProcessMetrics implements processorhelper.ProcessMetricsFunc
func (p *processor) ProcessMetrics(ctx context.Context, metrics pmetric.Metrics) (pmetric.Metrics, error) {
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

	ctx, _ = tag.New(ctx,
		tag.Insert(tagTenantID, tenantID))
	stats.Record(ctx, statMetricPerTenant.M(int64(metrics.MetricCount())))

	return metrics, nil

}

// ProcessTraces implements processorhelper.ProcessTracesFunc
func (p *processor) ProcessTraces(ctx context.Context, traces ptrace.Traces) (ptrace.Traces, error) {
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

	ctx, _ = tag.New(ctx,
		tag.Insert(tagTenantID, tenantID))
	stats.Record(ctx, statSpanPerTenant.M(int64(traces.SpanCount())))

	return traces, nil
}

func (p *processor) addTenantIdToSpans(traces ptrace.Traces, tenantIDHeaderValue string) {
	rss := traces.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		rs.Resource().Attributes().PutString(p.tenantIDAttributeKey, tenantIDHeaderValue)
	}
}

func (p *processor) addTenantIdToMetrics(metrics pmetric.Metrics, tenantIDHeaderValue string) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		rm.Resource().Attributes().PutString(p.tenantIDAttributeKey, tenantIDHeaderValue)
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			metrics := sm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				metricDataType := metric.DataType()
				switch metricDataType {
				case pmetric.MetricDataTypeNone:
					p.logger.Error("Cannot add tenantId to metric. Metric Data type not present for metric: " + metric.Name())
				case pmetric.MetricDataTypeGauge:
					metricData := metric.Gauge().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).Attributes().PutString(p.tenantIDAttributeKey, tenantIDHeaderValue)
					}
				case pmetric.MetricDataTypeSum:
					metricData := metric.Sum().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).Attributes().PutString(p.tenantIDAttributeKey, tenantIDHeaderValue)
					}
				case pmetric.MetricDataTypeHistogram:
					metricData := metric.Histogram().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).Attributes().PutString(p.tenantIDAttributeKey, tenantIDHeaderValue)
					}
				case pmetric.MetricDataTypeExponentialHistogram:
					metricData := metric.ExponentialHistogram().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).Attributes().PutString(p.tenantIDAttributeKey, tenantIDHeaderValue)
					}
				case pmetric.MetricDataTypeSummary:
					metricData := metric.Summary().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).Attributes().PutString(p.tenantIDAttributeKey, tenantIDHeaderValue)
					}
				}
			}
		}
	}
}
