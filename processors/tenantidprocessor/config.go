package tenantidprocessor

import (
	"go.opentelemetry.io/collector/config"
)

// Config defines config for tenant ID processor.
// The processor adds tenant ID attribute to every received span.
// The processor returns an error when the tenant ID is missing.
// The tenant ID header is obtained from the context object.
// The batch processor cleans context, therefore this processor
// has to run before it, ideally right after the receiver.
type Config struct {
	*config.ProcessorSettings `mapstructure:"-"`

	// TenantIDHeaderName defines tenant HTTP header name. Default x-tenant-id.
	TenantIDHeaderName string `mapstructure:"header_name"`
	// TenantIDAttributeKey defines span attribute key for tenant. Default tenant-id.
	TenantIDAttributeKey string `mapstructure:"attribute_key"`
}
