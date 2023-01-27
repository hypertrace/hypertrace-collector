package ratelimiter

import (
	"context"
	"net"
	"strconv"
	"time"

	pb "github.com/envoyproxy/go-control-plane/envoy/service/ratelimit/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	typeStr                         = "hypertrace_ratelimiter"
	defaultServiceHost              = "127.0.0.1"
	defaultServicePort              = uint16(8081)
	defaultDomain                   = "collector"
	defaultDomainSoftLimitThreshold = uint32(100000) // Soft limit kicks in when limit remaining under 100k
	defaultHeaderName               = "x-tenant-id"
	defaultTimeoutMillis            = uint16(1000) // 1 second
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
	rateLimitServiceClient, err := getRateLimitServiceClient(ctx, pCfg.RateLimitServiceHost, pCfg.RateLimitServicePort, pCfg.RateLimitServiceTimeoutMillis, params)
	if err != nil {
		params.Logger.Error("failed to connect to rate limit service ", zap.Error(err))
		return nil, err
	}
	processor := &processor{
		rateLimitServiceClient:   rateLimitServiceClient,
		domain:                   pCfg.Domain,
		domainSoftLimitThreshold: pCfg.DomainSoftRateLimitThreshold,
		logger:                   params.Logger,
		tenantIDHeaderName:       pCfg.TenantIDHeaderName,
		nextConsumer:             nextConsumer,
	}
	return processor, nil
}

func getRateLimitServiceClient(ctx context.Context, serviceHost string, servicePort uint16, timeoutMillis uint32, params component.ProcessorCreateSettings) (pb.RateLimitServiceClient, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Millisecond*time.Duration(timeoutMillis))
	defer cancel()
	var err error
	var conn *grpc.ClientConn
	dialString := net.JoinHostPort(serviceHost, strconv.Itoa(int(servicePort)))
	params.Logger.Info("connecting to rate limit service %s " + dialString)
	conn, err = grpc.DialContext(ctxWithTimeout, dialString, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return pb.NewRateLimitServiceClient(conn), nil
}
