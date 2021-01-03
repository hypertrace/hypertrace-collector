package piifilterprocessor_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"testing"

	"github.com/hypertrace/collector/processors/piifilterprocessor"
	"github.com/stretchr/testify/assert"

	"github.com/hypertrace/collector/processors/piifilterprocessor"
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

func TestConsumeTraceData(t *testing.T) {
	logger := zap.New(zapcore.NewNopCore())

	testCases := map[string]struct {
		config         piifilterprocessor.Config
		inputTraces    pdata.Traces
		expectedTraces pdata.Traces
	}{
		"no filter is applied": {
			config: piifilterprocessor.Config{
				KeyRegExs: []piifilterprocessor.PiiElement{
					{
						Regex: "^password$",
					},
				},
			},
			inputTraces:    newTraces(newTestSpan("tag1", "abc123")),
			expectedTraces: newTraces(newTestSpan("tag1", "abc123")),
		},
	}

	for name, testValues := range testCases {
		t.Run(name, func(t *testing.T) {
			sinkExporter := &consumertest.TracesSink{}
			config := testValues.config

			tp, err := piifilterprocessor.NewFactory().CreateTracesProcessor(
				context.Background(),
				component.ProcessorCreateParams{
					Logger: logger,
				},
				&config,
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
							assert.Equal(t, expectedValue, actualValue)
						})
					}
				}
			}
		})
	}
}
