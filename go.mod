module github.com/hypertrace/collector

go 1.15

require (
	github.com/apache/thrift v0.13.0
	github.com/jaegertracing/jaeger v1.22.0
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector v0.24.0
	go.uber.org/zap v1.16.0
	google.golang.org/grpc v1.36.1
)

// branch jaeger-thrift-http-headers
replace go.opentelemetry.io/collector => github.com/hypertrace/opentelemetry-collector v0.24.1-0.20210419093812-8ce3b4fab2c6
