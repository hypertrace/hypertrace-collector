package tenantidprocessor

import (
	"context"

	"go.opentelemetry.io/collector/config"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	typeStr             = "hypertrace_tenantid"
	defaultHeaderName   = "x-tenant-id"
	defaultAttributeKey = "tenant-id"
)

// NewFactory creates a factory for the tenant ID processor.
func NewFactory() component.ProcessorFactory {
	return processorhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		processorhelper.WithTraces(createTraceProcessor),
		processorhelper.WithMetrics(createMetricsProcessor),
	)
}

func createDefaultConfig() config.Processor {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(
			config.NewID(typeStr),
		),
		TenantIDHeaderName:   defaultHeaderName,
		TenantIDAttributeKey: defaultAttributeKey,
	}
}

func createTraceProcessor(
	_ context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Traces,
) (component.TracesProcessor, error) {
	pCfg := cfg.(*Config)
	return processorhelper.NewTracesProcessor(
		cfg,
		nextConsumer,
		&processor{
			tenantIDAttributeKey: pCfg.TenantIDAttributeKey,
			tenantIDHeaderName:   pCfg.TenantIDHeaderName,
			logger:               params.Logger,
		})
}

func createMetricsProcessor(
	_ context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Metrics,
) (component.MetricsProcessor, error) {
	pCfg := cfg.(*Config)
	return processorhelper.NewMetricsProcessor(
		cfg,
		nextConsumer,
		&processor{
			tenantIDAttributeKey: pCfg.TenantIDAttributeKey,
			tenantIDHeaderName:   pCfg.TenantIDHeaderName,
			logger:               params.Logger,
		})
}
