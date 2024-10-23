package metricresourceattrstoattrs

import (
	"context"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
)

func TestEmptyMetrics(t *testing.T) {
	p := &metricResourceAttrsProcessor{
		logger: zap.NewNop(),
	}
	metrics := pmetric.NewMetrics()
	gotMetrics, err := p.ProcessMetrics(context.Background(), metrics)
	require.NoError(t, err)
	assert.Equal(t, metrics, gotMetrics)
}

func TestCopyingResourceAttributesToMetricAttributes(t *testing.T) {
	logger := zap.NewNop()
	testCases := map[string]struct {
		inputResourceAttributes  map[string]string
		inputMetricAttributes    map[string]string
		expectedMetricAttributes map[string]string
		dt                       pmetric.MetricType
	}{
		"all concerned resource attrs present for sum metric: job and instance labels not added. existing job and instance labels are removed": {
			inputResourceAttributes: map[string]string{
				conventions.AttributeServiceName:       "test-service",
				conventions.AttributeServiceInstanceID: "test-instance-id",
				model.JobLabel:                         "test-job-name",
				model.InstanceLabel:                    "test-instance",
				"port":                                 "8888",
				"scheme":                               "http",
			},
			inputMetricAttributes: map[string]string{
				"foo10":             "baz10",
				"foo11":             "baz11",
				model.JobLabel:      "test-metric-job-name",
				model.InstanceLabel: "test-metric-instance",
			},
			expectedMetricAttributes: map[string]string{
				"foo10":                                "baz10",
				"foo11":                                "baz11",
				conventions.AttributeServiceName:       "test-service",
				conventions.AttributeServiceInstanceID: "test-instance-id",
				"port":                                 "8888",
				"scheme":                               "http",
			},
			dt: pmetric.MetricTypeSum,
		},
		"service name and instance resource attrs not present for sum metric: job and instance labels not added. existing job and instance labels are removed": {
			inputResourceAttributes: map[string]string{
				conventions.AttributeServiceName:       "test-service",
				conventions.AttributeServiceInstanceID: "test-instance-id",
				"port":                                 "8888",
				"scheme":                               "http",
			},
			inputMetricAttributes: map[string]string{
				"foo10":             "baz10",
				"foo11":             "baz11",
				model.JobLabel:      "test-metric-job-name",
				model.InstanceLabel: "test-metric-instance",
			},
			expectedMetricAttributes: map[string]string{
				"foo10":                                "baz10",
				"foo11":                                "baz11",
				conventions.AttributeServiceName:       "test-service",
				conventions.AttributeServiceInstanceID: "test-instance-id",
				"port":                                 "8888",
				"scheme":                               "http",
			},
			dt: pmetric.MetricTypeSum,
		},
		"no concerned labels present: job and instance labels retained": {
			inputResourceAttributes: map[string]string{
				"port":   "8888",
				"scheme": "http",
			},
			inputMetricAttributes: map[string]string{
				"foo10":             "baz10",
				"foo11":             "baz11",
				model.JobLabel:      "test-metric-job-name",
				model.InstanceLabel: "test-metric-instance",
			},
			expectedMetricAttributes: map[string]string{
				"foo10":             "baz10",
				"foo11":             "baz11",
				model.JobLabel:      "test-metric-job-name",
				model.InstanceLabel: "test-metric-instance",
				"port":              "8888",
				"scheme":            "http",
			},
			dt: pmetric.MetricTypeHistogram,
		},
		"service name and instance id resource attrs not present for sum metric: job and instance labels are added": {
			inputResourceAttributes: map[string]string{
				model.JobLabel:      "test-job-name",
				model.InstanceLabel: "test-instance",
				"port":              "8888",
				"scheme":            "http",
			},
			inputMetricAttributes: map[string]string{
				"foo10": "baz10",
				"foo11": "baz11",
			},
			expectedMetricAttributes: map[string]string{
				"foo10":             "baz10",
				"foo11":             "baz11",
				model.JobLabel:      "test-job-name",
				model.InstanceLabel: "test-instance",
				"port":              "8888",
				"scheme":            "http",
			},
			dt: pmetric.MetricTypeSum,
		},
		"service name and instance id resource attrs not present for sum metric. job and instance labels already present: job and instance labels are not added": {
			inputResourceAttributes: map[string]string{
				model.JobLabel:      "test-job-name",
				model.InstanceLabel: "test-instance",
				"port":              "8888",
				"scheme":            "http",
			},
			inputMetricAttributes: map[string]string{
				"foo10":             "baz10",
				"foo11":             "baz11",
				model.JobLabel:      "test-metric-job-name",
				model.InstanceLabel: "test-metric-instance",
			},
			expectedMetricAttributes: map[string]string{
				"foo10":             "baz10",
				"foo11":             "baz11",
				model.JobLabel:      "test-metric-job-name",
				model.InstanceLabel: "test-metric-instance",
				"port":              "8888",
				"scheme":            "http",
			},
			dt: pmetric.MetricTypeSum,
		},
		"all concerned resource attrs present for gauge metric: job and instance labels not added": {
			inputResourceAttributes: map[string]string{
				conventions.AttributeServiceName:       "test-service",
				conventions.AttributeServiceInstanceID: "test-instance-id",
				model.JobLabel:                         "test-job-name",
				model.InstanceLabel:                    "test-instance",
				"port":                                 "8888",
			},
			inputMetricAttributes: map[string]string{
				"foo10":             "baz10",
				model.JobLabel:      "test-metric-job-name",
				model.InstanceLabel: "test-metric-instance",
			},
			expectedMetricAttributes: map[string]string{
				"foo10":                                "baz10",
				conventions.AttributeServiceName:       "test-service",
				conventions.AttributeServiceInstanceID: "test-instance-id",
				"port":                                 "8888",
			},
			dt: pmetric.MetricTypeGauge,
		},
		"all concerned resource attrs present for histogram metric: job and instance labels not added": {
			inputResourceAttributes: map[string]string{
				conventions.AttributeServiceName:       "test-service",
				conventions.AttributeServiceInstanceID: "test-instance-id",
				model.JobLabel:                         "test-job-name",
				model.InstanceLabel:                    "test-instance",
				"port":                                 "8888",
			},
			inputMetricAttributes: map[string]string{
				"foo10":             "baz10",
				model.JobLabel:      "test-metric-job-name",
				model.InstanceLabel: "test-metric-instance",
			},
			expectedMetricAttributes: map[string]string{
				"foo10":                                "baz10",
				conventions.AttributeServiceName:       "test-service",
				conventions.AttributeServiceInstanceID: "test-instance-id",
				"port":                                 "8888",
			},
			dt: pmetric.MetricTypeHistogram,
		},
		"all concerned resource attrs present for exponential histogram metric: job and instance labels not added": {
			inputResourceAttributes: map[string]string{
				conventions.AttributeServiceName:       "test-service",
				conventions.AttributeServiceInstanceID: "test-instance-id",
				model.JobLabel:                         "test-job-name",
				model.InstanceLabel:                    "test-instance",
				"port":                                 "8888",
			},
			inputMetricAttributes: map[string]string{
				"foo10":             "baz10",
				model.JobLabel:      "test-metric-job-name",
				model.InstanceLabel: "test-metric-instance",
			},
			expectedMetricAttributes: map[string]string{
				"foo10":                                "baz10",
				conventions.AttributeServiceName:       "test-service",
				conventions.AttributeServiceInstanceID: "test-instance-id",
				"port":                                 "8888",
			},
			dt: pmetric.MetricTypeExponentialHistogram,
		},
		"all concerned resource attrs present for summary metric: job and instance labels not added": {
			inputResourceAttributes: map[string]string{
				conventions.AttributeServiceName:       "test-service",
				conventions.AttributeServiceInstanceID: "test-instance-id",
				model.JobLabel:                         "test-job-name",
				model.InstanceLabel:                    "test-instance",
				"port":                                 "8888",
			},
			inputMetricAttributes: map[string]string{
				"foo10":             "baz10",
				model.JobLabel:      "test-metric-job-name",
				model.InstanceLabel: "test-metric-instance",
			},
			expectedMetricAttributes: map[string]string{
				"foo10":                                "baz10",
				conventions.AttributeServiceName:       "test-service",
				conventions.AttributeServiceInstanceID: "test-instance-id",
				"port":                                 "8888",
			},
			dt: pmetric.MetricTypeSummary,
		},
		"service_instance_id attribute exists for gauge metric: service.instance.id resource attr not added": {
			inputResourceAttributes: map[string]string{
				conventions.AttributeServiceName:       "test-service",
				conventions.AttributeServiceInstanceID: "test-instance-id",
				"port":                                 "8888",
			},
			inputMetricAttributes: map[string]string{
				"foo10":                    "baz10",
				ocServiceInstanceIdAttrKey: "test-metric-instance-id",
			},
			expectedMetricAttributes: map[string]string{
				"foo10":                          "baz10",
				ocServiceInstanceIdAttrKey:       "test-metric-instance-id",
				conventions.AttributeServiceName: "test-service",
				"port":                           "8888",
			},
			dt: pmetric.MetricTypeGauge,
		},
		"service_instance_id attribute does not exists for gauge metric: service.instance.id resource attr added": {
			inputResourceAttributes: map[string]string{
				conventions.AttributeServiceName:       "test-service",
				conventions.AttributeServiceInstanceID: "test-instance-id",
				"port":                                 "8888",
			},
			inputMetricAttributes: map[string]string{
				"foo10": "baz10",
			},
			expectedMetricAttributes: map[string]string{
				"foo10":                                "baz10",
				conventions.AttributeServiceName:       "test-service",
				conventions.AttributeServiceInstanceID: "test-instance-id",
				"port":                                 "8888",
			},
			dt: pmetric.MetricTypeGauge,
		},
		"service.name resource attribute exists. service_name attribute exists for gauge metric: service_name attr removed": {
			inputResourceAttributes: map[string]string{
				conventions.AttributeServiceName:       "test-service",
				conventions.AttributeServiceInstanceID: "test-instance-id",
				"port":                                 "8888",
			},
			inputMetricAttributes: map[string]string{
				"foo10":              "baz10",
				ocServiceNameAttrKey: "different-service-name",
			},
			expectedMetricAttributes: map[string]string{
				"foo10":                                "baz10",
				conventions.AttributeServiceName:       "test-service",
				conventions.AttributeServiceInstanceID: "test-instance-id",
				"port":                                 "8888",
			},
			dt: pmetric.MetricTypeGauge,
		},
		"service_instance_id and service.instance.id attributes exist for gauge metric: service.instance.id attr removed": {
			inputResourceAttributes: map[string]string{
				conventions.AttributeServiceName:       "test-service",
				conventions.AttributeServiceInstanceID: "test-instance-id",
				"port":                                 "8888",
			},
			inputMetricAttributes: map[string]string{
				"foo10":                                "baz10",
				ocServiceInstanceIdAttrKey:             "test-metric-instance-id",
				conventions.AttributeServiceInstanceID: "test-instance-id",
			},
			expectedMetricAttributes: map[string]string{
				"foo10":                          "baz10",
				ocServiceInstanceIdAttrKey:       "test-metric-instance-id",
				conventions.AttributeServiceName: "test-service",
				"port":                           "8888",
			},
			dt: pmetric.MetricTypeGauge,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			metrics := generateMetricData(
				testCase.inputResourceAttributes,
				testCase.inputMetricAttributes,
				testCase.dt,
			)
			expectedProcessedMetrics := generateMetricData(
				testCase.inputResourceAttributes,
				testCase.expectedMetricAttributes,
				testCase.dt,
			)

			p := metricResourceAttrsProcessor{logger: logger}

			processedMetrics, err := p.ProcessMetrics(context.Background(), metrics)
			assert.Nil(t, err)
			// resource attrs should not change
			verifyAttributesEquality(
				t,
				expectedProcessedMetrics.ResourceMetrics().At(0).Resource().Attributes(),
				processedMetrics.ResourceMetrics().At(0).Resource().Attributes(),
			)
			// metric data point attributes can change
			expectedProcessedMetric := expectedProcessedMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
			processedMetric := processedMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
			verifyAttributesEquality(
				t,
				getMetricDataPointAttributes(expectedProcessedMetric, testCase.dt),
				getMetricDataPointAttributes(processedMetric, testCase.dt),
			)
		})
	}
}

func generateMetricData(resourceAttrs map[string]string, attrs map[string]string, dt pmetric.MetricType) pmetric.Metrics {
	md := pmetric.NewMetrics()
	md.ResourceMetrics().AppendEmpty()
	for k, v := range resourceAttrs {
		md.ResourceMetrics().At(0).Resource().Attributes().PutStr(k, v)
	}
	md.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty()
	md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
	metric := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)

	switch dt {
	case pmetric.MetricTypeSum:
		metric.SetEmptySum()
		metric.Sum().DataPoints().AppendEmpty()
		for k, v := range attrs {
			metric.Sum().DataPoints().At(0).Attributes().PutStr(k, v)
		}
	case pmetric.MetricTypeGauge:
		metric.SetEmptyGauge()
		metric.Gauge().DataPoints().AppendEmpty()
		for k, v := range attrs {
			metric.Gauge().DataPoints().At(0).Attributes().PutStr(k, v)
		}
	case pmetric.MetricTypeHistogram:
		metric.SetEmptyHistogram()
		metric.Histogram().DataPoints().AppendEmpty()
		for k, v := range attrs {
			metric.Histogram().DataPoints().At(0).Attributes().PutStr(k, v)
		}
	case pmetric.MetricTypeExponentialHistogram:
		metric.SetEmptyExponentialHistogram()
		metric.ExponentialHistogram().DataPoints().AppendEmpty()
		for k, v := range attrs {
			metric.ExponentialHistogram().DataPoints().At(0).Attributes().PutStr(k, v)
		}
	case pmetric.MetricTypeSummary:
		metric.SetEmptySummary()
		metric.Summary().DataPoints().AppendEmpty()
		for k, v := range attrs {
			metric.Summary().DataPoints().At(0).Attributes().PutStr(k, v)
		}
	}
	return md
}

func getMetricDataPointAttributes(metric pmetric.Metric, dt pmetric.MetricType) pcommon.Map {
	switch dt {
	case pmetric.MetricTypeSum:
		return metric.Sum().DataPoints().At(0).Attributes()
	case pmetric.MetricTypeGauge:
		return metric.Gauge().DataPoints().At(0).Attributes()
	case pmetric.MetricTypeHistogram:
		return metric.Histogram().DataPoints().At(0).Attributes()
	case pmetric.MetricTypeExponentialHistogram:
		return metric.ExponentialHistogram().DataPoints().At(0).Attributes()
	case pmetric.MetricTypeSummary:
		return metric.Summary().DataPoints().At(0).Attributes()
	}

	return pcommon.NewMap()
}

func verifyAttributesEquality(t *testing.T, m1 pcommon.Map, m2 pcommon.Map) {
	assert.Equal(t, m1.Len(), m2.Len())
	m1.Range(func(k string, v pcommon.Value) bool {
		v2, ok := m2.Get(k)
		assert.Truef(t, ok, "m2 does not have key %s found in m1", k)
		assert.Equalf(t, v, v2, "m2 has a different value for key %s. val: %v", k, v2.AsString())
		return true
	})
}
