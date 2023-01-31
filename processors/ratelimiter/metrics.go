package ratelimiter

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	tagTenantID               = tag.MustNewKey("tenant-id")
	droppedSpanCountSoftLimit = stats.Int64("tenant_id_dropped_span_count_soft_limit",
		"Number of spans dropped because of rate limiting per tenant due to soft limit", stats.UnitDimensionless)
	droppedSpanCount = stats.Int64("tenant_id_dropped_span_count", "Number of spans dropped per tenant due to global rate limiting", stats.UnitDimensionless)
)

func MetricViews() []*view.View {
	tags := []tag.Key{tagTenantID}

	viewDroppedSpanCountSoftLimit := &view.View{
		Name:        droppedSpanCountSoftLimit.Name(),
		Description: droppedSpanCountSoftLimit.Description(),
		Measure:     droppedSpanCountSoftLimit,
		Aggregation: view.Sum(),
		TagKeys:     tags,
	}

	viewDroppedSpanCount := &view.View{
		Name:        droppedSpanCount.Name(),
		Description: droppedSpanCount.Description(),
		Measure:     droppedSpanCount,
		Aggregation: view.Sum(),
		TagKeys:     tags,
	}

	return []*view.View{
		viewDroppedSpanCountSoftLimit,
		viewDroppedSpanCount,
	}
}
