package spancounter

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	tagSpanCriteriaLabel       = tag.MustNewKey("span-criteria-label")
	statCriteriaBasedSpanCount = stats.Int64("criteria_span_count", "Number of spans received from a tenant that match a certain criteria", stats.UnitDimensionless)
)

// MetricViews returns the metrics views for spancounter processor.
func MetricViews() []*view.View {
	tags := []tag.Key{tagSpanCriteriaLabel}

	viewCriteriaBasedSpanCount := &view.View{
		Name:        statCriteriaBasedSpanCount.Name(),
		Description: statCriteriaBasedSpanCount.Description(),
		Measure:     statCriteriaBasedSpanCount,
		Aggregation: view.Sum(),
		TagKeys:     tags,
	}

	return []*view.View{
		viewCriteriaBasedSpanCount,
	}
}
