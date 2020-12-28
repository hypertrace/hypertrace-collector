package matcher

import (
	"testing"

	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"github.com/stretchr/testify/assert"
)

func TestCompileRegexs(t *testing.T) {
	keyRegexes := []Regex{
		{
			Pattern: "^a$",
		},
		{
			Pattern:        "^b$",
			RedactStrategy: filters.Redact,
		},
		{
			Pattern:        "^c$",
			RedactStrategy: filters.Hash,
		},
	}

	compiledRegexes, err := compileRegexs(keyRegexes, filters.Redact)
	assert.NoError(t, err)

	for _, cr := range compiledRegexes {
		if cr.Regexp.String() == "^a$" || cr.Regexp.String() == "^b$" {
			assert.Equal(t, filters.Redact, cr.RedactStrategy)
		} else {
			assert.Equal(t, filters.Hash, cr.RedactStrategy)
		}
	}
}

func TestFilterMatchedKey(t *testing.T) {
	m, _ := NewRegexMatcher([]Regex{{Pattern: "^password$"}}, nil, filters.Redact)
	isModified, redacted := m.FilterMatchedKey(filters.Redact, "http.request.header.password", "abc123", "")
	assert.True(t, isModified)
	assert.Equal(t, "***", redacted)
}
