package regexmatcher

import (
	"regexp"
	"testing"

	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
	"github.com/stretchr/testify/assert"
)

func TestFilterMatchedKey(t *testing.T) {
	m, _ := NewMatcher(nil, []Regex{{Regexp: regexp.MustCompile("^password$")}}, nil)
	redacted := m.FilterMatchedKey(redaction.RedactRedactor, "http.request.header.password", "abc123", "")
	assert.Equal(t, "***", redacted)
}
