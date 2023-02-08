package tenantidprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	typeStr             = "hypertrace_tenantid"
	defaultHeaderName   = "x-tenant-id"
	defaultAttributeKey = "tenant-id"
)

// NewFactory creates a factory for the tenant ID processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		typeStr,
		createDefaultConfig,
		processor.WithTraces(createTraceProcessor, component.StabilityLevelStable),
		processor.WithMetrics(createMetricsProcessor, component.StabilityLevelStable),
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
	params processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	pCfg := cfg.(*Config)
	tenantProcessor := &tenantIdProcessor{
		tenantIDAttributeKey: pCfg.TenantIDAttributeKey,
		tenantIDHeaderName:   pCfg.TenantIDHeaderName,
		logger:               params.Logger,
	}
	return processorhelper.NewTracesProcessor(
		ctx,
		params,
		cfg,
		nextConsumer,
		tenantProcessor.ProcessTraces)
}

func createMetricsProcessor(
	ctx context.Context,
	params processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	pCfg := cfg.(*Config)
	tenantProcessor := &tenantIdProcessor{
		tenantIDAttributeKey: pCfg.TenantIDAttributeKey,
		tenantIDHeaderName:   pCfg.TenantIDHeaderName,
		logger:               params.Logger,
	}
	return processorhelper.NewMetricsProcessor(
		ctx,
		params,
		cfg,
		nextConsumer,
		tenantProcessor.ProcessMetrics,
	)
}
