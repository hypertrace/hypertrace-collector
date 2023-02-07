package ratelimiter

import (
	"context"
	"fmt"
	"strings"

	pb_struct "github.com/envoyproxy/go-control-plane/envoy/extensions/common/ratelimit/v3"
	pb "github.com/envoyproxy/go-control-plane/envoy/service/ratelimit/v3"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var _ component.TracesProcessor = (*processor)(nil)

func (p *processor) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (p *processor) Shutdown(_ context.Context) error {
	err := p.rateLimitServiceClientConn.Close()
	if err != nil {
		p.logger.Error("failure while closing rate limit service client connection ", zap.Error(err))
	}
	p.cancelFunc()
	return nil
}

func (p *processor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

type processor struct {
	rateLimitServiceClient     pb.RateLimitServiceClient
	domain                     string
	domainSoftLimitThreshold   uint32
	logger                     *zap.Logger
	tenantIDHeaderName         string
	nextConsumer               consumer.Traces
	rateLimitServiceClientConn *grpc.ClientConn
	cancelFunc                 context.CancelFunc
}

const (
	TenantSpans  = "tenant_spans"
	ClusterSpans = "cluster_spans"
)

// ConsumeTraces consume traces and drops the requests if it is rate limited,
// otherwise calls next consumer
func (p *processor) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	tenantId, err := p.getTenantId(ctx)
	if err != nil {
		// If tenantId is missing, rate limiting not applicable.
		p.logger.Error("unable to extract tenantId ", zap.Error(err))
		return p.nextConsumer.ConsumeTraces(ctx, traces)
	}
	// two descriptors, one for tenant and other one for domain(cluster)
	desc := make([]*pb_struct.RateLimitDescriptor, 2)
	desc[0] = &pb_struct.RateLimitDescriptor{
		Entries: []*pb_struct.RateLimitDescriptor_Entry{
			{
				Key:   TenantSpans,
				Value: tenantId,
			},
		},
	}
	desc[1] = &pb_struct.RateLimitDescriptor{
		Entries: []*pb_struct.RateLimitDescriptor_Entry{
			{
				Key: ClusterSpans,
			},
		},
	}
	spanCount := uint32(traces.SpanCount())
	response, err := p.rateLimitServiceClient.ShouldRateLimit(
		ctx,
		&pb.RateLimitRequest{
			Domain:      p.domain,
			Descriptors: desc,
			HitsAddend:  spanCount,
		})
	if err != nil {
		// Rate limit service call fails, spans will be forwarded as it is.
		p.logger.Error("rate limit service call failed", zap.Error(err))
		return p.nextConsumer.ConsumeTraces(ctx, traces)
	}
	descriptorStatuses := response.Statuses
	ctx, _ = tag.New(ctx,
		tag.Insert(tagTenantID, tenantId))
	if len(descriptorStatuses) == 2 {
		if descriptorStatuses[1].GetCode() == pb.RateLimitResponse_OVER_LIMIT {
			// If global rate limit exceeded drop all events
			p.logger.Warn(fmt.Sprintf("dropping spans for tenant %s due to global rate limit exceeded, of spancount: %d", tenantId, spanCount))
			stats.Record(ctx, droppedSpanCount.M(int64(spanCount)))
			return nil
		} else if descriptorStatuses[1].LimitRemaining < p.domainSoftLimitThreshold && descriptorStatuses[0].GetCode() == pb.RateLimitResponse_OVER_LIMIT {
			// If soft limit reached then drop only spans from tenants where tenant specific limit exceeded.
			p.logger.Warn(fmt.Sprintf("dropping spans for tenant %s due to soft limit reached, of spancount: %d", tenantId, spanCount))
			stats.Record(ctx, droppedSpanCountSoftLimit.M(int64(spanCount)))
			return nil
		}
		// Ignore dropping of spans when soft limit hasn't reached even though tenant rate limit reached.
	} else {
		p.logger.Error(fmt.Sprintf("unexpected descriptor status length from rate limit response: %s ", descriptorStatuses))
	}
	return p.nextConsumer.ConsumeTraces(ctx, traces)
}

func (p *processor) getTenantId(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", fmt.Errorf("could not extract headers from context")
	}

	tenantIDHeaders := md.Get(p.tenantIDHeaderName)
	if len(tenantIDHeaders) == 0 {
		return "", fmt.Errorf("missing header: %s", p.tenantIDHeaderName)
	} else if len(tenantIDHeaders) > 1 {
		return "", fmt.Errorf("multiple tenant ID headers were provided, %s: %s", p.tenantIDHeaderName, strings.Join(tenantIDHeaders, ", "))
	}

	return tenantIDHeaders[0], nil
}