package regexmatcher

import (
	"regexp"
	"testing"

	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
	"github.com/stretchr/testify/assert"
)

func TestFilterMatchedKey(t *testing.T) {
	m, _ := NewMatcher(nil, []Regex{{Regexp: regexp.MustCompile("^password$")}}, nil)
	isModified, redacted := m.FilterMatchedKey(redaction.RedactRedactor, "http.request.header.password", "abc123", "")
	assert.True(t, isModified)
	assert.Equal(t, "***", redacted)
}

//func TestFilterStringValueRegexs(t *testing.T) {
//	m, _ := NewMatcher(nil, []Regex{{Regexp: regexp.MustCompile("key_or_value")}}, nil)
//	isModified, redacted := m.FilterMatchedKey(redaction.RedactRedactor, "http.request.header.password", "abc123", "")
//	assert.True(t, isModified)
//	assert.Equal(t, "***", redacted)
//}
