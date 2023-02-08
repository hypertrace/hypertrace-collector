package metricremover

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	typeStr = "hypertrace_metrics_remover"
)

func NewFactory() component.ProcessorFactory {
	return component.NewProcessorFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsProcessor(createMetricsProcessor, component.StabilityLevelStable),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(
			component.NewIDWithName(typeStr),
		),
	}
}

func createMetricsProcessor(
	ctx context.Context,
	params component.ProcessorCreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (component.MetricsProcessor, error) {
	pCfg := cfg.(*Config)
	processor := &processor{
		logger:               params.Logger,
		removeNoneMetricType: pCfg.RemoveNoneMetricType,
	}
	return processorhelper.NewMetricsProcessor(
		ctx,
		params,
		cfg,
		nextConsumer,
		processor.ProcessMetrics)
}
