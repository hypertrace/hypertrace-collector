package metricremover

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type processor struct {
	removeNoneMetricType bool
	logger               *zap.Logger
}

// ProcessMetrics implements processorhelper.ProcessMetricsFunc
func (p *processor) ProcessMetrics(ctx context.Context, metrics pmetric.Metrics) (pmetric.Metrics, error) {
	if !p.removeNoneMetricType {
		return metrics, nil
	}

	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sms.At(j).Metrics().RemoveIf(func(m pmetric.Metric) bool {
				return m.Type() == pmetric.MetricTypeNone
			})
		}
	}
	return metrics, nil
}
