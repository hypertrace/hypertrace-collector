package ratelimiter

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	tagTenantID                = tag.MustNewKey("tenant-id")
	droppedSpanCount           = stats.Int64("tenant_id_dropped_span_count", "Number of spans dropped per tenant due to rate limiting", stats.UnitDimensionless)
	rateLimitServiceCallsCount = stats.Int64("tenant_id_rate_limit_service_calls_count", "Number of calls to rate limiter service from collector", stats.UnitDimensionless)
)

func MetricViews() []*view.View {
	tags := []tag.Key{tagTenantID}

	viewDroppedSpanCount := &view.View{
		Name:        droppedSpanCount.Name(),
		Description: droppedSpanCount.Description(),
		Measure:     droppedSpanCount,
		Aggregation: view.Sum(),
		TagKeys:     tags,
	}

	viewRateLimitServiceCallsCount := &view.View{
		Name:        rateLimitServiceCallsCount.Name(),
		Description: rateLimitServiceCallsCount.Description(),
		Measure:     rateLimitServiceCallsCount,
		Aggregation: view.Sum(),
		TagKeys:     tags,
	}

	return []*view.View{
		viewDroppedSpanCount, viewRateLimitServiceCallsCount,
	}
}
