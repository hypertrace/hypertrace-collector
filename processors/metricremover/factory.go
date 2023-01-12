package metricremover

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
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

func createDefaultConfig() config.Processor {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(
			config.NewComponentID(typeStr),
		),
	}
}

func createMetricsProcessor(
	ctx context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
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