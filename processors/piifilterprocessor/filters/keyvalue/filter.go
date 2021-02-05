package keyvalue

import (
	"github.com/hypertrace/collector/processors"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"

	"go.opentelemetry.io/collector/consumer/pdata"
)

type keyValueFilter struct {
	m *regexmatcher.Matcher
}

func NewFilter(m *regexmatcher.Matcher) filters.Filter {
	return &keyValueFilter{m}
}

func (f *keyValueFilter) Name() string {
	return "key-value"
}

func (f *keyValueFilter) RedactAttribute(key string, value pdata.AttributeValue) (*processors.ParsedAttribute, error) {
	if len(value.StringVal()) == 0 {
		return nil, nil
	}

	truncatedKey := f.m.GetTruncatedKey(key)
	if isRedacted, isSession, redactedValue := f.m.FilterKeyRegexs(truncatedKey, key, value.StringVal(), ""); isRedacted {
		if isSession {
			// TODO
		}
		attr := &processors.ParsedAttribute{
			Redacted: map[string]string{key: value.StringVal()},
		}
		value.SetStringVal(redactedValue)
		return attr, nil
	}

	if isRedacted, redactedValue := f.m.FilterStringValueRegexs(value.StringVal(), key, ""); isRedacted {
		attr := &processors.ParsedAttribute{
			Redacted: map[string]string{key: value.StringVal()},
		}
		value.SetStringVal(redactedValue)
		return attr, nil
	}

	return nil, nil
}
