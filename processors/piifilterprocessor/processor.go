package piifilterprocessor

import (
	"context"
	"fmt"

	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/cookie"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/json"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/urlencoded"

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

func toRegex(es []PiiElement) []regexmatcher.Regex {
	var rs []regexmatcher.Regex

	for _, e := range es {
		rs = append(rs, regexmatcher.Regex{
			Pattern:        e.Regex,
			RedactStrategy: e.RedactStrategy,
			FQN:            e.FQN,
		})
	}

	return rs
}

func newPIIFilterProcessor(
	logger *zap.Logger,
	next consumer.TracesConsumer,
	cfg *Config,
) (*piiFilterProcessor, error) {
	matcher, err := regexmatcher.NewMatcher(
		toRegex(cfg.KeyRegExs),
		toRegex(cfg.ValueRegExs),
		cfg.RedactStrategy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create regex matcher: %v", err)
	}

	var fs = []filters.Filter{
		cookie.NewFilter(matcher),
		urlencoded.NewFilter(matcher),
		json.NewFilter(matcher),
	}

	return &piiFilterProcessor{
		next:    next,
		logger:  logger,
		filters: fs,
	}, nil
}

func (p *piiFilterProcessor) ProcessTraces(_ context.Context, td pdata.Traces) (pdata.Traces, error) {
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
							p.logger.Sugar().Errorf("failed to apply filter %q to attribute with key %q: %v", filter.Name(), key, err)
						}
					}
				})
			}
		}
	}

	return td, nil
}
