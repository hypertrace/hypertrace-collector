package keyvalue

import (
	"regexp"
	"testing"

	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"
	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
)

func TestRedactsByKeyWithNoMatchings(t *testing.T) {
	filter := newFilter(t, nil, []regexmatcher.Regex{{
		Regexp:   regexp.MustCompile("password"),
		Redactor: redaction.RedactRedactor,
	}}, nil)

	attrValue := pdata.NewAttributeValueString("abc123")
	isRedacted, err := filter.RedactAttribute("unrelated", attrValue)
	assert.NoError(t, err)
	assert.False(t, isRedacted)
	assert.Equal(t, "abc123", attrValue.StringVal())
}

func TestRedactsByKeySuccess(t *testing.T) {
	filter := newFilter(t, nil, []regexmatcher.Regex{{
		Regexp:   regexp.MustCompile("^http.request.header.*"),
		Redactor: redaction.RedactRedactor,
	}}, nil)

	attrValue := pdata.NewAttributeValueString("abc123")
	isRedacted, err := filter.RedactAttribute("http.request.header.password", attrValue)
	assert.NoError(t, err)
	assert.True(t, isRedacted)
	assert.Equal(t, "***", attrValue.StringVal())
}

func TestRedactsByKeyAndPrefixSuccess(t *testing.T) {
	filter := newFilter(t, []string{"http.request.header."}, []regexmatcher.Regex{{
		Regexp:   regexp.MustCompile("^password$"),
		Redactor: redaction.RedactRedactor,
	}}, nil)

	attrValue := pdata.NewAttributeValueString("aaa123")
	isRedacted, err := filter.RedactAttribute("http.request.header.password", attrValue)
	assert.NoError(t, err)
	assert.True(t, isRedacted)
	assert.Equal(t, "***", attrValue.StringVal())

	attrValue = pdata.NewAttributeValueString("bbb123")
	isRedacted, err = filter.RedactAttribute("b.password", attrValue)
	assert.NoError(t, err)
	assert.False(t, isRedacted)
	assert.Equal(t, "bbb123", attrValue.StringVal())

	attrValue = pdata.NewAttributeValueString("ccc123")
	isRedacted, err = filter.RedactAttribute("password", attrValue)
	assert.NoError(t, err)
	assert.True(t, isRedacted)
	assert.Equal(t, "***", attrValue.StringVal())
}

func TestRedactsByChainOfRegexByValueSuccess(t *testing.T) {
	filter := newFilter(t, nil, nil, []regexmatcher.Regex{
		{Regexp: regexp.MustCompile("aaa"), Redactor: redaction.RedactRedactor},
		{Regexp: regexp.MustCompile("bbb"), Redactor: redaction.RedactRedactor},
	})

	attrValue := pdata.NewAttributeValueString("aaa bbb ccc aaa bbb ccc")
	isRedacted, err := filter.RedactAttribute("cc", attrValue)
	assert.NoError(t, err)
	assert.True(t, isRedacted)
	assert.Equal(t, "*** *** ccc *** *** ccc", attrValue.StringVal())
}

func TestKeyValueRedactsByValueSuccess(t *testing.T) {
	filter := newFilter(t, nil, nil, []regexmatcher.Regex{{
		Regexp:   regexp.MustCompile("(?:\\d[ -]*?){13,16}"),
		Redactor: redaction.RedactRedactor,
	}})

	attrValue := pdata.NewAttributeValueString("4111 2222 3333 4444")
	isRedacted, err := filter.RedactAttribute("http.request.body", attrValue)
	assert.NoError(t, err)
	assert.True(t, isRedacted)
	assert.Equal(t, "***", attrValue.StringVal())
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
