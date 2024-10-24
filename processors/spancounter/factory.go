package spancounter

import (
	"context"

	"github.com/google/uuid"
	"github.com/hypertrace/collector/processors/spancounter/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
)

var (
	Type = component.MustNewType("hypertrace_spancounter")
)

// NewFactory creates a factory for the spancounter processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, component.StabilityLevelStable),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TenantIDAttributeKey: defaultTenantIDAttributeKey,
	}
}

func createTracesProcessor(
	ctx context.Context,
	params processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	pCfg := cfg.(*Config)
	addUniqueLabelsToSpanConfigs(pCfg)
	params.Logger.Info("Criteria based span counter processor config", zap.Any("config", pCfg))
	// TelemetryBuilder will be used to setup metrics
	telemetryBuilder, err := metadata.NewTelemetryBuilder(params.TelemetrySettings)
	if err != nil {
		params.Logger.Error("error creating telemetry for the spancounter processor", zap.Error(err))
		return nil, err
	}
	processor := newProcessor(params.Logger, pCfg, telemetryBuilder)
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
