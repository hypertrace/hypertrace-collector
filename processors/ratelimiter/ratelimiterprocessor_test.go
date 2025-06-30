package ratelimiter

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	pb "github.com/envoyproxy/go-control-plane/envoy/service/ratelimit/v3"
	internalmetadata "github.com/hypertrace/collector/processors/ratelimiter/internal/metadata"
	"github.com/hypertrace/collector/processors/testutil"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	testTenantID       string = "jdoe"
	basicAuthPrefixStr string = "Basic: "
)

type MockRateLimitServiceClient struct {
	mock.Mock
}

type MockProcessorConsumer struct {
	mock.Mock
}

var _ processor.Traces = (*MockProcessorConsumer)(nil)

func (m *MockProcessorConsumer) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (m *MockProcessorConsumer) Shutdown(_ context.Context) error {
	return nil
}

func (m *MockProcessorConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (m *MockRateLimitServiceClient) ShouldRateLimit(ctx context.Context, in *pb.RateLimitRequest, opts ...grpc.CallOption) (*pb.RateLimitResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(1) == nil {
		return args.Get(0).(*pb.RateLimitResponse), nil
	}
	return nil, args.Get(1).(error)
}

func (m *MockProcessorConsumer) ConsumeTraces(ctx context.Context, ld ptrace.Traces) error {
	args := m.Called(ctx, ld)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(error)
}

func TestRateLimitingWhenEmptyTenantHeader(t *testing.T) {
	mockRateLimitServiceClientObj := new(MockRateLimitServiceClient)
	mockProcessorConsumerObj := new(MockProcessorConsumer)
	telemetryBuilder, err := internalmetadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	p := &rateLimiterProcessor{
		logger:                     zap.NewNop(),
		tenantIDHeaderName:         defaultHeaderName,
		domain:                     defaultDomain,
		rateLimitServiceClient:     mockRateLimitServiceClientObj,
		rateLimitServiceClientConn: &grpc.ClientConn{},
		cancelFunc:                 t.SkipNow,
		nextConsumer:               mockProcessorConsumerObj,
		telemetryBuilder:           telemetryBuilder,
	}
	mockRateLimitServiceClientObj.On("ShouldRateLimit", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("rate limit called failed"))
	mockProcessorConsumerObj.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)
	tokenString := base64.StdEncoding.EncodeToString([]byte("testuser:passw123"))
	span := testutil.NewTestSpan("http.request.header.authorization", fmt.Sprintf("%s%s", basicAuthPrefixStr, tokenString))
	traces := testutil.NewTestTraces(span)
	md := metadata.New(map[string]string{})
	ctx := metadata.NewIncomingContext(
		context.Background(),
		md,
	)
	err = p.ConsumeTraces(ctx, traces)
	require.NoError(t, err)
	mockRateLimitServiceClientObj.AssertNumberOfCalls(t, "ShouldRateLimit", 0)
	mockProcessorConsumerObj.AssertNumberOfCalls(t, "ConsumeTraces", 1)
}

func TestWhenRateLimitServiceCallFailed(t *testing.T) {
	mockRateLimitServiceClientObj := new(MockRateLimitServiceClient)
	mockProcessorConsumerObj := new(MockProcessorConsumer)
	telemetryBuilder, err := internalmetadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	p := &rateLimiterProcessor{
		logger:                     zap.NewNop(),
		tenantIDHeaderName:         defaultHeaderName,
		domain:                     defaultDomain,
		rateLimitServiceClient:     mockRateLimitServiceClientObj,
		rateLimitServiceClientConn: &grpc.ClientConn{},
		cancelFunc:                 t.SkipNow,
		nextConsumer:               mockProcessorConsumerObj,
		telemetryBuilder:           telemetryBuilder,
	}
	mockRateLimitServiceClientObj.On("ShouldRateLimit", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("rate limit called failed"))
	mockProcessorConsumerObj.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)
	tokenString := base64.StdEncoding.EncodeToString([]byte("testuser:passw123"))
	span := testutil.NewTestSpan("http.request.header.authorization", fmt.Sprintf("%s%s", basicAuthPrefixStr, tokenString))
	traces := testutil.NewTestTraces(span)
	md := metadata.New(map[string]string{p.tenantIDHeaderName: testTenantID})
	ctx := metadata.NewIncomingContext(
		context.Background(),
		md,
	)
	err = p.ConsumeTraces(ctx, traces)
	require.NoError(t, err)
	mockRateLimitServiceClientObj.AssertNumberOfCalls(t, "ShouldRateLimit", 1)
	mockProcessorConsumerObj.AssertNumberOfCalls(t, "ConsumeTraces", 1)
}

func TestRateLimitingWhenTenantLimitReached(t *testing.T) {
	mockRateLimitServiceClientObj := new(MockRateLimitServiceClient)
	mockProcessorConsumerObj := new(MockProcessorConsumer)
	telemetryBuilder, err := internalmetadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	p := &rateLimiterProcessor{
		logger:                     zap.NewNop(),
		tenantIDHeaderName:         defaultHeaderName,
		domain:                     defaultDomain,
		rateLimitServiceClient:     mockRateLimitServiceClientObj,
		rateLimitServiceClientConn: &grpc.ClientConn{},
		cancelFunc:                 t.SkipNow,
		nextConsumer:               mockProcessorConsumerObj,
		telemetryBuilder:           telemetryBuilder,
	}
	rateLimitResponse := &pb.RateLimitResponse{
		OverallCode: pb.RateLimitResponse_OK,
		Statuses: []*pb.RateLimitResponse_DescriptorStatus{
			{
				Code: pb.RateLimitResponse_OVER_LIMIT,
			},
		},
	}
	mockRateLimitServiceClientObj.On("ShouldRateLimit", mock.Anything, mock.Anything).Return(rateLimitResponse, nil)
	mockProcessorConsumerObj.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)
	tokenString := base64.StdEncoding.EncodeToString([]byte("testuser:passw123"))
	span := testutil.NewTestSpan("http.request.header.authorization", fmt.Sprintf("%s%s", basicAuthPrefixStr, tokenString))
	traces := testutil.NewTestTraces(span)
	md := metadata.New(map[string]string{p.tenantIDHeaderName: testTenantID})
	ctx := metadata.NewIncomingContext(
		context.Background(),
		md,
	)
	err = p.ConsumeTraces(ctx, traces)
	_, ok := err.(*RateLimitError)
	require.True(t, ok, "Expected error to be of type *RateLimitError")
	mockRateLimitServiceClientObj.AssertNumberOfCalls(t, "ShouldRateLimit", 1)
	mockProcessorConsumerObj.AssertNumberOfCalls(t, "ConsumeTraces", 0)
}

func TestRateLimitingReturnsGRPCStatus(t *testing.T) {
	mockRateLimitServiceClientObj := new(MockRateLimitServiceClient)
	mockProcessorConsumerObj := new(MockProcessorConsumer)
	telemetryBuilder, err := internalmetadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	p := &rateLimiterProcessor{
		logger:                     zap.NewNop(),
		tenantIDHeaderName:         defaultHeaderName,
		domain:                     defaultDomain,
		rateLimitServiceClient:     mockRateLimitServiceClientObj,
		rateLimitServiceClientConn: &grpc.ClientConn{},
		cancelFunc:                 t.SkipNow,
		nextConsumer:               mockProcessorConsumerObj,
		telemetryBuilder:           telemetryBuilder,
	}

	// Setup rate limit response to indicate OVER_LIMIT
	rateLimitResponse := &pb.RateLimitResponse{
		OverallCode: pb.RateLimitResponse_OK,
		Statuses: []*pb.RateLimitResponse_DescriptorStatus{
			{
				Code: pb.RateLimitResponse_OVER_LIMIT,
			},
		},
	}

	mockRateLimitServiceClientObj.On("ShouldRateLimit", mock.Anything, mock.Anything).Return(rateLimitResponse, nil)

	// Create test spans
	tokenString := base64.StdEncoding.EncodeToString([]byte("testuser:passw123"))
	span := testutil.NewTestSpan("http.request.header.authorization", fmt.Sprintf("%s%s", basicAuthPrefixStr, tokenString))
	traces := testutil.NewTestTraces(span)

	// Create context with tenant ID
	md := metadata.New(map[string]string{p.tenantIDHeaderName: testTenantID})
	ctx := metadata.NewIncomingContext(
		context.Background(),
		md,
	)

	// Call ConsumeTraces and verify the returned error
	err = p.ConsumeTraces(ctx, traces)

	// Verify that we got a RateLimitError
	require.Error(t, err, "Expected an error to be returned when rate limited")

	// Verify that the error is a RateLimitError
	rateLimitErr, ok := err.(*RateLimitError)
	require.True(t, ok, "Expected error to be of type *RateLimitError")
	require.Equal(t, testTenantID, rateLimitErr.TenantID, "Expected tenant ID to match")

	// Verify that the error provides the correct gRPC status
	st, ok := status.FromError(err)
	require.True(t, ok, "Expected error to be convertible to a gRPC status")
	require.Equal(t, codes.ResourceExhausted, st.Code(), "Expected gRPC status code to be ResourceExhausted")

	// Verify that the next consumer was not called
	mockRateLimitServiceClientObj.AssertNumberOfCalls(t, "ShouldRateLimit", 1)
	mockProcessorConsumerObj.AssertNumberOfCalls(t, "ConsumeTraces", 0)
}
