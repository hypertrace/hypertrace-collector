package enduserprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

func TestCreateDefaultConfig(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, configmodels.Type(typeStr), cfg.Type())
}

func TestCreateProcessor(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.EndUserConfig = append(cfg.EndUserConfig, EndUser{AttributeKey: "http.request.header.x-user", Type: "id"})
	require.NotNil(t, cfg)

	sink := new(consumertest.TracesSink)
	p, err := f.CreateTracesProcessor(context.Background(), component.ProcessorCreateParams{
		Logger: zap.NewNop(),
	}, cfg, sink)
	require.NoError(t, err)
	require.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	defer p.Shutdown(context.Background())

	td := pdata.NewTraces()
	td.ResourceSpans().Resize(1)
	td.ResourceSpans().At(0).InstrumentationLibrarySpans().Resize(1)
	td.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().Resize(1)
	spans := td.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans()
	spans.At(0).SetName("get")
	spans.At(0).Attributes().Insert("http.request.header.x-user", pdata.NewAttributeValueString("kevin"))

	err = p.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)
	traces := sink.AllTraces()
	require.Equal(t, 1, len(traces))
	trace := traces[0]
	require.Equal(t, 1, trace.SpanCount())
	span := trace.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().At(0)
	attr, ok := span.Attributes().Get(enduserIDAttribute)
	require.True(t, ok)
	require.Equal(t, "kevin", attr.StringVal())
}
