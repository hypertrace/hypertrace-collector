// Code adapted from otel collector's processor/batchprocessor/internal/metadata/generated_telemetry.go

package metadata

import (
	"errors"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
)

// Deprecated: [v0.108.0] use LeveledMeter instead.
func Meter(settings component.TelemetrySettings) metric.Meter {
	return settings.MeterProvider.Meter("github.com/hypertrace/collector/processors/tenantidprocessor")
}

func LeveledMeter(settings component.TelemetrySettings, level configtelemetry.Level) metric.Meter {
	return settings.LeveledMeterProvider(level).Meter("github.com/hypertrace/collector/processors/tenantidprocessor")
}

func Tracer(settings component.TelemetrySettings) trace.Tracer {
	return settings.TracerProvider.Tracer("github.com/hypertrace/collector/processors/tenantidprocessor")
}

// TelemetryBuilder provides an interface for components to report telemetry
// as defined in metadata and user config.
type TelemetryBuilder struct {
	// In generated code but unused
	// meter                     metric.Meter
	ProcessorSpansPerTenant   metric.Int64Counter
	ProcessorMetricsPerTenant metric.Int64Counter
	meters                    map[configtelemetry.Level]metric.Meter
}

// TelemetryBuilderOption applies changes to default builder.
type TelemetryBuilderOption interface {
	apply(*TelemetryBuilder)
}

type telemetryBuilderOptionFunc func(mb *TelemetryBuilder)

func (tbof telemetryBuilderOptionFunc) apply(mb *TelemetryBuilder) {
	tbof(mb)
}

// NewTelemetryBuilder provides a struct with methods to update all internal telemetry
// for a component
func NewTelemetryBuilder(settings component.TelemetrySettings, options ...TelemetryBuilderOption) (*TelemetryBuilder, error) {
	builder := TelemetryBuilder{meters: map[configtelemetry.Level]metric.Meter{}}
	for _, op := range options {
		op.apply(&builder)
	}
	builder.meters[configtelemetry.LevelBasic] = LeveledMeter(settings, configtelemetry.LevelBasic)
	builder.meters[configtelemetry.LevelDetailed] = LeveledMeter(settings, configtelemetry.LevelDetailed)
	var err, errs error

	builder.ProcessorSpansPerTenant, err = builder.meters[configtelemetry.LevelBasic].Int64Counter(
		// TODO: Do I need the _count suffix or will otel append it automatically?
		"otelcol_tenant_id_span_count",
		metric.WithDescription("Number of spans received from a tenant"),
		metric.WithUnit("{spans}"),
	)
	errs = errors.Join(errs, err)
	builder.ProcessorMetricsPerTenant, err = builder.meters[configtelemetry.LevelBasic].Int64Counter(
		"otelcol_tenant_id_metric_count",
		metric.WithDescription("Number of metrics received from a tenant"),
		metric.WithUnit("{spans}"),
	)
	errs = errors.Join(errs, err)
	return &builder, errs
}
