package ratelimiter

import (
	"context"
	"fmt"
	"strings"

	pb_struct "github.com/envoyproxy/go-control-plane/envoy/extensions/common/ratelimit/v3"
	pb "github.com/envoyproxy/go-control-plane/envoy/service/ratelimit/v3"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

type processor struct {
	rateLimitServiceClient   pb.RateLimitServiceClient
	domain                   string
	domainSoftLimitThreshold uint32
	logger                   *zap.Logger
	tenantIDHeaderName       string
}

const (
	TenantID       = "tenant_id"
	GlobalTenantID = "global_tenant_id"
)

// ProcessTraces implements processorhelper.ProcessTracesFunc
func (p *processor) ProcessTraces(ctx context.Context, traces ptrace.Traces) (ptrace.Traces, error) {
	tenantId, err := p.getTenantId(ctx)
	if err != nil {
		return traces, nil
	}
	// two descriptors, one for tenant and other one for domain
	desc := make([]*pb_struct.RateLimitDescriptor, 2)
	desc[0] = &pb_struct.RateLimitDescriptor{
		Entries: []*pb_struct.RateLimitDescriptor_Entry{
			{
				Key:   TenantID,
				Value: tenantId,
			},
		},
	}
	desc[1] = &pb_struct.RateLimitDescriptor{
		Entries: []*pb_struct.RateLimitDescriptor_Entry{
			{
				Key:   TenantID,
				Value: GlobalTenantID,
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
		return traces, nil
	}
	descriptorStatuses := response.Statuses
	if descriptorStatuses != nil {
		if descriptorStatuses[1].GetCode() == pb.RateLimitResponse_OVER_LIMIT {
			// If global rate limit exceeded drop all events
			return traces, fmt.Errorf("dropping spans for tenant %s due to global rate limit exceeded, of spancount: %d", tenantId, traces.SpanCount())
		}
		if descriptorStatuses[1].LimitRemaining < p.domainSoftLimitThreshold && descriptorStatuses[0].GetCode() == pb.RateLimitResponse_OVER_LIMIT {
			// If soft limit reached then drop only spans from tenants where tenant specific limit exceeded.
			return traces, fmt.Errorf("dropping spans for tenant %s due to soft limit reached, of spancount: %d", tenantId, traces.SpanCount())
		}
		// Ignore dropping of spans when soft limit hasn't reached even though tenant rate limit reached.
	}

	return traces, nil
}

func (p *processor) getTenantId(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", fmt.Errorf("could not extract headers from context. ")
	}

	tenantIDHeaders := md.Get(p.tenantIDHeaderName)
	if len(tenantIDHeaders) == 0 {
		return "", fmt.Errorf("missing header: %s", p.tenantIDHeaderName)
	} else if len(tenantIDHeaders) > 1 {
		return "", fmt.Errorf("multiple tenant ID headers were provided, %s: %s", p.tenantIDHeaderName, strings.Join(tenantIDHeaders, ", "))
	}

	return tenantIDHeaders[0], nil
}
