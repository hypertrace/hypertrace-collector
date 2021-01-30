package urlencoded

import (
	"net/url"
	"regexp"
	"testing"

	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"
	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
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
	isRedacted, err := filter.RedactAttribute("password", attrValue)
	assert.False(t, isRedacted)
	assert.NoError(t, err)

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
	isRedacted, err := filter.RedactAttribute("password", attrValue)
	assert.True(t, isRedacted)
	assert.NoError(t, err)

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
	isRedacted, err := filter.RedactAttribute("password", attrValue)
	assert.True(t, isRedacted)
	assert.NoError(t, err)

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
	isRedacted, err := filter.RedactAttribute("http.url", attrValue)
	assert.True(t, isRedacted)
	assert.NoError(t, err)

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
	isRedacted, err := filter.RedactAttribute("http.url", attrValue)
	assert.Error(t, err)
	assert.False(t, isRedacted)
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
	isRedacted, err := filter.RedactAttribute("whatever", attrValue)
	assert.NoError(t, err)
	assert.True(t, isRedacted)

	filteredParams, err := url.ParseQuery(attrValue.StringVal())
	assert.NoError(t, err)
	assert.Equal(t, grabURLValue(filteredParams, "key1"), "***")
	assert.Equal(t, grabURLValue(filteredParams, "key2"), "value2")
	assert.False(t, hasRemainingValues(filteredParams))
}
