package tenantidprocessor

import (
	"context"

	"github.com/hypertrace/collector/processors/tenantidprocessor/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
)

const (
	defaultHeaderName   = "x-tenant-id"
	defaultAttributeKey = "tenant-id"
)

var (
	Type = component.MustNewType("hypertrace_tenantid")
)

// NewFactory creates a factory for the tenant ID processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		Type,
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
	params processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	pCfg := cfg.(*Config)
	// TelemetryBuilder will be used to setup metrics
	telemetryBuilder, err := metadata.NewTelemetryBuilder(params.TelemetrySettings)
	if err != nil {
		params.Logger.Error("error creating telemetry for the tenantidprocessor processor", zap.Error(err))
		return nil, err
	}
	tenantProcessor := &tenantIdProcessor{
		tenantIDAttributeKey: pCfg.TenantIDAttributeKey,
		tenantIDHeaderName:   pCfg.TenantIDHeaderName,
		logger:               params.Logger,
		telemetryBuilder:     telemetryBuilder,
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
	params processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	pCfg := cfg.(*Config)
	// TelemetryBuilder will be used to setup metrics
	telemetryBuilder, err := metadata.NewTelemetryBuilder(params.TelemetrySettings)
	if err != nil {
		params.Logger.Error("error creating telemetry for the tenantidprocessor processor", zap.Error(err))
		return nil, err
	}
	tenantProcessor := &tenantIdProcessor{
		tenantIDAttributeKey: pCfg.TenantIDAttributeKey,
		tenantIDHeaderName:   pCfg.TenantIDHeaderName,
		logger:               params.Logger,
		telemetryBuilder:     telemetryBuilder,
	}
	return processorhelper.NewMetricsProcessor(
		ctx,
		params,
		cfg,
		nextConsumer,
		tenantProcessor.ProcessMetrics,
	)
}
