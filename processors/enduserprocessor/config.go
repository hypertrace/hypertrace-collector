package enduserprocessor

import "go.opentelemetry.io/collector/config/configmodels"

// Config defines config for the end user.
type Config struct {
	configmodels.ProcessorSettings `mapstructure:",squash"`

	EndUserConfig []EndUser `mapstructure:"end_users"`
}

type EndUser struct {
	AttributeKey     string      `mapstructure:"key"`
	Type             string      `mapstructure:"type"`
	Encoding         string      `mapstructure:"encoding"`
	CookieName       string      `mapstructure:"cookie_name"`
	RawSessionValue  bool        `mapstructure:"raw_session_value"`
	HashAlgo         string      `mapstructure:"hash_algo"`
	Conditions       []Condition `mapstructure:"conditions"`
	IDClaims         []string    `mapstructure:"id_claims"`
	IDPaths          []string    `mapstructure:"id_paths"`
	IDKeys           []string    `mapstructure:"id_keys"`
	RoleClaims       []string    `mapstructure:"role_claims"`
	RolePaths        []string    `mapstructure:"role_paths"`
	RoleKeys         []string    `mapstructure:"role_keys"`
	ScopeClaims      []string    `mapstructure:"scope_claims"`
	ScopePaths       []string    `mapstructure:"scope_paths"`
	ScopeKeys        []string    `mapstructure:"scope_keys"`
	SessionClaims    []string    `mapstructure:"session_claims"`
	SessionPaths     []string    `mapstructure:"session_paths"`
	SessionKeys      []string    `mapstructure:"session_keys"`
	SessionIndexes   []int       `mapstructure:"session_indexes"`
	SessionSeparator string      `mapstructure:"session_separator"`
}

// Condition is the condition that must be matched
// before trying to capture the user
type Condition struct {
	Key   string `json:"key"`
	Regex string `json:"regex"`
}
