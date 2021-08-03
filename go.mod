module github.com/hypertrace/collector

go 1.15

require (
	github.com/apache/thrift v0.14.2
	github.com/jaegertracing/jaeger v1.23.0
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector v0.29.0
	go.uber.org/zap v1.17.0
	google.golang.org/grpc v1.38.0
)

// branch jaeger-thrift-http-headers
replace go.opentelemetry.io/collector => github.com/hypertrace/opentelemetry-collector v0.29.0-2
