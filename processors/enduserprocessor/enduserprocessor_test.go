package enduserprocessor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/hypertrace/collector/processors/enduserprocessor/hash"
)

var (
	hmacSecret = []byte("123")
)

func Test_enduser_authHeader_bearer(t *testing.T) {
	hashAlgorithm, ok := hash.ResolveHashAlgorithm("SHA-1")
	require.True(t, ok)
	cfg := config{
		EndUser: EndUser{
			AttributeKey: "http.request.header.authorization",
			Type:         "authheader",
			Encoding:     "jwt",
			IDClaims:     []string{"sub"},
			RoleClaims:   []string{"role"},
			ScopeClaims:  []string{"scope"},
		},
		hashAlgorithm: hashAlgorithm,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   "dave",
		"role":  "user",
		"scope": "traceable",
	})
	tokenString, err := token.SignedString(hmacSecret)
	require.Nil(t, err)

	p := processor{
		logger:             zap.NewNop(),
		attributeConfigMap: map[string][]config{cfg.AttributeKey: {cfg}},
	}

	u := p.authHeaderCapture(cfg, "Bearer "+tokenString)
	assert.Equal(t, &user{id: "dave", role: "user", scope: "traceable", session: hashAlgorithm(tokenString)}, u)
}

func Test_enduser_authHeader_bearerNoSignature(t *testing.T) {
	hashAlgorithm, ok := hash.ResolveHashAlgorithm("SHA-1")
	require.True(t, ok)
	cfg := config{
		EndUser: EndUser{
			AttributeKey: "http.request.header.authorization",
			Type:         "authheader",
			Encoding:     "jwt",
			IDClaims:     []string{"sub"},
			RoleClaims:   []string{"role"},
			ScopeClaims:  []string{"scope"},
		},
		hashAlgorithm: hashAlgorithm,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   "dave",
		"role":  "user",
		"scope": "traceable",
	})
	tokenString, err := token.SignedString(hmacSecret)
	assert.Nil(t, err)

	p := processor{
		logger:             zap.NewNop(),
		attributeConfigMap: map[string][]config{cfg.AttributeKey: {cfg}},
	}

	tokenParts := strings.Split(tokenString, ".")
	truncatedToken := fmt.Sprintf("%s.%s", tokenParts[0], tokenParts[1])
	u := p.authHeaderCapture(cfg, "Bearer "+truncatedToken)
	assert.Equal(t, &user{id: "dave", role: "user", scope: "traceable", session: hashAlgorithm(truncatedToken)}, u)
}

func Test_enduser_authHeader_complexClaim(t *testing.T) {
	hashAlgorithm, ok := hash.ResolveHashAlgorithm("SHA-1")
	require.True(t, ok)
	cfg := config{
		EndUser: EndUser{
			AttributeKey: "http.request.header.authorization",
			Type:         "authheader",
			Encoding:     "jwt",
			RoleClaims:   []string{"role"},
		},
		hashAlgorithm: hashAlgorithm,
	}

	var complexRole interface{}
	err := json.Unmarshal([]byte(`{"role": {"a": ["b", "c"]}}`), &complexRole)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"role": complexRole,
	})
	tokenString, err := token.SignedString(hmacSecret)
	assert.Nil(t, err)

	p := processor{
		logger:             zap.NewNop(),
		attributeConfigMap: map[string][]config{cfg.AttributeKey: {cfg}},
	}

	u := p.authHeaderCapture(cfg, "Bearer "+tokenString)
	assert.Equal(t, &user{role: `{"role":{"a":["b","c"]}}`, session: hashAlgorithm(tokenString)}, u)
}

func Test_enduser_authHeader_complexClaimPath(t *testing.T) {
	hashAlgorithm, ok := hash.ResolveHashAlgorithm("SHA-1")
	require.True(t, ok)
	cfg := config{
		EndUser: EndUser{
			AttributeKey:  "http.request.header.authorization",
			Type:          "authheader",
			Encoding:      "jwt",
			IDClaims:      []string{"data"},
			IDPaths:       []string{"$.uuid"},
			RoleClaims:    []string{"data"},
			RolePaths:     []string{"$.role"},
			ScopeClaims:   []string{"data"},
			ScopePaths:    []string{"$.scope"},
			SessionClaims: []string{"data"},
			SessionPaths:  []string{"$.token"},
		},
		hashAlgorithm: hashAlgorithm,
	}

	var complexID interface{}
	err := json.Unmarshal([]byte(`{"uuid": "dave", "role": "user", "scope": "traceable", "token": "abc"}`), &complexID)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"data": complexID,
	})

	tokenString, err := token.SignedString(hmacSecret)
	require.Nil(t, err)

	p := processor{
		logger:             zap.NewNop(),
		attributeConfigMap: map[string][]config{cfg.AttributeKey: {cfg}},
	}

	u := p.authHeaderCapture(cfg, "Bearer "+tokenString)
	assert.Equal(t, &user{id: "dave", role: "user", scope: "traceable", session: hashAlgorithm("abc")}, u)
}

func Test_enduser_authHeader_basic(t *testing.T) {
	cfg := config{
		EndUser: EndUser{
			AttributeKey: "http.request.header.authorization",
			Type:         "authheader",
		},
	}

	auth := "dave:pw123"
	tokenString := base64.StdEncoding.EncodeToString([]byte(auth))

	p := processor{
		logger:             zap.NewNop(),
		attributeConfigMap: map[string][]config{cfg.AttributeKey: {cfg}},
	}

	u := p.authHeaderCapture(cfg, "Basic "+tokenString)
	assert.Equal(t, &user{id: "dave"}, u)
}

func Test_enduser_id(t *testing.T) {
	cfg := config{
		EndUser: EndUser{
			AttributeKey: "http.request.header.x-user",
			Type:         "id",
		}}

	p := processor{
		logger:             zap.NewNop(),
		attributeConfigMap: map[string][]config{cfg.AttributeKey: {cfg}},
	}

	u := p.idCapture("dave")
	assert.NotNil(t, user{
		id: "dave",
	}, u)
}

func Test_enduser_role(t *testing.T) {
	cfg := config{
		EndUser: EndUser{
			AttributeKey: "http.request.header.x-role",
			Type:         "role",
		},
	}

	p := processor{
		logger:             zap.NewNop(),
		attributeConfigMap: map[string][]config{cfg.AttributeKey: {cfg}},
	}

	u := p.roleCapture("user")
	assert.Equal(t, &user{role: "user"}, u)
}

func Test_enduser_scope(t *testing.T) {
	cfg := config{
		EndUser: EndUser{
			AttributeKey: "http.request.header.x-scope",
			Type:         "scope",
		}}

	p := processor{
		logger:             zap.NewNop(),
		attributeConfigMap: map[string][]config{cfg.AttributeKey: {cfg}},
	}

	u := p.scopeCapture("traceable")
	assert.Equal(t, &user{scope: "traceable"}, u)
}

func Test_enduser_json(t *testing.T) {
	hashAlgorithm, ok := hash.ResolveHashAlgorithm("SHA-1")
	require.True(t, ok)
	cfg := config{
		EndUser: EndUser{
			AttributeKey: "http.response.body",
			Type:         "json",
			IDPaths:      []string{"$.userInfo.name"},
			RolePaths:    []string{"$.userInfo.role"},
			ScopePaths:   []string{"$.userInfo.scope"},
			SessionPaths: []string{"$.token"},
		},
		hashAlgorithm: hashAlgorithm,
	}

	jsonStr := `
	{
			"userInfo": {
					"name": "dave",
					"role": "user",
					"scope": "traceable"
			},
			"token": "abc"
	}`

	p := processor{
		logger:             zap.NewNop(),
		attributeConfigMap: map[string][]config{cfg.AttributeKey: {cfg}},
	}

	u := p.jsonCapture(cfg, jsonStr)
	assert.Equal(t, &user{id: "dave", role: "user", scope: "traceable", session: hashAlgorithm("abc")}, u)
}

func Test_enduser_complexJson(t *testing.T) {
	hashAlgorithm, ok := hash.ResolveHashAlgorithm("SHA-1")
	require.True(t, ok)
	cfg := config{
		EndUser: EndUser{
			AttributeKey: "http.response.body",
			Type:         "json",
			IDPaths:      []string{"$.userInfo.name"},
			RolePaths:    []string{"$.userInfo.role"},
			ScopePaths:   []string{"$.userInfo.scope"},
			SessionPaths: []string{"$.token"},
		},
		hashAlgorithm: hashAlgorithm,
	}
	jsonStr := `
	{
			"userInfo": {
					"name": "dave",
					"role": ["user", "admin"],
					"scope": "traceable"
			},
			"token": "abc"
	}`

	p := processor{
		logger:             zap.NewNop(),
		attributeConfigMap: map[string][]config{cfg.AttributeKey: {cfg}},
	}

	u := p.jsonCapture(cfg, jsonStr)
	assert.Equal(t, &user{id: "dave", role: `["user","admin"]`, scope: "traceable", session: hashAlgorithm("abc")}, u)
}

func Test_enduser_truncatedJson(t *testing.T) {
	hashAlgorithm, ok := hash.ResolveHashAlgorithm("SHA-1")
	require.True(t, ok)
	cfg := config{
		EndUser: EndUser{
			AttributeKey: "http.response.body",
			Type:         "json",
			IDPaths:      []string{"$.userInfo.name"},
			RolePaths:    []string{"$.userInfo.role"},
			ScopePaths:   []string{"$.userInfo.scope"},
			SessionPaths: []string{"$.token"},
		}, hashAlgorithm: hashAlgorithm}
	jsonStr := `
	{
			"userInfo": {
					"name": "dave",
					"role": ["user", "admin"],
					"scope": "traceable"
			},
			"token": "abc"
	` // note the missing }

	p := processor{
		logger:             zap.NewNop(),
		attributeConfigMap: map[string][]config{cfg.AttributeKey: {cfg}},
	}

	u := p.jsonCapture(cfg, jsonStr)
	assert.Equal(t, &user{id: "dave", role: `["user","admin"]`, scope: "traceable", session: hashAlgorithm("abc")}, u)
}

func Test_enduser_urlencoded(t *testing.T) {
	hashAlgorithm, ok := hash.ResolveHashAlgorithm("SHA-1")
	require.True(t, ok)
	cfg := config{
		EndUser: EndUser{
			AttributeKey: "http.response.body",
			Type:         "urlencoded",
			IDKeys:       []string{"name"},
			RoleKeys:     []string{"role"},
			ScopeKeys:    []string{"scope"},
			SessionKeys:  []string{"session"},
		}, hashAlgorithm: hashAlgorithm}

	v := url.Values{}
	v.Add("name", "dave")
	v.Add("role", "user")
	v.Add("scope", "traceable")
	v.Add("session", "abc")

	p := processor{
		logger:             zap.NewNop(),
		attributeConfigMap: map[string][]config{cfg.AttributeKey: {cfg}},
	}

	u := p.urlencodedCapture(cfg, v.Encode())
	assert.Equal(t, &user{id: "dave", role: "user", scope: "traceable", session: hashAlgorithm("abc")}, u)
}

func Test_enduser_cookie(t *testing.T) {
	hashAlgorithm, ok := hash.ResolveHashAlgorithm("SHA-1")
	require.True(t, ok)
	cfg := config{
		EndUser: EndUser{
			AttributeKey: "http.response.header.set-cookie",
			Type:         "cookie",
			IDKeys:       []string{"name"},
			RoleKeys:     []string{"role"},
			ScopeKeys:    []string{"scope"},
			SessionKeys:  []string{"session"},
		}, hashAlgorithm: hashAlgorithm}

	cookieStr := "name=dave;role=user;scope=traceable;session=abc"

	p := processor{
		logger:             zap.NewNop(),
		attributeConfigMap: map[string][]config{cfg.AttributeKey: {cfg}},
	}

	u := p.cookieCapture(cfg, cookieStr)
	assert.Equal(t, &user{id: "dave", role: "user", scope: "traceable", session: hashAlgorithm("abc")}, u)
}

func Test_enduser_cookieJwt(t *testing.T) {
	hashAlgorithm, ok := hash.ResolveHashAlgorithm("SHA-1")
	require.True(t, ok)
	cfg := config{
		EndUser: EndUser{
			AttributeKey: "http.request.header.cookie",
			Type:         "cookie",
			CookieName:   "token",
			Encoding:     "jwt",
			IDClaims:     []string{"sub"},
			RoleClaims:   []string{"role"},
			ScopeClaims:  []string{"scope"},
		}, hashAlgorithm: hashAlgorithm}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   "dave",
		"role":  "user",
		"scope": "traceable",
	})
	tokenString, err := token.SignedString(hmacSecret)
	require.NoError(t, err)
	cookieStr := "otherCookie=abc;token=" + tokenString

	p := processor{
		logger:             zap.NewNop(),
		attributeConfigMap: map[string][]config{cfg.AttributeKey: {cfg}},
	}

	u := p.cookieCapture(cfg, cookieStr)
	assert.Equal(t, &user{id: "dave", role: "user", scope: "traceable", session: hashAlgorithm(tokenString)}, u)
}

func Test_enduser_condition(t *testing.T) {
	cfg := config{
		EndUser: EndUser{
			AttributeKey: "http.response.body",
			Type:         "json",
			Conditions: []Condition{{
				Key:   "http.url",
				Regex: "login",
			}},
			IDPaths: []string{"$.userInfo.name"},
		}}

	jsonMatchStr := `
	{
			"userInfo": {
					"name": "match_name"
	}`

	jsonNoMatchStr := `
	{
			"userInfo": {
					"name": "no_match_name"
			}
	}`

	td := pdata.NewTraces()
	td.ResourceSpans().Resize(1)
	td.ResourceSpans().At(0).InstrumentationLibrarySpans().Resize(1)
	td.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().Resize(2)
	spans := td.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans()
	spans.At(0).SetName("span_match")
	spans.At(0).Attributes().Insert("http.response.body", pdata.NewAttributeValueString(jsonMatchStr))
	spans.At(0).Attributes().Insert("http.url", pdata.NewAttributeValueString("http://localhost/login"))
	spans.At(1).SetName("span_match")
	spans.At(1).Attributes().Insert("http.response.body", pdata.NewAttributeValueString(jsonNoMatchStr))
	spans.At(1).Attributes().Insert("http.url", pdata.NewAttributeValueString("http://localhost/foo"))

	p := processor{
		logger:             zap.NewNop(),
		attributeConfigMap: map[string][]config{cfg.AttributeKey: {cfg}},
	}
	td, err := p.ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	attr, ok := td.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().At(0).Attributes().Get("enduser.id")
	require.True(t, ok)
	assert.Equal(t, "match_name", attr.StringVal())
	_, ok = td.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().At(1).Attributes().Get("enduser.id")
	require.False(t, ok)
}

func Test_enduser_authHeader_sessionIndexes(t *testing.T) {
	hashAlgorithm, ok := hash.ResolveHashAlgorithm("SHA-1")
	require.True(t, ok)
	cfg := config{
		EndUser: EndUser{
			AttributeKey:     "http.request.header.authorization",
			Type:             "authheader",
			Encoding:         "jwt",
			SessionSeparator: ".",
			SessionIndexes:   []int{0, 2},
		}, hashAlgorithm: hashAlgorithm}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   "dave",
		"role":  "user",
		"scope": "traceable",
	})
	tokenString, err := token.SignedString(hmacSecret)
	tokenParts := strings.Split(tokenString, ".")
	assert.Nil(t, err)

	u := &user{}
	p := processor{
		logger:             zap.NewNop(),
		attributeConfigMap: map[string][]config{cfg.AttributeKey: {cfg}},
	}
	p.setSession(cfg, u, tokenString)
	assert.Equal(t, &user{session: hashAlgorithm(tokenParts[0] + "." + tokenParts[2])}, u)
}

func Test_enduser_shake256(t *testing.T) {
	hashAlgorithm, ok := hash.ResolveHashAlgorithm("SHAKE256")
	require.True(t, ok)
	cfg := config{
		EndUser: EndUser{
			HashAlgo:     "SHAKE256",
			AttributeKey: "http.request.header.authorization",
			Type:         "authheader",
			Encoding:     "jwt",
			IDClaims:     []string{"sub"},
			RoleClaims:   []string{"role"},
			ScopeClaims:  []string{"scope"},
		}, hashAlgorithm: hashAlgorithm}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   "dave",
		"role":  "user",
		"scope": "traceable",
	})
	tokenString, err := token.SignedString(hmacSecret)
	assert.Nil(t, err)

	p := processor{
		logger:             zap.NewNop(),
		attributeConfigMap: map[string][]config{cfg.AttributeKey: {cfg}},
	}
	u := p.authHeaderCapture(cfg, "Bearer "+tokenString)
	assert.Equal(t, &user{id: "dave", role: "user", scope: "traceable", session: hashAlgorithm(tokenString)}, u)
}
