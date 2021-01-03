package json

import (
	"errors"
	"reflect"
	"testing"

	stdjson "encoding/json"

	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/internal/json"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/internal/matcher"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
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

func createJSONFilter(t *testing.T, keyRegExs []matcher.Regex) *jsonFilter {
	m, err := matcher.NewRegexMatcher(keyRegExs, []matcher.Regex{}, filters.Redact)

	assert.NoError(t, err)

	return &jsonFilter{m: m, mu: json.DefaultMarshalUnmarshaler}
}

func TestFilterSuccessOnEmptyString(t *testing.T) {
	filter := createJSONFilter(t, []matcher.Regex{})

	attrValue := pdata.NewAttributeValueString("")
	isRedacted, err := filter.RedactAttribute("attrib_key", attrValue)
	assert.False(t, isRedacted)
	assert.NoError(t, err)
}

func TestFilterFailsOnInvalidJSON(t *testing.T) {
	filter := createJSONFilter(t, []matcher.Regex{})

	attrValue := pdata.NewAttributeValueString("bob")
	isRedacted, err := filter.RedactAttribute("attrib_key", attrValue)
	assert.False(t, isRedacted)
	assert.Error(t, err)
	assert.Equal(t, filters.ErrUnprocessableValue, errors.Unwrap(err))
}

func TestJSONFieldRedaction(t *testing.T) {
	tCases := map[string]struct {
		unredactedValue           string
		expectedRedactedAttrValue string
	}{
		"for outer array": {
			unredactedValue:           "[{\"a\":\"1\"},{\"password\":\"abc\"}]",
			expectedRedactedAttrValue: "[{\"a\":\"1\"},{\"password\":\"***\"}]",
		},
		"for inner array": {
			unredactedValue:           "{\"a\": [{\"b\": \"1\"}, {\"password\": \"abc\"}]}",
			expectedRedactedAttrValue: "{\"a\": [{\"b\": \"1\"}, {\"password\": \"***\"}]}",
		},
		"for array in key": {
			unredactedValue:           "{\"a\": [{\"b\": \"1\"}, {\"password\": [\"12\",\"34\",\"56\"]}]}",
			expectedRedactedAttrValue: "{\"a\": [{\"b\": \"1\"}, {\"password\": [\"***\",\"***\",\"***\"]}]}",
		},
		"for object in key": {
			unredactedValue:           "{\"a\": [{\"b\": \"1\"}, {\"password\":{\"key1\":[\"12\",\"34\",\"56\"], \"key2\":\"val\"}}]}",
			expectedRedactedAttrValue: "{\"a\": [{\"b\": \"1\"}, {\"password\": {\"key1\":[\"***\",\"***\",\"***\"], \"key2\":\"***\"}}]}",
		},
		"for non string scalar": {
			unredactedValue:           "{\"a\": [{\"b\": \"1\"}, {\"password\":{\"key1\":[12,34.1,true], \"key2\":false}}]}",
			expectedRedactedAttrValue: "{\"a\": [{\"b\": \"1\"}, {\"password\": {\"key1\":[\"***\",\"***\",\"***\"], \"key2\":\"***\"}}]}",
		},
	}

	for name, tCase := range tCases {
		t.Run(name, func(t *testing.T) {
			filter := createJSONFilter(t, []matcher.Regex{{Pattern: "^password$"}})

			attrValue := pdata.NewAttributeValueString(tCase.unredactedValue)
			isRedacted, err := filter.RedactAttribute("attrib_key", attrValue)
			assert.True(t, isRedacted)
			assert.NoError(t, err)
			assertJSONEqual(t, tCase.expectedRedactedAttrValue, attrValue.StringVal())
		})
	}
}

func Test_piifilterprocessor_json_SimpleArrayFilter(t *testing.T) {
	filter := createJSONFilter(t, []matcher.Regex{{Pattern: "^password$"}})
	attrValue := pdata.NewAttributeValueString("[\"12\",\"34\",\"56\"]")
	isRedacted, err := filter.RedactAttribute("attrib_key", attrValue)
	assert.False(t, isRedacted)
	assert.NoError(t, err)
	assertJSONEqual(t, "[\"12\",\"34\",\"56\"]", attrValue.StringVal())
}

func Test_piifilterprocessor_json_ObjectInKeyFilter_fqn(t *testing.T) {
	tCases := map[string]struct {
		pattern                   string
		unredactedValue           string
		expectedRedactedAttrValue string
	}{
		"all elements in a password array": {
			pattern:                   "^\\$\\.a\\[1\\]\\.password$",
			unredactedValue:           "{\"a\": [{\"b\": \"1\"}, {\"password\": [\"12\",\"34\",\"56\"]}]}",
			expectedRedactedAttrValue: "{\"a\": [{\"b\": \"1\"}, {\"password\": [\"***\",\"***\",\"***\"]}]}",
		},
		"one element in a password array": {
			pattern:                   "^\\$\\.a\\[1\\]\\.password\\[1\\]$",
			unredactedValue:           "{\"a\": [{\"b\": \"1\"}, {\"password\": [\"12\",\"34\",\"56\"]}]}",
			expectedRedactedAttrValue: "{\"a\": [{\"b\": \"1\"}, {\"password\": [\"12\",\"***\",\"56\"]}]}",
		},
		"all elements in a password object": {
			pattern:                   "^\\$\\.a\\[1\\]\\.password.key1$",
			unredactedValue:           "{\"a\": [{\"b\": \"1\"}, {\"password\":{\"key1\":[12,34,56], \"key2\":\"val\"}}]}",
			expectedRedactedAttrValue: "{\"a\": [{\"b\": \"1\"}, {\"password\": {\"key1\":[\"***\",\"***\",\"***\"], \"key2\":\"val\"}}]}",
		},
		"one element in a password object": {
			pattern:                   "^\\$\\.a\\[1\\]\\.password.key1\\[1\\]$",
			unredactedValue:           "{\"a\": [{\"b\": \"1\"}, {\"password\":{\"key1\":[12,34,56], \"key2\":\"val\"}}]}",
			expectedRedactedAttrValue: "{\"a\": [{\"b\": \"1\"}, {\"password\": {\"key1\":[12,\"***\",56], \"key2\":\"val\"}}]}",
		},
	}

	for name, tCase := range tCases {
		t.Run(name, func(t *testing.T) {
			filter := createJSONFilter(t, []matcher.Regex{{Pattern: tCase.pattern, Fqn: true}})
			attrValue := pdata.NewAttributeValueString(tCase.unredactedValue)
			isRedacted, err := filter.RedactAttribute("attrib_key", attrValue)
			assert.True(t, isRedacted)
			assert.NoError(t, err)
			assertJSONEqual(t, tCase.expectedRedactedAttrValue, attrValue.StringVal())
		})
	}
}

func TestFQNSuccess(t *testing.T) {
	tCases := map[string]struct {
		pattern                   string
		unredactedValue           string
		expectedRedactedAttrValue string
	}{
		"simple array is redacted": {
			pattern:                   "^\\$\\[1\\]$",
			unredactedValue:           "[\"12\",\"34\",\"56\"]",
			expectedRedactedAttrValue: "[\"12\",\"***\",\"56\"]",
		},
		"simple object is redacted": {
			pattern:                   "^\\$\\.password$",
			unredactedValue:           "{\"a\": \"1\",\"password\": \"abc\"}",
			expectedRedactedAttrValue: "{\"a\": \"1\",\"password\": \"***\"}",
		},
	}

	for name, tCase := range tCases {
		t.Run(name, func(t *testing.T) {
			filter := createJSONFilter(t, []matcher.Regex{{Pattern: tCase.pattern, Fqn: true}})
			attrValue := pdata.NewAttributeValueString(tCase.unredactedValue)
			isRedacted, err := filter.RedactAttribute("attrib_key", attrValue)
			assert.True(t, isRedacted)
			assert.NoError(t, err)
			assertJSONEqual(t, tCase.expectedRedactedAttrValue, attrValue.StringVal())
		})
	}

}
