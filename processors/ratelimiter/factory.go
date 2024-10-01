package ratelimiter

import (
	"context"
	"net"
	"strconv"
	"time"

	pb "github.com/envoyproxy/go-control-plane/envoy/service/ratelimit/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultServiceHost   = "127.0.0.1"
	defaultServicePort   = uint16(8081)
	defaultDomain        = "collector"
	defaultHeaderName    = "x-tenant-id"
	defaultTimeoutMillis = uint32(1000) // 1 second
)

var (
	Type = component.MustNewType("hypertrace_ratelimiter")
)

// NewFactory creates a factory for the ratelimit processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		Type,
		createDefaultConfig,
		processor.WithTraces(createTraceProcessor, component.StabilityLevelStable),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ServiceHost:        defaultServiceHost,
		ServicePort:        defaultServicePort,
		Domain:             defaultDomain,
		TenantIDHeaderName: defaultHeaderName,
		TimeoutMillis:      defaultTimeoutMillis,
	}
}

func createTraceProcessor(
	ctx context.Context,
	params processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	pCfg := cfg.(*Config)
	rateLimitServiceClient, rateLimitServiceClientConn, cancelFunc, err := getRateLimitServiceClient(ctx, pCfg.ServiceHost, pCfg.ServicePort, pCfg.TimeoutMillis, params)
	if err != nil {
		params.Logger.Error("failed to connect to rate limit service ", zap.Error(err))
		return nil, err
	}
	rateLimiter := &rateLimiterProcessor{
		rateLimitServiceClient:     rateLimitServiceClient,
		domain:                     pCfg.Domain,
		logger:                     params.Logger,
		tenantIDHeaderName:         pCfg.TenantIDHeaderName,
		nextConsumer:               nextConsumer,
		rateLimitServiceClientConn: rateLimitServiceClientConn,
		cancelFunc:                 cancelFunc,
	}
	return rateLimiter, nil
}

func getRateLimitServiceClient(ctx context.Context, serviceHost string, servicePort uint16,
	timeoutMillis uint32, params processor.Settings) (pb.RateLimitServiceClient, *grpc.ClientConn, context.CancelFunc, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Millisecond*time.Duration(timeoutMillis))
	var err error
	var conn *grpc.ClientConn
	dialString := net.JoinHostPort(serviceHost, strconv.Itoa(int(servicePort)))
	params.Logger.Info("connecting to rate limit service %s " + dialString)
	conn, err = grpc.DialContext(ctxWithTimeout, dialString, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		params.Logger.Error("Unable to connect to rate limit service", zap.Error(err))
		cancel()
		return nil, nil, nil, err
	}
	return pb.NewRateLimitServiceClient(conn), conn, cancel, nil
}
