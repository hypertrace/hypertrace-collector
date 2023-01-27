package testutil

import (
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func NewTestTraces(spans ...ptrace.Span) ptrace.Traces {
	t := ptrace.NewTraces()
	rs := ptrace.NewResourceSpans()
	scsp := ptrace.NewScopeSpans()

	for i := 0; i < len(spans); i++ {
		scsp.Spans().AppendEmpty()
	}
	for i, s := range spans {
		s.CopyTo(scsp.Spans().At(i))
	}

	rs.ScopeSpans().AppendEmpty()
	scsp.CopyTo(rs.ScopeSpans().At(0))

	t.ResourceSpans().AppendEmpty()
	rs.CopyTo(t.ResourceSpans().At(0))
	return t
}

// NewTestSpan creates a new span with a set of attributes. This reduces the burden
// of wrapping values continuously inside tests.
func NewTestSpan(attrKVs ...interface{}) ptrace.Span {
	return NewTestSpanWithTraceId(CreateNewTraceId(), attrKVs...)
}

func NewTestSpanWithNameAndSpanKind(spanKind ptrace.SpanKind, name string, attrKVs ...interface{}) ptrace.Span {
	return NewTestSpanWithTraceIdAndNameAndSpanKind(CreateNewTraceId(), CreateNewSpanId(), name, spanKind, attrKVs...)
}

func NewTestSpanWithTraceId(traceId pcommon.TraceID, attrKVs ...interface{}) ptrace.Span {
	return NewTestSpanWithTraceIdAndNameAndSpanKind(traceId, CreateNewSpanId(), "test", ptrace.SpanKindServer, attrKVs...)
}

func NewTestSpanWithTraceIdAndSpanId(traceId pcommon.TraceID, spanId pcommon.SpanID, attrKVs ...interface{}) ptrace.Span {
	return NewTestSpanWithTraceIdAndNameAndSpanKind(traceId, spanId, "test", ptrace.SpanKindServer, attrKVs...)
}

func CreateNewTraceId() pcommon.TraceID {
	traceUuidBytesSlice, _ := uuid.New().MarshalBinary()
	var traceUuidBytes [16]byte
	copy(traceUuidBytes[:], traceUuidBytesSlice)
	return traceUuidBytes
}

func CreateNewSpanId() pcommon.SpanID {
	uuidBytesSlice, _ := uuid.New().MarshalBinary()
	var spanUuidBytes [8]byte
	copy(spanUuidBytes[:], uuidBytesSlice)
	return spanUuidBytes
}

func NewTestSpanWithTraceIdAndNameAndSpanKind(traceId pcommon.TraceID, spanId pcommon.SpanID, name string, spanKind ptrace.SpanKind, attrKVs ...interface{}) ptrace.Span {
	s := ptrace.NewSpan()
	s.SetTraceID(traceId)
	s.SetSpanID(spanId)
	s.SetName(name)
	s.SetKind(spanKind)

	for i := 0; i < len(attrKVs); i = i + 2 {
		var val pcommon.Value
		switch attrKVs[i+1].(type) {
		case string:
			val = pcommon.NewValueString(attrKVs[i+1].(string))
			s.Attributes().PutString(attrKVs[i].(string), val.Str())
		case int, int8, int16, int32, int64:
			val = pcommon.NewValueInt(int64(attrKVs[i+1].(int)))
			s.Attributes().PutInt(attrKVs[i].(string), val.Int())
		case bool:
			val = pcommon.NewValueBool(attrKVs[i+1].(bool))
			s.Attributes().PutBool(attrKVs[i].(string), val.Bool())
		}
	}

	return s
}

func NewAttributeMap() pcommon.Map {
	attributeMap := pcommon.NewMap()
	return attributeMap
}

func NewAttributeMapFromStringMap(m map[string]string) pcommon.Map {
	attributeMap := pcommon.NewMap()
	for k, v := range m {
		attributeMap.PutString(k, v)
	}
	return attributeMap
}
