package metricresourceattrstoattrs

import (
	"context"

	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
)

const ocServiceInstanceIdAttrKey = "service_instance_id"

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
				// - service.instance.id(conventions.AttributeServiceInstanceID) if the metric attributes already has
				//   "service_instance_id"
				// These will be added by the prometheus exporter.
				resourceAttrs.Range(func(key string, v pcommon.Value) bool {
					if (key == model.JobLabel && hasResourceServiceNameAttr) ||
						(key == model.InstanceLabel && hasResourceServiceInstanceIDAttr) {
						return true
					}
					applyToMetricAttributes(metric, func(am pcommon.Map) {
						// Copy service.instance.id if "service_instance_id" does not exist
						if key == conventions.AttributeServiceInstanceID {
							if _, ok := am.Get(ocServiceInstanceIdAttrKey); !ok {
								am.PutString(key, v.AsString())
							}
						} else {
							if _, ok := am.Get(key); !ok {
								am.PutString(key, v.AsString())
							}
						}
					})
					return true
				})
				// Remove job and instance labels from the metric attributes if hasResourceServiceNameAttr OR hasResourceServiceInstanceIDAttr is true.
				// This is because they will be added by the prometheus exporter based on serviceName and serviceInstanceId
				// and if already present they will be duplicated and will cause an error while processing metrics.
				if hasResourceServiceNameAttr || hasResourceServiceInstanceIDAttr {
					applyToMetricAttributes(metric, func(am pcommon.Map) {
						if hasResourceServiceNameAttr {
							am.Remove(model.JobLabel)
						}
						if hasResourceServiceInstanceIDAttr {
							am.Remove(model.InstanceLabel)
						}
					})
				}
			}
		}
	}
	return metrics, nil
}

// applyToMetricAttributes casts out the correct struct type for the metric so that it can access the attributes map and apply a function
// to it.
func applyToMetricAttributes(metric pmetric.Metric, fn func(pcommon.Map)) {
	metricDataType := metric.Type()
	switch metricDataType {
	case pmetric.MetricTypeGauge:
		metricData := metric.Gauge().DataPoints()
		for l := 0; l < metricData.Len(); l++ {
			fn(metricData.At(l).Attributes())
		}
	case pmetric.MetricTypeSum:
		metricData := metric.Sum().DataPoints()
		for l := 0; l < metricData.Len(); l++ {
			fn(metricData.At(l).Attributes())
		}
	case pmetric.MetricTypeHistogram:
		metricData := metric.Histogram().DataPoints()
		for l := 0; l < metricData.Len(); l++ {
			fn(metricData.At(l).Attributes())
		}
	case pmetric.MetricTypeExponentialHistogram:
		metricData := metric.ExponentialHistogram().DataPoints()
		for l := 0; l < metricData.Len(); l++ {
			fn(metricData.At(l).Attributes())
		}
	case pmetric.MetricTypeSummary:
		metricData := metric.Summary().DataPoints()
		for l := 0; l < metricData.Len(); l++ {
			fn(metricData.At(l).Attributes())
		}
	}
}
