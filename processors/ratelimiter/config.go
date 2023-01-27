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

	// RateLimitServiceHost defines host where rate limiter service is running default "localhost".
	RateLimitServiceHost string `mapstructure:"service_host"`
	// RateLimitServicePort defines port where rate limiter service is running. Default 8081.
	RateLimitServicePort uint16 `mapstructure:"service_port"`
	//Domain  rate limit configuration domain to query. Default collector
	Domain string `mapstructure:"domain"`
	// DomainSoftRateLimitThreshold represents the threshold where soft limit window started.
	DomainSoftRateLimitThreshold uint32 `mapstructure:"domain_soft_limit_threshold"`
	// TenantIDHeaderName defines tenant HTTP header name. Default x-tenant-id.
	TenantIDHeaderName string `mapstructure:"header_name"`
	// Timeout in millis for grpc call.
	RateLimitServiceTimeoutMillis uint32 `mapstructure:"timeout_millis"`
}
