package spancounter

import (
	"go.opentelemetry.io/collector/config"
)

type Config struct {
	config.ProcessorSettings `mapstructure:"-"`
	// TenantIDAttributeKey defines span attribute key for tenant. Default tenant-id.
	TenantIDAttributeKey string         `mapstructure:"tenant_id_attribute_key"`
	TenantConfigs        []TenantConfig `mapstructure:"tenant_configs"`
}

type TenantConfig struct {
	TenantId       string          `mapstructure:"tenant_id"`
	ServiceConfigs []ServiceConfig `mapstructure:"service_configs"`
}

type ServiceConfig struct {
	ServiceName string       `mapstructure:"service_name"`
	SpanConfigs []SpanConfig `mapstructure:"span_configs"`
}

type SpanConfig struct {
	// This is used to identify matches in the metrics. It should be unique.
	Label          string          `mapstructure:"label"`
	SpanName       string          `mapstructure:"span_name"`
	SpanAttributes []SpanAttribute `mapstructure:"span_attributes"`
}

type SpanAttribute struct {
	Key   string `mapstructure:"key"`
	Value string `mapstructure:"value"`
}
