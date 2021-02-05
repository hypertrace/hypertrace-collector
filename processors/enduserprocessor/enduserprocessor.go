package enduserprocessor

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PaesslerAG/jsonpath"
	"github.com/dgrijalva/jwt-go"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"

	"github.com/hypertrace/collector/processors/enduserprocessor/hash"
	"github.com/hypertrace/collector/processors/piifilterprocessor"
)

const (
	enduserIDAttribute      = "enduser.id"
	enduserRoleAttribute    = "enduser.role"
	enduserScopeAttribute   = "enduser.scope"
	enduserSessionAttribute = "session.id"
)

type config struct {
	EndUser
	hashAlgorithm hash.Algorithm
}

type processor struct {
	logger             *zap.Logger
	attributeConfigMap map[string][]config
}

type user struct {
	id      string
	role    string
	scope   string
	session string
}

var _ processorhelper.TProcessor = (*processor)(nil)

func (p *processor) ProcessTraces(_ context.Context, traces pdata.Traces) (pdata.Traces, error) {
	rss := traces.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)

		ilss := rs.InstrumentationLibrarySpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)

			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)

				span.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
					unindexedKey := piifilterprocessor.UnindexedKey(k)
					if configs, ok := p.attributeConfigMap[unindexedKey]; ok {
						for _, cfg := range configs {
							p.capture(span, cfg, v.StringVal())
						}
					}
				})
			}
		}
	}

	return traces, nil
}

func (p *processor) capture(span pdata.Span, enduser config, value string) {
	if !p.passesConditions(span, enduser.Conditions) {
		return
	}

	var u *user
	switch enduser.Type {
	case "id":
		u = p.idCapture(value)
	case "role":
		u = p.roleCapture(value)
	case "scope":
		u = p.scopeCapture(value)
	case "authheader":
		u = p.authHeaderCapture(enduser, value)
	case "json":
		u = p.jsonCapture(enduser, value)
	case "urlencoded":
		u = p.urlencodedCapture(enduser, value)
	case "cookie":
		u = p.cookieCapture(enduser, value)
	default:
		p.logger.Warn("Unknown enduser type, skipping", zap.String("type", enduser.Type), zap.String("key", enduser.AttributeKey))
	}

	if u == nil {
		return
	}

	if len(u.id) > 0 {
		addSpanAttribute(span, enduserIDAttribute, u.id)
	}

	if len(u.role) > 0 {
		addSpanAttribute(span, enduserRoleAttribute, u.role)
	}

	if len(u.scope) > 0 {
		addSpanAttribute(span, enduserScopeAttribute, u.scope)
	}

	if len(u.session) > 0 {
		addSpanAttribute(span, enduserSessionAttribute, u.session)
	}
}

func addSpanAttribute(span pdata.Span, key string, value string) {
	attributes := span.Attributes()
	// don't overwrite existing attributes
	if _, ok := attributes.Get(key); ok {
		return
	}
	attributes.Insert(key, pdata.NewAttributeValueString(value))
}

// only capture the user info if all conditions
// are true.  If an conditions key does not exist
// in the span, that is considered a failed condition
func (p *processor) passesConditions(span pdata.Span, conditions []Condition) bool {
	attribMap := span.Attributes()

	for _, condition := range conditions {
		attrib, ok := attribMap.Get(condition.Key)
		if !ok {
			return false
		}

		value := attrib.StringVal()
		if value == "" {
			return false
		}

		matched, err := regexp.MatchString(condition.Regex, value)
		if err != nil {
			p.logger.Warn("Could not evaluate enduser condition", zap.Error(err))
			return false
		}
		if !matched {
			return false
		}
	}

	return true
}

func (p *processor) idCapture(value string) *user {
	return &user{id: value}
}

func (p *processor) roleCapture(value string) *user {
	return &user{role: value}
}

func (p *processor) scopeCapture(value string) *user {
	return &user{scope: value}
}

func (p *processor) authHeaderCapture(enduser config, value string) *user {
	lcValue := strings.ToLower(value)
	if strings.HasPrefix(lcValue, "bearer ") {
		tokenString := value[7:]
		var u *user
		switch enduser.Encoding {
		case "jwt":
			u = p.decodeJwt(enduser, tokenString)
			// by default use the jwt token as the session id
			p.setSession(enduser, u, tokenString)
		default:
			p.logger.Info("Unknown auth header encoding", zap.String("encoding", enduser.Encoding))
			return nil
		}
		return u
	} else if strings.HasPrefix(lcValue, "basic ") {
		token := value[6:]
		str, err := base64.StdEncoding.DecodeString(token)
		if err != nil {
			p.logger.Info("Could not decode basic token", zap.String("value", value))
			return nil
		}
		creds := bytes.SplitN(str, []byte(":"), 2)
		if len(creds) != 2 {
			p.logger.Info("Invalid basic token", zap.String("value", value))
			return nil
		}

		u := new(user)
		u.id = string(creds[0])
		return u
	} else {
		p.logger.Info("Authorization header must be basic or bearer", zap.String("value", value))
	}

	return nil
}

func (p *processor) jsonCapture(enduser config, value string) *user {
	v := p.getJSON(value)

	u := new(user)
	p.getUserIDFromPath(enduser, u, v)
	p.getUserRoleFromPath(enduser, u, v)
	p.getUserScopeFromPath(enduser, u, v)
	p.getUserSessionFromPath(enduser, u, v)
	return u
}

func (p *processor) getJSON(value string) interface{} {
	var v interface{}
	err := jsoniter.Config{
		EscapeHTML:              false,
		MarshalFloatWith6Digits: false,
		ValidateJsonRawMessage:  false,
	}.Froze().UnmarshalFromString(value, &v)
	// if there's an error parsing the json, log it for debuggin, but carry on
	// as we can usually still extract the user info from truncated json.
	if err != nil {
		p.logger.Debug("Could not parse json to capture user", zap.Error(err))
	}

	return v
}

func (p *processor) getUserIDFromPath(enduser config, u *user, value interface{}) bool {
	for _, path := range enduser.IDPaths {
		if value, ok := p.getJSONElement(path, value); ok {
			u.id = value
			return true
		}
	}

	return false
}

func (p *processor) getJSONElement(path string, json interface{}) (string, bool) {
	elem, err := jsonpath.Get(path, json)
	if err == nil {
		return p.jsonToString(elem), true
	}

	return "", false
}

func (p *processor) jsonToString(value interface{}) string {
	// only unmarshal the value if it's a complex type, as we don't
	// want all string values to be quoted
	valueString, ok := value.(string)
	if ok {
		return valueString
	}

	json, err := json.Marshal(value)
	if err != nil {
		p.logger.Info("invalid json value", zap.Error(err))
		return ""
	}

	return string(json)
}

func (p *processor) decodeJwt(enduser config, tokenString string) *user {
	claims := jwt.MapClaims{}

	// if the jwt only has two parts, it may be because the signature is
	// stored elsewhere.  just add a dummy signature to the end to get
	// past the ParseUnverified call which doesn't make use of the
	// signature but expects it to be there.
	parts := strings.Split(tokenString, ".")
	if len(parts) == 2 {
		tokenString += ".dummy_sig"
	}

	_, _, err := new(jwt.Parser).ParseUnverified(tokenString, claims)
	if err != nil {
		p.logger.Debug("Couldn't parse jwt", zap.Error(err))
		return new(user)
	}

	u := new(user)
	for _, claim := range enduser.IDClaims {
		if value, ok := claims[claim]; ok {
			if !p.getUserIDFromPath(enduser, u, value) {
				u.id = p.jsonToString(value)
			}
			break
		}
	}
	for _, claim := range enduser.RoleClaims {
		if value, ok := claims[claim]; ok {
			if !p.getUserRoleFromPath(enduser, u, value) {
				u.role = p.jsonToString(value)
			}
			break
		}
	}
	for _, claim := range enduser.ScopeClaims {
		if value, ok := claims[claim]; ok {
			if !p.getUserScopeFromPath(enduser, u, value) {
				u.scope = p.jsonToString(value)
			}
			break
		}
	}
	for _, claim := range enduser.SessionClaims {
		if value, ok := claims[claim]; ok {
			if !p.getUserSessionFromPath(enduser, u, value) {
				p.setSession(enduser, u, p.jsonToString(value))
			}
			break
		}
	}
	return u
}

func (p *processor) getUserRoleFromPath(enduser config, u *user, value interface{}) bool {
	for _, path := range enduser.RolePaths {
		if value, ok := p.getJSONElement(path, value); ok {
			u.role = value
			return true
		}
	}

	return false
}

func (p *processor) getUserScopeFromPath(enduser config, u *user, value interface{}) bool {
	for _, path := range enduser.ScopePaths {
		if val, ok := p.getJSONElement(path, value); ok {
			u.scope = val
			return true
		}
	}

	return false
}

func (p *processor) getUserSessionFromPath(enduser config, u *user, value interface{}) bool {
	for _, path := range enduser.SessionPaths {
		if val, ok := p.getJSONElement(path, value); ok {
			p.setSession(enduser, u, val)
			return true
		}
	}

	return false
}

func (p *processor) setSession(enduser config, u *user, value string) {
	// don't override an existing session value
	if len(u.session) > 0 {
		return
	}

	u.session = value
	if len(enduser.SessionSeparator) > 0 {
		u.session = formatSessionIdentifier(p.logger, enduser.SessionSeparator, enduser.SessionIndexes, value)
	}

	if !enduser.RawSessionValue {
		u.session = enduser.hashAlgorithm(u.session)
	}
}

func (p *processor) urlencodedCapture(enduser config, val string) *user {
	params, err := url.ParseQuery(val)
	if err != nil {
		p.logger.Info("Could not parse urlencoded to capture user", zap.Error(err))
	}

	u := new(user)
	for _, key := range enduser.IDKeys {
		if values, ok := params[key]; ok {
			for _, value := range values {
				if len(value) > 0 {
					u.id = value
					break
				}
			}
		}
		if len(u.id) > 0 {
			break
		}
	}
	for _, key := range enduser.RoleKeys {
		if values, ok := params[key]; ok {
			for _, value := range values {
				if len(value) > 0 {
					u.role = value
				}
			}
		}
		if len(u.role) > 0 {
			break
		}
	}
	for _, key := range enduser.ScopeKeys {
		if values, ok := params[key]; ok {
			for _, value := range values {
				if len(value) > 0 {
					u.scope = value
				}
			}
		}
		if len(u.scope) > 0 {
			break
		}
	}
	for _, key := range enduser.SessionKeys {
		if values, ok := params[key]; ok {
			for _, value := range values {
				if len(value) > 0 {
					p.setSession(enduser, u, value)
				}
			}
		}
		if len(u.session) > 0 {
			break
		}
	}

	return u
}

func (p *processor) cookieCapture(enduser config, value string) *user {
	header := http.Header{}
	header.Add("Cookie", value)
	request := http.Request{Header: header}
	cookies := request.Cookies()

	if len(enduser.Encoding) > 0 {
		switch enduser.Encoding {
		case "jwt":
			if len(enduser.CookieName) > 0 {
				for _, cookie := range cookies {
					if cookie.Name == enduser.CookieName {
						user := p.decodeJwt(enduser, cookie.Value)
						if user == nil {
							return nil
						}
						// use the jwt cookie as the session string
						p.setSession(enduser, user, cookie.Value)
						return user
					}
				}
			} else {
				p.logger.Info("cookie_name must be specified when using jwt encoding")
				return nil
			}
		default:
			p.logger.Info("Unknown cookie encoding", zap.String("encoding", enduser.Encoding))
			return nil
		}
	}

	u := new(user)
	for _, key := range enduser.IDKeys {
		for _, cookie := range cookies {
			if cookie.Name == key {
				u.id = cookie.Value
				break
			}
		}
		if len(u.id) > 0 {
			break
		}
	}
	for _, key := range enduser.RoleKeys {
		for _, cookie := range cookies {
			if cookie.Name == key {
				u.role = cookie.Value
				break
			}
		}
		if len(u.role) > 0 {
			break
		}
	}
	for _, key := range enduser.ScopeKeys {
		for _, cookie := range cookies {
			if cookie.Name == key {
				u.scope = cookie.Value
				break
			}
		}
		if len(u.scope) > 0 {
			break
		}
	}
	for _, key := range enduser.SessionKeys {
		for _, cookie := range cookies {
			if cookie.Name == key {
				p.setSession(enduser, u, cookie.Value)
				break
			}
		}
		if len(u.session) > 0 {
			break
		}
	}

	return u
}

func formatSessionIdentifier(logger *zap.Logger, separator string, indexes []int, value string) string {
	parts := strings.Split(value, separator)
	var session string
	for i, index := range indexes {
		if index >= len(parts) {
			logger.Debug("Session index greater than number parts", zap.Int("index", index), zap.Int("parts", len(parts)))
			break
		}
		if i > 0 {
			session += separator
		}
		session += parts[index]
	}
	return session
}
