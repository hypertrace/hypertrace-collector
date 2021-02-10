package piifilterprocessor

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"strings"

	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/cookie"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/json"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/keyvalue"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/sql"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/urlencoded"
	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
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
			Regexp:   e.Regex,
			Redactor: rd,
			FQN:      e.FQN,
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

func (p *piiFilterProcessor) ProcessTraces(_ context.Context, td pdata.Traces) (pdata.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)

		ilss := rs.InstrumentationLibrarySpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)

				span.Attributes().ForEach(func(key string, value pdata.AttributeValue) {
					if p.attributeKeyContainsComplexData(key) {
						// if attribute contains complex data skip the processing
						return
					}

					p.processMatchingAttributes(key, value)
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

func (p *piiFilterProcessor) processMatchingAttributes(key string, value pdata.AttributeValue) {
	for _, filter := range p.scalarFilters {
		if isRedacted, err := filter.RedactAttribute(key, value); err != nil {
			if !errors.Is(err, filters.ErrUnprocessableValue) {
				p.logger.Error(
					"failed to apply filter",
					zap.String("attribute_key", key),
					zap.String("pii_filter_name", filter.Name()),
					zap.Error(err),
				)
			}
		} else if isRedacted {
			// if an attribute is redacted by one filter we don't want to process
			// it again.
			p.logger.Debug(
				"attribute redacted",
				zap.String("attribute_key", key),
				zap.String("pii_filter_name", filter.Name()),
			)
			break
		}
	}
}

func (p *piiFilterProcessor) processComplexData(attrs pdata.Span) {
	for attrKey, elem := range p.structuredData {
		attr, found := attrs.Attributes().Get(attrKey)
		if !found {
			continue
		}

		if attr.StringVal() == "" {
			p.logger.Debug(
				"empty string attribute",
				zap.String("attribute_key", attrKey),
			)
			continue
		}

		var dataType = elem.Type
		if len(dataType) == 0 {
			if typeValue, ok := attrs.Attributes().Get(elem.TypeKey); ok {
				var err error
				dataType, err = getDataTypeFromContentType(typeValue.StringVal())
				if err != nil {
					p.logger.Debug(
						fmt.Sprintf("could not parse media type %q", typeValue.StringVal()),
						zap.Error(err),
					)
					continue
				}
			}
		}

		filter, ok := p.structuredDataFilters[dataType]
		if !ok {
			p.logger.Sugar().Errorf("unknown data type %s", dataType)
			continue
		}

		if isRedacted, err := filter.RedactAttribute(elem.Key, attr); isRedacted {
			p.logger.Debug(
				"attribute redacted",
				zap.String("attribute_key", attrKey),
				zap.String("pii_filter_name", filter.Name()),
			)
		} else if err != nil {
			p.logger.Error(
				"failed to apply filter",
				zap.String("attribute_key", attrKey),
				zap.String("pii_filter_name", filter.Name()),
				zap.Error(err),
			)
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
