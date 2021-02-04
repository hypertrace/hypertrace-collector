package enduserprocessor

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	assert.NoError(t, err)

	factories.Processors[typeStr] = NewFactory()

	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	endUserCfg := cfg.Processors[typeStr].(*Config)
	assert.Equal(t, 7, len(endUserCfg.EndUserConfig))

	endUser := endUserCfg.EndUserConfig[0]
	assert.Equal(t, "http.request.header.authorization", endUser.AttributeKey)
	assert.Equal(t, "authheader", endUser.Type)
	assert.Equal(t, "jwt", endUser.Encoding)
	assert.Equal(t, []string{"sub"}, endUser.IDClaims)
	assert.Equal(t, []string{"role"}, endUser.RoleClaims)
	assert.Equal(t, []string{"scope"}, endUser.ScopeClaims)

	endUser = endUserCfg.EndUserConfig[1]
	assert.Equal(t, "http.request.header.cookie", endUser.AttributeKey)
	assert.Equal(t, "cookie", endUser.Type)
	assert.Equal(t, "token", endUser.CookieName)
	assert.Equal(t, "jwt", endUser.Encoding)
	assert.Equal(t, []string{"sub"}, endUser.IDClaims)
	assert.Equal(t, []string{"role"}, endUser.RoleClaims)
	assert.Equal(t, []string{"scope"}, endUser.ScopeClaims)

	endUser = endUserCfg.EndUserConfig[2]
	assert.Equal(t, "http.request.header.authorization", endUser.AttributeKey)
	assert.Equal(t, "authheader", endUser.Type)
	assert.Equal(t, "jwt", endUser.Encoding)
	assert.Equal(t, []int{0, 1}, endUser.SessionIndexes)
	assert.Equal(t, ".", endUser.SessionSeparator)
	assert.Equal(t, []string{"data"}, endUser.IDClaims)
	assert.Equal(t, []string{"$.id"}, endUser.IDPaths)
	assert.Equal(t, []string{"data"}, endUser.RoleClaims)
	assert.Equal(t, []string{"$.role"}, endUser.RolePaths)
	assert.Equal(t, []string{"data"}, endUser.ScopeClaims)
	assert.Equal(t, []string{"$.scope"}, endUser.ScopePaths)

	endUser = endUserCfg.EndUserConfig[3]
	assert.Equal(t, "http.response.body", endUser.AttributeKey)
	assert.Equal(t, "json", endUser.Type)
	assert.Equal(t, []string{"$.user.id"}, endUser.IDPaths)
	assert.Equal(t, []string{"$.user.role"}, endUser.RolePaths)
	assert.Equal(t, []string{"$.user.scope"}, endUser.ScopePaths)
	assert.Equal(t, []string{"$.token"}, endUser.SessionPaths)
	assert.Equal(t, 1, len(endUser.Conditions))
	assert.Equal(t, "http.url", endUser.Conditions[0].Key)
	assert.Equal(t, "\\/login", endUser.Conditions[0].Regex)

	endUser = endUserCfg.EndUserConfig[4]
	assert.Equal(t, "http.request.header.x-user", endUser.AttributeKey)
	assert.Equal(t, "id", endUser.Type)

	endUser = endUserCfg.EndUserConfig[5]
	assert.Equal(t, "http.request.header.x-role", endUser.AttributeKey)
	assert.Equal(t, "role", endUser.Type)

	endUser = endUserCfg.EndUserConfig[6]
	assert.Equal(t, "http.request.header.x-scope", endUser.AttributeKey)
	assert.Equal(t, "scope", endUser.Type)
}
