package ratelimiter

import (
	"context"
	"net"
	"strconv"

	pb "github.com/envoyproxy/go-control-plane/envoy/service/ratelimit/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	typeStr                         = "hypertrace_ratelimiter"
	defaultServiceHost              = "127.0.0.1"
	defaultServicePort              = uint16(8081)
	defaultDomain                   = "collector"
	defaultDomainSoftLimitThreshold = uint32(100000)
	defaultHeaderName               = "x-tenant-id"
)

// NewFactory creates a factory for the ratelimit processor.
func NewFactory() component.ProcessorFactory {
	return component.NewProcessorFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesProcessor(createTraceProcessor, component.StabilityLevelStable),
	)
}

func createDefaultConfig() config.Processor {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(
			config.NewComponentID(typeStr),
		),
		RateLimitServiceHost:         defaultServiceHost,
		RateLimitServicePort:         defaultServicePort,
		Domain:                       defaultDomain,
		DomainSoftRateLimitThreshold: defaultDomainSoftLimitThreshold,
		TenantIDHeaderName:           defaultHeaderName,
	}
}

func createTraceProcessor(
	ctx context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Traces,
) (component.TracesProcessor, error) {
	pCfg := cfg.(*Config)
	rateLimitServiceClient, err := getRateLimitServiceClient(pCfg.RateLimitServiceHost, pCfg.RateLimitServicePort, params)
	if err != nil {
		params.Logger.Error("failed to connect to rate limit service ", zap.Error(err))
		return nil, err
	}
	processor := &processor{
		rateLimitServiceClient:   rateLimitServiceClient,
		domain:                   pCfg.Domain,
		domainSoftLimitThreshold: pCfg.DomainSoftRateLimitThreshold,
		logger:                   params.Logger,
	}
	return processorhelper.NewTracesProcessor(
		ctx,
		params,
		cfg,
		nextConsumer,
		processor.ProcessTraces)
}

func getRateLimitServiceClient(serviceHost string, servicePort uint16, params component.ProcessorCreateSettings) (pb.RateLimitServiceClient, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	var err error
	var conn *grpc.ClientConn
	dialString := net.JoinHostPort(serviceHost, strconv.Itoa(int(servicePort)))
	params.Logger.Info("connecting to rate limit service %s " + dialString)
	conn, err = grpc.Dial(dialString, opts...)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return pb.NewRateLimitServiceClient(conn), nil
}
