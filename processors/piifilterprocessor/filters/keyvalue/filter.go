package keyvalue

import (
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

func (f *keyValueFilter) RedactAttribute(key string, value pdata.AttributeValue) (bool, error) {
	if len(value.StringVal()) == 0 {
		return false, nil
	}

	truncatedKey := f.m.GetTruncatedKey(key)
	if isRedacted, redactedValue := f.m.FilterKeyRegexs(truncatedKey, key, value.StringVal(), ""); isRedacted {
		value.SetStringVal(redactedValue)
		return true, nil
	}

	if isRedacted, redactedValue := f.m.FilterStringValueRegexs(value.StringVal(), key, ""); isRedacted {
		value.SetStringVal(redactedValue)
		return true, nil
	}

	return false, nil
}
