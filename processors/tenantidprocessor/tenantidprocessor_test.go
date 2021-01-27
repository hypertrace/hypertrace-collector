package tenantidprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/testutil"
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

func TestMultipleTenantHeaders(t *testing.T) {
	p := &processor{
		logger:               zap.NewNop(),
		tenantIDHeaderName:   defaultTenantIdHeaderName,
		tenantIDAttributeKey: defaultTenantIdHeaderName,
	}

	md := metadata.New(map[string]string{p.tenantIDHeaderName: testTenantID})
	md.Append(p.tenantIDHeaderName, "jdoe2")
	ctx := metadata.NewIncomingContext(
		context.Background(),
		md,
	)

	_, err := p.ProcessTraces(ctx, pdata.NewTraces())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple tenant ID headers")
}

func TestEmptyTraces(t *testing.T) {
	p := &processor{
		logger:               zap.NewNop(),
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
