package keyvalue

import (
	"testing"

	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"
	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
)

func TestRedactsByKeyWithNoMatchings(t *testing.T) {
	filter := newFilter(t, []regexmatcher.Regex{{
		Pattern:  "password",
		Redacter: redaction.RedactRedacter,
	}}, nil)

	attrValue := pdata.NewAttributeValueString("abc123")
	isRedacted, err := filter.RedactAttribute("unrelated", attrValue)
	assert.NoError(t, err)
	assert.False(t, isRedacted)
	assert.Equal(t, "abc123", attrValue.StringVal())
}

func TestRedactsByKeySuccess(t *testing.T) {
	filter := newFilter(t, []regexmatcher.Regex{{
		Pattern:  "^http.request.header.*",
		Redacter: redaction.RedactRedacter,
	}}, nil)

	attrValue := pdata.NewAttributeValueString("abc123")
	isRedacted, err := filter.RedactAttribute("http.request.header.password", attrValue)
	assert.NoError(t, err)
	assert.True(t, isRedacted)
	assert.Equal(t, "***", attrValue.StringVal())
}

func TestRedactsByChainOfRegexByValueSuccess(t *testing.T) {
	filter := newFilter(t, nil, []regexmatcher.Regex{
		{Pattern: "aaa", Redacter: redaction.RedactRedacter},
		{Pattern: "bbb", Redacter: redaction.RedactRedacter},
	})

	attrValue := pdata.NewAttributeValueString("aaa bbb ccc aaa bbb ccc")
	isRedacted, err := filter.RedactAttribute("cc", attrValue)
	assert.NoError(t, err)
	assert.True(t, isRedacted)
	assert.Equal(t, "*** *** ccc *** *** ccc", attrValue.StringVal())
}

func TestKeyValueRedactsByValueSuccess(t *testing.T) {
	filter := newFilter(t, nil, []regexmatcher.Regex{{
		Pattern:  "(?:\\d[ -]*?){13,16}",
		Redacter: redaction.RedactRedacter,
	}})

	attrValue := pdata.NewAttributeValueString("4111 2222 3333 4444")
	isRedacted, err := filter.RedactAttribute("http.request.body", attrValue)
	assert.NoError(t, err)
	assert.True(t, isRedacted)
	assert.Equal(t, "***", attrValue.StringVal())
}

func newFilter(
	t *testing.T,
	keyRegExs []regexmatcher.Regex,
	valueRegExs []regexmatcher.Regex,
) *keyValueFilter {
	m, err := regexmatcher.NewMatcher(keyRegExs, valueRegExs)
	if err != nil {
		t.Fatalf("failed to create cookie filter: %v\n", err)
	}

	return &keyValueFilter{m: m}
}
