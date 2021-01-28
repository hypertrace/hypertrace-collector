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
			Pattern:  "^b$",
			Redacter: redaction.RedactRedacter,
		},
		{
			Pattern:  "^c$",
			Redacter: redaction.HashRedacter,
		},
	}

	_, err := compileRegexs(keyRegexes)
	assert.NoError(t, err)
}

func TestFilterMatchedKey(t *testing.T) {
	m, _ := NewMatcher([]Regex{{Pattern: "^password$"}}, nil)
	isModified, redacted := m.FilterMatchedKey(redaction.RedactRedacter, "http.request.header.password", "abc123", "")
	assert.True(t, isModified)
	assert.Equal(t, "***", redacted)
}
