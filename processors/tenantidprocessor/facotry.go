package tenantidprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	typeStr = "hypertrace_tenantid"
	defaultTenantIdHeaderName = "x-tenant-id"
	defaultTenantIdAttributeKey = "tenant-id"
)

// NewFactory creates a factory for the tenantid processor.
func NewFactory() component.ProcessorFactory {
	return processorhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		processorhelper.WithTraces(createTraceProcessor),
	)
}

func createDefaultConfig() configmodels.Processor {
	return &Config{
		ProcessorSettings: configmodels.ProcessorSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		TenantIDHeaderName:   defaultTenantIdHeaderName,
		TenantIDAttributeKey: defaultTenantIdAttributeKey,
	}
}

func createTraceProcessor(
	_ context.Context,
	params component.ProcessorCreateParams,
	cfg configmodels.Processor,
	nextConsumer consumer.TracesConsumer,
) (component.TracesProcessor, error) {
	pCfg := cfg.(*Config)
	return processorhelper.NewTraceProcessor(
		cfg,
		nextConsumer,
		&processor{
			logger: params.Logger,
			tenantIDAttributeKey: pCfg.TenantIDAttributeKey,
			tenantIDHeaderName: pCfg.TenantIDHeaderName,
		})
}

