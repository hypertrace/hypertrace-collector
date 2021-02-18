module github.com/hypertrace/collector

go 1.15

require (
	github.com/antlr/antlr4 v0.0.0-20210127121638-62a0b02bf460 // indirect
	github.com/apache/thrift v0.13.0
	github.com/jaegertracing/jaeger v1.21.0
	github.com/json-iterator/go v1.1.10 // indirect
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.22.5
	go.opentelemetry.io/collector v0.18.0
	go.uber.org/zap v1.16.0
	google.golang.org/grpc v1.35.0
)

// branch jaeger-thrift-http-headers
replace go.opentelemetry.io/collector => github.com/hypertrace/opentelemetry-collector v0.7.1-0.20210203140508-3124456ebb6a
