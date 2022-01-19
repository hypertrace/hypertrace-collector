package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

// Similar to jaegerMarshaler except we log details of spans greater than producer maxMessageBytes. When doing otel
// upgrades pull in updates from jaeger_mashaler.go:Marshal function
import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/Shopify/sarama"
	jaegerproto "github.com/jaegertracing/jaeger/model"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"

	jaegertranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

const (
	maximumRecordOverhead   = 5*binary.MaxVarintLen32 + binary.MaxVarintLen64 + 1
	producerMessageOverhead = 26 // the metadata overhead of CRC, flags, etc.
)

type jaegerMarshalerDebug struct {
	marshaler       jaegerSpanMarshaler
	version         sarama.KafkaVersion
	maxMessageBytes int
}

var _ TracesMarshaler = (*jaegerMarshaler)(nil)

func (j jaegerMarshalerDebug) Marshal(traces pdata.Traces, topic string) ([]*sarama.ProducerMessage, error) {
	batches, err := jaegertranslator.InternalTracesToJaegerProto(traces)
	if err != nil {
		return nil, err
	}
	var messages []*sarama.ProducerMessage

	var errs error
	for _, batch := range batches {
		for _, span := range batch.Spans {
			span.Process = batch.Process
			bts, err := j.marshaler.marshal(span)
			// continue to process spans that can be serialized
			if err != nil {
				errs = multierr.Append(errs, err)
				continue
			}
			key := []byte(span.TraceID.String())
			msg := &sarama.ProducerMessage{
				Topic: topic,
				Value: sarama.ByteEncoder(bts),
				Key:   sarama.ByteEncoder(key),
			}
			// Computed the same way as in https://github.com/Shopify/sarama/blob/a060ecaa8887587485754af088bd8a521f6d55e9/async_producer.go#L233
			messageSize := byteSize(msg, j.version)
			if messageSize > j.maxMessageBytes {
				// Log span info for a span that exceeds the max message size
				// We log instead of throwing an error since the caller for this tracesPusher() will return an error and not even
				// send those messages that didn't exceed the max message size.
				log.Printf("span exceeds max message size: %d vs %d. span%s\n", messageSize, j.maxMessageBytes, spanAsString(span))
			}
			messages = append(messages, msg)
		}
	}
	return messages, errs
}

func (j jaegerMarshalerDebug) Encoding() string {
	return j.marshaler.encoding()
}

func spanAsString(span *jaegerproto.Span) string {
	var sb strings.Builder

	sb.WriteString("{")
	sb.WriteString(fmt.Sprintf("trace_id: %s, ", span.TraceID.String()))
	sb.WriteString(fmt.Sprintf("span_id: %s, ", span.SpanID.String()))
	sb.WriteString(fmt.Sprintf("name: %s, ", span.OperationName))
	sb.WriteString(fmt.Sprintf("start_time: %s, ", span.StartTime.String()))
	sb.WriteString(fmt.Sprintf("duration: %s, ", span.Duration.String()))
	if span.Process == nil {
		sb.WriteString("process: nil")
	} else {
		sb.WriteString("process: {")
		sb.WriteString(fmt.Sprintf("service_name: %s, ", span.Process.ServiceName))
		sb.WriteString("tags: [")
		for _, kv := range span.Process.Tags {
			sb.WriteString("{")
			sb.WriteString(fmt.Sprintf("key: %s, ", kv.Key))
			sb.WriteString(fmt.Sprintf("value: %s", valueToString(kv)))
			sb.WriteString("},")
		}
		sb.WriteString("]")
		sb.WriteString("}")
	}
	sb.WriteString("}")

	return sb.String()
}

func valueToString(kv jaegerproto.KeyValue) string {
	if kv.VType == jaegerproto.ValueType_STRING {
		return kv.GetVStr()
	} else if kv.VType == jaegerproto.ValueType_BOOL {
		return strconv.FormatBool(kv.GetVBool())
	} else if kv.VType == jaegerproto.ValueType_INT64 {
		return strconv.FormatInt(kv.GetVInt64(), 10)
	} else if kv.VType == jaegerproto.ValueType_FLOAT64 {
		return fmt.Sprintf("%f", kv.GetVFloat64())
	} else if kv.VType == jaegerproto.ValueType_BINARY {
		return hex.EncodeToString(kv.GetVBinary())
	} else {
		return ""
	}
}

func byteSize(m *sarama.ProducerMessage, v sarama.KafkaVersion) int {
	var size int
	if v.IsAtLeast(sarama.V0_11_0_0) {
		size = maximumRecordOverhead
		for _, h := range m.Headers {
			size += len(h.Key) + len(h.Value) + 2*binary.MaxVarintLen32
		}
	} else {
		size = producerMessageOverhead
	}
	if m.Key != nil {
		size += m.Key.Length()
	}
	if m.Value != nil {
		size += m.Value.Length()
	}
	return size
}
