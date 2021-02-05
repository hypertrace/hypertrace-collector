package processors

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	"testing"
)

var spanID = [8]byte{0, 1, 2}

func TestFromContext(t *testing.T) {
	ctx := context.Background()
	ctx, parsedTracesData := FromContext(ctx)
	require.NotNil(t, parsedTracesData)
	ctx2, parsedTracesData2 := FromContext(ctx)
	assert.Equal(t, ctx, ctx2)
	assert.Equal(t, parsedTracesData, parsedTracesData2)

	spanData := parsedTracesData.GetParsedSpanData(pdata.NewSpanID(spanID))
	attr := spanData.GetAttribute("foo")
	require.NotNil(t, attr)
}
