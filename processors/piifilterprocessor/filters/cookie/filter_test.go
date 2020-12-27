package cookie

import (
	"testing"

	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/internal/matcher"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
)

func Test_piifilterprocessor_cookie_FilterKey(t *testing.T) {
	key := "http.request.header.cookie"
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
	key := "http.request.header.cookie"
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
	m, err := matcher.NewRegexMatcher([]matcher.Regex{{
		Pattern: "^password$",
	}}, []matcher.Regex{}, filters.Redact)
	if err != nil {
		t.Fatalf("failed to create cookie filter: %v\n", err)
	}

	return &cookieFilter{m: m}
}
