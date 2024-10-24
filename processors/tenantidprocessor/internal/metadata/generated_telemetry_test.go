// Code adapted from otel collector's processor/batchprocessor/internal/metadata/generated_telemetry_test.go

package metadata

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	embeddedmetric "go.opentelemetry.io/otel/metric/embedded"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	embeddedtrace "go.opentelemetry.io/otel/trace/embedded"
	nooptrace "go.opentelemetry.io/otel/trace/noop"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
)

type mockMeter struct {
	noopmetric.Meter
	name string
}
type mockMeterProvider struct {
	embeddedmetric.MeterProvider
}

func (m mockMeterProvider) Meter(name string, opts ...metric.MeterOption) metric.Meter {
	return mockMeter{name: name}
}

type mockTracer struct {
	nooptrace.Tracer
	name string
}

type mockTracerProvider struct {
	embeddedtrace.TracerProvider
}

func (m mockTracerProvider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	return mockTracer{name: name}
}

func TestProviders(t *testing.T) {
	set := component.TelemetrySettings{
		LeveledMeterProvider: func(_ configtelemetry.Level) metric.MeterProvider {
			return mockMeterProvider{}
		},
		MeterProvider:  mockMeterProvider{},
		TracerProvider: mockTracerProvider{},
	}

	meter := Meter(set)
	if m, ok := meter.(mockMeter); ok {
		require.Equal(t, "github.com/hypertrace/collector/processors/tenantidprocessor", m.name)
	} else {
		require.Fail(t, "returned Meter not mockMeter")
	}

	tracer := Tracer(set)
	if m, ok := tracer.(mockTracer); ok {
		require.Equal(t, "github.com/hypertrace/collector/processors/tenantidprocessor", m.name)
	} else {
		require.Fail(t, "returned Meter not mockTracer")
	}
}

func TestNewTelemetryBuilder(t *testing.T) {
	set := component.TelemetrySettings{
		LeveledMeterProvider: func(_ configtelemetry.Level) metric.MeterProvider {
			return mockMeterProvider{}
		},
		MeterProvider:  mockMeterProvider{},
		TracerProvider: mockTracerProvider{},
	}
	applied := false
	_, err := NewTelemetryBuilder(set, telemetryBuilderOptionFunc(func(b *TelemetryBuilder) {
		applied = true
	}))
	require.NoError(t, err)
	require.True(t, applied)
}
