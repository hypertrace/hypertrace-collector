package piifilterprocessor

import (
	"context"

	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
)

var _ processorhelper.TProcessor = (*piiFilterProcessor)(nil)

type piiFilterProcessor struct {
	next    consumer.TracesConsumer
	logger  *zap.Logger
	filters []filters.Filter
}

func newPIIFilterProcessor(logger *zap.Logger, next consumer.TracesConsumer) *piiFilterProcessor {
	return &piiFilterProcessor{
		next:   next,
		logger: logger,
	}
}

func (p *piiFilterProcessor) ProcessTraces(ctx context.Context, td pdata.Traces) (pdata.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		if rs.IsNil() {
			continue
		}

		ilss := rs.InstrumentationLibrarySpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			if ils.IsNil() {
				continue
			}
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if span.IsNil() {
					// Do not create empty spans just to add attributes
					continue
				}

				span.Attributes().ForEach(func(key string, value pdata.AttributeValue) {
					for _, filter := range p.filters {
						if _, err := filter.RedactAttribute(key, value); err != nil {
							p.logger.Sugar().Errorf("failed to filter attributes: %v", err)
						}
					}
				})
			}
		}
	}

	return td, nil
}
