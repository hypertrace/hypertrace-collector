package tenantidprocessor

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"strings"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"google.golang.org/grpc/metadata"
)

type processor struct {
	logger *zap.Logger
	tenantIDHeaderName string
	tenantIDAttributeKey string
}

var _ processorhelper.TProcessor = (*processor)(nil)

// ProcessTraces implements processorhelper.TProcessor
func (p processor) ProcessTraces(ctx context.Context, traces pdata.Traces) (pdata.Traces, error) {
	fmt.Println("received")
	fmt.Println(traces.SpanCount())
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		p.logger.Warn("Could not extract headers from context, tenantid will not be added to spans")
		return traces, nil
	}

	tenantIDHeaders := md.Get(p.tenantIDHeaderName)
	if len(tenantIDHeaders) == 0 {
		return traces, nil
	} else if len(tenantIDHeaders) > 0{
		p.logger.Warn("Multiple tenant IDs provided, only the first one will be used",
			zap.String("header-name", p.tenantIDHeaderName), zap.String("header-value", strings.Join(tenantIDHeaders,",")))
	}

	tenantIdHeaderValue := tenantIDHeaders[0]
	p.addTenantIdToSpans(traces, tenantIdHeaderValue)
	return traces, nil
}

func (p processor) addTenantIdToSpans(traces pdata.Traces, tenantIDHeaderValue string) {
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
