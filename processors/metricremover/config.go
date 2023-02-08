package metricremover

type Config struct {
	// RemoveNoneMetricType enables the dropping of "None" metric types which would fail
	// translation to prometheus metrics.
	RemoveNoneMetricType bool `mapstructure:"remove_none_metric_type"`
}
