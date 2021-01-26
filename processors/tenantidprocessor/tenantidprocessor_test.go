package tenantidprocessor

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/receiver/jaegerreceiver"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestEndToEndJaegerGRPC(t *testing.T) {
	// prepare
	config := &jaegerreceiver.Config{
		Protocols: jaegerreceiver.Protocols{
			GRPC: &configgrpc.GRPCServerSettings{
				NetAddr: confignet.NetAddr{
					// do not collide with the standard port
					Endpoint: "localhost:14255",
				},
			},
		},
	}
	sink := new(consumertest.TracesSink)

	tenantProcessor := &processor{
		logger:               zap.NewNop(),
		tenantIDHeaderName:   defaultTenantIdHeaderName,
		tenantIDAttributeKey: defaultTenantIdAttributeKey,
	}

	params := component.ReceiverCreateParams{Logger: zap.NewNop()}
	jFactory := jaegerreceiver.NewFactory()
	jr, err := jFactory.CreateTracesReceiver(context.Background(), params, config, multiConsumer{
		sink:              sink,
		tenantIDprocessor: tenantProcessor,
	})
	require.NoError(t, err)
	defer jr.Shutdown(context.Background())

	require.NoError(t, jr.Start(context.Background(), componenttest.NewNopHost()))

	conn, err := grpc.Dial(config.Protocols.GRPC.NetAddr.Endpoint, grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	cl := api_v2.NewCollectorServiceClient(conn)
	req := grpcFixture(time.Now(), time.Hour, time.Hour*2)

	tenant := "jdoe"
	md := metadata.New(map[string]string{tenantProcessor.tenantIDHeaderName : tenant})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	resp, err := cl.PostSpans(ctx, req, grpc.WaitForReady(true))
	require.NoError(t, err)
	require.NotNil(t, resp)
	traces := sink.AllTraces()
	assert.Equal(t, 1, len(traces))
	trace := traces[0]
	rss := trace.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)

		ilss := rs.InstrumentationLibrarySpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)

			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				tenantAttr, ok := span.Attributes().Get(tenantProcessor.tenantIDAttributeKey)
				require.True(t, ok)
				assert.Equal(t, pdata.AttributeValueSTRING, tenantAttr.Type())
				assert.Equal(t, tenant, tenantAttr.StringVal())
			}
		}
	}
}


type multiConsumer struct {
	sink              *consumertest.TracesSink
	tenantIDprocessor *processor
}

var _ consumer.TracesConsumer = (*multiConsumer)(nil)

func (f multiConsumer) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	traces, err := f.tenantIDprocessor.ProcessTraces(ctx, td)
	if err != nil {
		return err
	}
	return f.sink.ConsumeTraces(ctx, traces)
}

func grpcFixture(t1 time.Time, d1, d2 time.Duration) *api_v2.PostSpansRequest {
	traceID := model.TraceID{}
	traceID.Unmarshal([]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80})
	parentSpanID := model.NewSpanID(binary.BigEndian.Uint64([]byte{0x1F, 0x1E, 0x1D, 0x1C, 0x1B, 0x1A, 0x19, 0x18}))
	childSpanID := model.NewSpanID(binary.BigEndian.Uint64([]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8}))

	return &api_v2.PostSpansRequest{
		Batch: model.Batch{
			Process: &model.Process{
				ServiceName: "issaTest",
				Tags: []model.KeyValue{
					model.Bool("bool", true),
					model.String("string", "yes"),
					model.Int64("int64", 1e7),
				},
			},
			Spans: []*model.Span{
				{
					TraceID:       traceID,
					SpanID:        childSpanID,
					OperationName: "DBSearch",
					StartTime:     t1,
					Duration:      d1,
					Tags: []model.KeyValue{
						model.String(tracetranslator.TagStatusMsg, "Stale indices"),
						model.Int64(tracetranslator.TagStatusCode, int64(pdata.StatusCodeError)),
						model.Bool("error", true),
					},
					References: []model.SpanRef{
						{
							TraceID: traceID,
							SpanID:  parentSpanID,
							RefType: model.SpanRefType_CHILD_OF,
						},
					},
				},
				{
					TraceID:       traceID,
					SpanID:        parentSpanID,
					OperationName: "ProxyFetch",
					StartTime:     t1.Add(d1),
					Duration:      d2,
					Tags: []model.KeyValue{
						model.String(tracetranslator.TagStatusMsg, "Frontend crash"),
						model.Int64(tracetranslator.TagStatusCode, int64(pdata.StatusCodeError)),
						model.Bool("error", true),
						model.String("somekey", "somevalue"),
					},
				},
			},
		},
	}
}
