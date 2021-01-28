package regexmatcher

import (
	"testing"

	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
	"github.com/stretchr/testify/assert"
)

func TestCompileRegexs(t *testing.T) {
	keyRegexes := []Regex{
		{
			Pattern: "^a$",
		},
		{
			Pattern:        "^b$",
			RedactStrategy: redaction.Redact,
		},
		{
			Pattern:        "^c$",
			RedactStrategy: redaction.Hash,
		},
	}

	compiledRegexes, err := compileRegexs(keyRegexes, redaction.Redact)
	assert.NoError(t, err)

	for _, cr := range compiledRegexes {
		if cr.Regexp.String() == "^a$" || cr.Regexp.String() == "^b$" {
			assert.Equal(t, redaction.Redact, cr.RedactStrategy)
		} else {
			assert.Equal(t, redaction.Hash, cr.RedactStrategy)
		}
	}
}

func TestFilterMatchedKey(t *testing.T) {
	m, _ := NewMatcher([]Regex{{Pattern: "^password$"}}, nil, redaction.Redact)
	isModified, redacted := m.FilterMatchedKey(redaction.Redact, "http.request.header.password", "abc123", "")
	assert.True(t, isModified)
	assert.Equal(t, "***", redacted)
}
