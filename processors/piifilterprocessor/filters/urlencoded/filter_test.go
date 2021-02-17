package urlencoded

import (
	"net/url"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/hypertrace/collector/processors"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"
	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
)

func createURLEncodedFilter(t *testing.T, keyRegexs, valueRegexs []regexmatcher.Regex) *urlEncodedFilter {
	m, err := regexmatcher.NewMatcher(nil, keyRegexs, valueRegexs)

	assert.NoError(t, err)

	return &urlEncodedFilter{m: m}
}

// grabURLValue obtains the first value associated with a given key
// and remove it to the map, this is useful for testing purposes
// as you can later do assertions about the remaining values with
// isEmptyURLValue
func grabURLValue(v url.Values, key string) string {
	defer v.Del(key)
	return v.Get(key)
}

// hasRemainingValues returns true if the URL values are empty. This is
// useful to make sure the test covers all the URL values.
func hasRemainingValues(v url.Values) bool {
	return len(v) > 0
}

func TestURLEncodedFilterSuccessOnNoSensitiveValue(t *testing.T) {
	filter := createURLEncodedFilter(t, []regexmatcher.Regex{
		{Regexp: regexp.MustCompile("^password$"), Redactor: redaction.RedactRedactor},
	}, []regexmatcher.Regex{})

	v := url.Values{}
	v.Add("user", "dave")

	attrValue := pdata.NewAttributeValueString(v.Encode())
	parsedAttr, newAttr, err := filter.RedactAttribute("password", attrValue)
	assert.NoError(t, err)
	assert.Equal(t, &processors.ParsedAttribute{
		Flattened: map[string]string{
			"user": "dave",
		},
		Redacted: map[string]string{},
	}, parsedAttr)
	assert.Nil(t, newAttr)

	filteredParams, err := url.ParseQuery(attrValue.StringVal())
	assert.NoError(t, err)
	assert.Equal(t, grabURLValue(filteredParams, "user"), "dave")
	assert.False(t, hasRemainingValues(filteredParams))
}

func TestURLEncodedFilterSuccessForSensitiveKey(t *testing.T) {
	filter := createURLEncodedFilter(t, []regexmatcher.Regex{
		{Regexp: regexp.MustCompile("^password$"), Redactor: redaction.RedactRedactor},
	}, []regexmatcher.Regex{})

	v := url.Values{}
	v.Add("user", "dave")
	v.Add("password", "mypw$")

	attrValue := pdata.NewAttributeValueString(v.Encode())
	parsedAttr, newAttr, err := filter.RedactAttribute("password", attrValue)
	assert.NoError(t, err)
	assert.Equal(t, &processors.ParsedAttribute{
		Redacted:  map[string]string{"password": "mypw$"},
		Flattened: map[string]string{"password": "mypw$", "user": "dave"},
	}, parsedAttr)
	assert.Nil(t, newAttr)

	filteredParams, err := url.ParseQuery(attrValue.StringVal())
	assert.NoError(t, err)
	assert.Equal(t, grabURLValue(filteredParams, "user"), "dave")
	assert.Equal(t, grabURLValue(filteredParams, "password"), "***")
	assert.False(t, hasRemainingValues(filteredParams))
}

func TestURLEncodedFilterSuccessForSensitiveKeyMultiple(t *testing.T) {
	filter := createURLEncodedFilter(t, []regexmatcher.Regex{
		{Regexp: regexp.MustCompile("^password$"), Redactor: redaction.RedactRedactor},
	}, []regexmatcher.Regex{})

	v := url.Values{}
	v.Add("user", "dave")
	v.Add("password", "mypw$")
	v.Add("password", "mypw#")

	attrValue := pdata.NewAttributeValueString(v.Encode())
	parsedAttribute, newAttr, err := filter.RedactAttribute("password", attrValue)
	assert.NoError(t, err)
	assert.Equal(t, &processors.ParsedAttribute{
		Flattened: map[string]string{
			"user":     "dave",
			"password": "mypw#",
		},
		Redacted: map[string]string{
			"password": "mypw#",
		},
	}, parsedAttribute)
	assert.Nil(t, newAttr)

	filteredParams, err := url.ParseQuery(attrValue.StringVal())
	assert.NoError(t, err)
	assert.Equal(t, grabURLValue(filteredParams, "user"), "dave")
	assert.Equal(t, grabURLValue(filteredParams, "password"), "***")
	assert.False(t, hasRemainingValues(filteredParams))
}

func TestURLEncodedFilterSuccessForURL(t *testing.T) {
	filter := createURLEncodedFilter(t, []regexmatcher.Regex{
		{Regexp: regexp.MustCompile("^password$"), Redactor: redaction.RedactRedactor},
	}, nil)

	testURL := "http://traceshop.dev/login?username=george&password=washington"

	attrValue := pdata.NewAttributeValueString(testURL)
	parsedAttribute, newAttr, err := filter.RedactAttribute("http.url", attrValue)
	assert.NoError(t, err)
	assert.Equal(t, &processors.ParsedAttribute{
		Redacted:  map[string]string{"password": "washington"},
		Flattened: map[string]string{"password": "washington", "username": "george"},
	}, parsedAttribute)
	assert.Nil(t, newAttr)

	u, err := url.Parse(attrValue.StringVal())
	assert.NoError(t, err)

	filteredParams, err := url.ParseQuery(u.RawQuery)
	assert.NoError(t, err)
	assert.Equal(t, "george", grabURLValue(filteredParams, "username"))
	assert.Equal(t, "***", grabURLValue(filteredParams, "password"))
	assert.False(t, hasRemainingValues(filteredParams))
}

func TestURLEncodedFilterFailsParsingURL(t *testing.T) {
	filter := createURLEncodedFilter(t, []regexmatcher.Regex{
		{Regexp: regexp.MustCompile("^password$")},
	}, []regexmatcher.Regex{})

	testURL := "http://x: namedport"

	attrValue := pdata.NewAttributeValueString(testURL)
	parsedAttribute, newAttr, err := filter.RedactAttribute("http.url", attrValue)
	assert.Error(t, err)
	assert.Nil(t, parsedAttribute)
	assert.Nil(t, newAttr)
	assert.Equal(t, testURL, attrValue.StringVal())
}

func TestURLEncodedFilterSuccessForSensitiveValue(t *testing.T) {
	filter := createURLEncodedFilter(t, nil, []regexmatcher.Regex{
		{
			Regexp:   regexp.MustCompile("^filter_value$"),
			Redactor: redaction.RedactRedactor,
		},
	})

	v := url.Values{}
	v.Add("key1", "filter_value")
	v.Add("key2", "value2")

	attrValue := pdata.NewAttributeValueString(v.Encode())
	parsedAttribute, newAttr, err := filter.RedactAttribute("whatever", attrValue)
	assert.NoError(t, err)
	assert.Equal(t, &processors.ParsedAttribute{
		Flattened: map[string]string{"key1": "filter_value", "key2": "value2"},
		Redacted:  map[string]string{"key1": "filter_value"},
	}, parsedAttribute)
	assert.Nil(t, newAttr)

	filteredParams, err := url.ParseQuery(attrValue.StringVal())
	assert.NoError(t, err)
	assert.Equal(t, grabURLValue(filteredParams, "key1"), "***")
	assert.Equal(t, grabURLValue(filteredParams, "key2"), "value2")
	assert.False(t, hasRemainingValues(filteredParams))
}

func TestSessionAttribute(t *testing.T) {
	filter := createURLEncodedFilter(t, []regexmatcher.Regex{
		{Regexp: regexp.MustCompile("^session$"), Redactor: redaction.HashRedactor, SessionIdentifier: true},
	}, nil)

	testURL := "http://traceshop.dev/login?username=george&session=foobar"

	attrValue := pdata.NewAttributeValueString(testURL)
	parsedAttribute, newAttr, err := filter.RedactAttribute("http.url", attrValue)
	assert.NoError(t, err)
	assert.Equal(t, &processors.ParsedAttribute{
		Redacted:  map[string]string{"session": "foobar"},
		Flattened: map[string]string{"session": "foobar", "username": "george"},
	}, parsedAttribute)
	assert.Equal(t, &filters.Attribute{Key: "session.id", Value: redaction.HashRedactor("foobar")}, newAttr)

	u, err := url.Parse(attrValue.StringVal())
	assert.NoError(t, err)

	filteredParams, err := url.ParseQuery(u.RawQuery)
	assert.NoError(t, err)
	assert.Equal(t, "george", grabURLValue(filteredParams, "username"))
	assert.Equal(t, redaction.HashRedactor("foobar"), grabURLValue(filteredParams, "session"))
	assert.False(t, hasRemainingValues(filteredParams))
}
