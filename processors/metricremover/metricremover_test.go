package metricremover

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestEmptyMetrics(t *testing.T) {
	p := &metricRemoverProcessor{
		logger:               zap.NewNop(),
		removeNoneMetricType: true,
	}
	metrics := pmetric.NewMetrics()
	gotMetrics, err := p.ProcessMetrics(context.Background(), metrics)
	require.NoError(t, err)
	assert.Equal(t, metrics, gotMetrics)
}

func TestMetricsRemoval(t *testing.T) {
	logger := zap.NewNop()
	testCases := map[string]struct {
		removeNoneMetricType bool
		inputDtArr           []pmetric.MetricType
		expectedDtArr        []pmetric.MetricType
	}{
		"config enabled none metrics removed": {
			removeNoneMetricType: true,
			inputDtArr: []pmetric.MetricType{pmetric.MetricTypeGauge, pmetric.MetricTypeSum, pmetric.MetricTypeEmpty, pmetric.MetricTypeHistogram,
				pmetric.MetricTypeExponentialHistogram, pmetric.MetricTypeEmpty, pmetric.MetricTypeSummary},
			expectedDtArr: []pmetric.MetricType{pmetric.MetricTypeGauge, pmetric.MetricTypeSum, pmetric.MetricTypeHistogram,
				pmetric.MetricTypeExponentialHistogram, pmetric.MetricTypeSummary},
		},
		"config disabled none metrics are not removed": {
			removeNoneMetricType: false,
			inputDtArr: []pmetric.MetricType{pmetric.MetricTypeGauge, pmetric.MetricTypeSum, pmetric.MetricTypeEmpty, pmetric.MetricTypeHistogram,
				pmetric.MetricTypeExponentialHistogram, pmetric.MetricTypeEmpty, pmetric.MetricTypeSummary},
			expectedDtArr: []pmetric.MetricType{pmetric.MetricTypeGauge, pmetric.MetricTypeSum, pmetric.MetricTypeEmpty, pmetric.MetricTypeHistogram,
				pmetric.MetricTypeExponentialHistogram, pmetric.MetricTypeEmpty, pmetric.MetricTypeSummary},
		},
		"no none metrics": {
			removeNoneMetricType: true,
			inputDtArr: []pmetric.MetricType{pmetric.MetricTypeGauge, pmetric.MetricTypeSum, pmetric.MetricTypeHistogram,
				pmetric.MetricTypeExponentialHistogram, pmetric.MetricTypeSummary},
			expectedDtArr: []pmetric.MetricType{pmetric.MetricTypeGauge, pmetric.MetricTypeSum, pmetric.MetricTypeHistogram,
				pmetric.MetricTypeExponentialHistogram, pmetric.MetricTypeSummary},
		},
		"only none metrics config disabled": {
			removeNoneMetricType: false,
			inputDtArr:           []pmetric.MetricType{pmetric.MetricTypeEmpty, pmetric.MetricTypeEmpty, pmetric.MetricTypeEmpty, pmetric.MetricTypeEmpty},
			expectedDtArr:        []pmetric.MetricType{pmetric.MetricTypeEmpty, pmetric.MetricTypeEmpty, pmetric.MetricTypeEmpty, pmetric.MetricTypeEmpty},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			p := &metricRemoverProcessor{
				logger:               logger,
				removeNoneMetricType: testCase.removeNoneMetricType,
			}
			metrics := generateMetricData(testCase.inputDtArr)
			expectedMetrics := generateMetricData(testCase.expectedDtArr)
			gotMetrics, err := p.ProcessMetrics(context.Background(), metrics)
			require.NoError(t, err)
			assert.Equal(t, expectedMetrics, gotMetrics)
		})
	}
}

// Can't add this test to the test array above since the test compares metric_arr(nil)
// and metric_arr{} which are technically the same thing but not equal.
func TestMetricsRemovalAllNoneMetrics(t *testing.T) {
	p := &metricRemoverProcessor{
		logger:               zap.NewNop(),
		removeNoneMetricType: true,
	}
	dtArr := []pmetric.MetricType{pmetric.MetricTypeEmpty, pmetric.MetricTypeEmpty, pmetric.MetricTypeEmpty, pmetric.MetricTypeEmpty}
	metrics := generateMetricData(dtArr)
	gotMetrics, err := p.ProcessMetrics(context.Background(), metrics)
	require.NoError(t, err)
	assert.Equal(t, 0, gotMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
}

func generateMetricData(dtArr []pmetric.MetricType) pmetric.Metrics {
	md := pmetric.NewMetrics()
	md.ResourceMetrics().AppendEmpty()
	md.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty()
	for i, dt := range dtArr {
		md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
		metric := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(i)

		switch dt {
		case pmetric.MetricTypeSum:
			metric.SetEmptySum()
			metric.Sum().DataPoints().AppendEmpty()
		case pmetric.MetricTypeGauge:
			metric.SetEmptyGauge()
			metric.Gauge().DataPoints().AppendEmpty()
		case pmetric.MetricTypeHistogram:
			metric.SetEmptyHistogram()
			metric.Histogram().DataPoints().AppendEmpty()
		case pmetric.MetricTypeExponentialHistogram:
			metric.SetEmptyExponentialHistogram()
			metric.ExponentialHistogram().DataPoints().AppendEmpty()
		case pmetric.MetricTypeSummary:
			metric.SetEmptySummary()
			metric.Summary().DataPoints().AppendEmpty()
		case pmetric.MetricTypeEmpty:
			metric.SetName("none.metric")
		}
	}

	return md
}
