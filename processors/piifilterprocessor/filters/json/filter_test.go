package json

import (
	"errors"
	"reflect"
	"regexp"
	"testing"

	stdjson "encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/hypertrace/collector/processors"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/internal/json"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"
	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
)

// assertJSONEqual asserts two JSONs are equal no matter the
func assertJSONEqual(t *testing.T, expected, actual string) {
	var jExpected, jActual interface{}
	if err := stdjson.Unmarshal([]byte(expected), &jExpected); err != nil {
		t.Error(err)
	}
	if err := stdjson.Unmarshal([]byte(actual), &jActual); err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(jExpected, jActual) {
		msgExpected, _ := stdjson.Marshal(jExpected)
		msgActual, _ := stdjson.Marshal(jActual)
		assert.Equal(t, string(msgExpected), string(msgActual))
	}
	assert.True(t, true)
}

func createJSONFilter(t *testing.T, keyRegExs, valueRegExs []regexmatcher.Regex) *jsonFilter {
	m, err := regexmatcher.NewMatcher(nil, keyRegExs, valueRegExs)

	assert.NoError(t, err)

	return &jsonFilter{m: m, mu: json.DefaultMarshalUnmarshaler, logger: zap.NewNop()}
}

func TestFilterSuccessOnEmptyString(t *testing.T) {
	filter := createJSONFilter(t, []regexmatcher.Regex{}, nil)

	attrValue := pdata.NewAttributeValueString("")
	parsedAttribute, err := filter.RedactAttribute("attrib_key", attrValue)
	assert.Nil(t, parsedAttribute)
	assert.NoError(t, err)
}

func TestFilterFailsOnInvalidJSON(t *testing.T) {
	filter := createJSONFilter(t, []regexmatcher.Regex{}, nil)

	attrValue := pdata.NewAttributeValueString("bob")
	parsedAttribute, err := filter.RedactAttribute("attrib_key", attrValue)
	assert.Nil(t, parsedAttribute)
	assert.Error(t, err)
	assert.Equal(t, filters.ErrUnprocessableValue, errors.Unwrap(err))
}

func TestSimpleArrayRemainsTheSameOnNotMatchingRegex(t *testing.T) {
	filter := createJSONFilter(t, []regexmatcher.Regex{
		{Regexp: regexp.MustCompile("^password$"), Redactor: redaction.RedactRedactor},
	}, nil)
	attrValue := pdata.NewAttributeValueString("[\"12\",\"34\",\"56\"]")
	parsedAttr, err := filter.RedactAttribute("attrib_key", attrValue)
	assert.Equal(t, &processors.ParsedAttribute{
		Flattened: map[string]string{"attrib_key$[0]": "12", "attrib_key$[1]": "34", "attrib_key$[2]": "56"},
		Redacted:  map[string]string{},
	}, parsedAttr)
	assert.NoError(t, err)
	assertJSONEqual(t, "[\"12\",\"34\",\"56\"]", attrValue.StringVal())
}

func TestJSONFieldRedaction(t *testing.T) {
	tCases := map[string]struct {
		unredactedValue           string
		expectedRedactedAttrValue string
		parsedAttribute           *processors.ParsedAttribute
	}{
		"for outer array": {
			unredactedValue:           `[{"a":"1"},{"password":"abc"}]`,
			expectedRedactedAttrValue: `[{"a":"1"},{"password":"***"}]`,
			parsedAttribute: &processors.ParsedAttribute{
				Redacted: map[string]string{
					"attrib_key$[1].password": "abc",
				},
				Flattened: map[string]string{
					"attrib_key$[0].a":        "1",
					"attrib_key$[1].password": "abc",
				},
			},
		},
		"for inner array": {
			unredactedValue:           `{"a": [{"b": "1"}, {"password": "abc"}]}`,
			expectedRedactedAttrValue: `{"a": [{"b": "1"}, {"password": "***"}]}`,
			parsedAttribute: &processors.ParsedAttribute{
				Redacted: map[string]string{
					"attrib_key$.a[1].password": "abc",
				},
				Flattened: map[string]string{
					"attrib_key$.a[0].b":        "1",
					"attrib_key$.a[1].password": "abc",
				},
			},
		},
		"for array in key": {
			unredactedValue:           `{"a": [{"b": "1"}, {"password": ["12","34","56"]}]}`,
			expectedRedactedAttrValue: `{"a": [{"b": "1"}, {"password": ["***","***","***"]}]}`,
			parsedAttribute: &processors.ParsedAttribute{
				Redacted: map[string]string{
					"attrib_key$.a[1].password[0]": "12",
					"attrib_key$.a[1].password[1]": "34",
					"attrib_key$.a[1].password[2]": "56",
				},
				Flattened: map[string]string{
					"attrib_key$.a[0].b":           "1",
					"attrib_key$.a[1].password[0]": "12",
					"attrib_key$.a[1].password[1]": "34",
					"attrib_key$.a[1].password[2]": "56",
				},
			},
		},
		"for object in key": {
			unredactedValue: "{\"a\": [{\"b\": \"1\"}, " +
				"{\"password\":{\"key1\":[\"12\",\"34\",\"56\"], \"key2\":\"val\"}}]}",
			expectedRedactedAttrValue: "{\"a\": [{\"b\": \"1\"}, " +
				"{\"password\": {\"key1\":[\"***\",\"***\",\"***\"], \"key2\":\"***\"}}]}",
			parsedAttribute: &processors.ParsedAttribute{
				Redacted: map[string]string{
					"attrib_key$.a[1].password.key1[0]": "12",
					"attrib_key$.a[1].password.key1[1]": "34",
					"attrib_key$.a[1].password.key1[2]": "56",
					"attrib_key$.a[1].password.key2":    "val",
				},
				Flattened: map[string]string{
					"attrib_key$.a[0].b":                "1",
					"attrib_key$.a[1].password.key1[0]": "12",
					"attrib_key$.a[1].password.key1[1]": "34",
					"attrib_key$.a[1].password.key1[2]": "56",
					"attrib_key$.a[1].password.key2":    "val",
				},
			},
		},
		"for non string scalar": {
			unredactedValue: "{\"a\": [{\"b\": \"1\"}, " +
				"{\"password\":{\"key1\":[12,34.1,true], \"key2\":false}}]}",
			expectedRedactedAttrValue: "{\"a\": [{\"b\": \"1\"}, " +
				"{\"password\": {\"key1\":[\"***\",\"***\",\"***\"], \"key2\":\"***\"}}]}",
			parsedAttribute: &processors.ParsedAttribute{
				Redacted: map[string]string{
					"attrib_key$.a[1].password.key1[0]": "12",
					"attrib_key$.a[1].password.key1[1]": "34.1",
					"attrib_key$.a[1].password.key1[2]": "true",
					"attrib_key$.a[1].password.key2":    "false",
				},
				Flattened: map[string]string{
					"attrib_key$.a[0].b":                "1",
					"attrib_key$.a[1].password.key1[0]": "12",
					"attrib_key$.a[1].password.key1[1]": "34.1",
					"attrib_key$.a[1].password.key1[2]": "true",
					"attrib_key$.a[1].password.key2":    "false",
				},
			},
		},
	}

	for name, tCase := range tCases {
		t.Run(name, func(t *testing.T) {
			filter := createJSONFilter(t, []regexmatcher.Regex{
				{Regexp: regexp.MustCompile("^password$"), Redactor: redaction.RedactRedactor},
			}, nil)

			attrValue := pdata.NewAttributeValueString(tCase.unredactedValue)
			parsedAttribute, err := filter.RedactAttribute("attrib_key", attrValue)
			require.NoError(t, err)
			assert.Equal(t, tCase.parsedAttribute, parsedAttribute)
			assertJSONEqual(t, tCase.expectedRedactedAttrValue, attrValue.StringVal())
		})
	}
}

func TestRedactionOnMatchingValuesByFQN(t *testing.T) {
	tCases := map[string]struct {
		pattern                   string
		unredactedValue           string
		expectedRedactedAttrValue string
		parsedAttribute           *processors.ParsedAttribute
	}{
		"one element in a simple array is redacted": {
			pattern:                   "^\\$\\[1\\]$",
			unredactedValue:           `["12","34","56"]`,
			expectedRedactedAttrValue: `["12","***","56"]`,
			parsedAttribute: &processors.ParsedAttribute{
				Redacted: map[string]string{
					"attrib_key$[1]": "34",
				},
				Flattened: map[string]string{
					"attrib_key$[0]": "12",
					"attrib_key$[1]": "34",
					"attrib_key$[2]": "56",
				},
			},
		},
		"one element in a simple object is redacted": {
			pattern:                   "^\\$\\.password$",
			unredactedValue:           `{"a": "1","password": "abc"}`,
			expectedRedactedAttrValue: `{"a": "1","password": "***"}`,
			parsedAttribute: &processors.ParsedAttribute{
				Redacted: map[string]string{
					"attrib_key$.password": "abc",
				},
				Flattened: map[string]string{
					"attrib_key$.a":        "1",
					"attrib_key$.password": "abc",
				},
			},
		},
		"all elements in a password array": {
			pattern:                   "^\\$\\.a\\[1\\]\\.password$",
			unredactedValue:           `{"a": [{"b": "1"}, {"password": ["12","34","56"]}]}`,
			expectedRedactedAttrValue: `{"a": [{"b": "1"}, {"password": ["***","***","***"]}]}`,
			parsedAttribute: &processors.ParsedAttribute{
				Redacted: map[string]string{
					"attrib_key$.a[1].password[0]": "12",
					"attrib_key$.a[1].password[1]": "34",
					"attrib_key$.a[1].password[2]": "56",
				},
				Flattened: map[string]string{
					"attrib_key$.a[0].b":           "1",
					"attrib_key$.a[1].password[0]": "12",
					"attrib_key$.a[1].password[1]": "34",
					"attrib_key$.a[1].password[2]": "56",
				},
			},
		},
		"one element in a password array": {
			pattern:                   "^\\$\\.a\\[1\\]\\.password\\[1\\]$",
			unredactedValue:           `{"a": [{"b": "1"}, {"password": ["12","34","56"]}]}`,
			expectedRedactedAttrValue: `{"a": [{"b": "1"}, {"password": ["12","***","56"]}]}`,
			parsedAttribute: &processors.ParsedAttribute{
				Redacted: map[string]string{
					"attrib_key$.a[1].password[1]": "34",
				},
				Flattened: map[string]string{
					"attrib_key$.a[0].b":           "1",
					"attrib_key$.a[1].password[0]": "12",
					"attrib_key$.a[1].password[1]": "34",
					"attrib_key$.a[1].password[2]": "56",
				},
			},
		},
		"all elements in a password object": {
			pattern:                   "^\\$\\.a\\[1\\]\\.password.key1$",
			unredactedValue:           `{"a": [{"b": "1"}, {"password":{"key1":[12,34,56], "key2":"val"}}]}`,
			expectedRedactedAttrValue: `{"a": [{"b": "1"}, {"password": {"key1":["***","***","***"], "key2":"val"}}]}`,
			parsedAttribute: &processors.ParsedAttribute{
				Redacted: map[string]string{
					"attrib_key$.a[1].password.key1[0]": "12",
					"attrib_key$.a[1].password.key1[1]": "34",
					"attrib_key$.a[1].password.key1[2]": "56",
				},
				Flattened: map[string]string{
					"attrib_key$.a[0].b":                "1",
					"attrib_key$.a[1].password.key1[0]": "12",
					"attrib_key$.a[1].password.key1[1]": "34",
					"attrib_key$.a[1].password.key1[2]": "56",
					"attrib_key$.a[1].password.key2":    "val",
				},
			},
		},
		"one element in a password object": {
			pattern:                   "^\\$\\.a\\[1\\]\\.password.key1\\[1\\]$",
			unredactedValue:           `{"a": [{"b": "1"}, {"password":{"key1":[12,34,56], "key2":"val"}}]}`,
			expectedRedactedAttrValue: `{"a": [{"b": "1"}, {"password": {"key1":[12,"***",56], "key2":"val"}}]}`,
			parsedAttribute: &processors.ParsedAttribute{
				Redacted: map[string]string{
					"attrib_key$.a[1].password.key1[1]": "34",
				},
				Flattened: map[string]string{
					"attrib_key$.a[0].b":                "1",
					"attrib_key$.a[1].password.key1[0]": "12",
					"attrib_key$.a[1].password.key1[1]": "34",
					"attrib_key$.a[1].password.key1[2]": "56",
					"attrib_key$.a[1].password.key2":    "val",
				},
			},
		},
	}

	for name, tCase := range tCases {
		t.Run(name, func(t *testing.T) {
			filter := createJSONFilter(t, []regexmatcher.Regex{
				{Regexp: regexp.MustCompile(tCase.pattern), FQN: true, Redactor: redaction.RedactRedactor},
			}, nil)
			attrValue := pdata.NewAttributeValueString(tCase.unredactedValue)
			parsedAttribute, err := filter.RedactAttribute("attrib_key", attrValue)
			require.NoError(t, err)
			assert.Equal(t, tCase.parsedAttribute, parsedAttribute)
			assertJSONEqual(t, tCase.expectedRedactedAttrValue, attrValue.StringVal())
		})
	}
}

func TestRedactInvalidJSON(t *testing.T) {
	invalidJSONInput := `{
	"key_or_value":{
		a:"aaa",
		"b":"key_or_value"
		},
	}`

	invalidJSONExpected := `{
	"***":{
		a:"aaa",
		"b":"***"
		},
	}`

	filter := createJSONFilter(t, nil, []regexmatcher.Regex{
		{Regexp: regexp.MustCompile("key_or_value"), Redactor: redaction.RedactRedactor},
	})
	attrValue := pdata.NewAttributeValueString(invalidJSONInput)
	parsedAttribute, err := filter.RedactAttribute("http.request.body", attrValue)
	require.NoError(t, err)
	assert.Equal(t, &processors.ParsedAttribute{
		Redacted: map[string]string{"http.request.body": invalidJSONInput},
	}, parsedAttribute)
	assert.Equal(t, invalidJSONExpected, attrValue.StringVal())
}
