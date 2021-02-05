package enduserprocessor

import "go.opentelemetry.io/collector/config/configmodels"

// Config defines config for the end user processor.
// The end user processor parses span attributes (e.g. auth token, headers, body)
// and extracts user identification information - id, role, scope, session (hashed)
// as span attributes `enduser.id`, `enduser.role`, `enduser.scope` and `session.id`.
type Config struct {
	configmodels.ProcessorSettings `mapstructure:",squash"`

	EndUserConfig []EndUser `mapstructure:"end_users"`
}

type EndUser struct {
	AttributeKey    string `mapstructure:"key"`
	// Type - id, role, scope, authheader, json, urlencoded, cookie
	Type            string `mapstructure:"type"`
	// Encoding (e.g. jwt)
	Encoding        string `mapstructure:"encoding"`
	CookieName      string `mapstructure:"cookie_name"`
	Conditions       []Condition `mapstructure:"conditions"`
	// ID (e.g. jdoe) configuration.
	IDClaims         []string    `mapstructure:"id_claims"`
	IDPaths          []string    `mapstructure:"id_paths"`
	IDKeys           []string    `mapstructure:"id_keys"`
	// Role (e.g. user) configuration
	RoleClaims       []string    `mapstructure:"role_claims"`
	RolePaths        []string    `mapstructure:"role_paths"`
	RoleKeys         []string    `mapstructure:"role_keys"`
	// Scope (e.g. hypertrace) configuration.
	ScopeClaims      []string    `mapstructure:"scope_claims"`
	ScopePaths       []string    `mapstructure:"scope_paths"`
	ScopeKeys        []string    `mapstructure:"scope_keys"`
	// Session (e.g. token) configuration.
	// This configuration is deprecated.
	// The session should be captured in PII processor
	// The PII processor can also remove this field from data which is the most
	// common use case.
	SessionClaims    []string    `mapstructure:"session_claims"`
	SessionPaths     []string    `mapstructure:"session_paths"`
	SessionKeys      []string    `mapstructure:"session_keys"`
	SessionIndexes   []int       `mapstructure:"session_indexes"`
	SessionSeparator string      `mapstructure:"session_separator"`
	// Whether to capture raw session or hash it.
	RawSessionValue bool   `mapstructure:"raw_session_value"`
	// Hash algorithm used to hash user session e.g. SHA-1, SHAKE256
	HashAlgo         string      `mapstructure:"hash_algo"`
}

// Condition is the condition that must be matched
// before trying to capture the user
type Condition struct {
	Key   string `json:"key"`
	Regex string `json:"regex"`
}
