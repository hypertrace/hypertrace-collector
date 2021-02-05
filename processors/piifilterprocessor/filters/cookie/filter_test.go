package cookie

import (
	"github.com/hypertrace/collector/processors"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"
	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
)

func Test_CookieFilterNoReduction(t *testing.T) {
	key := headerCookie
	cookieValue := "cookie1=value1"
	expectedCookieFilteredValue := "cookie1=value1"

	filter := newCookieFilter(t)

	attrValue := pdata.NewAttributeValueString(cookieValue)
	parsedAttr, err := filter.RedactAttribute(key, attrValue)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(parsedAttr.Redacted))
	assert.Equal(t, map[string]string{"http.request.header.cookie.cookie1": "value1"}, parsedAttr.Flattered)
	assert.Equal(t, &processors.ParsedAttribute{
		Flattered: map[string]string{"http.request.header.cookie.cookie1": "value1"},
		Redacted:  map[string]string{},
	}, parsedAttr)
	assert.Equal(t, expectedCookieFilteredValue, attrValue.StringVal())
}

func TestCookieFilterFiltersCookieKey(t *testing.T) {
	key := headerCookie
	cookieValue := "cookie1=value1; password=value2"
	expectedCookieFilteredValue := "cookie1=value1; password=***"
	filter := newCookieFilter(t)

	attrValue := pdata.NewAttributeValueString(cookieValue)
	parsedAttr, err := filter.RedactAttribute(key, attrValue)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"http.request.header.cookie.password": "value2"}, parsedAttr.Redacted)
	assert.Equal(t, expectedCookieFilteredValue, attrValue.StringVal())
}

func TestCookieFilterFiltersSetCookieKey(t *testing.T) {
	key := "http.response.header.set-cookie"
	cookieValue := "password=value2; SameSite=Strict"
	expectedCookieFilteredValue := "password=***; SameSite=Strict"
	filter := newCookieFilter(t)

	attrValue := pdata.NewAttributeValueString(cookieValue)
	redacted, err := filter.RedactAttribute(key, attrValue)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"http.response.header.set-cookie.password": "value2"}, redacted.Redacted)
	assert.Equal(t, expectedCookieFilteredValue, attrValue.StringVal())
}

func newCookieFilter(t *testing.T) *cookieFilter {
	m, err := regexmatcher.NewMatcher(nil, []regexmatcher.Regex{{
		Regexp:   regexp.MustCompile("^password$"),
		Redactor: redaction.RedactRedactor,
	}}, nil)
	if err != nil {
		t.Fatalf("failed to create cookie filter: %v\n", err)
	}

	return &cookieFilter{m: m}
}
