package metricresourceattrstoattrs

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	typeStr = "hypertrace_metrics_resource_attrs_to_attrs"
)

// NewFactory creates a factory for the metricresourceattrstoattrs processor.
func NewFactory() component.ProcessorFactory {
	return component.NewProcessorFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsProcessor(createMetricsProcessor, component.StabilityLevelStable),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createMetricsProcessor(
	ctx context.Context,
	params component.ProcessorCreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (component.MetricsProcessor, error) {
	processor := &processor{
		logger: params.Logger,
	}
	return processorhelper.NewMetricsProcessor(
		ctx,
		params,
		cfg,
		nextConsumer,
		processor.ProcessMetrics)
}
