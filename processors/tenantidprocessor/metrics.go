package tenantidprocessor

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	tagTenantID = tag.MustNewKey("tenant-id")

	statSpanPerTenant = stats.Int64("tenant_id_span_count", "Number of spans received from a tenant", stats.UnitDimensionless)
)

// MetricViews returns the metrics views for tenant id processor.
func MetricViews() []*view.View {
	tags := []tag.Key{tagTenantID}

	viewSpanCount := &view.View{
		Name:        statSpanPerTenant.Name(),
		Description: statSpanPerTenant.Description(),
		Measure:     statSpanPerTenant,
		Aggregation: view.Sum(),
		TagKeys:     tags,
	}

	return []*view.View{
		viewSpanCount,
	}
}
