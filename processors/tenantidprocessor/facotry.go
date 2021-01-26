package tenantidprocessor

import (
	"context"
	"go.opencensus.io/stats/view"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	typeStr                     = "hypertrace_tenantid"
	defaultTenantIdHeaderName   = "x-tenant-id"
	defaultTenantIdAttributeKey = "tenant-id"
)

// NewFactory creates a factory for the tenantid processor.
// The processor adds tenant ID to every received span.
// The processor returns an error when the tenant ID is missing.
// The tenant ID header is obtained from the context object.
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
			tenantIDAttributeKey: pCfg.TenantIDAttributeKey,
			tenantIDHeaderName:   pCfg.TenantIDHeaderName,
			logger:               params.Logger,
			tenantIDViews:        make(map[string]*view.View),
		})
}
