package metricremover

import (
	"go.opentelemetry.io/collector/config"
)

type Config struct {
	config.ProcessorSettings `mapstructure:"-"`

	// RemoveNoneMetricType enables the dropping of "None" metric types which would fail
	// translation to prometheus metrics.
	RemoveNoneMetricType bool `mapstructure:"remove_none_metric_type"`
}
