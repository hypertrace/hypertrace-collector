package cookie

import (
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/hypertrace/collector/processors"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"
)

const (
	headerCookie    = "http.request.header.cookie"
	headerSetCookie = "http.response.header.set-cookie"
)

var _ filters.Filter = (*cookieFilter)(nil)

type cookieFilter struct {
	m *regexmatcher.Matcher
}

func NewFilter(m *regexmatcher.Matcher) filters.Filter {
	return &cookieFilter{m}
}

func (f *cookieFilter) Name() string {
	return "cookie"
}

func (f *cookieFilter) RedactAttribute(key string, value pdata.AttributeValue) (*processors.ParsedAttribute, *filters.Attribute, error) {
	if len(value.StringVal()) == 0 {
		return nil, nil, nil
	}

	cookies := parseCookies(key, value.StringVal())
	if cookies == nil {
		return nil, nil, filters.WrapError(filters.ErrUnprocessableValue, "no cookie values")
	}

	parsedAttr := &processors.ParsedAttribute{
		Redacted:  map[string]string{},
		Flattened: map[string]string{},
	}

	var attr *filters.Attribute
	for _, cookie := range cookies {
		fqn := fmt.Sprintf("%s.%s", key, cookie.Name)
		parsedAttr.Flattened[cookie.Name] = cookie.Value

		if isRedactedByKey, isSession, redactedValue := f.m.FilterKeyRegexs(cookie.Name, key, cookie.Value, cookie.Name); isRedactedByKey {
			if isSession {
				attr = &filters.Attribute{
					Key:   "session.id",
					Value: pdata.NewAttributeValueString(redactedValue),
				}
			}
			parsedAttr.Redacted[fqn] = cookie.Value
			cookie.Value = redactedValue
		} else if isRedactedByValue, redactedValue := f.m.FilterStringValueRegexs(cookie.Value, key, cookie.Name); isRedactedByValue {
			parsedAttr.Redacted[fqn] = cookie.Value
			cookie.Value = redactedValue
		}
	}

	if len(parsedAttr.Redacted) > 0 {
		value.SetStringVal(stitchCookies(cookies))
	}

	return parsedAttr, attr, nil
}

func parseCookies(key string, value string) []*http.Cookie {
	unindexedKey := strings.Split(key, "[")[0]
	switch unindexedKey {
	case headerCookie:
		header := http.Header{"Cookie": {value}}
		request := http.Request{Header: header}
		return request.Cookies()

	case headerSetCookie:
		header := http.Header{"Set-Cookie": {value}}
		response := http.Response{Header: header}
		return response.Cookies()
	}
	return nil
}

func stitchCookies(cookies []*http.Cookie) string {
	cookieStrSlice := make([]string, len(cookies))
	for idx, cookie := range cookies {
		cookieStrSlice[idx] = cookie.String()
	}
	return strings.Join(cookieStrSlice, "; ")
}
