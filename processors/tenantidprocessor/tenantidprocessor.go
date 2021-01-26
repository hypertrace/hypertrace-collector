package tenantidprocessor

import (
	"context"
	"fmt"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"strings"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

type processor struct {
	tenantIDHeaderName   string
	tenantIDAttributeKey string
	logger               *zap.Logger
	tenantIDViews        map[string]*view.View
}

var _ processorhelper.TProcessor = (*processor)(nil)

// ProcessTraces implements processorhelper.TProcessor
func (p *processor) ProcessTraces(ctx context.Context, traces pdata.Traces) (pdata.Traces, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		p.logger.Error("Could not extract headers from context", zap.Int("num-spans", traces.SpanCount()))
		return traces, fmt.Errorf("missing header %s", p.tenantIDHeaderName)
	}

	tenantIDHeaders := md.Get(p.tenantIDHeaderName)
	if len(tenantIDHeaders) == 0 {
		return traces, nil
	} else if len(tenantIDHeaders) > 1 {
		p.logger.Warn("Multiple tenant IDs provided, only the first one will be used",
			zap.String("header-name", p.tenantIDHeaderName), zap.String("header-value", strings.Join(tenantIDHeaders, ",")))
	}

	tenantID := tenantIDHeaders[0]
	p.addTenantIdToSpans(traces, tenantID)

	ctx, _ = tag.New(ctx,
		tag.Insert(tagTenantID, tenantID))
	stats.Record(ctx, statSpanPerTenant.M(int64(traces.SpanCount())))

	return traces, nil
}

func (p *processor) addTenantIdToSpans(traces pdata.Traces, tenantIDHeaderValue string) {
	rss := traces.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)

		ilss := rs.InstrumentationLibrarySpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)

			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				span.Attributes().Insert(p.tenantIDAttributeKey, pdata.NewAttributeValueString(tenantIDHeaderValue))
			}
		}
	}
}
