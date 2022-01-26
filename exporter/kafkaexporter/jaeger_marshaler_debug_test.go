package kafkaexporter

import (
	"bytes"
	//"fmt"
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

	// Create a string whose size is maxMessageBytes
	// var b strings.Builder
	// b.Grow(maxMessageBytes)
	// for i := 0; i < maxMessageBytes; i++ {
	// 	b.WriteString("a")
	// }
	// s := b.String()

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
	// td := pdata.NewTraces()
	// rs := td.ResourceSpans().AppendEmpty()
	// rs.Resource().Attributes().Insert("test-key", pdata.NewAttributeValueString("test-val"))
	// ils := rs.InstrumentationLibrarySpans().AppendEmpty()

	// // Will add this span to the messages queue to export
	// span := ils.Spans().AppendEmpty()
	// span.SetName("foo")
	// span.SetStartTimestamp(pdata.Timestamp(10))
	// span.SetEndTimestamp(pdata.Timestamp(20))
	// span.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	// span.SetSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	// span.Attributes().Insert("tag1", pdata.NewAttributeValueString("tag1-val"))

	// // Create a string whose size is maxMessageBytes
	// var b strings.Builder
	// b.Grow(maxMessageBytes)
	// for i := 0; i < maxMessageBytes; i++ {
	// 	b.WriteString("a")
	// }
	// s := b.String()

	// // Will log on this span that exceeds max message size.
	// span = ils.Spans().AppendEmpty()
	// span.SetName("bar")
	// span.SetStartTimestamp(pdata.Timestamp(100))
	// span.SetEndTimestamp(pdata.Timestamp(225))
	// span.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	// span.SetSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	// span.Attributes().Insert("tag10", pdata.NewAttributeValueString("tag10-val"))
	// span.Attributes().Insert("big-tag", pdata.NewAttributeValueString(s))

	jaegerSpan := &jaegerproto.Span{
		TraceID: jaegerproto.TraceID{Low: 100, High: 2000},
		SpanID:  123,
		Tags: []jaegerproto.KeyValue{
			{Key: "tag-1", VType: jaegerproto.ValueType_STRING, VStr: "simple string"},
			{Key: "tag-2", VType: jaegerproto.ValueType_STRING, VStr: createLongString(128, "fo")},
			{Key: "tag-3", VType: jaegerproto.ValueType_BOOL, VBool: true},
			{Key: "tag-4", VType: jaegerproto.ValueType_BINARY, VBinary: createLongByteArray(500)},
			{Key: "tag-5", VType: jaegerproto.ValueType_STRING, VStr: createLongString(256, "ba")},
		},
	}

	expectedJaegerSpan := &jaegerproto.Span{
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
	}

	marshaler := jaegerProtoSpanMarshaler{}
	expectedMsgBytes, err := marshaler.marshal(expectedJaegerSpan)
	require.NoError(t, err)
	expectedMsgBytesEncoder := sarama.ByteEncoder(expectedMsgBytes)

	j := jaegerMarshalerDebug{
		marshaler:             marshaler,
		version:               sarama.V2_0_0_0,
		maxMessageBytes:       maxMessageBytes,
		maxAttributeValueSize: maxAttributeValueSize,
		cureSpans:             true,
	}

	msg, err := j.cureSpan(jaegerSpan, "test-topic")
	require.NoError(t, err)
	assert.Equal(t, expectedMsgBytesEncoder, msg.Value)

	// batches, err := jaegertranslator.InternalTracesToJaegerProto(td)
	// require.NoError(t, err)

	// //jsonMarshaler := &jsonpb.Marshaler{}

	// batches[0].Spans[0].Process = batches[0].Process
	// jaegerProtoBytes0, err := batches[0].Spans[0].Marshal()
	// messageKey := []byte(batches[0].Spans[0].TraceID.String())
	// require.NoError(t, err)
	// require.NotNil(t, jaegerProtoBytes0)

	// batches[0].Spans[1].Process = batches[0].Process
	// // jaegerProtoBytes1, err := batches[0].Spans[1].Marshal()
	// // require.NoError(t, err)
	// // require.NotNil(t, jaegerProtoBytes1)

	// j := jaegerMarshalerDebug{
	// 	marshaler:             jaegerProtoSpanMarshaler{},
	// 	version:               sarama.V2_0_0_0,
	// 	maxMessageBytes:       maxMessageBytes,
	// 	maxAttributeValueSize: maxAttributeValueSize,
	// 	dumpSpanAttributes:    true,
	// }

	// fmt.Printf("span 0: %s\n", j.spanAsString(batches[0].Spans[0]))
	// fmt.Printf("span 1: %s\n", j.spanAsString(batches[0].Spans[1]))
	// fmt.Println("curing")
	// curedSpan0, err := j.cureSpan(batches[0].Spans[0], "foo")
	// require.NoError(t, err)
	// curedSpan1, err := j.cureSpan(batches[0].Spans[1], "foo")
	// require.NoError(t, err)

	// fmt.Printf("cured span 0: %s\n", j.spanAsString(curedSpan0))
	// fmt.Printf("cured span 1: %s\n", j.spanAsString(curedSpan1))
	// fmt.Println("done")

	// tests := []struct {
	// 	unmarshaler TracesMarshaler
	// 	encoding    string
	// 	messages    []*sarama.ProducerMessage
	// }{
	// 	{
	// 		unmarshaler: jaegerMarshalerDebug{
	// 			marshaler:       jaegerProtoSpanMarshaler{},
	// 			version:         sarama.V2_0_0_0,
	// 			maxMessageBytes: maxMessageBytes,
	// 		},
	// 		encoding: "jaeger_proto",
	// 		messages: []*sarama.ProducerMessage{
	// 			{Topic: "topic", Value: sarama.ByteEncoder(jaegerProtoBytes0), Key: sarama.ByteEncoder(messageKey)},
	// 			{Topic: "topic", Value: sarama.ByteEncoder(jaegerProtoBytes1), Key: sarama.ByteEncoder(messageKey)}},
	// 	},
	// 	{}
	// }
}

func TestJaegerMarshalerDebugCureSpans(t *testing.T) {
	maxMessageBytes := 1024
	maxAttributeValueSize := 256
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

	// Create a string whose size is maxMessageBytes
	// var b strings.Builder
	// b.Grow(maxMessageBytes)
	// for i := 0; i < maxMessageBytes; i++ {
	// 	b.WriteString("a")
	// }
	// s := b.String()

	// Will log on this span that exceeds max message size.
	span = ils.Spans().AppendEmpty()
	span.SetName("bar")
	span.SetStartTimestamp(pdata.Timestamp(100))
	span.SetEndTimestamp(pdata.Timestamp(225))
	span.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.Attributes().Insert("tag10", pdata.NewAttributeValueString("tag10-val"))
	span.Attributes().Insert("big-tag", pdata.NewAttributeValueString(createLongString(maxMessageBytes, "a")))

	// cured spans
	curedTd := pdata.NewTraces()
	curedRs := curedTd.ResourceSpans().AppendEmpty()
	curedRs.Resource().Attributes().Insert("test-key", pdata.NewAttributeValueString("test-val"))
	curedIls := curedRs.InstrumentationLibrarySpans().AppendEmpty()

	// Create a string whose size is maxAttributeValueSize
	// var b1 strings.Builder
	// b1.Grow(maxAttributeValueSize)
	// for i := 0; i < maxAttributeValueSize; i++ {
	// 	b1.WriteString("a")
	// }
	//s1 := b1.String()

	curedSpan := curedIls.Spans().AppendEmpty()
	curedSpan.SetName("bar")
	curedSpan.SetStartTimestamp(pdata.Timestamp(100))
	curedSpan.SetEndTimestamp(pdata.Timestamp(225))
	curedSpan.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	curedSpan.SetSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	curedSpan.Attributes().Insert("tag10", pdata.NewAttributeValueString("tag10-val"))
	curedSpan.Attributes().Insert("big-tag", pdata.NewAttributeValueString(createLongString(maxAttributeValueSize, "a")))
	curedSpan.Attributes().Insert("big-tag.truncated", pdata.NewAttributeValueBool(true))

	batches, err := jaegertranslator.InternalTracesToJaegerProto(td)
	require.NoError(t, err)

	curedBatches, err := jaegertranslator.InternalTracesToJaegerProto(curedTd)
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
	arr := make([]byte, n, n)
	for i := 0; i < n; i++ {
		arr[i] = byte(i % 10)
	}
	return arr
}
