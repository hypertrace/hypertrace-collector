package metricresourceattrstoattrs

import (
	"context"

	"github.com/prometheus/common/model"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.6.1"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type processor struct {
	logger *zap.Logger
}

// ProcessMetrics implements processorhelper.ProcessMetricsFunc
func (p *processor) ProcessMetrics(ctx context.Context, metrics pmetric.Metrics) (pmetric.Metrics, error) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		resourceAttrs := rm.Resource().Attributes()
		// Check if service.name and service.instance.id are set as resource attributes since in
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/12cc610f93429fbd9dec71c5f486d266844f11c2/exporter/prometheusexporter/collector.go#L96
		// they are used to add job and instance metric labels.
		_, hasResourceServiceNameAttr := resourceAttrs.Get(conventions.AttributeServiceName)
		_, hasResourceServiceInstanceIDAttr := resourceAttrs.Get(conventions.AttributeServiceInstanceID)
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			metrics := sm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				// Add all resource attributes to labels except for:
				// - model.JobLabel if hasResourceServiceNameAttr is true
				// - model.InstanceLabel if hasResourceServiceInstanceIDAttr is true
				// These will be added by the prometheus exporter.
				resourceAttrs.Range(func(key string, v pcommon.Value) bool {
					if (key == model.JobLabel && hasResourceServiceNameAttr) ||
						(key == model.InstanceLabel && hasResourceServiceInstanceIDAttr) {
						return true
					}
					metricDataType := metric.DataType()
					switch metricDataType {
					case pmetric.MetricDataTypeGauge:
						metricData := metric.Gauge().DataPoints()
						for l := 0; l < metricData.Len(); l++ {
							metricData.At(l).Attributes().Insert(key, v)
						}
					case pmetric.MetricDataTypeSum:
						metricData := metric.Sum().DataPoints()
						for l := 0; l < metricData.Len(); l++ {
							metricData.At(l).Attributes().Insert(key, v)
						}
					case pmetric.MetricDataTypeHistogram:
						metricData := metric.Histogram().DataPoints()
						for l := 0; l < metricData.Len(); l++ {
							metricData.At(l).Attributes().Insert(key, v)
						}
					case pmetric.MetricDataTypeExponentialHistogram:
						metricData := metric.ExponentialHistogram().DataPoints()
						for l := 0; l < metricData.Len(); l++ {
							metricData.At(l).Attributes().Insert(key, v)
						}
					case pmetric.MetricDataTypeSummary:
						metricData := metric.Summary().DataPoints()
						for l := 0; l < metricData.Len(); l++ {
							metricData.At(l).Attributes().Insert(key, v)
						}
					}
					return true
				})
			}
		}
	}
	return metrics, nil
}
