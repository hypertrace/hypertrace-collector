package kafkaexporter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/gogo/protobuf/jsonpb"
	jaegerproto "github.com/jaegertracing/jaeger/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"

	jaegertranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

func TestJaegerMarshalerDebug(t *testing.T) {
	maxMessageBytes := 1024
	td := pdata.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().Insert("test-key", pdata.NewAttributeValueString("test-val"))
	ils := rs.InstrumentationLibrarySpans().AppendEmpty()

	// Will add this span to the messages queue to export
	span := ils.Spans().AppendEmpty()
	span.SetName("foo")
	span.SetStartTimestamp(pdata.Timestamp(10))
	span.SetEndTimestamp(pdata.Timestamp(20))
	span.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.Attributes().Insert("tag1", pdata.NewAttributeValueString("tag1-val"))

	// Will log on this span that exceeds max message size.
	span = ils.Spans().AppendEmpty()
	span.SetName("bar")
	span.SetStartTimestamp(pdata.Timestamp(100))
	span.SetEndTimestamp(pdata.Timestamp(225))
	span.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.Attributes().Insert("tag10", pdata.NewAttributeValueString("tag10-val"))
	span.Attributes().Insert("big-tag", pdata.NewAttributeValueString(createLongString(maxMessageBytes, "a")))

	batches, err := jaegertranslator.InternalTracesToJaegerProto(td)
	require.NoError(t, err)

	jsonMarshaler := &jsonpb.Marshaler{}

	batches[0].Spans[0].Process = batches[0].Process
	jaegerProtoBytes0, err := batches[0].Spans[0].Marshal()
	messageKey := []byte(batches[0].Spans[0].TraceID.String())
	require.NoError(t, err)
	require.NotNil(t, jaegerProtoBytes0)

	jsonByteBuffer0 := new(bytes.Buffer)
	require.NoError(t, jsonMarshaler.Marshal(jsonByteBuffer0, batches[0].Spans[0]))

	batches[0].Spans[1].Process = batches[0].Process
	jaegerProtoBytes1, err := batches[0].Spans[1].Marshal()
	require.NoError(t, err)
	require.NotNil(t, jaegerProtoBytes1)

	jsonByteBuffer1 := new(bytes.Buffer)
	require.NoError(t, jsonMarshaler.Marshal(jsonByteBuffer1, batches[0].Spans[1]))

	tests := []struct {
		unmarshaler TracesMarshaler
		encoding    string
		messages    []*sarama.ProducerMessage
	}{
		{
			unmarshaler: jaegerMarshalerDebug{
				marshaler:       jaegerProtoSpanMarshaler{},
				version:         sarama.V2_0_0_0,
				maxMessageBytes: maxMessageBytes,
			},
			encoding: "jaeger_proto",
			messages: []*sarama.ProducerMessage{
				{Topic: "topic", Value: sarama.ByteEncoder(jaegerProtoBytes0), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(jaegerProtoBytes1), Key: sarama.ByteEncoder(messageKey)}},
		},
		{
			unmarshaler: jaegerMarshalerDebug{
				marshaler: jaegerJSONSpanMarshaler{
					pbMarshaler: &jsonpb.Marshaler{},
				},
				version:         sarama.V2_0_0_0,
				maxMessageBytes: maxMessageBytes,
			},
			encoding: "jaeger_json",
			messages: []*sarama.ProducerMessage{
				{Topic: "topic", Value: sarama.ByteEncoder(jsonByteBuffer0.Bytes()), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(jsonByteBuffer1.Bytes()), Key: sarama.ByteEncoder(messageKey)},
			},
		},
		{
			unmarshaler: jaegerMarshalerDebug{
				marshaler:          jaegerProtoSpanMarshaler{},
				version:            sarama.V2_0_0_0,
				maxMessageBytes:    maxMessageBytes,
				dumpSpanAttributes: true, // test setting this to true
			},
			encoding: "jaeger_proto",
			messages: []*sarama.ProducerMessage{
				{Topic: "topic", Value: sarama.ByteEncoder(jaegerProtoBytes0), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(jaegerProtoBytes1), Key: sarama.ByteEncoder(messageKey)}},
		},
	}
	for _, test := range tests {
		t.Run(test.encoding, func(t *testing.T) {
			messages, err := test.unmarshaler.Marshal(td, "topic")
			require.NoError(t, err)
			assert.Equal(t, test.messages, messages)
			assert.Equal(t, test.encoding, test.unmarshaler.Encoding())
		})
	}
}

func TestJaegerMarshalerDebug_error_covert_traceID(t *testing.T) {
	marshaler := jaegerMarshalerDebug{
		marshaler: jaegerProtoSpanMarshaler{},
	}
	td := pdata.NewTraces()
	td.ResourceSpans().AppendEmpty().InstrumentationLibrarySpans().AppendEmpty().Spans().AppendEmpty()
	// fails in zero traceID
	messages, err := marshaler.Marshal(td, "topic")
	require.Error(t, err)
	assert.Nil(t, messages)
}

func TestCureSpans(t *testing.T) {
	maxMessageBytes := 1024
	maxAttributeValueSize := 256

	tests := []struct {
		inputSpan    *jaegerproto.Span
		expectedSpan *jaegerproto.Span
	}{
		{
			inputSpan: &jaegerproto.Span{
				TraceID: jaegerproto.TraceID{Low: 101, High: 2001},
				SpanID:  124,
				Tags: []jaegerproto.KeyValue{
					{Key: "tag-1", VType: jaegerproto.ValueType_STRING, VStr: "simple string"},
					{Key: "tag-2", VType: jaegerproto.ValueType_STRING, VStr: createLongString(32, "fo")},
					{Key: "tag-3", VType: jaegerproto.ValueType_BOOL, VBool: true},
					{Key: "tag-4", VType: jaegerproto.ValueType_BINARY, VBinary: createLongByteArray(50)},
					{Key: "tag-5", VType: jaegerproto.ValueType_STRING, VStr: createLongString(70, "ba")},
				},
			},
			expectedSpan: &jaegerproto.Span{
				TraceID: jaegerproto.TraceID{Low: 101, High: 2001},
				SpanID:  124,
				Tags: []jaegerproto.KeyValue{
					{Key: "tag-1", VType: jaegerproto.ValueType_STRING, VStr: "simple string"},
					{Key: "tag-2", VType: jaegerproto.ValueType_STRING, VStr: createLongString(32, "fo")},
					{Key: "tag-3", VType: jaegerproto.ValueType_BOOL, VBool: true},
					{Key: "tag-4", VType: jaegerproto.ValueType_BINARY, VBinary: createLongByteArray(50)},
					{Key: "tag-5", VType: jaegerproto.ValueType_STRING, VStr: createLongString(70, "ba")},
				},
			},
		},
		{
			inputSpan: &jaegerproto.Span{
				TraceID: jaegerproto.TraceID{Low: 100, High: 2000},
				SpanID:  123,
				Tags: []jaegerproto.KeyValue{
					{Key: "tag-1", VType: jaegerproto.ValueType_STRING, VStr: "simple string"},
					{Key: "tag-2", VType: jaegerproto.ValueType_STRING, VStr: createLongString(128, "fo")},
					{Key: "tag-3", VType: jaegerproto.ValueType_BOOL, VBool: true},
					{Key: "tag-4", VType: jaegerproto.ValueType_BINARY, VBinary: createLongByteArray(500)},
					{Key: "tag-5", VType: jaegerproto.ValueType_STRING, VStr: createLongString(256, "ba")},
				},
			},
			expectedSpan: &jaegerproto.Span{
				TraceID: jaegerproto.TraceID{Low: 100, High: 2000},
				SpanID:  123,
				Tags: []jaegerproto.KeyValue{
					{Key: "tag-1", VType: jaegerproto.ValueType_STRING, VStr: "simple string"},
					{Key: "tag-2", VType: jaegerproto.ValueType_STRING, VStr: createLongString(128, "fo")},
					{Key: "tag-3", VType: jaegerproto.ValueType_BOOL, VBool: true},
					{Key: "tag-4", VType: jaegerproto.ValueType_BINARY, VBinary: createLongByteArray(256)},
					{Key: "tag-5", VType: jaegerproto.ValueType_STRING, VStr: createLongString(128, "ba")},
					{Key: "tag-4.truncated", VType: jaegerproto.ValueType_BOOL, VBool: true},
					{Key: "tag-5.truncated", VType: jaegerproto.ValueType_BOOL, VBool: true},
				},
			},
		},
		{
			inputSpan: &jaegerproto.Span{
				TraceID: jaegerproto.TraceID{Low: 102, High: 2002},
				SpanID:  125,
				Tags: []jaegerproto.KeyValue{
					{Key: "tag-1", VType: jaegerproto.ValueType_STRING, VStr: "simple string"},
					{Key: "tag-2", VType: jaegerproto.ValueType_STRING, VStr: createLongString(128, "fo")},
					{Key: "tag-3", VType: jaegerproto.ValueType_INT64, VInt64: 68},
					{Key: "tag-4", VType: jaegerproto.ValueType_BINARY, VBinary: createLongByteArray(500)},
					{Key: "tag-5", VType: jaegerproto.ValueType_STRING, VStr: createLongString(256, "ba")},
					{Key: "tag-6", VType: jaegerproto.ValueType_STRING, VStr: createLongString(256, "wx")},
				},
			},
			expectedSpan: &jaegerproto.Span{
				TraceID: jaegerproto.TraceID{Low: 102, High: 2002},
				SpanID:  125,
				Tags: []jaegerproto.KeyValue{
					{Key: "tag-1", VType: jaegerproto.ValueType_STRING, VStr: "simple string"},
					{Key: "tag-2", VType: jaegerproto.ValueType_STRING, VStr: createLongString(64, "fo")},
					{Key: "tag-3", VType: jaegerproto.ValueType_INT64, VInt64: 68},
					{Key: "tag-4", VType: jaegerproto.ValueType_BINARY, VBinary: createLongByteArray(128)},
					{Key: "tag-5", VType: jaegerproto.ValueType_STRING, VStr: createLongString(64, "ba")},
					{Key: "tag-6", VType: jaegerproto.ValueType_STRING, VStr: createLongString(64, "wx")},
					{Key: "tag-4.truncated", VType: jaegerproto.ValueType_BOOL, VBool: true},
					{Key: "tag-5.truncated", VType: jaegerproto.ValueType_BOOL, VBool: true},
					{Key: "tag-6.truncated", VType: jaegerproto.ValueType_BOOL, VBool: true},
					{Key: "tag-2.truncated", VType: jaegerproto.ValueType_BOOL, VBool: true},
				},
			},
		},
		{
			inputSpan: &jaegerproto.Span{
				TraceID: jaegerproto.TraceID{Low: 103, High: 2003},
				SpanID:  126,
				Tags: []jaegerproto.KeyValue{
					{Key: "tag-1", VType: jaegerproto.ValueType_STRING, VStr: "simple string"},
					{Key: "tag-2", VType: jaegerproto.ValueType_STRING, VStr: createLongString(128, "fo")},
					{Key: "tag-3", VType: jaegerproto.ValueType_INT64, VInt64: 68},
					{Key: "tag-4", VType: jaegerproto.ValueType_BINARY, VBinary: createLongByteArray(500)},
					{Key: "tag-5", VType: jaegerproto.ValueType_STRING, VStr: createLongString(256, "ba")},
					{Key: "tag-6", VType: jaegerproto.ValueType_STRING, VStr: createLongString(256, "wx")},
				},
			},
			expectedSpan: &jaegerproto.Span{
				TraceID: jaegerproto.TraceID{Low: 103, High: 2003},
				SpanID:  126,
				Tags: []jaegerproto.KeyValue{
					{Key: "tag-1", VType: jaegerproto.ValueType_STRING, VStr: "simple string"},
					{Key: "tag-2", VType: jaegerproto.ValueType_STRING, VStr: createLongString(64, "fo")},
					{Key: "tag-3", VType: jaegerproto.ValueType_INT64, VInt64: 68},
					{Key: "tag-4", VType: jaegerproto.ValueType_BINARY, VBinary: createLongByteArray(128)},
					{Key: "tag-5", VType: jaegerproto.ValueType_STRING, VStr: createLongString(64, "ba")},
					{Key: "tag-6", VType: jaegerproto.ValueType_STRING, VStr: createLongString(64, "wx")},
					{Key: "tag-4.truncated", VType: jaegerproto.ValueType_BOOL, VBool: true},
					{Key: "tag-5.truncated", VType: jaegerproto.ValueType_BOOL, VBool: true},
					{Key: "tag-6.truncated", VType: jaegerproto.ValueType_BOOL, VBool: true},
					{Key: "tag-2.truncated", VType: jaegerproto.ValueType_BOOL, VBool: true},
				},
			},
		},
	}

	marshaler := jaegerProtoSpanMarshaler{}

	j := jaegerMarshalerDebug{
		marshaler:             jaegerProtoSpanMarshaler{},
		version:               sarama.V2_0_0_0,
		maxMessageBytes:       maxMessageBytes,
		maxAttributeValueSize: maxAttributeValueSize,
		cureSpans:             true,
	}

	for _, test := range tests {
		msg, err := j.cureSpan(test.inputSpan, "test-topic")
		require.NoError(t, err)

		expectedMsgBytes, err := marshaler.marshal(test.expectedSpan)
		require.NoError(t, err)
		expectedMsgBytesEncoder := sarama.ByteEncoder(expectedMsgBytes)
		assert.Equal(t, expectedMsgBytesEncoder, msg.Value)
	}
}

func TestJaegerMarshalerDebugCureSpansFail(t *testing.T) {
	maxMessageBytes := 1024
	maxAttributeValueSize := 256

	span := &jaegerproto.Span{
		TraceID: jaegerproto.TraceID{Low: 113, High: 2103},
		SpanID:  1270,
	}

	var tags []jaegerproto.KeyValue
	for i := 0; i < 64; i++ {
		kv := jaegerproto.KeyValue{Key: "tag-2", VType: jaegerproto.ValueType_STRING, VStr: createLongString(256, "fo")}
		tags = append(tags, kv)
	}
	span.Tags = tags

	j := jaegerMarshalerDebug{
		marshaler:             jaegerProtoSpanMarshaler{},
		version:               sarama.V2_0_0_0,
		maxMessageBytes:       maxMessageBytes,
		maxAttributeValueSize: maxAttributeValueSize,
		cureSpans:             true,
	}

	msg, err := j.cureSpan(span, "test-topic-2")
	require.Error(t, err)
	assert.Nil(t, msg)
}

func TestJaegerMarshalerDebugCureSpans(t *testing.T) {
	maxMessageBytes := 1024
	maxAttributeValueSize := 256
	jsonMarshaler := &jsonpb.Marshaler{}

	td := pdata.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().Insert("test-key", pdata.NewAttributeValueString("test-val"))
	ils := rs.InstrumentationLibrarySpans().AppendEmpty()

	// Will add this span to the messages queue to export
	span := ils.Spans().AppendEmpty()
	span.SetName("foo")
	span.SetStartTimestamp(pdata.Timestamp(10))
	span.SetEndTimestamp(pdata.Timestamp(20))
	span.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.Attributes().Insert("tag1", pdata.NewAttributeValueString("tag1-val"))

	// Will log on this span that exceeds max message size.
	span = ils.Spans().AppendEmpty()
	span.SetName("bar")
	span.SetStartTimestamp(pdata.Timestamp(100))
	span.SetEndTimestamp(pdata.Timestamp(225))
	span.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.Attributes().Insert("tag10", pdata.NewAttributeValueString("tag10-val"))
	span.Attributes().Insert("big-tag", pdata.NewAttributeValueString(createLongString(maxMessageBytes, "a")))

	batches, err := jaegertranslator.InternalTracesToJaegerProto(td)
	require.NoError(t, err)

	batches[0].Spans[0].Process = batches[0].Process
	jaegerProtoBytes0, err := batches[0].Spans[0].Marshal()
	messageKey := []byte(batches[0].Spans[0].TraceID.String())
	require.NoError(t, err)
	require.NotNil(t, jaegerProtoBytes0)

	jsonByteBuffer0 := new(bytes.Buffer)
	require.NoError(t, jsonMarshaler.Marshal(jsonByteBuffer0, batches[0].Spans[0]))

	// expected cured spans should be similar to spans that came in as if they were already cured.
	// batches[0].Spans[1] when cured will be the same as curedBatches[0].Spans[0]
	curedTd := pdata.NewTraces()
	curedRs := curedTd.ResourceSpans().AppendEmpty()
	curedRs.Resource().Attributes().Insert("test-key", pdata.NewAttributeValueString("test-val"))
	curedIls := curedRs.InstrumentationLibrarySpans().AppendEmpty()

	curedSpan := curedIls.Spans().AppendEmpty()
	curedSpan.SetName("bar")
	curedSpan.SetStartTimestamp(pdata.Timestamp(100))
	curedSpan.SetEndTimestamp(pdata.Timestamp(225))
	curedSpan.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	curedSpan.SetSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	curedSpan.Attributes().Insert("tag10", pdata.NewAttributeValueString("tag10-val"))
	curedSpan.Attributes().Insert("big-tag", pdata.NewAttributeValueString(createLongString(maxAttributeValueSize, "a")))
	curedSpan.Attributes().Insert("big-tag.truncated", pdata.NewAttributeValueBool(true))

	curedBatches, err := jaegertranslator.InternalTracesToJaegerProto(curedTd)
	require.NoError(t, err)

	curedBatches[0].Spans[0].Process = curedBatches[0].Process
	curedJaegerProtoBytes1, err := curedBatches[0].Spans[0].Marshal()
	require.NoError(t, err)
	require.NotNil(t, curedJaegerProtoBytes1)

	curedJsonByteBuffer1 := new(bytes.Buffer)
	require.NoError(t, jsonMarshaler.Marshal(curedJsonByteBuffer1, curedBatches[0].Spans[0]))

	tests := []struct {
		unmarshaler TracesMarshaler
		encoding    string
		messages    []*sarama.ProducerMessage
	}{
		{
			unmarshaler: jaegerMarshalerDebug{
				marshaler:             jaegerProtoSpanMarshaler{},
				version:               sarama.V2_0_0_0,
				maxMessageBytes:       maxMessageBytes,
				maxAttributeValueSize: maxAttributeValueSize,
				cureSpans:             true,
			},
			encoding: "jaeger_proto",
			messages: []*sarama.ProducerMessage{
				{Topic: "topic", Value: sarama.ByteEncoder(jaegerProtoBytes0), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(curedJaegerProtoBytes1), Key: sarama.ByteEncoder(messageKey)}},
		},
		{
			unmarshaler: jaegerMarshalerDebug{
				marshaler: jaegerJSONSpanMarshaler{
					pbMarshaler: &jsonpb.Marshaler{},
				},
				version:               sarama.V2_0_0_0,
				maxMessageBytes:       maxMessageBytes,
				maxAttributeValueSize: maxAttributeValueSize,
				cureSpans:             true,
			},
			encoding: "jaeger_json",
			messages: []*sarama.ProducerMessage{
				{Topic: "topic", Value: sarama.ByteEncoder(jsonByteBuffer0.Bytes()), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(curedJsonByteBuffer1.Bytes()), Key: sarama.ByteEncoder(messageKey)},
			},
		},
		{
			unmarshaler: jaegerMarshalerDebug{
				marshaler:             jaegerProtoSpanMarshaler{},
				version:               sarama.V2_0_0_0,
				maxMessageBytes:       maxMessageBytes,
				maxAttributeValueSize: maxAttributeValueSize,
				dumpSpanAttributes:    true, // test setting this to true
				cureSpans:             true,
			},
			encoding: "jaeger_proto",
			messages: []*sarama.ProducerMessage{
				{Topic: "topic", Value: sarama.ByteEncoder(jaegerProtoBytes0), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(curedJaegerProtoBytes1), Key: sarama.ByteEncoder(messageKey)}},
		},
	}
	for _, test := range tests {
		t.Run(test.encoding, func(t *testing.T) {
			messages, err := test.unmarshaler.Marshal(td, "topic")
			require.NoError(t, err)
			assert.Equal(t, test.messages, messages)
			assert.Equal(t, test.encoding, test.unmarshaler.Encoding())
		})
	}
}

func createLongString(n int, s string) string {
	var b strings.Builder
	b.Grow(n * len(s))
	for i := 0; i < n; i++ {
		b.WriteString(s)
	}
	return b.String()
}

func createLongByteArray(n int) []byte {
	arr := make([]byte, n)
	for i := 0; i < n; i++ {
		arr[i] = byte(i % 10)
	}
	return arr
}
