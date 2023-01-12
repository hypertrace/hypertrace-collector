package spancounter

import (
	"context"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
)

const (
	typeStr = "hypertrace_spancounter"
)

// NewFactory creates a factory for the spancounter processor.
func NewFactory() component.ProcessorFactory {
	return component.NewProcessorFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesProcessor(createTracesProcessor, component.StabilityLevelStable),
	)
}

func createDefaultConfig() config.Processor {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(
			config.NewComponentID(typeStr),
		),
		TenantIDAttributeKey: defaultTenantIDAttributeKey,
	}
}

func createTracesProcessor(
	ctx context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Traces,
) (component.TracesProcessor, error) {
	pCfg := cfg.(*Config)
	addUniqueLabelsToSpanConfigs(pCfg)
	params.Logger.Info("Criteria based span counter processor config", zap.Any("config", pCfg))
	processor := newProcessor(params.Logger, pCfg)
	return processorhelper.NewTracesProcessor(
		ctx,
		params,
		cfg,
		nextConsumer,
		processor.ProcessTraces)
}

// addUniqueLabelsToSpanConfigs adds unique labels to the span configs so that the span count metrics are uniquely identified
// and we can match it to the matching span config.
func addUniqueLabelsToSpanConfigs(c *Config) {
	for tenantIndex, tc := range c.TenantConfigs {
		for serviceIndex, sc := range tc.ServiceConfigs {
			for spanIndex, spanConfig := range sc.SpanConfigs {
				if len(spanConfig.Label) == 0 {
					c.TenantConfigs[tenantIndex].ServiceConfigs[serviceIndex].SpanConfigs[spanIndex].Label = uuid.NewString()
				}
			}
		}
	}
}
