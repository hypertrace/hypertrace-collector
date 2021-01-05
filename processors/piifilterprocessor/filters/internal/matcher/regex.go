package matcher

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
)

// Regex is a regex representation. It should be private
type Regex struct {
	Pattern        string
	RedactStrategy filters.RedactionStrategy
	FQN            bool
}

// CompiledRegex is a compiled regex representation. It should be private
type CompiledRegex struct {
	*regexp.Regexp
	Regex
}

var _ Matcher = (*regexMatcher)(nil)

type regexMatcher struct {
	hash        func(string) string
	keyRegexs   []CompiledRegex
	valueRegexs []CompiledRegex
}

func NewRegexMatcher(
	keyRegexs,
	valueRegexs []Regex,
	globalStrategy filters.RedactionStrategy,
) (*regexMatcher, error) {
	compiledKeyRegexs, err := compileRegexs(keyRegexs, globalStrategy)
	if err != nil {
		return nil, err
	}

	compiledValueRegexs, err := compileRegexs(valueRegexs, globalStrategy)
	if err != nil {
		return nil, err
	}

	return &regexMatcher{
		keyRegexs:   compiledKeyRegexs,
		valueRegexs: compiledValueRegexs,
	}, nil
}

func (rm *regexMatcher) FilterKeyRegexs(keyToMatch string, actualKey string, value string, path string) (bool, string) {
	for _, r := range rm.keyRegexs {
		if r.Regexp.MatchString(keyToMatch) {
			return rm.FilterMatchedKey(r.RedactStrategy, actualKey, value, path)
		}
	}

	return false, ""
}

func (rm *regexMatcher) FilterStringValueRegexs(value string, key string, path string) (bool, string) {
	inspectorKey := getFullyQualifiedInspectorKey(key, path)

	filtered := false
	for _, r := range rm.valueRegexs {
		filtered, value = rm.replacingRegex(value, inspectorKey, r.Regexp, r.RedactStrategy)
	}

	return filtered, value
}

func (rm *regexMatcher) replacingRegex(value string, key string, regex *regexp.Regexp, rs filters.RedactionStrategy) (bool, string) {
	matchCount := 0

	filtered := regex.ReplaceAllStringFunc(value, func(src string) string {
		matchCount++
		_, str := rm.redactAndFilterData(rs, src, key)
		return str
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

func getFullyQualifiedInspectorKey(actualKey string, path string) string {
	inspectorKey, enrichedPath := mapRawToEnriched(actualKey, path)

	if len(enrichedPath) > 0 {
		inspectorKey = fmt.Sprintf("%s.%s", inspectorKey, enrichedPath)
	}

	return inspectorKey
}

func (rm *regexMatcher) redactAndFilterData(redact filters.RedactionStrategy, value string, _ string) (bool, string) {
	var redactedValue string
	var isModified = true
	switch redact {
	case filters.Redact:
		redactedValue = filters.RedactedText
	case filters.Hash:
		redactedValue = rm.hash(value)
	case filters.Raw:
		redactedValue = value
		// should we return turn isModified = false here?
	default:
		redactedValue = filters.RedactedText
	}

	return isModified, redactedValue
}

func (rm *regexMatcher) FilterMatchedKey(redactionStrategy filters.RedactionStrategy, actualKey string, value string, path string) (bool, string) {
	inspectorKey := getFullyQualifiedInspectorKey(actualKey, path)

	return rm.redactAndFilterData(redactionStrategy, value, inspectorKey)
}

func (rm *regexMatcher) MatchKeyRegexs(keyToMatch string, path string) (bool, *CompiledRegex) {
	for _, r := range rm.keyRegexs {
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

func compileRegexs(regexs []Regex, defaultStrategy filters.RedactionStrategy) ([]CompiledRegex, error) {
	compiledRegexs := make([]CompiledRegex, len(regexs))
	for i, r := range regexs {
		cr, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, fmt.Errorf("error compiling key regex %s already specified", r.Pattern)
		}

		if r.RedactStrategy == filters.Unknown {
			r.RedactStrategy = defaultStrategy
		}

		compiledRegexs[i] = CompiledRegex{
			Regex:  r,
			Regexp: cr,
		}
	}

	return compiledRegexs, nil
}
