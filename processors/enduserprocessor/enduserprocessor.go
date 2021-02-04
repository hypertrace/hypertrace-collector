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

	var user *user
	switch enduser.Type {
	case "id":
		user = p.idCapture(value)
	case "role":
		user = p.roleCapture(value)
	case "scope":
		user = p.scopeCapture(value)
	case "authheader":
		user = p.authHeaderCapture(enduser, value)
	case "json":
		user = p.jsonCapture(enduser, value)
	case "urlencoded":
		user = p.urlencodedCapture(enduser, value)
	case "cookie":
		user = p.cookieCapture(enduser, value)
	default:
		p.logger.Warn("Unknown enduser type, skipping", zap.String("type", enduser.Type), zap.String("key", enduser.AttributeKey))
	}

	if user == nil {
		return
	}

	if len(user.id) > 0 {
		addSpanAttribute(span, enduserIDAttribute, user.id)
	}

	if len(user.role) > 0 {
		addSpanAttribute(span, enduserRoleAttribute, user.role)
	}

	if len(user.scope) > 0 {
		addSpanAttribute(span, enduserScopeAttribute, user.scope)
	}

	if len(user.session) > 0 {
		addSpanAttribute(span, enduserSessionAttribute, user.session)
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
		// TODO is it correct? it was `value == nil` before
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

type user struct {
	id      string
	role    string
	scope   string
	session string
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
		var user *user
		switch enduser.Encoding {
		case "jwt":
			user = p.decodeJwt(enduser, tokenString)
			// by default use the jwt token as the session id
			p.setSession(enduser, user, tokenString)
		default:
			p.logger.Info("Unknown auth header encoding", zap.String("encoding", enduser.Encoding))
			return nil
		}
		return user
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

		user := new(user)
		user.id = string(creds[0])
		return user
	} else {
		p.logger.Info("Authorization header must be basic or bearer", zap.String("value", value))
	}

	return nil
}

func (p *processor) jsonCapture(enduser config, value string) *user {
	v := p.getJSON(value)

	user := new(user)
	p.getUserIDFromPath(enduser, user, v)
	p.getUserRoleFromPath(enduser, user, v)
	p.getUserScopeFromPath(enduser, user, v)
	p.getUserSessionFromPath(enduser, user, v)
	return user
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

func (p *processor) getUserIDFromPath(enduser config, user *user, value interface{}) bool {
	for _, path := range enduser.IDPaths {
		if value, ok := p.getJSONElement(path, value); ok {
			user.id = value
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

	user := new(user)
	for _, claim := range enduser.IDClaims {
		if value, ok := claims[claim]; ok {
			if !p.getUserIDFromPath(enduser, user, value) {
				user.id = p.jsonToString(value)
			}
			break
		}
	}
	for _, claim := range enduser.RoleClaims {
		if value, ok := claims[claim]; ok {
			if !p.getUserRoleFromPath(enduser, user, value) {
				user.role = p.jsonToString(value)
			}
			break
		}
	}
	for _, claim := range enduser.ScopeClaims {
		if value, ok := claims[claim]; ok {
			if !p.getUserScopeFromPath(enduser, user, value) {
				user.scope = p.jsonToString(value)
			}
			break
		}
	}
	for _, claim := range enduser.SessionClaims {
		if value, ok := claims[claim]; ok {
			if !p.getUserSessionFromPath(enduser, user, value) {
				p.setSession(enduser, user, p.jsonToString(value))
			}
			break
		}
	}
	return user
}

func (p *processor) getUserRoleFromPath(enduser config, user *user, value interface{}) bool {
	for _, path := range enduser.RolePaths {
		if value, ok := p.getJSONElement(path, value); ok {
			user.role = value
			return true
		}
	}

	return false
}

func (p *processor) getUserScopeFromPath(enduser config, user *user, value interface{}) bool {
	for _, path := range enduser.ScopePaths {
		if value, ok := p.getJSONElement(path, value); ok {
			user.scope = value
			return true
		}
	}

	return false
}

func (p *processor) getUserSessionFromPath(enduser config, user *user, value interface{}) bool {
	for _, path := range enduser.SessionPaths {
		if value, ok := p.getJSONElement(path, value); ok {
			p.setSession(enduser, user, value)
			return true
		}
	}

	return false
}

func (p *processor) setSession(enduser config, user *user, value string) {
	// don't override an existing session value
	if len(user.session) > 0 {
		return
	}

	user.session = value
	if len(enduser.SessionSeparator) > 0 {
		user.session = formatSessionIdentifier(p.logger, enduser.SessionSeparator, enduser.SessionIndexes, value)
	}

	if !enduser.RawSessionValue {
		user.session = enduser.hashAlgorithm(user.session)
	}
}

func (p *processor) urlencodedCapture(enduser config, value string) *user {
	params, err := url.ParseQuery(value)
	if err != nil {
		p.logger.Info("Could not parse urlencoded to capture user", zap.Error(err))
	}

	user := new(user)
	for _, key := range enduser.IDKeys {
		if values, ok := params[key]; ok {
			for _, value := range values {
				if len(value) > 0 {
					user.id = value
					break
				}
			}
		}
		if len(user.id) > 0 {
			break
		}
	}
	for _, key := range enduser.RoleKeys {
		if values, ok := params[key]; ok {
			for _, value := range values {
				if len(value) > 0 {
					user.role = value
				}
			}
		}
		if len(user.role) > 0 {
			break
		}
	}
	for _, key := range enduser.ScopeKeys {
		if values, ok := params[key]; ok {
			for _, value := range values {
				if len(value) > 0 {
					user.scope = value
				}
			}
		}
		if len(user.scope) > 0 {
			break
		}
	}
	for _, key := range enduser.SessionKeys {
		if values, ok := params[key]; ok {
			for _, value := range values {
				if len(value) > 0 {
					p.setSession(enduser, user, value)
				}
			}
		}
		if len(user.session) > 0 {
			break
		}
	}

	return user
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

	user := new(user)
	for _, key := range enduser.IDKeys {
		for _, cookie := range cookies {
			if cookie.Name == key {
				user.id = cookie.Value
				break
			}
		}
		if len(user.id) > 0 {
			break
		}
	}
	for _, key := range enduser.RoleKeys {
		for _, cookie := range cookies {
			if cookie.Name == key {
				user.role = cookie.Value
				break
			}
		}
		if len(user.role) > 0 {
			break
		}
	}
	for _, key := range enduser.ScopeKeys {
		for _, cookie := range cookies {
			if cookie.Name == key {
				user.scope = cookie.Value
				break
			}
		}
		if len(user.scope) > 0 {
			break
		}
	}
	for _, key := range enduser.SessionKeys {
		for _, cookie := range cookies {
			if cookie.Name == key {
				p.setSession(enduser, user, cookie.Value)
				break
			}
		}
		if len(user.session) > 0 {
			break
		}
	}

	return user
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
