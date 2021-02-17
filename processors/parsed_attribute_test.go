package processors

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	"testing"
)

func TestFromContext(t *testing.T) {
	ctx := context.Background()
	ctx, parsedTracesData := FromContext(ctx)
	require.NotNil(t, parsedTracesData)
	ctx2, parsedTracesData2 := FromContext(ctx)
	assert.Equal(t, ctx, ctx2)
	assert.Equal(t, parsedTracesData, parsedTracesData2)

	span := pdata.NewSpan()
	spanData := parsedTracesData.GetParsedSpanData(span)
	attr := spanData.GetAttribute("foo")
	require.NotNil(t, attr)
	attr.Flattened["a"] = "b"

	spanData2 := parsedTracesData.GetParsedSpanData(pdata.NewSpan())
	attr2 := spanData2.GetAttribute("foo")
	assert.Equal(t, &ParsedAttribute{
		Flattened: map[string]string{},
		Redacted: map[string]string{},
	}, attr2)

	assert.Equal(t, &ParsedAttribute{
		Flattened: map[string]string{"a": "b"},
		Redacted: map[string]string{},
	}, parsedTracesData.GetParsedSpanData(span).GetAttribute("foo"))
}
