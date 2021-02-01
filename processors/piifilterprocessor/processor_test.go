package piifilterprocessor_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/hypertrace/collector/processors/piifilterprocessor"
	"github.com/stretchr/testify/assert"

	stdjson "encoding/json"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// newTestSpan creates a new span with a set of attributes. This reduces the burden
// of wrapping values continuously inside tests.
func newTestSpan(attrKVs ...interface{}) pdata.Span {
	s := pdata.NewSpan()
	s.SetName("test")

	for i := 0; i < len(attrKVs); i = i + 2 {
		var val pdata.AttributeValue
		switch attrKVs[i+1].(type) {
		case string:
			val = pdata.NewAttributeValueString(attrKVs[i+1].(string))
		case int, int8, int16, int32, int64:
			val = pdata.NewAttributeValueInt(int64(attrKVs[i+1].(int)))
		}

		s.Attributes().Insert(attrKVs[i].(string), val)
	}

	return s
}

func newTraces(spans ...pdata.Span) pdata.Traces {
	traces := pdata.NewTraces()

	rss := pdata.NewResourceSpans()

	ilss := pdata.NewInstrumentationLibrarySpans()

	for _, s := range spans {
		ilss.Spans().Append(s)
	}

	rss.InstrumentationLibrarySpans().Append(ilss)
	traces.ResourceSpans().Append(rss)
	return traces
}

const (
	keyJSONInput = `{"a":"aaa","password":"root_pw","b":{"b_1":"bbb","password":"nested_pw"},` +
		`"c":[{"c_1":"ccc"},{"password":"array_pw"}]}`

	keyJSONExpected = `{"a":"aaa","password":"***","b":{"b_1":"bbb","password":"***"},` +
		`"c":[{"c_1":"ccc"},{"password":"***"}]}`

	valueJSONInput = `{"key_or_value":{"a":"aaa","b":"key_or_value"}}`

	valueJSONExpected = `{"key_or_value":{"a":"aaa","b":"***"}}`
)

func TestConsumeTraceData(t *testing.T) {
	logger := zap.New(zapcore.NewNopCore())

	testCases := map[string]struct {
		config         piifilterprocessor.TransportConfig
		inputTraces    pdata.Traces
		expectedTraces pdata.Traces
	}{
		"no filter is applied": {
			config: piifilterprocessor.TransportConfig{
				KeyRegExs: []piifilterprocessor.TransportPiiElement{
					{
						RegexPattern: "^password$",
					},
				},
				RedactStrategyName: "redact",
			},
			inputTraces:    newTraces(newTestSpan("tag1", "abc123")),
			expectedTraces: newTraces(newTestSpan("tag1", "abc123")),
		},
		"auth bearer hash": {
			config: piifilterprocessor.TransportConfig{
				KeyRegExs: []piifilterprocessor.TransportPiiElement{
					{RegexPattern: "http.request.header.authorization$"},
				},
				RedactStrategyName: "hash",
			},
			inputTraces: newTraces(newTestSpan("http.request.header.authorization", "Bearer abc123")),
			expectedTraces: newTraces(newTestSpan(
				"http.request.header.authorization", "1232de241a44c348f44bfba95206afe9c6e90718",
			)),
		},
		"JSON key filter": {
			config: piifilterprocessor.TransportConfig{
				KeyRegExs: []piifilterprocessor.TransportPiiElement{
					{RegexPattern: "^password$"},
				},
				RedactStrategyName: "redact",
				ComplexData: []piifilterprocessor.TransportPiiComplexData{
					{
						Key:     "http.request.body",
						TypeKey: "http.request.headers.content-type",
					},
				},
			},
			inputTraces: newTraces(newTestSpan(
				"http.request.body", keyJSONInput,
				"http.request.headers.content-type", "application/json;charset=utf-8",
			)),
			expectedTraces: newTraces(newTestSpan(
				"http.request.body", keyJSONExpected,
				"http.request.headers.content-type", "application/json;charset=utf-8",
			)),
		},
		"multiple attributes": {
			config: piifilterprocessor.TransportConfig{
				KeyRegExs: []piifilterprocessor.TransportPiiElement{
					{RegexPattern: "^password$"},
					{RegexPattern: "^auth-key$"},
				},
				RedactStrategyName: "redact",
				ComplexData: []piifilterprocessor.TransportPiiComplexData{
					{
						Key:  "http.request.body",
						Type: "json",
					},
				},
			},
			inputTraces: newTraces(newTestSpan(
				"http.request.body", keyJSONInput,
				"auth-key", "some-auth-key",
			)),
			expectedTraces: newTraces(newTestSpan(
				"http.request.body", keyJSONExpected,
				"auth-key", "***",
			)),
		},
		"JSON value filter": {
			config: piifilterprocessor.TransportConfig{
				ValueRegExs: []piifilterprocessor.TransportPiiElement{
					{RegexPattern: "key_or_value"},
				},
				RedactStrategyName: "redact",
				ComplexData: []piifilterprocessor.TransportPiiComplexData{
					{
						Key:  "http.request.body",
						Type: "json",
					},
				},
			},
			inputTraces: newTraces(newTestSpan(
				"http.request.body", valueJSONInput,
			)),
			expectedTraces: newTraces(newTestSpan(
				"http.request.body", valueJSONExpected,
			)),
		},
	}

	for name, testValues := range testCases {
		t.Run(name, func(t *testing.T) {
			sinkExporter := &consumertest.TracesSink{}

			tp, err := piifilterprocessor.NewFactory().CreateTracesProcessor(
				context.Background(),
				component.ProcessorCreateParams{
					Logger: logger,
				},
				&testValues.config,
				sinkExporter,
			)
			assert.NoError(t, err)

			err = tp.ConsumeTraces(context.Background(), testValues.inputTraces)
			assert.NoError(t, err)

			td := sinkExporter.AllTraces()[0]

			arss := td.ResourceSpans()
			erss := testValues.expectedTraces.ResourceSpans()
			for i := 0; i < arss.Len(); i++ {
				ars := arss.At(i)
				ers := erss.At(i)

				ailss := ars.InstrumentationLibrarySpans()
				eilss := ers.InstrumentationLibrarySpans()

				for j := 0; j < ailss.Len(); j++ {
					actualSpans := ailss.At(j).Spans()
					expectedSpans := eilss.At(j).Spans()
					for k := 0; k < actualSpans.Len(); k++ {
						actualSpan := actualSpans.At(k)
						expectedSpan := expectedSpans.At(k)

						assert.Equal(t, actualSpan.Attributes().Len(), expectedSpan.Attributes().Len())

						expectedSpan.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
							expectedValue, _ := expectedSpan.Attributes().Get(k)
							actualValue, ok := actualSpan.Attributes().Get(k)

							assert.True(t, ok)

							// JSON serialization doesn't produce the same order for fields all
							// the time, hence comparing strings will be flaky. This check attempts
							// detect JSON payloads to do a proper comparison.
							if isJSONPayload(expectedValue.StringVal()) {
								assertJSONEqual(t, expectedValue.StringVal(), actualValue.StringVal())
							} else {
								assert.Equal(t, expectedValue, actualValue)
							}
						})
					}
				}
			}
		})
	}
}

func isJSONPayload(s string) bool {
	err := json.Unmarshal([]byte(s), &struct{}{})
	return err == nil
}

// assertJSONEqual asserts two JSONs are equal no matter the
func assertJSONEqual(t *testing.T, expected, actual string) {
	var jExpected, jActual interface{}
	if err := stdjson.Unmarshal([]byte(expected), &jExpected); err != nil {
		t.Error(err)
	}
	if err := stdjson.Unmarshal([]byte(actual), &jActual); err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(jExpected, jActual) {
		msgExpected, _ := stdjson.Marshal(jExpected)
		msgActual, _ := stdjson.Marshal(jActual)
		assert.Equal(t, string(msgExpected), string(msgActual))
	}
	assert.True(t, true)
}
