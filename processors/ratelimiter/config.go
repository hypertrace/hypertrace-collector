package ratelimiter

import (
	"go.opentelemetry.io/collector/config"
)

// Config defines config for ratelimit processor.
// The processor calls rate limit service for group of spans.
// The processor either drops or forwards data based on rate limit response.
// The tenant ID header is obtained from the context object.
// The processor run immediately after tenantId processor
type Config struct {
	config.ProcessorSettings `mapstructure:"-"`

	// ServiceHost defines host where rate limiter service is running default "localhost".
	ServiceHost string `mapstructure:"service_host"`
	// ServicePort defines port where rate limiter service is running. Default 8081.
	ServicePort uint16 `mapstructure:"service_port"`
	//Domain  rate limit configuration domain to query. Default collector
	Domain string `mapstructure:"domain"`
	// TenantIDHeaderName defines tenant HTTP header name. Default x-tenant-id.
	TenantIDHeaderName string `mapstructure:"tenant_id_header_name"`
	// Timeout in millis for grpc call.
	TimeoutMillis uint32 `mapstructure:"timeout_millis"`
}
