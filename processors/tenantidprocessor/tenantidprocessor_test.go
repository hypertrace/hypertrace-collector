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
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/receiver/jaegerreceiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/testutil"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const testTenantID = "jdoe"

func TestMissingTenantHeader(t *testing.T) {
	p := &processor{
		logger:               zap.NewNop(),
		tenantIDHeaderName:   defaultTenantIdHeaderName,
		tenantIDAttributeKey: defaultTenantIdHeaderName,
	}
	_, err := p.ProcessTraces(context.Background(), pdata.NewTraces())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing header")
}

func TestEmptyTraces(t *testing.T) {
	p := &processor{
		logger:               zap.NewNop(),
		tenantIDViews:        make(map[string]*view.View),
		tenantIDHeaderName:   defaultTenantIdHeaderName,
		tenantIDAttributeKey: defaultTenantIdHeaderName,
	}
	traces := pdata.NewTraces()
	md := metadata.New(map[string]string{p.tenantIDHeaderName: testTenantID})
	ctx := metadata.NewIncomingContext(
		context.Background(),
		md,
	)
	gotTraces, err := p.ProcessTraces(ctx, traces)
	require.NoError(t, err)
	assert.Equal(t, traces, gotTraces)
}

func TestReceiveOTLPGRPC(t *testing.T) {
	sink := new(consumertest.TracesSink)
	tenantProcessor := &processor{
		logger:               zap.NewNop(),
		tenantIDViews:        make(map[string]*view.View),
		tenantIDHeaderName:   defaultTenantIdHeaderName,
		tenantIDAttributeKey: defaultTenantIdAttributeKey,
	}

	addr := testutil.GetAvailableLocalAddress(t)
	factory := otlpreceiver.NewFactory()
	cfg := factory.CreateDefaultConfig().(*otlpreceiver.Config)
	cfg.GRPC.NetAddr.Endpoint = addr
	cfg.HTTP = nil
	params := component.ReceiverCreateParams{Logger: zap.NewNop()}
	otlpRec, err := factory.CreateTracesReceiver(context.Background(), params, cfg, multiConsumer{
		sink:              sink,
		tenantIDprocessor: tenantProcessor,
	})
	require.NoError(t, err)
	err = otlpRec.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer otlpRec.Shutdown(context.Background())

	conn, err := grpc.Dial(cfg.GRPC.NetAddr.Endpoint, grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	otlpExpFac := otlpexporter.NewFactory()
	exporter, err := otlpExpFac.CreateTracesExporter(
		context.Background(),
		component.ExporterCreateParams{Logger: zap.NewNop()},
		&otlpexporter.Config{
			GRPCClientSettings: configgrpc.GRPCClientSettings{
				Headers:      map[string]string{tenantProcessor.tenantIDHeaderName: testTenantID},
				Endpoint:     addr,
				WaitForReady: true,
				TLSSetting: configtls.TLSClientSetting{
					Insecure: true,
				},
			},
		},
	)
	require.NoError(t, err)
	reqTraces := GenerateTraceDataOneSpan()
	err = exporter.ConsumeTraces(context.Background(), reqTraces)
	require.NoError(t, err)

	traces := sink.AllTraces()
	assert.Equal(t, 1, len(traces))
	tenantAttrsFound := assertTenantAttributeExists(
		t,
		traces[0],
		tenantProcessor.tenantIDAttributeKey,
		testTenantID,
	)
	assert.Equal(t, reqTraces.SpanCount(), tenantAttrsFound)
}

func TestReceiveJaegerGRPC(t *testing.T) {
	// prepare
	addr := testutil.GetAvailableLocalAddress(t)
	config := &jaegerreceiver.Config{
		Protocols: jaegerreceiver.Protocols{
			GRPC: &configgrpc.GRPCServerSettings{
				NetAddr: confignet.NetAddr{
					Endpoint: addr,
				},
			},
		},
	}
	sink := new(consumertest.TracesSink)
	tenantProcessor := &processor{
		logger:               zap.NewNop(),
		tenantIDViews:        make(map[string]*view.View),
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

	md := metadata.New(map[string]string{tenantProcessor.tenantIDHeaderName: testTenantID})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	resp, err := cl.PostSpans(ctx, req, grpc.WaitForReady(true))
	require.NoError(t, err)
	require.NotNil(t, resp)
	traces := sink.AllTraces()
	assert.Equal(t, 1, len(traces))
	tenantAttrsFound := assertTenantAttributeExists(t, traces[0], tenantProcessor.tenantIDAttributeKey, testTenantID)
	assert.Equal(t, len(req.Batch.Spans), tenantAttrsFound)
}

func assertTenantAttributeExists(t *testing.T, trace pdata.Traces, tenantAttrKey string, tenantID string) int {
	numOfTenantAttrs := 0
	rss := trace.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)

		ilss := rs.InstrumentationLibrarySpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)

			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				tenantAttr, ok := span.Attributes().Get(tenantAttrKey)
				require.True(t, ok)
				numOfTenantAttrs++
				assert.Equal(t, pdata.AttributeValueSTRING, tenantAttr.Type())
				assert.Equal(t, tenantID, tenantAttr.StringVal())
			}
		}
	}
	return numOfTenantAttrs
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

var (
	resourceAttributes1    = map[string]pdata.AttributeValue{"resource-attr": pdata.NewAttributeValueString("resource-attr-val-1")}
	TestSpanStartTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	TestSpanStartTimestamp = pdata.TimestampUnixNano(TestSpanStartTime.UnixNano())
	TestSpanEventTime      = time.Date(2020, 2, 11, 20, 26, 13, 123, time.UTC)
	TestSpanEventTimestamp = pdata.TimestampUnixNano(TestSpanEventTime.UnixNano())

	TestSpanEndTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	TestSpanEndTimestamp = pdata.TimestampUnixNano(TestSpanEndTime.UnixNano())
	spanEventAttributes  = map[string]pdata.AttributeValue{"span-event-attr": pdata.NewAttributeValueString("span-event-attr-val")}
)

func GenerateTraceDataOneSpan() pdata.Traces {
	td := GenerateTraceDataOneEmptyInstrumentationLibrary()
	rs0ils0 := td.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0)
	rs0ils0.Spans().Resize(1)
	fillSpanOne(rs0ils0.Spans().At(0))
	return td
}

func GenerateTraceDataOneEmptyInstrumentationLibrary() pdata.Traces {
	td := GenerateTraceDataNoLibraries()
	rs0 := td.ResourceSpans().At(0)
	rs0.InstrumentationLibrarySpans().Resize(1)
	return td
}

func GenerateTraceDataNoLibraries() pdata.Traces {
	td := GenerateTraceDataOneEmptyResourceSpans()
	rs0 := td.ResourceSpans().At(0)
	initResource1(rs0.Resource())
	return td
}

func GenerateTraceDataOneEmptyResourceSpans() pdata.Traces {
	td := GenerateTraceDataEmpty()
	td.ResourceSpans().Resize(1)
	return td
}

func GenerateTraceDataEmpty() pdata.Traces {
	td := pdata.NewTraces()
	return td
}

func initResource1(r pdata.Resource) {
	initResourceAttributes1(r.Attributes())
}

func initResourceAttributes1(dest pdata.AttributeMap) {
	dest.InitFromMap(resourceAttributes1)
}

func fillSpanOne(span pdata.Span) {
	span.SetName("operationA")
	span.SetStartTime(TestSpanStartTimestamp)
	span.SetEndTime(TestSpanEndTimestamp)
	span.SetDroppedAttributesCount(1)
	evs := span.Events()
	evs.Resize(2)
	ev0 := evs.At(0)
	ev0.SetTimestamp(TestSpanEventTimestamp)
	ev0.SetName("event-with-attr")
	initSpanEventAttributes(ev0.Attributes())
	ev0.SetDroppedAttributesCount(2)
	ev1 := evs.At(1)
	ev1.SetTimestamp(TestSpanEventTimestamp)
	ev1.SetName("event")
	ev1.SetDroppedAttributesCount(2)
	span.SetDroppedEventsCount(1)
	status := span.Status()
	status.SetCode(pdata.StatusCodeError)
	status.SetMessage("status-cancelled")
}

func initSpanEventAttributes(dest pdata.AttributeMap) {
	dest.InitFromMap(spanEventAttributes)
}
