package cookie

import (
	"testing"

	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"
	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
)

func Test_piifilterprocessor_cookie_FilterKey(t *testing.T) {
	key := headerCookie
	cookieValue := "cookie1=value1"
	expectedCookieFilteredValue := "cookie1=value1"

	filter := newCookieFilter(t)

	attrValue := pdata.NewAttributeValueString(cookieValue)
	isRedacted, err := filter.RedactAttribute(key, attrValue)
	assert.NoError(t, err)
	assert.False(t, isRedacted)
	assert.Equal(t, expectedCookieFilteredValue, attrValue.StringVal())
}

func TestCookieFilterFiltersCookieKey(t *testing.T) {
	key := headerCookie
	cookieValue := "cookie1=value1; password=value2"
	expectedCookieFilteredValue := "cookie1=value1; password=***"
	filter := newCookieFilter(t)

	attrValue := pdata.NewAttributeValueString(cookieValue)
	isRedacted, err := filter.RedactAttribute(key, attrValue)
	assert.NoError(t, err)
	assert.True(t, isRedacted)
	assert.Equal(t, expectedCookieFilteredValue, attrValue.StringVal())
}

func TestCookieFilterFiltersSetCookieKey(t *testing.T) {
	key := "http.response.header.set-cookie"
	cookieValue := "password=value2; SameSite=Strict"
	expectedCookieFilteredValue := "password=***; SameSite=Strict"
	filter := newCookieFilter(t)

	attrValue := pdata.NewAttributeValueString(cookieValue)
	isRedacted, err := filter.RedactAttribute(key, attrValue)
	assert.NoError(t, err)
	assert.True(t, isRedacted)
	assert.Equal(t, expectedCookieFilteredValue, attrValue.StringVal())
}

func newCookieFilter(t *testing.T) *cookieFilter {
	m, err := regexmatcher.NewMatcher([]regexmatcher.Regex{{
		Pattern:  "^password$",
		Redacter: redaction.RedactRedacter,
	}}, []regexmatcher.Regex{})
	if err != nil {
		t.Fatalf("failed to create cookie filter: %v\n", err)
	}

	return &cookieFilter{m: m}
}
