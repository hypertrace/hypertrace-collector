module github.com/hypertrace/collector

go 1.16

require (
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/apache/thrift v0.14.2
	github.com/jaegertracing/jaeger v1.25.0
	github.com/pquerna/cachecontrol v0.1.0 // indirect
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector v0.33.0
	go.opentelemetry.io/collector/model v0.33.0
	go.opentelemetry.io/otel/trace v1.0.0-RC2
	go.uber.org/zap v1.19.0
	google.golang.org/grpc v1.40.0
)

// branch jaeger-thrift-http-headers
replace go.opentelemetry.io/collector => github.com/hypertrace/opentelemetry-collector v0.33.0-1
