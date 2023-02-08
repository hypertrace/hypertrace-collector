package tenantidprocessor

import (
	"context"

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
	return component.NewProcessorFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesProcessor(createTraceProcessor, component.StabilityLevelStable),
		component.WithMetricsProcessor(createMetricsProcessor, component.StabilityLevelStable),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TenantIDHeaderName:   defaultHeaderName,
		TenantIDAttributeKey: defaultAttributeKey,
	}
}

func createTraceProcessor(
	ctx context.Context,
	params component.ProcessorCreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (component.TracesProcessor, error) {
	pCfg := cfg.(*Config)
	processor := &processor{
		tenantIDAttributeKey: pCfg.TenantIDAttributeKey,
		tenantIDHeaderName:   pCfg.TenantIDHeaderName,
		logger:               params.Logger,
	}
	return processorhelper.NewTracesProcessor(
		ctx,
		params,
		cfg,
		nextConsumer,
		processor.ProcessTraces)
}

func createMetricsProcessor(
	ctx context.Context,
	params component.ProcessorCreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (component.MetricsProcessor, error) {
	pCfg := cfg.(*Config)
	processor := &processor{
		tenantIDAttributeKey: pCfg.TenantIDAttributeKey,
		tenantIDHeaderName:   pCfg.TenantIDHeaderName,
		logger:               params.Logger,
	}
	return processorhelper.NewMetricsProcessor(
		ctx,
		params,
		cfg,
		nextConsumer,
		processor.ProcessMetrics,
	)
}
