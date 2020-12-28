package cookie

import (
	"net/http"
	"strings"

	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/internal/matcher"
	"go.opentelemetry.io/collector/consumer/pdata"
)

var _ filters.Filter = (*cookieFilter)(nil)

type cookieFilter struct {
	m  matcher.Matcher
	rs filters.RedactionStrategy
}

func (f *cookieFilter) RedactAttribute(key string, value pdata.AttributeValue) (bool, error) {
	if len(value.StringVal()) == 0 {
		return false, nil
	}

	cookies := parseCookies(key, value.StringVal())
	if cookies == nil {
		return false, nil
	}

	isRedacted := false
	for _, cookie := range cookies {
		if isRedactedByKey, redactedValue := f.m.FilterKeyRegexs(cookie.Name, key, cookie.Value, cookie.Name); isRedactedByKey {
			cookie.Value = redactedValue
			isRedacted = true
		} else if isRedactedByValue, redactedValue := f.m.FilterStringValueRegexs(cookie.Value, key, cookie.Name); isRedactedByValue {
			cookie.Value = redactedValue
			isRedacted = true
		}
	}

	if isRedacted {
		value.SetStringVal(stitchCookies(cookies))
	}

	return isRedacted, nil
}

func parseCookies(key string, value string) []*http.Cookie {
	unindexedKey := strings.Split(key, "[")[0]
	switch unindexedKey {
	case "http.request.header.cookie":
		header := http.Header{"Cookie": {value}}
		request := http.Request{Header: header}
		return request.Cookies()

	case "http.response.header.set-cookie":
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
