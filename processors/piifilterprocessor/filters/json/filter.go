package json

import (
	"fmt"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/hypertrace/collector/processors"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/internal/json"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"
)

var _ filters.Filter = (*jsonFilter)(nil)

type jsonFilter struct {
	logger *zap.Logger
	m      *regexmatcher.Matcher
	mu     json.MarshalUnmarshaler
}

// NewFilter creates a JSON filter to be used
func NewFilter(m *regexmatcher.Matcher, logger *zap.Logger) filters.Filter {
	return &jsonFilter{logger, m, jsoniter.ConfigDefault}
}

func (f *jsonFilter) Name() string {
	return "JSON"
}

const jsonPathPrefix = "$"

func (f *jsonFilter) RedactAttribute(key string, value pdata.AttributeValue) (*processors.ParsedAttribute, error) {
	if len(value.StringVal()) == 0 {
		return nil, nil
	}

	var jsonPayload interface{}

	err := f.mu.UnmarshalFromString(value.StringVal(), &jsonPayload)
	if err != nil {
		// if json is invalid, run the value filter on the json string to try and
		// filter out any keywords out of the string
		f.logger.Debug("Problem parsing json. Falling back to value regex filtering")

		if isRedacted, redactedValue := f.m.FilterStringValueRegexs(value.StringVal(), key, ""); isRedacted {
			attr := &processors.ParsedAttribute{
				Redacted: map[string]string{key: value.StringVal()},
			}
			value.SetStringVal(redactedValue)
			return attr, nil
		}

		return nil, filters.WrapError(filters.ErrUnprocessableValue, err.Error())
	}

	parsedAttr := &processors.ParsedAttribute{
		Redacted:  map[string]string{},
		Flattened: map[string]string{},
	}
	isRedacted, redactedValue := f.filterJSON(parsedAttr, jsonPayload, nil, "", key, jsonPathPrefix, false)
	if !isRedacted {
		return parsedAttr, nil
	}

	redactedValueAsString, err := f.mu.MarshalToString(redactedValue)
	if err != nil {
		return nil, err
	}

	value.SetStringVal(redactedValueAsString)

	return parsedAttr, nil
}

func (f *jsonFilter) filterJSON(
	parsedAttr *processors.ParsedAttribute,
	value interface{},
	matchedRegex *regexmatcher.Regex,
	key string,
	actualKey string,
	jsonPath string,
	checked bool) (bool, interface{}) {
	switch tValue := value.(type) {
	case []interface{}:
		return f.filterJSONArray(parsedAttr, tValue, matchedRegex, key, actualKey, jsonPath, checked)
	case map[string]interface{}:
		return f.filterJSONMap(parsedAttr, tValue, matchedRegex, key, actualKey, jsonPath, checked)
	default:
		return f.filterJSONScalar(parsedAttr, tValue, matchedRegex, key, actualKey, jsonPath, checked)
	}
}

func (f *jsonFilter) filterJSONArray(
	parsedAttr *processors.ParsedAttribute,
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

		modified, redacted := f.filterJSON(parsedAttr, v, matchedPiiElem, key, actualKey, tempJSONPath, true)
		if modified {
			arrValue[i] = redacted
		}
		filtered = modified || filtered
	}

	return filtered, arrValue
}

func (f *jsonFilter) filterJSONMap(
	parsedAttr *processors.ParsedAttribute,
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
		modified, redacted := f.filterJSON(parsedAttr, value, matchedPiiElem, key, actualKey, mapJSONPath, true)
		if modified {
			mValue[key] = redacted
		}
		filtered = modified || filtered
	}

	return filtered, mValue
}

func (f *jsonFilter) filterJSONScalar(
	parsedAttr *processors.ParsedAttribute,
	value interface{},
	matchedRegex *regexmatcher.Regex,
	key string,
	actualKey string,
	jsonPath string,
	checked bool,
) (bool, interface{}) {
	fqn := fmt.Sprintf("%s%s", actualKey, jsonPath)
	parsedAttr.Flattened[jsonPath] = fmt.Sprintf("%v", value)

	if matchedRegex == nil && !checked {
		_, matchedRegex = f.m.MatchKeyRegexs(key, jsonPath)
	}

	switch tt := value.(type) {
	case string:
		if matchedRegex != nil {
			parsedAttr.Redacted[fqn] = tt
			return true, f.m.FilterMatchedKey(matchedRegex.Redactor, actualKey, tt, jsonPath)
		}
		stringValueFiltered, vvFiltered := f.m.FilterStringValueRegexs(tt, actualKey, jsonPath)
		if stringValueFiltered {
			parsedAttr.Redacted[fqn] = tt
			return true, vvFiltered
		}
	case interface{}:
		if matchedRegex != nil {
			str := fmt.Sprintf("%v", tt)
			parsedAttr.Redacted[fqn] = str
			return true, f.m.FilterMatchedKey(matchedRegex.Redactor, actualKey, str, jsonPath)
		}
	}

	return false, value
}
