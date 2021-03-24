package tenantidprocessor

import (
	"context"
	"fmt"
	"strings"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

type processor struct {
	tenantIDHeaderName   string
	tenantIDAttributeKey string
	logger               *zap.Logger
}

var _ processorhelper.TProcessor = (*processor)(nil)

var _ processorhelper.MProcessor = (*processor)(nil)

func (p *processor) ProcessMetrics(ctx context.Context, metrics pdata.Metrics) (pdata.Metrics, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		p.logger.Error("Could not extract headers from context", zap.Int("num-metrics", metrics.MetricCount()))
		return metrics, fmt.Errorf("could not extract headers from context")
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

// ProcessTraces implements processorhelper.TProcessor
func (p *processor) ProcessTraces(ctx context.Context, traces pdata.Traces) (pdata.Traces, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		p.logger.Error("Could not extract headers from context", zap.Int("num-spans", traces.SpanCount()))
		return traces, fmt.Errorf("could not extract headers from context")
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

		ilss := rs.InstrumentationLibrarySpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)

			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				span.Attributes().Insert(p.tenantIDAttributeKey, pdata.NewAttributeValueString(tenantIDHeaderValue))
			}
		}
	}
}

func (p *processor) addTenantIdToMetrics(metrics pdata.Metrics, tenantIDHeaderValue string) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)

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
				case "IntGauge":
					metricData := metric.IntGauge().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).LabelsMap().Insert(p.tenantIDAttributeKey, tenantIDHeaderValue)
					}
				case "DoubleGauge":
					metricData := metric.DoubleGauge().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).LabelsMap().Insert(p.tenantIDAttributeKey, tenantIDHeaderValue)
					}
				case "IntSum":
					metricData := metric.IntSum().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).LabelsMap().Insert(p.tenantIDAttributeKey, tenantIDHeaderValue)
					}
				case "DoubleSum":
					metricData := metric.DoubleSum().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).LabelsMap().Insert(p.tenantIDAttributeKey, tenantIDHeaderValue)
					}
				case "IntHistogram":
					metricData := metric.IntHistogram().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).LabelsMap().Insert(p.tenantIDAttributeKey, tenantIDHeaderValue)
					}
				case "DoubleHistogram":
					metricData := metric.DoubleHistogram().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).LabelsMap().Insert(p.tenantIDAttributeKey, tenantIDHeaderValue)
					}
				case "DoubleSummary":
					metricData := metric.DoubleSummary().DataPoints()
					for l := 0; l < metricData.Len(); l++ {
						metricData.At(l).LabelsMap().Insert(p.tenantIDAttributeKey, tenantIDHeaderValue)
					}
				}
			}
		}
	}
}
