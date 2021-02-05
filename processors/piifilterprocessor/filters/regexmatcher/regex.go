package regexmatcher

import (
	"regexp"
	"strings"

	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
)

// Regex is a regex representation. It should be private
type Regex struct {
	*regexp.Regexp
	Redactor          redaction.Redactor
	FQN               bool
	SessionIdentifier bool
}

type Matcher struct {
	prefixes    []string
	keyRegExs   []Regex
	valueRegExs []Regex
}

func NewMatcher(
	prefixes []string,
	keyRegExs,
	valueRegExs []Regex,
) (*Matcher, error) {
	return &Matcher{
		prefixes:    prefixes,
		keyRegExs:   keyRegExs,
		valueRegExs: valueRegExs,
	}, nil
}

// FilterKeyRegexs looks into the key to decide whether filter the value or not
func (rm *Matcher) FilterKeyRegexs(keyToMatch string, actualKey string, value string, path string) (bool, bool, string) {
	for _, r := range rm.keyRegExs {
		if r.Regexp.MatchString(keyToMatch) {
			return true, r.SessionIdentifier, rm.FilterMatchedKey(r.Redactor, actualKey, value, path)
		}
	}

	return false, false, ""
}

// FilterStringValueRegexs looks into the string value to decide whether filter the value or not
func (rm *Matcher) FilterStringValueRegexs(value string, key string, path string) (bool, string) {
	var (
		isRedacted      bool
		isRegexRedacted bool
	)

	for _, r := range rm.valueRegExs {
		isRegexRedacted, value = rm.replacingRegex(value, r.Regexp, r.Redactor)
		isRedacted = isRedacted || isRegexRedacted
	}

	return isRedacted, value
}

func (rm *Matcher) replacingRegex(value string, regex *regexp.Regexp, redactor redaction.Redactor) (bool, string) {
	matchCount := 0

	filtered := regex.ReplaceAllStringFunc(value, func(src string) string {
		matchCount++
		return redactor(src)
	})

	return matchCount > 0, filtered
}

func unindexedKey(key string) string {
	if len(key) == 0 {
		return ""
	}
	return strings.Split(key, "[")[0]
}

const (
	queryParamTag     = "http.request.query.param"
	requestCookieTag  = "http.request.cookie"
	responseCookieTag = "http.response.cookie"
	// In case of empty json path, platform uses strings defined here as path
	requestBodyEmptyJSONPath  = "REQUEST_BODY"
	responseBodyEmptyJSONPath = "RESPONSE_BODY"
)

func mapRawToEnriched(rawTag string, path string) (string, string) {
	enrichedTag := rawTag
	enrichedPath := path

	unindexedKey := unindexedKey(rawTag)
	switch unindexedKey {
	case "http.url":
		enrichedTag = queryParamTag
	case "http.request.header.cookie":
		enrichedTag = requestCookieTag
	case "http.response.header.set-cookie":
		enrichedTag = responseCookieTag
	case "http.request.body":
		if len(path) == 0 {
			enrichedPath = requestBodyEmptyJSONPath
		}
	case "http.response.body":
		if len(path) == 0 {
			enrichedPath = responseBodyEmptyJSONPath
		}
	}

	return enrichedTag, enrichedPath
}

func (rm *Matcher) FilterMatchedKey(redactor redaction.Redactor, actualKey string, value string, path string) string {
	return redactor(value)
}

// MatchKeyRegexs matches a key or a path form the regex matcher and returns the matching
// regex. IT SHOULD BE AVOIDED as it leaks internal details from regex matcher.
// It will be removed soon.
func (rm *Matcher) MatchKeyRegexs(keyToMatch string, path string) (bool, *Regex) {
	for _, r := range rm.keyRegExs {
		if r.FQN {
			if r.Regexp.MatchString(path) {
				return true, &r
			}
		} else {
			if r.Regexp.MatchString(keyToMatch) {
				return true, &r
			}
		}

	}
	return false, nil
}

func (rm *Matcher) GetTruncatedKey(key string) string {
	for _, prefix := range rm.prefixes {
		if strings.HasPrefix(key, prefix) {
			return strings.TrimPrefix(key, prefix)
		}
	}

	return key
}
