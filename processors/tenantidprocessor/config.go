package tenantidprocessor

import "go.opentelemetry.io/collector/config/configmodels"

type Config struct {
	configmodels.ProcessorSettings `mapstructure:",squash"`

	TenantIDHeaderName   string `mapstructure:"tenantid_header_name"`
	TenantIDAttributeKey string `mapstructure:"tenantid_attribute_key"`
}
