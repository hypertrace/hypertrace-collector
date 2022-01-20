package kafkaexporter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/gogo/protobuf/jsonpb"
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
	var b strings.Builder
	b.Grow(maxMessageBytes)
	for i := 0; i < maxMessageBytes; i++ {
		b.WriteString("a")
	}
	s := b.String()

	// Will log on this span that exceeds max message size.
	span = ils.Spans().AppendEmpty()
	span.SetName("bar")
	span.SetStartTimestamp(pdata.Timestamp(100))
	span.SetEndTimestamp(pdata.Timestamp(225))
	span.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.Attributes().Insert("tag10", pdata.NewAttributeValueString("tag10-val"))
	span.Attributes().Insert("big-tag", pdata.NewAttributeValueString(s))

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
