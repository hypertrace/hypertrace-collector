package keyvalue

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/hypertrace/collector/processors"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"
	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
)

func TestRedactsByKeyWithNoMatchings(t *testing.T) {
	filter := newFilter(t, nil, []regexmatcher.Regex{{
		Regexp:   regexp.MustCompile("password"),
		Redactor: redaction.RedactRedactor,
	}}, nil)

	attrValue := pdata.NewAttributeValueString("abc123")
	redacted, newAttr, err := filter.RedactAttribute("unrelated", attrValue)
	assert.NoError(t, err)
	assert.Nil(t, newAttr)
	assert.Nil(t, redacted)
	assert.Equal(t, "abc123", attrValue.StringVal())
}

func TestRedactsByKeySuccess(t *testing.T) {
	filter := newFilter(t, nil, []regexmatcher.Regex{{
		Regexp:   regexp.MustCompile("^http.request.header.*"),
		Redactor: redaction.RedactRedactor,
	}}, nil)

	attrValue := pdata.NewAttributeValueString("abc123")
	redacted, newAttr, err := filter.RedactAttribute("http.request.header.password", attrValue)
	assert.NoError(t, err)
	assert.Nil(t, newAttr)
	assert.Equal(t, map[string]string{"http.request.header.password": "abc123"}, redacted.Redacted)
	assert.Equal(t, "***", attrValue.StringVal())
}

func TestRedactsByKeyAndPrefixSuccess(t *testing.T) {
	filter := newFilter(t, []string{"http.request.header."}, []regexmatcher.Regex{{
		Regexp:   regexp.MustCompile("^password$"),
		Redactor: redaction.RedactRedactor,
	}}, nil)

	attrValue := pdata.NewAttributeValueString("aaa123")
	parsedAttribute, newAttr, err := filter.RedactAttribute("http.request.header.password", attrValue)
	assert.NoError(t, err)
	assert.Nil(t, newAttr)
	assert.Equal(t, &processors.ParsedAttribute{Redacted: map[string]string{"http.request.header.password": "aaa123"}}, parsedAttribute)
	assert.Equal(t, "***", attrValue.StringVal())

	attrValue = pdata.NewAttributeValueString("bbb123")
	parsedAttribute, newAttr, err = filter.RedactAttribute("b.password", attrValue)
	assert.NoError(t, err)
	assert.Nil(t, newAttr)
	assert.Nil(t, parsedAttribute)
	assert.Equal(t, "bbb123", attrValue.StringVal())

	attrValue = pdata.NewAttributeValueString("ccc123")
	parsedAttribute, newAttr, err = filter.RedactAttribute("password", attrValue)
	assert.NoError(t, err)
	assert.Nil(t, newAttr)
	assert.Equal(t, map[string]string{"password": "ccc123"}, parsedAttribute.Redacted)
	assert.Equal(t, "***", attrValue.StringVal())
}

func TestRedactsByChainOfRegexByValueSuccess(t *testing.T) {
	filter := newFilter(t, nil, nil, []regexmatcher.Regex{
		{Regexp: regexp.MustCompile("aaa"), Redactor: redaction.RedactRedactor},
		{Regexp: regexp.MustCompile("bbb"), Redactor: redaction.RedactRedactor},
	})

	attrValue := pdata.NewAttributeValueString("aaa bbb ccc aaa bbb ccc")
	parsedAttribute, newAttr, err := filter.RedactAttribute("cc", attrValue)
	assert.NoError(t, err)
	assert.Nil(t, newAttr)
	assert.Equal(t, &processors.ParsedAttribute{Redacted: map[string]string{"cc": "aaa bbb ccc aaa bbb ccc"}}, parsedAttribute)
	assert.Equal(t, "*** *** ccc *** *** ccc", attrValue.StringVal())
}

func TestKeyValueRedactsByValueSuccess(t *testing.T) {
	filter := newFilter(t, nil, nil, []regexmatcher.Regex{{
		Regexp:   regexp.MustCompile("(?:\\d[ -]*?){13,16}"),
		Redactor: redaction.RedactRedactor,
	}})

	attrValue := pdata.NewAttributeValueString("4111 2222 3333 4444")
	redacted, newAttr, err := filter.RedactAttribute("http.request.body", attrValue)
	assert.NoError(t, err)
	assert.Nil(t, newAttr)
	assert.Equal(t, &processors.ParsedAttribute{Redacted: map[string]string{"http.request.body": "4111 2222 3333 4444"}}, redacted)
	assert.Equal(t, "***", attrValue.StringVal())
}

func TestSessionAttribute(t *testing.T) {
	regexes := []regexmatcher.Regex{{
		Regexp:            regexp.MustCompile("^http.request.header.session"),
		Redactor:          redaction.HashRedactor,
		SessionIdentifier: true,
	}}
	m, err := regexmatcher.NewMatcher(nil, regexes, nil)
	filter := &keyValueFilter{m: m}
	attrValue := pdata.NewAttributeValueString("foobar")
	hashedSession := redaction.HashRedactor(attrValue.StringVal())
	parsedAttribute, newAttr, err := filter.RedactAttribute("http.request.header.session", attrValue)
	require.NoError(t, err)
	assert.Equal(t, &filters.Attribute{Key: "session.id", Value: pdata.NewAttributeValueString(hashedSession)}, newAttr)
	assert.Equal(t, &processors.ParsedAttribute{
		Redacted: map[string]string{
			"http.request.header.session": "foobar",
		},
	}, parsedAttribute)
}

func newFilter(
	t *testing.T,
	prefixes []string,
	keyRegExs []regexmatcher.Regex,
	valueRegExs []regexmatcher.Regex,
) *keyValueFilter {
	m, err := regexmatcher.NewMatcher(prefixes, keyRegExs, valueRegExs)
	if err != nil {
		t.Fatalf("failed to create cookie filter: %v\n", err)
	}

	return &keyValueFilter{m: m}
}
