package haproxyverifier

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

var emptyStruct struct{} = struct{}{}
var separator string = "-"

type processor struct {
	sync.Mutex
	cfg    *Config
	logger *zap.Logger
	spanSet map[string]struct{}
}

func newProcessor(logger *zap.Logger, cfg *Config) *processor {
	processor := &processor{
		cfg:     cfg,
		logger:  logger,
		spanSet: make(map[string]struct{}, 1024),
	}
	processor.logSpanSetSize()
	return processor
}

// ProcessTraces implements processorhelper.ProcessTracesFunc
func (p *processor) ProcessTraces(ctx context.Context, traces ptrace.Traces) (ptrace.Traces, error) {

	rss := traces.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			scss := rs.ScopeSpans().At(j)
			for k := 0; k < scss.Spans().Len(); k++ {
				span := scss.Spans().At(k)
				// Both the request and response spans have the same parent id: of the root span that starts the trace.
				if spanMatchesConfig(span, p.cfg.RequestSpan) {
					setItem := fmt.Sprintf("%s%s%s", span.TraceID().HexString(), separator, span.ParentSpanID().HexString())
					p.Lock()
					p.spanSet[setItem] = emptyStruct
					p.Unlock()
					continue
				}

				if spanMatchesConfig(span, p.cfg.ResponseSpan) {
					setItem := fmt.Sprintf("%s%s%s", span.TraceID().HexString(), separator, span.ParentSpanID().HexString())
					p.Lock()
					delete(p.spanSet, setItem)
					p.Unlock()
				}
			}
		}
	}

	return traces, nil
}

func spanMatchesConfig(span ptrace.Span, spanConfig SpanConfig) bool {
	if span.Name() != spanConfig.SpanName {
		return false
	}

	for _, attr := range spanConfig.SpanAttributes {
		v, ok := span.Attributes().Get(attr.Key)
		if !ok {
			return false
		}

		// empty value means we are just checking for the presence of attribute.
		if len(attr.Value) == 0 || attr.Value == v.Str() {
			continue
		} else {
			return false
		}
	}

	return true
}

func (p *processor) logSpanSetSize() {
	go func() {
		for range time.Tick(time.Duration(p.cfg.LogIntervalSeconds) * time.Second) {
			p.Lock()
			setSize := len(p.spanSet)
			p.Unlock()
			p.logger.Info("span set size", zap.Int("size", setSize))
		}
	}()
}
