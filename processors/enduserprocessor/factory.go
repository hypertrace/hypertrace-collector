package enduserprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"

	"github.com/hypertrace/collector/processors/enduserprocessor/hash"
)

const (
	typeStr = "hypertrace_enduser"
)

// NewFactory creates a factory for the end user processor.
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
	}
}

func createTraceProcessor(
	_ context.Context,
	params component.ProcessorCreateParams,
	cfg configmodels.Processor,
	nextConsumer consumer.TracesConsumer,
) (component.TracesProcessor, error) {
	endUserCfg := cfg.(*Config)

	endUserMap := make(map[string][]config)
	for _, enduser := range endUserCfg.EndUserConfig {
		hashAlgorithm, ok := hash.ResolveHashAlgorithm(enduser.HashAlgo)
		if !ok {
			params.Logger.Warn("Failed to resolve hash algorithm, using default sha1", zap.String("config-algorithm", enduser.HashAlgo))
		}

		pcfg := config{
			EndUser:       enduser,
			hashAlgorithm: hashAlgorithm,
		}
		endUserMap[enduser.AttributeKey] = append(endUserMap[enduser.AttributeKey], pcfg)
	}

	return processorhelper.NewTraceProcessor(
		cfg,
		nextConsumer,
		&processor{
			logger:             params.Logger,
			attributeConfigMap: endUserMap,
		})
}
