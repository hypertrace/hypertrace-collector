package cookie

import (
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"github.com/stretchr/testify/require"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/hypertrace/collector/processors"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"
	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
)

func Test_CookieFilterNoReduction(t *testing.T) {
	key := headerCookie
	cookieValue := "cookie1=value1"
	expectedCookieFilteredValue := "cookie1=value1"

	filter := newCookieFilter(t)

	attrValue := pdata.NewAttributeValueString(cookieValue)
	parsedAttr, newAttr, err := filter.RedactAttribute(key, attrValue)
	assert.NoError(t, err)
	assert.Nil(t, newAttr)
	assert.Equal(t, 0, len(parsedAttr.Redacted))
	assert.Equal(t, &processors.ParsedAttribute{
		Flattened: map[string]string{"cookie1": "value1"},
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
	parsedAttr, newAttr, err := filter.RedactAttribute(key, attrValue)
	assert.NoError(t, err)
	assert.Nil(t, newAttr)
	assert.Equal(t, &processors.ParsedAttribute{
		Flattened: map[string]string{
			"cookie1":  "value1",
			"password": "value2",
		},
		Redacted: map[string]string{"http.request.header.cookie.password": "value2"},
	}, parsedAttr)
	assert.Equal(t, expectedCookieFilteredValue, attrValue.StringVal())
}

func TestCookieFilterFiltersSetCookieKey(t *testing.T) {
	key := "http.response.header.set-cookie"
	cookieValue := "password=value2; SameSite=Strict"
	expectedCookieFilteredValue := "password=***; SameSite=Strict"
	filter := newCookieFilter(t)

	attrValue := pdata.NewAttributeValueString(cookieValue)
	parsedAttribute, newAttr, err := filter.RedactAttribute(key, attrValue)
	assert.NoError(t, err)
	assert.Nil(t, newAttr)
	assert.Equal(t, &processors.ParsedAttribute{
		Flattened: map[string]string{
			"password": "value2",
		}, Redacted: map[string]string{
			"http.response.header.set-cookie.password": "value2",
		},
	}, parsedAttribute)
	assert.Equal(t, expectedCookieFilteredValue, attrValue.StringVal())
}

func TestSessionAttribute(t *testing.T) {
	m, err := regexmatcher.NewMatcher(nil, []regexmatcher.Regex{{
		Regexp:            regexp.MustCompile("^sessionToken$"),
		Redactor:          redaction.HashRedactor,
		SessionIdentifier: true,
	}}, nil)
	require.NoError(t, err)
	filter := &cookieFilter{m: m}

	hashedSession := redaction.HashRedactor("foobar")

	key := "http.request.header.cookie"
	cookieValue := "sessionToken=foobar"
	expectedCookieFilteredValue := "sessionToken=" + hashedSession
	attrValue := pdata.NewAttributeValueString(cookieValue)
	parsedAttribute, newAttr, err := filter.RedactAttribute(key, attrValue)
	assert.NoError(t, err)
	assert.Equal(t, &filters.Attribute{Key: "session.id", Value: pdata.NewAttributeValueString(hashedSession)}, newAttr)
	assert.Equal(t, &processors.ParsedAttribute{
		Flattened: map[string]string{
			"sessionToken": "foobar",
		}, Redacted: map[string]string{
			"http.request.header.cookie.sessionToken": "foobar",
		},
	}, parsedAttribute)
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
