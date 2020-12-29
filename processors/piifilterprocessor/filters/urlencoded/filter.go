package urlencoded

import (
	"fmt"
	"net/url"

	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/internal/matcher"
	"go.opentelemetry.io/collector/consumer/pdata"
)

type urlEncodedFilter struct {
	m matcher.Matcher
}

const urlAttributeStr = "http.url"

func (f *urlEncodedFilter) RedactAttribute(key string, value pdata.AttributeValue) (bool, error) {
	if len(value.StringVal()) == 0 {
		return false, nil
	}

	var u *url.URL
	var err error

	rawString := value.StringVal()
	isURLAttr := key == urlAttributeStr
	if isURLAttr {
		u, err = url.Parse(value.StringVal())
		if err != nil {
			return false, err
		}
		rawString = u.RawQuery
	}

	params, err := url.ParseQuery(rawString)
	if err != nil {
		return false, err
	}

	v := url.Values{}
	var isRedacted bool
	for param, values := range params {
		for idx, value := range values {
			path := param
			if !isURLAttr {
				if len(values) > 1 {
					path = fmt.Sprintf("$.%s[%d]", param, idx)
				} else {
					path = fmt.Sprintf("$.%s", param)
				}
			}

			if isRedactedByKey, redactedValue := f.m.FilterKeyRegexs(param, key, value, path); isRedactedByKey {
				isRedacted = true
				v.Add(param, redactedValue)
			} else if isRedactedByValue, redactedValue := f.m.FilterStringValueRegexs(value, key, path); isRedactedByValue {
				isRedacted = true
				v.Add(param, redactedValue)
			} else {
				v.Add(param, value)
			}
		}
	}

	if isRedacted {
		encoded := v.Encode()
		if isURLAttr {
			u.RawQuery = encoded
			value.SetStringVal(u.String())
		} else {
			value.SetStringVal(encoded)
		}
	}

	return isRedacted, nil
}
