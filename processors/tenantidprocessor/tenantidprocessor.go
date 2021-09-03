package tenantidprocessor

import (
	"context"
	"fmt"
	"strings"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

type processor struct {
	tenantIDHeaderName   string
	tenantIDAttributeKey string
	logger               *zap.Logger
}

// ProcessMetrics implements processorhelper.ProcessMetricsFunc
func (p *processor) ProcessMetrics(ctx context.Context, metrics pdata.Metrics) (pdata.Metrics, error) {
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
func (p *processor) ProcessTraces(ctx context.Context, traces pdata.Traces) (pdata.Traces, error) {
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

func (p *processor) addTenantIdToSpans(traces pdata.Traces, tenantIDHeaderValue string) {
	rss := traces.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		rs.Resource().Attributes().Insert(p.tenantIDAttributeKey, pdata.NewAttributeValueString(tenantIDHeaderValue))
	}
}

func (p *processor) addTenantIdToMetrics(metrics pdata.Metrics, tenantIDHeaderValue string) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		rm.Resource().Attributes().Insert(p.tenantIDAttributeKey, pdata.NewAttributeValueString(tenantIDHeaderValue))
		ilms := rm.InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)
			metrics := ilm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				metricDataType := metric.DataType().String()
				switch metricDataType {
				case "None":
					p.logger.Error("Cannot add tenantId to metric. Metric Data type not present for metric: " + metric.Name())
				case "Gauge":
					metricData := metric.Gauge().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).Attributes().Insert(p.tenantIDAttributeKey, pdata.NewAttributeValueString(tenantIDHeaderValue))
					}
				case "Sum":
					metricData := metric.Sum().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).Attributes().Insert(p.tenantIDAttributeKey, pdata.NewAttributeValueString(tenantIDHeaderValue))
					}
				case "Histogram":
					metricData := metric.Histogram().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).Attributes().Insert(p.tenantIDAttributeKey, pdata.NewAttributeValueString(tenantIDHeaderValue))
					}
				case "Summary":
					metricData := metric.Summary().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).Attributes().Insert(p.tenantIDAttributeKey, pdata.NewAttributeValueString(tenantIDHeaderValue))
					}
				}
			}
		}
	}
}
