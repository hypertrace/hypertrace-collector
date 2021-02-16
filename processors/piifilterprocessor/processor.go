package piifilterprocessor

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"strings"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"

	"github.com/hypertrace/collector/processors"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/cookie"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/json"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/keyvalue"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/sql"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/urlencoded"
	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
)

var _ processorhelper.TProcessor = (*piiFilterProcessor)(nil)

type piiFilterProcessor struct {
	logger                *zap.Logger
	scalarFilters         []filters.Filter
	structuredDataFilters map[string]filters.Filter
	structuredData        map[string]PiiComplexData
}

func toRegex(es []PiiElement, globalStrategy redaction.Strategy) []regexmatcher.Regex {
	var rs []regexmatcher.Regex

	for _, e := range es {
		rd := redaction.DefaultRedactor
		if globalStrategy != redaction.Unknown {
			rd = redaction.Redactors[globalStrategy]
		}

		if e.RedactStrategy != redaction.Unknown {
			rd = redaction.Redactors[e.RedactStrategy]
		}

		rs = append(rs, regexmatcher.Regex{
			Regexp:            e.Regex,
			Redactor:          rd,
			FQN:               e.FQN,
			SessionIdentifier: e.SessionIdentifier,
		})
	}

	return rs
}

func newPIIFilterProcessor(logger *zap.Logger, cfg *Config) (*piiFilterProcessor, error) {
	matcher, err := regexmatcher.NewMatcher(
		cfg.Prefixes,
		toRegex(cfg.KeyRegExs, cfg.RedactStrategy),
		toRegex(cfg.ValueRegExs, cfg.RedactStrategy),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create regex matcher: %v", err)
	}

	var scalarFilters = []filters.Filter{
		keyvalue.NewFilter(matcher),
	}

	var structuredDataFilters = map[string]filters.Filter{
		"cookie":     cookie.NewFilter(matcher),
		"urlencoded": urlencoded.NewFilter(matcher),
		"json":       json.NewFilter(matcher, logger),
		"sql":        sql.NewFilter(redaction.Redactors[cfg.RedactStrategy]),
	}

	var complexData = map[string]PiiComplexData{}
	for _, e := range cfg.ComplexData {
		complexData[e.Key] = e
	}

	return &piiFilterProcessor{
		logger:                logger,
		scalarFilters:         scalarFilters,
		structuredDataFilters: structuredDataFilters,
		structuredData:        complexData,
	}, nil
}

func (p *piiFilterProcessor) ProcessTraces(ctx context.Context, td pdata.Traces) (pdata.Traces, error) {
	rss := td.ResourceSpans()

	ctxWithData, parsedTraceData := processors.FromContext(ctx)
	ctx = ctxWithData

	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)

		ilss := rs.InstrumentationLibrarySpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				parsedSpanData := parsedTraceData.GetParsedSpanData(span.SpanID())

				span.Attributes().ForEach(func(key string, value pdata.AttributeValue) {
					if p.attributeKeyContainsComplexData(key) {
						// if attribute contains complex data skip the processing
						return
					}

					parsedAttr, newAttr := p.processMatchingAttributes(key, value)
					if parsedAttr != nil {
						parsedSpanData.PutParsedAttribute(key, parsedAttr)
					}
					if newAttr != nil {
						span.Attributes().Insert(newAttr.Key, newAttr.Value)
					}
				})

				p.processComplexData(span)
			}
		}
	}

	return td, nil
}

func getDataTypeFromContentType(dataType string) (string, error) {
	mt, _, err := mime.ParseMediaType(dataType)
	if err != nil {
		return "", err
	}

	lcDataType := mt
	switch lcDataType {
	case "json", "text/json", "text/x-json", "application/json":
		lcDataType = "json"
	case "application/x-www-form-urlencoded":
		lcDataType = "urlencoded"
	case "sql":
		lcDataType = "sql"
	default:
	}

	return lcDataType, nil
}

func (p *piiFilterProcessor) processMatchingAttributes(key string, value pdata.AttributeValue) (*processors.ParsedAttribute, *filters.Attribute) {
	for _, filter := range p.scalarFilters {
		parsedAttribute, newAtt, err := filter.RedactAttribute(key, value)
		if err != nil {
			if errors.Is(err, filters.ErrUnprocessableValue) {
				p.logger.Sugar().Debugf(
					"failed to apply filter %q to attribute with key %q. Unsuitable value.",
					filter.Name(),
					key,
				)
			} else {
				p.logger.Sugar().Errorf(
					"failed to apply filter %q to attribute with key %q", filter.Name(), key, err,
				)
			}
		} else if parsedAttribute != nil && len(parsedAttribute.Redacted) > 0 {
			// if an attribute is redacted by one filter we don't want to process
			// it again.
			p.logger.Sugar().Debugf("attribute with key %q redacted by filter %q", key, filter.Name())
			return parsedAttribute, newAtt
		}
	}
	return nil, nil
}

// http.request.body = {"authorization": {"b": "c"} }
// http.request.body.authorization.b = c -> redacted http.request.body.authorization = ***

func (p *piiFilterProcessor) processComplexData(span pdata.Span) {
	for attrKey, elem := range p.structuredData {
		attr, found := span.Attributes().Get(attrKey)
		if !found {
			continue
		}

		if attr.StringVal() == "" {
			p.logger.Sugar().Debug("empty string attribute with key %q", attrKey)
			continue
		}

		var dataType = elem.Type
		if len(dataType) == 0 {
			if typeValue, ok := span.Attributes().Get(elem.TypeKey); ok {
				var err error
				dataType, err = getDataTypeFromContentType(typeValue.StringVal())
				if err != nil {
					p.logger.Sugar().Debugf("could not parse media type %q: %v", typeValue.StringVal(), err)
					continue
				}
			}
		}

		filter, ok := p.structuredDataFilters[dataType]
		if !ok {
			p.logger.Sugar().Debugf("unknown data type %s", dataType)
			continue
		}

		if parsedAttr, newAttr, err := filter.RedactAttribute(elem.Key, attr); len(parsedAttr.Redacted) > 0 {
			p.logger.Sugar().Debugf("attribute with key %q redacted by filter %q", attrKey, filter.Name())
		} else if err != nil {
			p.logger.Sugar().Errorf(
				"failed to apply filter %q to attribute with key %q: %v",
				filter.Name(),
				attrKey,
				err,
			)
		} else if newAttr != nil {
			span.Attributes().Insert(newAttr.Key, newAttr.Value)
		}
	}
}

func (p *piiFilterProcessor) attributeKeyContainsComplexData(key string) bool {
	_, ok := p.structuredData[unindexedKey(key)]
	return ok
}

func unindexedKey(key string) string {
	if len(key) == 0 {
		return key
	}

	return strings.Split(key, "[")[0]
}
