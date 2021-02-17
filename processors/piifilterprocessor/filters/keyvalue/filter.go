package keyvalue

import (
	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/hypertrace/collector/processors"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"
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

func (f *keyValueFilter) RedactAttribute(key string, value pdata.AttributeValue) (*processors.ParsedAttribute, *filters.Attribute, error) {
	if len(value.StringVal()) == 0 {
		return nil, nil, nil
	}

	truncatedKey := f.m.GetTruncatedKey(key)
	if isRedacted, isSession, redactedValue := f.m.FilterKeyRegexs(truncatedKey, key, value.StringVal(), ""); isRedacted {
		var newAttr *filters.Attribute
		if isSession {
			newAttr = &filters.Attribute{
				Key:   "session.id",
				Value: redactedValue,
			}
		}
		attr := &processors.ParsedAttribute{
			Redacted: map[string]string{key: value.StringVal()},
		}
		value.SetStringVal(redactedValue)
		return attr, newAttr, nil
	}

	if isRedacted, isSession, redactedValue := f.m.FilterStringValueRegexs(value.StringVal()); isRedacted {
		var newAttr *filters.Attribute
		if isSession {
			newAttr = &filters.Attribute{
				Key:   "session.id",
				Value: redactedValue,
			}
		}
		attr := &processors.ParsedAttribute{
			Redacted: map[string]string{key: value.StringVal()},
		}
		value.SetStringVal(redactedValue)
		return attr, newAttr, nil
	}

	return nil, nil, nil
}
