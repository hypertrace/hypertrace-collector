package haproxyverifier

import (
	"go.opentelemetry.io/collector/config"
)

type Config struct {
	config.ProcessorSettings `mapstructure:"-"`
	RequestSpan              SpanConfig `mapstructure:"request_span"`
	ResponseSpan             SpanConfig `mapstructure:"response_span"`
	LogIntervalSeconds       int        `mapstructure:"log_interval_seconds"`
	VerifyHaproxy            bool       `mapstructure:"verify_haproxy"`
}

type SpanConfig struct {
	SpanName       string          `mapstructure:"span_name"`
	SpanAttributes []SpanAttribute `mapstructure:"span_attributes"`
}

type SpanAttribute struct {
	Key   string `mapstructure:"key"`
	Value string `mapstructure:"value"`
}
