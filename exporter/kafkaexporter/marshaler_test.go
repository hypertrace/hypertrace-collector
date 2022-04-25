// Copyright  The OpenTelemetry Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kafkaexporter

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	semconv "go.opentelemetry.io/collector/model/semconv/v1.6.1"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestDefaultTracesMarshalers(t *testing.T) {
	expectedEncodings := []string{
		"otlp_proto",
		"otlp_json",
		"jaeger_proto",
		"jaeger_json",
	}
	marshalers := tracesMarshalers()
	assert.Equal(t, len(expectedEncodings), len(marshalers))
	for _, e := range expectedEncodings {
		t.Run(e, func(t *testing.T) {
			m, ok := marshalers[e]
			require.True(t, ok)
			assert.NotNil(t, m)
		})
	}
}

func TestDefaultMetricsMarshalers(t *testing.T) {
	expectedEncodings := []string{
		"otlp_proto",
		"otlp_json",
	}
	marshalers := metricsMarshalers()
	assert.Equal(t, len(expectedEncodings), len(marshalers))
	for _, e := range expectedEncodings {
		t.Run(e, func(t *testing.T) {
			m, ok := marshalers[e]
			require.True(t, ok)
			assert.NotNil(t, m)
		})
	}
}

func TestDefaultLogsMarshalers(t *testing.T) {
	expectedEncodings := []string{
		"otlp_proto",
		"otlp_json",
	}
	marshalers := logsMarshalers()
	assert.Equal(t, len(expectedEncodings), len(marshalers))
	for _, e := range expectedEncodings {
		t.Run(e, func(t *testing.T) {
			m, ok := marshalers[e]
			require.True(t, ok)
			assert.NotNil(t, m)
		})
	}
}

func TestOTLPTracesJsonMarshaling(t *testing.T) {
	t.Parallel()

	now := time.Unix(1, 0)

	traces := ptrace.NewTraces()
	traces.ResourceSpans().AppendEmpty()

	rs := traces.ResourceSpans().At(0)
	rs.SetSchemaUrl(semconv.SchemaURL)
	rs.ScopeSpans().AppendEmpty()

	ils := rs.ScopeSpans().At(0)
	ils.SetSchemaUrl(semconv.SchemaURL)
	ils.Spans().AppendEmpty()

	span := ils.Spans().At(0)
	span.SetKind(ptrace.SpanKindInternal)
	span.SetName(t.Name())
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(time.Second)))
	span.SetSpanID(pcommon.NewSpanID([8]byte{0, 1, 2, 3, 4, 5, 6, 7}))
	span.SetParentSpanID(pcommon.NewSpanID([8]byte{8, 9, 10, 11, 12, 13, 14}))

	marshaler, ok := tracesMarshalers()["otlp_json"]
	require.True(t, ok, "Must have otlp json marshaller")

	msg, err := marshaler.Marshal(traces, t.Name())
	require.NoError(t, err, "Must have marshaled the data without error")
	require.Len(t, msg, 1, "Must have one entry in the message")

	data, err := msg[0].Value.Encode()
	require.NoError(t, err, "Must not error when encoding value")
	require.NotNil(t, data, "Must have valid data to test")

	// Since marshaling json is not guaranteed to be in order
	// within a string, using a map to compare that the expected values are there
	expectedJSON := map[string]interface{}{
		"resourceSpans": []interface{}{
			map[string]interface{}{
				"resource": map[string]interface{}{},
				"scopeSpans": []interface{}{
					map[string]interface{}{
						"scope": map[string]interface{}{},
						"spans": []interface{}{
							map[string]interface{}{
								"traceId":           "",
								"spanId":            "0001020304050607",
								"parentSpanId":      "08090a0b0c0d0e00",
								"name":              t.Name(),
								"kind":              ptrace.SpanKindInternal.String(),
								"startTimeUnixNano": fmt.Sprint(now.UnixNano()),
								"endTimeUnixNano":   fmt.Sprint(now.Add(time.Second).UnixNano()),
								"status":            map[string]interface{}{},
							},
						},
						"schemaUrl": semconv.SchemaURL,
					},
				},
				"schemaUrl": semconv.SchemaURL,
			},
		},
	}

	var final map[string]interface{}
	err = json.Unmarshal(data, &final)
	require.NoError(t, err, "Must not error marshaling expected data")

	assert.Equal(t, expectedJSON, final, "Must match the expected value")
}
