package json

import (
	"fmt"

	jsoniter "github.com/json-iterator/go"

	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/internal/json"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"
	"go.opentelemetry.io/collector/consumer/pdata"
)

var _ filters.Filter = (*jsonFilter)(nil)

type jsonFilter struct {
	m  *regexmatcher.Matcher
	mu json.MarshalUnmarshaler
}

func NewFilter(m *regexmatcher.Matcher) filters.Filter {
	return &jsonFilter{m, jsoniter.ConfigDefault}
}

func (f *jsonFilter) Name() string {
	return "JSON"
}

const jsonPathPrefix = "$"

func (f *jsonFilter) RedactAttribute(key string, value pdata.AttributeValue) (bool, error) {
	if len(value.StringVal()) == 0 {
		return false, nil
	}

	var jsonPayload interface{}

	err := f.mu.UnmarshalFromString(value.StringVal(), &jsonPayload)
	if err != nil {
		return false, filters.WrapError(filters.ErrUnprocessableValue, err.Error())
	}

	isRedacted, redactedValue := f.filterJSON(jsonPayload, nil, "", key, jsonPathPrefix, false)
	if !isRedacted {
		return false, nil
	}

	redactedValueAsString, err := f.mu.MarshalToString(redactedValue)
	if err != nil {
		return false, err
	}

	value.SetStringVal(redactedValueAsString)

	return true, nil
}

func (f *jsonFilter) filterJSON(value interface{}, matchedRegex *regexmatcher.Regex, key string, actualKey string, jsonPath string, checked bool) (bool, interface{}) {
	switch tValue := value.(type) {
	case []interface{}:
		return f.filterJSONArray(tValue, matchedRegex, key, actualKey, jsonPath, checked)
	case map[string]interface{}:
		return f.filterJSONMap(tValue, matchedRegex, key, actualKey, jsonPath, checked)
	default:
		return f.filterJSONScalar(tValue, matchedRegex, key, actualKey, jsonPath, checked)
	}
}

func (f *jsonFilter) filterJSONArray(
	arrValue []interface{},
	matchedRegex *regexmatcher.Regex,
	key string,
	actualKey string,
	jsonPath string,
	_ bool,
) (bool, interface{}) {
	filtered := false
	for i, v := range arrValue {
		tempJSONPath := fmt.Sprintf("%s[%d]", jsonPath, i)

		matchedPiiElem := matchedRegex
		if matchedRegex == nil {
			_, matchedPiiElem = f.m.MatchKeyRegexs(key, tempJSONPath)
		}

		modified, redacted := f.filterJSON(v, matchedPiiElem, key, actualKey, tempJSONPath, true)
		if modified {
			arrValue[i] = redacted
		}
		filtered = modified || filtered
	}

	return filtered, arrValue
}

func (f *jsonFilter) filterJSONMap(
	mValue map[string]interface{},
	matchedRegex *regexmatcher.Regex,
	_ string,
	actualKey string,
	jsonPath string,
	_ bool,
) (bool, interface{}) {
	filtered := false
	for key, value := range mValue {
		mapJSONPath := jsonPath + "." + key

		matchedPiiElem := matchedRegex
		if matchedPiiElem == nil {
			_, matchedPiiElem = f.m.MatchKeyRegexs(key, mapJSONPath)
		}
		modified, redacted := f.filterJSON(value, matchedPiiElem, key, actualKey, mapJSONPath, true)
		if modified {
			mValue[key] = redacted
		}
		filtered = modified || filtered
	}

	return filtered, mValue
}

func (f *jsonFilter) filterJSONScalar(
	value interface{},
	matchedRegex *regexmatcher.Regex,
	key string,
	actualKey string,
	jsonPath string,
	checked bool,
) (bool, interface{}) {
	if matchedRegex == nil && !checked {
		_, matchedRegex = f.m.MatchKeyRegexs(key, jsonPath)
	}

	switch tt := value.(type) {
	case string:
		if matchedRegex != nil {
			_, redacted := f.m.FilterMatchedKey(matchedRegex.Redactor, actualKey, tt, jsonPath)
			return true, redacted
		}
		stringValueFiltered, vvFiltered := f.m.FilterStringValueRegexs(tt, actualKey, jsonPath)
		if stringValueFiltered {
			return true, vvFiltered
		}
	case interface{}:
		if matchedRegex != nil {
			str := fmt.Sprintf("%v", tt)
			isModified, redacted := f.m.FilterMatchedKey(matchedRegex.Redactor, actualKey, str, jsonPath)
			return isModified, redacted
		}
	}

	return false, value
}
