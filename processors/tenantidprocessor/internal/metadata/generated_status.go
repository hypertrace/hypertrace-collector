// Code adapted from otel collector's processor/batchprocessor/internal/metadata/generated_status.go

package metadata

import (
	"go.opentelemetry.io/collector/component"
)

var (
	Type      = component.MustNewType("hypertrace_tenantid")
	ScopeName = "github.com/hypertrace/collector/processors/tenantidprocessor"
)

const (
	TracesStability  = component.StabilityLevelBeta
	MetricsStability = component.StabilityLevelBeta
	LogsStability    = component.StabilityLevelBeta
)
