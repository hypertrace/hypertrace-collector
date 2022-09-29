package kafkaexporter

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/gogo/protobuf/jsonpb"
	jaegerproto "github.com/jaegertracing/jaeger/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

func TestJaegerMarshalerCurer(t *testing.T) {
	maxMessageBytes := 1024
	maxAttributeValueSize := 256
	jsonMarshaler := &jsonpb.Marshaler{}
	ts := pcommon.NewTimestampFromTime(time.Now())

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutString("test-key", "test-val")
	ils := rs.ScopeSpans().AppendEmpty()

	// Will add this span to the messages queue to export
	span := ils.Spans().AppendEmpty()
	span.SetName("foo")
	span.SetStartTimestamp(pcommon.Timestamp(10))
	span.SetEndTimestamp(pcommon.Timestamp(20))
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.Attributes().PutString("tag1", "tag1-val")

	// Will cure this span
	span = ils.Spans().AppendEmpty()
	span.SetName("bar")
	span.SetStartTimestamp(pcommon.Timestamp(100))
	span.SetEndTimestamp(pcommon.Timestamp(225))
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.Attributes().PutString("tag10", "tag10-val")
	span.Attributes().PutString("big-tag", createLongString(maxMessageBytes, "a"))

	// Will be unable to cure this span. Depending on the test config, will drop it or not.
	span = ils.Spans().AppendEmpty()
	span.SetName("buzz")
	span.SetStartTimestamp(pcommon.Timestamp(100))
	span.SetEndTimestamp(pcommon.Timestamp(225))
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.Attributes().PutString("tag10", "tag10-val")
	for i := 0; i < 64; i++ {
		span.Attributes().PutString(fmt.Sprintf("big-tag-%d", i), createLongString(maxMessageBytes, "a"))
	}
	span.Attributes().PutString("big-tag", createLongString(maxMessageBytes, "a"))

	// Will cure this span by curing the span logs
	span = ils.Spans().AppendEmpty()
	span.SetName("bar")
	span.SetStartTimestamp(pcommon.Timestamp(101))
	span.SetEndTimestamp(pcommon.Timestamp(226))
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.Attributes().PutString("tag10", "tag10-val")
	span.Attributes().PutString("tag11", "tag11-val")
	// Add events to span
	for i := 0; i < 128; i++ {
		se := span.Events().AppendEmpty()
		se.SetName(createLongString(1, "a"))
		se.SetTimestamp(ts)
	}

	// Will be unable to cure this span
	span = ils.Spans().AppendEmpty()
	span.SetName("bar")
	span.SetStartTimestamp(pcommon.Timestamp(102))
	span.SetEndTimestamp(pcommon.Timestamp(227))
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.Attributes().PutString("tag10", "tag10-val")
	span.Attributes().PutString("tag11", "tag11-val")
	// Add events to span
	for i := 0; i < 1024; i++ {
		se := span.Events().AppendEmpty()
		se.SetName(createLongString(1, "a"))
		se.SetTimestamp(ts)
	}

	batches, err := jaeger.ProtoFromTraces(td)
	require.NoError(t, err)

	batches[0].Spans[0].Process = batches[0].Process
	jaegerProtoBytes0, err := batches[0].Spans[0].Marshal()
	messageKey := []byte(batches[0].Spans[0].TraceID.String())
	require.NoError(t, err)
	require.NotNil(t, jaegerProtoBytes0)

	jsonByteBuffer0 := new(bytes.Buffer)
	require.NoError(t, jsonMarshaler.Marshal(jsonByteBuffer0, batches[0].Spans[0]))

	// Get the marshalled bytes of the 3rd span that cannot be cured. Will be the needed for expected value of the tests
	// depending on whether dropSpans is turned on or not.
	batches[0].Spans[2].Process = batches[0].Process
	jaegerProtoBytes2, err := batches[0].Spans[2].Marshal()
	require.NoError(t, err)
	require.NotNil(t, jaegerProtoBytes2)

	jsonByteBuffer2 := new(bytes.Buffer)
	require.NoError(t, jsonMarshaler.Marshal(jsonByteBuffer2, batches[0].Spans[2]))

	// Get the marshalled bytes of the 5th span that cannot be cured. Will be the needed for expected value of the tests
	// depending on whether dropSpans is turned on or not.
	batches[0].Spans[4].Process = batches[0].Process
	jaegerProtoBytes4, err := batches[0].Spans[4].Marshal()
	require.NoError(t, err)
	require.NotNil(t, jaegerProtoBytes4)

	jsonByteBuffer4 := new(bytes.Buffer)
	require.NoError(t, jsonMarshaler.Marshal(jsonByteBuffer4, batches[0].Spans[4]))

	// expected cured spans should be similar to spans that came in as if they were already cured.
	// batches[0].Spans[1] when cured will be the same as curedBatches[0].Spans[0] except for the
	// cured attribute.
	curedTd := ptrace.NewTraces()
	curedRs := curedTd.ResourceSpans().AppendEmpty()
	curedRs.Resource().Attributes().PutString("test-key", "test-val")
	curedIls := curedRs.ScopeSpans().AppendEmpty()

	curedSpan := curedIls.Spans().AppendEmpty()
	curedSpan.SetName("bar")
	curedSpan.SetStartTimestamp(pcommon.Timestamp(100))
	curedSpan.SetEndTimestamp(pcommon.Timestamp(225))
	curedSpan.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	curedSpan.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	curedSpan.Attributes().PutString("tag10", "tag10-val")
	curedSpan.Attributes().PutString("big-tag", createLongString(maxAttributeValueSize, "a"))
	curedSpan.Attributes().PutBool("big-tag"+truncationTagSuffix, true)

	// batches[0].Spans[3] when cured will be the same as curedBatches[0].Spans[1] except for the truncated log events.
	curedSpan = curedIls.Spans().AppendEmpty()
	curedSpan.SetName("bar")
	curedSpan.SetStartTimestamp(pcommon.Timestamp(101))
	curedSpan.SetEndTimestamp(pcommon.Timestamp(226))
	curedSpan.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	curedSpan.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	curedSpan.Attributes().PutString("tag10", "tag10-val")
	curedSpan.Attributes().PutString("tag11", "tag11-val")
	curedSpan.Attributes().PutBool(spanLogsTruncationTagName, true)
	// Add events to span
	for i := 0; i < 16; i++ {
		se := curedSpan.Events().AppendEmpty()
		se.SetName(createLongString(1, "a"))
		se.SetTimestamp(ts)
	}

	// For the jaegerJSONSpanMarshaler{ pbMarshaler: &jsonpb.Marshaler{}}, the marshaled log events
	// are large even for small log messages. So still the same as batches[0].Spans[3] except for the
	// truncated log events.
	curedSpan = curedIls.Spans().AppendEmpty()
	curedSpan.SetName("bar")
	curedSpan.SetStartTimestamp(pcommon.Timestamp(101))
	curedSpan.SetEndTimestamp(pcommon.Timestamp(226))
	curedSpan.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	curedSpan.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	curedSpan.Attributes().PutString("tag10", "tag10-val")
	curedSpan.Attributes().PutString("tag11", "tag11-val")
	curedSpan.Attributes().PutBool(spanLogsTruncationTagName, true)
	// Add events to span
	for i := 0; i < 4; i++ {
		se := curedSpan.Events().AppendEmpty()
		se.SetName(createLongString(1, "a"))
		se.SetTimestamp(ts)
	}

	curedBatches, err := jaeger.ProtoFromTraces(curedTd)
	require.NoError(t, err)

	curedBatches[0].Spans[0].Process = curedBatches[0].Process
	curedJaegerProtoBytes1, err := curedBatches[0].Spans[0].Marshal()
	require.NoError(t, err)
	require.NotNil(t, curedJaegerProtoBytes1)

	curedJsonByteBuffer1 := new(bytes.Buffer)
	require.NoError(t, jsonMarshaler.Marshal(curedJsonByteBuffer1, curedBatches[0].Spans[0]))

	curedBatches[0].Spans[1].Process = curedBatches[0].Process
	curedJaegerProtoBytes2, err := curedBatches[0].Spans[1].Marshal()
	require.NoError(t, err)
	require.NotNil(t, curedJaegerProtoBytes2)

	curedJsonByteBuffer2 := new(bytes.Buffer)
	require.NoError(t, jsonMarshaler.Marshal(curedJsonByteBuffer2, curedBatches[0].Spans[1]))

	curedBatches[0].Spans[2].Process = curedBatches[0].Process
	curedJaegerProtoBytes3, err := curedBatches[0].Spans[2].Marshal()
	require.NoError(t, err)
	require.NotNil(t, curedJaegerProtoBytes3)

	curedJsonByteBuffer3 := new(bytes.Buffer)
	require.NoError(t, jsonMarshaler.Marshal(curedJsonByteBuffer3, curedBatches[0].Spans[2]))

	tests := []struct {
		unmarshaler TracesMarshaler
		encoding    string
		messages    []*sarama.ProducerMessage
	}{
		{
			unmarshaler: jaegerMarshalerCurer{
				marshaler:             jaegerProtoSpanMarshaler{},
				version:               sarama.V2_0_0_0,
				maxMessageBytes:       maxMessageBytes,
				maxAttributeValueSize: maxAttributeValueSize,
			},
			encoding: "jaeger_proto",
			messages: []*sarama.ProducerMessage{
				{Topic: "topic", Value: sarama.ByteEncoder(jaegerProtoBytes0), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(curedJaegerProtoBytes1), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(jaegerProtoBytes2), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(curedJaegerProtoBytes2), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(jaegerProtoBytes4), Key: sarama.ByteEncoder(messageKey)},
			},
		},
		{
			unmarshaler: jaegerMarshalerCurer{
				marshaler: jaegerJSONSpanMarshaler{
					pbMarshaler: &jsonpb.Marshaler{},
				},
				version:               sarama.V2_0_0_0,
				maxMessageBytes:       maxMessageBytes,
				maxAttributeValueSize: maxAttributeValueSize,
			},
			encoding: "jaeger_json",
			messages: []*sarama.ProducerMessage{
				{Topic: "topic", Value: sarama.ByteEncoder(jsonByteBuffer0.Bytes()), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(curedJsonByteBuffer1.Bytes()), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(jsonByteBuffer2.Bytes()), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(curedJsonByteBuffer3.Bytes()), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(jsonByteBuffer4.Bytes()), Key: sarama.ByteEncoder(messageKey)},
			},
		},
		{
			unmarshaler: jaegerMarshalerCurer{
				marshaler:             jaegerProtoSpanMarshaler{},
				version:               sarama.V2_0_0_0,
				maxMessageBytes:       maxMessageBytes,
				maxAttributeValueSize: maxAttributeValueSize,
				dumpSpanAttributes:    true, // test setting this to true
			},
			encoding: "jaeger_proto",
			messages: []*sarama.ProducerMessage{
				{Topic: "topic", Value: sarama.ByteEncoder(jaegerProtoBytes0), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(curedJaegerProtoBytes1), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(jaegerProtoBytes2), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(curedJaegerProtoBytes2), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(jaegerProtoBytes4), Key: sarama.ByteEncoder(messageKey)},
			},
		},
		{
			unmarshaler: jaegerMarshalerCurer{
				marshaler:             jaegerProtoSpanMarshaler{},
				version:               sarama.V2_0_0_0,
				maxMessageBytes:       maxMessageBytes,
				maxAttributeValueSize: maxAttributeValueSize,
				dropSpans:             true, // Test that the 3rd span is dropped since it cannot be cured
			},
			encoding: "jaeger_proto",
			messages: []*sarama.ProducerMessage{
				{Topic: "topic", Value: sarama.ByteEncoder(jaegerProtoBytes0), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(curedJaegerProtoBytes1), Key: sarama.ByteEncoder(messageKey)},
				{Topic: "topic", Value: sarama.ByteEncoder(curedJaegerProtoBytes2), Key: sarama.ByteEncoder(messageKey)},
			},
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
					{Key: "tag-1", VType: jaegerproto.ValueType_STRING, VStr: "simple"},
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
					{Key: "tag-1", VType: jaegerproto.ValueType_STRING, VStr: "simple"},
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
					{Key: "tag-1", VType: jaegerproto.ValueType_STRING, VStr: "simple"},
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
					{Key: "tag-1", VType: jaegerproto.ValueType_STRING, VStr: "simple"},
					{Key: "tag-2", VType: jaegerproto.ValueType_STRING, VStr: createLongString(128, "fo")},
					{Key: "tag-3", VType: jaegerproto.ValueType_BOOL, VBool: true},
					{Key: "tag-4", VType: jaegerproto.ValueType_BINARY, VBinary: createLongByteArray(256)},
					{Key: "tag-5", VType: jaegerproto.ValueType_STRING, VStr: createLongString(128, "ba")},
					{Key: "tag-4" + truncationTagSuffix, VType: jaegerproto.ValueType_BOOL, VBool: true},
					{Key: "tag-5" + truncationTagSuffix, VType: jaegerproto.ValueType_BOOL, VBool: true},
				},
			},
		},
		{
			inputSpan: &jaegerproto.Span{
				TraceID: jaegerproto.TraceID{Low: 102, High: 2002},
				SpanID:  125,
				Tags: []jaegerproto.KeyValue{
					{Key: "tag-1", VType: jaegerproto.ValueType_STRING, VStr: "simple"},
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
					{Key: "tag-1", VType: jaegerproto.ValueType_STRING, VStr: "simple"},
					{Key: "tag-2", VType: jaegerproto.ValueType_STRING, VStr: createLongString(64, "fo")},
					{Key: "tag-3", VType: jaegerproto.ValueType_INT64, VInt64: 68},
					{Key: "tag-4", VType: jaegerproto.ValueType_BINARY, VBinary: createLongByteArray(128)},
					{Key: "tag-5", VType: jaegerproto.ValueType_STRING, VStr: createLongString(64, "ba")},
					{Key: "tag-6", VType: jaegerproto.ValueType_STRING, VStr: createLongString(64, "wx")},
					{Key: "tag-4" + truncationTagSuffix, VType: jaegerproto.ValueType_BOOL, VBool: true},
					{Key: "tag-5" + truncationTagSuffix, VType: jaegerproto.ValueType_BOOL, VBool: true},
					{Key: "tag-6" + truncationTagSuffix, VType: jaegerproto.ValueType_BOOL, VBool: true},
					{Key: "tag-2" + truncationTagSuffix, VType: jaegerproto.ValueType_BOOL, VBool: true},
				},
			},
		},
		{
			inputSpan: &jaegerproto.Span{
				TraceID: jaegerproto.TraceID{Low: 103, High: 2003},
				SpanID:  126,
				Tags: []jaegerproto.KeyValue{
					{Key: "tag-1", VType: jaegerproto.ValueType_STRING, VStr: "simple"},
					{Key: "tag-2", VType: jaegerproto.ValueType_STRING, VStr: createLongString(1050, "fo")},
					{Key: "tag-3", VType: jaegerproto.ValueType_BOOL, VBool: false},
					{Key: "tag-4", VType: jaegerproto.ValueType_BINARY, VBinary: createLongByteArray(32)},
					{Key: "tag-5", VType: jaegerproto.ValueType_STRING, VStr: createLongString(10, "ba")},
					{Key: "tag-6", VType: jaegerproto.ValueType_STRING, VStr: createLongString(10, "wx")},
				},
			},
			expectedSpan: &jaegerproto.Span{
				TraceID: jaegerproto.TraceID{Low: 103, High: 2003},
				SpanID:  126,
				Tags: []jaegerproto.KeyValue{
					{Key: "tag-1", VType: jaegerproto.ValueType_STRING, VStr: "simple"},
					{Key: "tag-2", VType: jaegerproto.ValueType_STRING, VStr: createLongString(128, "fo")},
					{Key: "tag-3", VType: jaegerproto.ValueType_BOOL, VBool: false},
					{Key: "tag-4", VType: jaegerproto.ValueType_BINARY, VBinary: createLongByteArray(32)},
					{Key: "tag-5", VType: jaegerproto.ValueType_STRING, VStr: createLongString(10, "ba")},
					{Key: "tag-6", VType: jaegerproto.ValueType_STRING, VStr: createLongString(10, "wx")},
					{Key: "tag-2" + truncationTagSuffix, VType: jaegerproto.ValueType_BOOL, VBool: true},
				},
			},
		},
	}

	marshaler := jaegerProtoSpanMarshaler{}

	j := jaegerMarshalerCurer{
		marshaler:             jaegerProtoSpanMarshaler{},
		version:               sarama.V2_0_0_0,
		maxMessageBytes:       maxMessageBytes,
		maxAttributeValueSize: maxAttributeValueSize,
	}

	for _, test := range tests {
		// Sanity test logging the string
		fmt.Printf("span: %s\n", j.spanAsString(test.inputSpan))
		msg, err := j.cureSpan(test.inputSpan, "test-topic")
		require.NoError(t, err)

		expectedMsgBytes, err := marshaler.marshal(test.expectedSpan)
		require.NoError(t, err)
		expectedMsgBytesEncoder := sarama.ByteEncoder(expectedMsgBytes)
		assert.Equal(t, expectedMsgBytesEncoder, msg.Value)
	}
}

func TestJaegerMarshalerCurerCureSpansFail(t *testing.T) {
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

	j := jaegerMarshalerCurer{
		marshaler:             jaegerProtoSpanMarshaler{},
		version:               sarama.V2_0_0_0,
		maxMessageBytes:       maxMessageBytes,
		maxAttributeValueSize: maxAttributeValueSize,
	}

	msg, err := j.cureSpan(span, "test-topic-2")
	require.Error(t, err)
	assert.Nil(t, msg)
}

func TestCutSpanLogsByHalf(t *testing.T) {
	now := time.Now()
	jpl1 := jaegerproto.Log{Timestamp: now}
	jpl2 := jaegerproto.Log{Timestamp: now.Add(time.Minute * 2)}
	jpl3 := jaegerproto.Log{Timestamp: now.Add(time.Minute * 4)}
	jpl4 := jaegerproto.Log{Timestamp: now.Add(time.Minute * 6)}
	jpl5 := jaegerproto.Log{Timestamp: now.Add(time.Minute * 8)}
	jpl6 := jaegerproto.Log{Timestamp: now.Add(time.Minute * 10)}
	assert.Equal(t,
		[]jaegerproto.Log{jpl1},
		cutSpanLogsByHalf([]jaegerproto.Log{jpl1}))
	assert.Equal(t,
		[]jaegerproto.Log{jpl1},
		cutSpanLogsByHalf([]jaegerproto.Log{jpl1, jpl2}))
	assert.Equal(t,
		[]jaegerproto.Log{jpl1, jpl3},
		cutSpanLogsByHalf([]jaegerproto.Log{jpl1, jpl2, jpl3}))
	assert.Equal(t,
		[]jaegerproto.Log{jpl1, jpl3},
		cutSpanLogsByHalf([]jaegerproto.Log{jpl1, jpl2, jpl3, jpl4}))
	assert.Equal(t,
		[]jaegerproto.Log{jpl1, jpl3, jpl5},
		cutSpanLogsByHalf([]jaegerproto.Log{jpl1, jpl2, jpl3, jpl4, jpl5}))
	assert.Equal(t,
		[]jaegerproto.Log{jpl1, jpl3, jpl5},
		cutSpanLogsByHalf([]jaegerproto.Log{jpl1, jpl2, jpl3, jpl4, jpl5, jpl6}))
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
