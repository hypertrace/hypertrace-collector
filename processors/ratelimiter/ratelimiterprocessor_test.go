package ratelimiter

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	pb "github.com/envoyproxy/go-control-plane/envoy/service/ratelimit/v3"
	"github.com/hypertrace/collector/processors/testutil"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const testTenantID = "jdoe"

type MockRateLimitServiceClient struct {
	mock.Mock
}

type MockProcessorConsumer struct {
	mock.Mock
}

var _ component.TracesProcessor = (*MockProcessorConsumer)(nil)

func (p *MockProcessorConsumer) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (p *MockProcessorConsumer) Shutdown(_ context.Context) error {
	return nil
}

func (p *MockProcessorConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (m *MockRateLimitServiceClient) ShouldRateLimit(ctx context.Context, in *pb.RateLimitRequest, opts ...grpc.CallOption) (*pb.RateLimitResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(1) == nil {
		return args.Get(0).(*pb.RateLimitResponse), nil
	}
	return nil, args.Get(1).(error)
}

func (f *MockProcessorConsumer) ConsumeTraces(ctx context.Context, ld ptrace.Traces) error {
	args := f.Called(ctx, ld)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(error)
}

func TestRateLimitingWhenEmptyTenantHeader(t *testing.T) {
	mockRateLimitServiceClientObj := new(MockRateLimitServiceClient)
	mockProcessorConsumerObj := new(MockProcessorConsumer)
	p := &processor{
		logger:                   zap.NewNop(),
		tenantIDHeaderName:       defaultHeaderName,
		domain:                   defaultDomain,
		domainSoftLimitThreshold: defaultDomainSoftLimitThreshold,
		rateLimitServiceClient:   mockRateLimitServiceClientObj,
		nextConsumer:             mockProcessorConsumerObj,
	}
	mockRateLimitServiceClientObj.On("ShouldRateLimit", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("rate limit called failed"))
	mockProcessorConsumerObj.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)
	tokenString := base64.StdEncoding.EncodeToString([]byte("testuser:passw123"))
	span := testutil.NewTestSpan("http.request.header.authorization", "Basic: "+tokenString)
	traces := testutil.NewTestTraces(span)
	md := metadata.New(map[string]string{})
	ctx := metadata.NewIncomingContext(
		context.Background(),
		md,
	)
	err := p.ConsumeTraces(ctx, traces)
	require.NoError(t, err)
	mockRateLimitServiceClientObj.AssertNumberOfCalls(t, "ShouldRateLimit", 0)
	mockProcessorConsumerObj.AssertNumberOfCalls(t, "ConsumeTraces", 1)
}

func TestWhenRateLimitServiceCallFailed(t *testing.T) {
	mockRateLimitServiceClientObj := new(MockRateLimitServiceClient)
	mockProcessorConsumerObj := new(MockProcessorConsumer)
	p := &processor{
		logger:                   zap.NewNop(),
		tenantIDHeaderName:       defaultHeaderName,
		domain:                   defaultDomain,
		domainSoftLimitThreshold: defaultDomainSoftLimitThreshold,
		rateLimitServiceClient:   mockRateLimitServiceClientObj,
		nextConsumer:             mockProcessorConsumerObj,
	}
	mockRateLimitServiceClientObj.On("ShouldRateLimit", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("rate limit called failed"))
	mockProcessorConsumerObj.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)
	tokenString := base64.StdEncoding.EncodeToString([]byte("testuser:passw123"))
	span := testutil.NewTestSpan("http.request.header.authorization", "Basic: "+tokenString)
	traces := testutil.NewTestTraces(span)
	md := metadata.New(map[string]string{p.tenantIDHeaderName: testTenantID})
	ctx := metadata.NewIncomingContext(
		context.Background(),
		md,
	)
	err := p.ConsumeTraces(ctx, traces)
	require.NoError(t, err)
	mockRateLimitServiceClientObj.AssertNumberOfCalls(t, "ShouldRateLimit", 1)
	mockProcessorConsumerObj.AssertNumberOfCalls(t, "ConsumeTraces", 1)
}

func TestRateLimitingWhenSoftLimitNotReachedButTenantLimitReached(t *testing.T) {
	mockRateLimitServiceClientObj := new(MockRateLimitServiceClient)
	mockProcessorConsumerObj := new(MockProcessorConsumer)
	p := &processor{
		logger:                   zap.NewNop(),
		tenantIDHeaderName:       defaultHeaderName,
		domain:                   defaultDomain,
		domainSoftLimitThreshold: 2,
		rateLimitServiceClient:   mockRateLimitServiceClientObj,
		nextConsumer:             mockProcessorConsumerObj,
	}
	rateLimitResponse := &pb.RateLimitResponse{
		OverallCode: pb.RateLimitResponse_OK,
		Statuses: []*pb.RateLimitResponse_DescriptorStatus{
			{
				Code: pb.RateLimitResponse_OVER_LIMIT,
			},
			{
				Code:           pb.RateLimitResponse_OK,
				LimitRemaining: 5,
			},
		},
	}
	mockRateLimitServiceClientObj.On("ShouldRateLimit", mock.Anything, mock.Anything).Return(rateLimitResponse, nil)
	mockProcessorConsumerObj.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)
	tokenString := base64.StdEncoding.EncodeToString([]byte("testuser:passw123"))
	span := testutil.NewTestSpan("http.request.header.authorization", "Basic: "+tokenString)
	traces := testutil.NewTestTraces(span)
	md := metadata.New(map[string]string{p.tenantIDHeaderName: testTenantID})
	ctx := metadata.NewIncomingContext(
		context.Background(),
		md,
	)
	err := p.ConsumeTraces(ctx, traces)
	require.NoError(t, err)
	mockRateLimitServiceClientObj.AssertNumberOfCalls(t, "ShouldRateLimit", 1)
	mockProcessorConsumerObj.AssertNumberOfCalls(t, "ConsumeTraces", 1)
}

func TestRateLimitingWhenSoftLimitReachedAndTenantLimitReached(t *testing.T) {
	mockRateLimitServiceClientObj := new(MockRateLimitServiceClient)
	mockProcessorConsumerObj := new(MockProcessorConsumer)
	p := &processor{
		logger:                   zap.NewNop(),
		tenantIDHeaderName:       defaultHeaderName,
		domain:                   defaultDomain,
		domainSoftLimitThreshold: 2,
		rateLimitServiceClient:   mockRateLimitServiceClientObj,
		nextConsumer:             mockProcessorConsumerObj,
	}
	rateLimitResponse := &pb.RateLimitResponse{
		OverallCode: pb.RateLimitResponse_OK,
		Statuses: []*pb.RateLimitResponse_DescriptorStatus{
			{
				Code: pb.RateLimitResponse_OVER_LIMIT,
			},
			{
				Code:           pb.RateLimitResponse_OK,
				LimitRemaining: 1,
			},
		},
	}
	mockRateLimitServiceClientObj.On("ShouldRateLimit", mock.Anything, mock.Anything).Return(rateLimitResponse, nil)
	mockProcessorConsumerObj.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)
	tokenString := base64.StdEncoding.EncodeToString([]byte("testuser:passw123"))
	span := testutil.NewTestSpan("http.request.header.authorization", "Basic: "+tokenString)
	traces := testutil.NewTestTraces(span)
	md := metadata.New(map[string]string{p.tenantIDHeaderName: testTenantID})
	ctx := metadata.NewIncomingContext(
		context.Background(),
		md,
	)
	err := p.ConsumeTraces(ctx, traces)
	require.NoError(t, err)
	mockRateLimitServiceClientObj.AssertNumberOfCalls(t, "ShouldRateLimit", 1)
	mockProcessorConsumerObj.AssertNumberOfCalls(t, "ConsumeTraces", 0)
}

func TestRateLimitingWhenHardLimitReached(t *testing.T) {
	mockRateLimitServiceClientObj := new(MockRateLimitServiceClient)
	mockProcessorConsumerObj := new(MockProcessorConsumer)
	p := &processor{
		logger:                   zap.NewNop(),
		tenantIDHeaderName:       defaultHeaderName,
		domain:                   defaultDomain,
		domainSoftLimitThreshold: 2,
		rateLimitServiceClient:   mockRateLimitServiceClientObj,
		nextConsumer:             mockProcessorConsumerObj,
	}
	rateLimitResponse := &pb.RateLimitResponse{
		OverallCode: pb.RateLimitResponse_OK,
		Statuses: []*pb.RateLimitResponse_DescriptorStatus{
			{
				Code: pb.RateLimitResponse_OK,
			},
			{
				Code: pb.RateLimitResponse_OVER_LIMIT,
			},
		},
	}
	mockRateLimitServiceClientObj.On("ShouldRateLimit", mock.Anything, mock.Anything).Return(rateLimitResponse, nil)
	mockProcessorConsumerObj.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)
	tokenString := base64.StdEncoding.EncodeToString([]byte("testuser:passw123"))
	span := testutil.NewTestSpan("http.request.header.authorization", "Basic: "+tokenString)
	traces := testutil.NewTestTraces(span)
	md := metadata.New(map[string]string{p.tenantIDHeaderName: testTenantID})
	ctx := metadata.NewIncomingContext(
		context.Background(),
		md,
	)
	err := p.ConsumeTraces(ctx, traces)
	require.NoError(t, err)
	mockRateLimitServiceClientObj.AssertNumberOfCalls(t, "ShouldRateLimit", 1)
	mockProcessorConsumerObj.AssertNumberOfCalls(t, "ConsumeTraces", 0)
}
