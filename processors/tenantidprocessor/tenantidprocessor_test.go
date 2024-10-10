package tenantidprocessor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	internalmetadata "github.com/hypertrace/collector/processors/tenantidprocessor/internal/metadata"
	"github.com/jaegertracing/jaeger/model"
	jaegerconvert "github.com/jaegertracing/jaeger/model/converter/thrift/jaeger"
	jaegerthrift "github.com/jaegertracing/jaeger/thrift-gen/jaeger"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const testTenantID = "jdoe"

func TestMissingMetadataInContext(t *testing.T) {
	p := &tenantIdProcessor{
		logger:               zap.NewNop(),
		tenantIDHeaderName:   defaultHeaderName,
		tenantIDAttributeKey: defaultHeaderName,
		telemetryBuilder:     createTelemetryBuilder(t),
	}
	_, err := p.ProcessTraces(context.Background(), ptrace.NewTraces())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not extract headers")

	_, err = p.ProcessMetrics(context.Background(), pmetric.NewMetrics())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not extract headers")
}

func TestMissingTenantHeader(t *testing.T) {
	p := &tenantIdProcessor{
		logger:               zap.NewNop(),
		tenantIDHeaderName:   defaultHeaderName,
		tenantIDAttributeKey: defaultHeaderName,
		telemetryBuilder:     createTelemetryBuilder(t),
	}

	md := metadata.New(map[string]string{})
	ctx := metadata.NewIncomingContext(
		context.Background(),
		md,
	)
	_, err := p.ProcessTraces(ctx, ptrace.NewTraces())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing header")

	_, err = p.ProcessMetrics(ctx, pmetric.NewMetrics())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing header")
}

func TestMultipleTenantHeaders(t *testing.T) {
	p := &tenantIdProcessor{
		logger:               zap.NewNop(),
		tenantIDHeaderName:   defaultHeaderName,
		tenantIDAttributeKey: defaultHeaderName,
		telemetryBuilder:     createTelemetryBuilder(t),
	}

	md := metadata.New(map[string]string{p.tenantIDHeaderName: testTenantID})
	md.Append(p.tenantIDHeaderName, "jdoe2")
	ctx := metadata.NewIncomingContext(
		context.Background(),
		md,
	)
	_, err := p.ProcessTraces(ctx, ptrace.NewTraces())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple tenant ID headers")

	_, err = p.ProcessMetrics(ctx, pmetric.NewMetrics())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple tenant ID headers")
}

func TestEmptyTraces(t *testing.T) {
	p := &tenantIdProcessor{
		logger:               zap.NewNop(),
		tenantIDHeaderName:   defaultHeaderName,
		tenantIDAttributeKey: defaultHeaderName,
		telemetryBuilder:     createTelemetryBuilder(t),
	}
	traces := ptrace.NewTraces()
	md := metadata.New(map[string]string{p.tenantIDHeaderName: testTenantID})
	ctx := metadata.NewIncomingContext(
		context.Background(),
		md,
	)
	gotTraces, err := p.ProcessTraces(ctx, traces)
	require.NoError(t, err)
	assert.Equal(t, traces, gotTraces)
}

func TestEmptyMetrics(t *testing.T) {
	p := &tenantIdProcessor{
		logger:               zap.NewNop(),
		tenantIDHeaderName:   defaultHeaderName,
		tenantIDAttributeKey: defaultHeaderName,
		telemetryBuilder:     createTelemetryBuilder(t),
	}
	metrics := pmetric.NewMetrics()
	md := metadata.New(map[string]string{p.tenantIDHeaderName: testTenantID})
	ctx := metadata.NewIncomingContext(
		context.Background(),
		md,
	)
	gotMetrics, err := p.ProcessMetrics(ctx, metrics)
	require.NoError(t, err)
	assert.Equal(t, metrics, gotMetrics)
}

// GetAvailableLocalAddress finds an available local port and returns an endpoint
// describing it. The port is available for opening when this function returns
// provided that there is no race by some other code to grab the same port
// immediately.
func getAvailableLocalAddress(t *testing.T) string {
	ln, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err, "Failed to get a free local port")
	// There is a possible race if something else takes this same port before
	// the test uses it, however, that is unlikely in practice.
	defer ln.Close()
	return ln.Addr().String()
}

func createOTLPTracesReceiver(t *testing.T, nextConsumer consumer.Traces) (string, receiver.Traces) {
	addr := getAvailableLocalAddress(t)
	factory := otlpreceiver.NewFactory()
	cfg := factory.CreateDefaultConfig().(*otlpreceiver.Config)
	cfg.GRPC.NetAddr.Endpoint = addr
	cfg.HTTP = nil
	params := receivertest.NewNopSettings()
	otlpTracesRec, err := factory.CreateTracesReceiver(context.Background(), params, cfg, nextConsumer)
	require.NoError(t, err)

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	return addr, otlpTracesRec
}

func TestReceiveOTLPGRPC_Traces(t *testing.T) {
	tracesSink := new(consumertest.TracesSink)
	tenantProcessor := &tenantIdProcessor{
		logger:               zap.NewNop(),
		tenantIDHeaderName:   defaultHeaderName,
		tenantIDAttributeKey: defaultAttributeKey,
		telemetryBuilder:     createTelemetryBuilder(t),
	}

	tracesConsumer := tracesMultiConsumer{
		tracesSink:      tracesSink,
		tenantProcessor: tenantProcessor,
	}

	addr, otlpTracesRec := createOTLPTracesReceiver(t, tracesConsumer)

	err := otlpTracesRec.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer otlpTracesRec.Shutdown(context.Background())

	otlpExpFac := otlpexporter.NewFactory()
	tracesExporter, err := otlpExpFac.CreateTracesExporter(
		context.Background(),
		exporter.Settings{
			TelemetrySettings: component.TelemetrySettings{
				Logger:               zap.NewNop(),
				TracerProvider:       tracenoop.NewTracerProvider(),
				MeterProvider:        noop.NewMeterProvider(),
				LeveledMeterProvider: componenttest.NewNopTelemetrySettings().LeveledMeterProvider,
			},
		},
		&otlpexporter.Config{
			ClientConfig: configgrpc.ClientConfig{
				Headers:      map[string]configopaque.String{tenantProcessor.tenantIDHeaderName: configopaque.String(testTenantID)},
				Endpoint:     addr,
				WaitForReady: true,
				TLSSetting: configtls.ClientConfig{
					Insecure: true,
				},
			},
		},
	)
	require.NoError(t, err)

	err = tracesExporter.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	reqTraces := generateTraceDataOneSpan()
	err = tracesExporter.ConsumeTraces(context.Background(), reqTraces)
	require.NoError(t, err)

	traces := tracesSink.AllTraces()
	assert.Equal(t, 1, len(traces))
	tenantAttrsFound := assertTenantAttributeExists(
		t,
		traces[0],
		tenantProcessor.tenantIDAttributeKey,
		testTenantID,
	)
	assert.Equal(t, reqTraces.ResourceSpans().Len(), tenantAttrsFound)
}

func createOTLPMetricsReceiver(t *testing.T, nextConsumer consumer.Metrics) (string, receiver.Metrics) {
	addr := getAvailableLocalAddress(t)
	factory := otlpreceiver.NewFactory()
	cfg := factory.CreateDefaultConfig().(*otlpreceiver.Config)
	cfg.GRPC.NetAddr.Endpoint = addr
	cfg.HTTP = nil
	params := receivertest.NewNopSettings()

	otlpMetricsRec, err := factory.CreateMetricsReceiver(
		context.Background(),
		params,
		cfg,
		nextConsumer,
	)
	require.NoError(t, err)

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	return addr, otlpMetricsRec
}

func generateMetricData() pmetric.Metrics {
	md := pmetric.NewMetrics()
	md.ResourceMetrics().AppendEmpty()
	md.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty()
	md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
	metric := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	metric.SetEmptySum()
	metric.Sum().DataPoints().AppendEmpty()
	return md
}

func TestReceiveOTLPGRPC_Metrics(t *testing.T) {
	tenantProcessor := &tenantIdProcessor{
		logger:               zap.NewNop(),
		tenantIDHeaderName:   defaultHeaderName,
		tenantIDAttributeKey: defaultAttributeKey,
		telemetryBuilder:     createTelemetryBuilder(t),
	}

	metricsSink := new(consumertest.MetricsSink)

	metricsConsumer := metricsMultiConsumer{
		metricsSink:     metricsSink,
		tenantProcessor: tenantProcessor,
	}

	addr, otlpMetricsRec := createOTLPMetricsReceiver(t, metricsConsumer)
	err := otlpMetricsRec.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer otlpMetricsRec.Shutdown(context.Background())

	metricsExporter, err := otlpexporter.NewFactory().CreateMetricsExporter(
		context.Background(),
		exporter.Settings{
			TelemetrySettings: component.TelemetrySettings{
				Logger:               zap.NewNop(),
				TracerProvider:       tracenoop.NewTracerProvider(),
				MeterProvider:        noop.NewMeterProvider(),
				LeveledMeterProvider: componenttest.NewNopTelemetrySettings().LeveledMeterProvider,
			},
		},
		&otlpexporter.Config{
			ClientConfig: configgrpc.ClientConfig{
				Headers:      map[string]configopaque.String{tenantProcessor.tenantIDHeaderName: testTenantID},
				Endpoint:     addr,
				WaitForReady: true,
				TLSSetting: configtls.ClientConfig{
					Insecure: true,
				},
			},
		},
	)
	require.NoError(t, err)
	err = metricsExporter.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	reqMetrics := generateMetricData()

	err = metricsExporter.ConsumeMetrics(context.Background(), reqMetrics)
	require.NoError(t, err)

	metrics := metricsSink.AllMetrics()
	assert.Equal(t, 1, len(metrics))
	tenantAttrsFound := assertTenantTagExists(
		t,
		metrics[0],
		tenantProcessor.tenantIDAttributeKey,
		testTenantID,
	)
	assert.Equal(t, reqMetrics.MetricCount(), tenantAttrsFound)
}

func TestReceiveJaegerThriftHTTP_Traces(t *testing.T) {
	sink := new(consumertest.TracesSink)
	tenantProcessor := &tenantIdProcessor{
		logger:               zap.NewNop(),
		tenantIDHeaderName:   defaultHeaderName,
		tenantIDAttributeKey: defaultAttributeKey,
		telemetryBuilder:     createTelemetryBuilder(t),
	}

	addr := getAvailableLocalAddress(t)
	cfg := &jaegerreceiver.Config{
		Protocols: jaegerreceiver.Protocols{
			ThriftHTTP: &confighttp.ServerConfig{
				Endpoint: addr,
			},
		},
	}
	params := receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger:               zap.NewNop(),
			TracerProvider:       tracenoop.NewTracerProvider(),
			MeterProvider:        noop.NewMeterProvider(),
			LeveledMeterProvider: componenttest.NewNopTelemetrySettings().LeveledMeterProvider,
		},
	}
	jrf := jaegerreceiver.NewFactory()
	rec, err := jrf.CreateTracesReceiver(context.Background(), params, cfg, tracesMultiConsumer{
		tracesSink:      sink,
		tenantProcessor: tenantProcessor,
	})
	require.NoError(t, err)

	err = rec.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer rec.Shutdown(context.Background())

	td := generateTraceDataOneSpan()
	batches, err := jaeger.ProtoFromTraces(td)
	require.NoError(t, err)

	collectorAddr := fmt.Sprintf("http://%s/api/traces", addr)
	for _, batch := range batches {
		err := sendToJaegerHTTPThrift(collectorAddr, map[string]string{tenantProcessor.tenantIDHeaderName: testTenantID}, jaegerModelToThrift(batch))
		require.NoError(t, err)
	}

	traces := sink.AllTraces()
	assert.Equal(t, 1, len(traces))
	tenantAttrsFound := assertTenantAttributeExists(
		t,
		traces[0],
		tenantProcessor.tenantIDAttributeKey,
		testTenantID,
	)
	assert.Equal(t, td.ResourceSpans().Len(), tenantAttrsFound)
}

func assertTenantAttributeExists(t *testing.T, trace ptrace.Traces, tenantAttrKey string, tenantID string) int {
	numOfTenantAttrs := 0
	rss := trace.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		tenantAttr, ok := rs.Resource().Attributes().Get(tenantAttrKey)
		require.True(t, ok)
		numOfTenantAttrs++
		assert.Equal(t, pcommon.ValueTypeStr, tenantAttr.Type())
		assert.Equal(t, tenantID, tenantAttr.Str())
	}
	return numOfTenantAttrs
}

func assertTenantTagExists(t *testing.T, metricData pmetric.Metrics, tenantAttrKey string, tenantID string) int {
	numOfTenantAttrs := 0
	rms := metricData.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)

		ilms := rm.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)

			metrics := ilm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				metricDataPoints := metric.Sum().DataPoints()
				for l := 0; l < metricDataPoints.Len(); l++ {
					tenantAttr, ok := metricDataPoints.At(l).Attributes().Get(tenantAttrKey)
					require.True(t, ok)
					numOfTenantAttrs++
					assert.Equal(t, pcommon.NewValueStr(tenantID), tenantAttr)
				}
			}
		}
	}
	return numOfTenantAttrs
}

type tracesMultiConsumer struct {
	tracesSink      *consumertest.TracesSink
	tenantProcessor *tenantIdProcessor
}

var _ consumer.Traces = (*tracesMultiConsumer)(nil)

func (f tracesMultiConsumer) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	traces, err := f.tenantProcessor.ProcessTraces(ctx, td)
	if err != nil {
		return err
	}
	return f.tracesSink.ConsumeTraces(ctx, traces)
}

func (f tracesMultiConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

type metricsMultiConsumer struct {
	metricsSink     consumer.Metrics //*consumertest.MetricsSink
	tenantProcessor *tenantIdProcessor
}

var _ consumer.Metrics = (*metricsMultiConsumer)(nil)

func (f metricsMultiConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	metrics, err := f.tenantProcessor.ProcessMetrics(ctx, md)
	if err != nil {
		return err
	}
	return f.metricsSink.ConsumeMetrics(ctx, metrics)
}

func (f metricsMultiConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

var (
	TestSpanStartTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	TestSpanStartTimestamp = pcommon.NewTimestampFromTime(TestSpanStartTime)
	TestSpanEventTime      = time.Date(2020, 2, 11, 20, 26, 13, 123, time.UTC)
	TestSpanEventTimestamp = pcommon.NewTimestampFromTime(TestSpanEventTime)

	TestSpanEndTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	TestSpanEndTimestamp = pcommon.NewTimestampFromTime(TestSpanEndTime)
)

func generateTraceDataOneSpan() ptrace.Traces {
	td := generateTraceDataOneEmptyScope()
	rs0ils0 := td.ResourceSpans().At(0).ScopeSpans().At(0)
	rs0ils0.Spans().AppendEmpty()
	fillSpanOne(rs0ils0.Spans().At(0))
	return td
}

func generateTraceDataOneEmptyScope() ptrace.Traces {
	td := generateTraceDataNoScope()
	rs0 := td.ResourceSpans().At(0)
	rs0.ScopeSpans().AppendEmpty()
	return td
}

func generateTraceDataNoScope() ptrace.Traces {
	td := generateTraceDataOneEmptyResourceSpans()
	rs0 := td.ResourceSpans().At(0)
	initResource1(rs0.Resource())
	return td
}

func generateTraceDataOneEmptyResourceSpans() ptrace.Traces {
	td := generateTraceDataEmpty()
	td.ResourceSpans().AppendEmpty()
	return td
}

func generateTraceDataEmpty() ptrace.Traces {
	td := ptrace.NewTraces()
	return td
}

func initResource1(r pcommon.Resource) {
	initResourceAttributes1(r.Attributes())
}

func initResourceAttributes1(dest pcommon.Map) {
	dest.PutStr("resource-attr", "resource-attr-val-1")
}

func fillSpanOne(span ptrace.Span) {
	span.SetName("operationA")
	span.SetStartTimestamp(TestSpanStartTimestamp)
	span.SetEndTimestamp(TestSpanEndTimestamp)
	span.SetDroppedAttributesCount(1)
	span.SetTraceID([16]byte{0, 1, 2})
	span.SetSpanID([8]byte{0, 1})
	evs := span.Events()
	evs.AppendEmpty()
	evs.AppendEmpty()
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
	status.SetCode(ptrace.StatusCodeError)
	status.SetMessage("status-cancelled")
}

func initSpanEventAttributes(dest pcommon.Map) {
	dest.PutStr("span-event-attr", "span-event-attr-val")
}

func jaegerModelToThrift(batch *model.Batch) *jaegerthrift.Batch {
	return &jaegerthrift.Batch{
		Process: jaegerProcessModelToThrift(batch.Process),
		Spans:   jaegerconvert.FromDomain(batch.Spans),
	}
}

func jaegerProcessModelToThrift(process *model.Process) *jaegerthrift.Process {
	if process == nil {
		return nil
	}
	return &jaegerthrift.Process{
		ServiceName: process.ServiceName,
	}
}

func sendToJaegerHTTPThrift(endpoint string, headers map[string]string, batch *jaegerthrift.Batch) error {
	buf, err := thrift.NewTSerializer().Write(context.Background(), batch)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-thrift")
	for k, v := range headers {
		req.Header.Add(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to upload traces; HTTP status code: %d", resp.StatusCode)
	}
	return nil
}

func createTelemetryBuilder(t *testing.T) *internalmetadata.TelemetryBuilder {
	telemetryBuilder, err := internalmetadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	return telemetryBuilder
}
