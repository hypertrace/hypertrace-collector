package piifilterprocessor

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	typeStr = "hypertrace_piifilter"
)

// NewFactory creates a factory for the routing processor.
func NewFactory() component.ProcessorFactory {
	return processorhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		processorhelper.WithTraces(createTraceProcessor),
	)
}

func createDefaultConfig() configmodels.Processor {
	return &TransportConfig{
		ProcessorSettings: configmodels.ProcessorSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
	}
}

func createTraceProcessor(
	_ context.Context,
	params component.ProcessorCreateParams,
	cfg configmodels.Processor,
	nextConsumer consumer.TracesConsumer,
) (component.TracesProcessor, error) {
	transportCfg := cfg.(*TransportConfig)

	piiCfg, err := transportCfg.toConfig()
	if err != nil {
		return nil, err
	}

	proc, err := newPIIFilterProcessor(params.Logger, piiCfg, nextConsumer)
	if err != nil {
		return nil, fmt.Errorf("failed to create the PII trace processor: %v", err)
	}

	return proc, nil
}
