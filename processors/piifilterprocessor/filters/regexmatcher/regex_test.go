package regexmatcher

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
)

func TestFilterMatchedKey(t *testing.T) {
	m, _ := NewMatcher(nil, []Regex{{Regexp: regexp.MustCompile("^password$")}}, nil)
	redacted := m.FilterMatchedKey(redaction.RedactRedactor, "http.request.header.password", "abc123", "")
	assert.Equal(t, "***", redacted)
}
