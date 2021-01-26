package tenantidprocessor

import "go.opentelemetry.io/collector/config/configmodels"

// Config defines config for tenantid processor
type Config struct {
	configmodels.ProcessorSettings `mapstructure:",squash"`

	// TenantIDHeaderName defines tenant HTTP header name
	TenantIDHeaderName string `mapstructure:"tenantid_header_name"`
	// TenantIDAttributeKey defines span attribute key for tenant
	TenantIDAttributeKey string `mapstructure:"tenantid_attribute_key"`
}
