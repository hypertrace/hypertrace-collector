module github.com/hypertrace/collector

go 1.16

require (
	github.com/apache/thrift v0.15.0
	github.com/jaegertracing/jaeger v1.27.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter v0.38.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter v0.38.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter v0.38.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.38.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension v0.38.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.38.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver v0.38.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver v0.38.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver v0.38.0
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector v0.38.0
	go.opentelemetry.io/collector/model v0.38.0
	go.opentelemetry.io/otel/trace v1.0.1
	go.uber.org/zap v1.19.1
	google.golang.org/grpc v1.41.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver v0.38.0 => ./receiver/jaegerreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter v0.38.0 => ./exporter/kafkaexporter
