// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusexporter

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type mockAccumulator struct {
	metrics            []pmetric.Metric
	resourceAttributes pcommon.Map // Same attributes for all metrics.
}

func (a *mockAccumulator) Accumulate(pmetric.ResourceMetrics) (n int) {
	return 0
}

func (a *mockAccumulator) Collect() ([]pmetric.Metric, []pcommon.Map) {
	rAttrs := make([]pcommon.Map, len(a.metrics))
	for i := range rAttrs {
		rAttrs[i] = a.resourceAttributes
	}

	return a.metrics, rAttrs
}

func TestConvertInvalidDataType(t *testing.T) {
	metric := pmetric.NewMetric()
	metric.SetDataType(-100)
	c := collector{
		accumulator: &mockAccumulator{
			[]pmetric.Metric{metric},
			pcommon.NewMap(),
		},
		logger: zap.NewNop(),
	}

	_, err := c.convertMetric(metric, pcommon.NewMap())
	require.Equal(t, errUnknownMetricType, err)

	ch := make(chan prometheus.Metric, 1)
	go func() {
		c.Collect(ch)
		close(ch)
	}()

	j := 0
	for range ch {
		require.Fail(t, "Expected no reported metrics")
		j++
	}
}

func TestConvertInvalidMetric(t *testing.T) {
	for _, mType := range []pmetric.MetricDataType{
		pmetric.MetricDataTypeHistogram,
		pmetric.MetricDataTypeSum,
		pmetric.MetricDataTypeGauge,
	} {
		metric := pmetric.NewMetric()
		metric.SetDataType(mType)
		switch metric.DataType() {
		case pmetric.MetricDataTypeGauge:
			metric.Gauge().DataPoints().AppendEmpty()
		case pmetric.MetricDataTypeSum:
			metric.Sum().DataPoints().AppendEmpty()
		case pmetric.MetricDataTypeHistogram:
			metric.Histogram().DataPoints().AppendEmpty()
		}
		c := collector{}

		_, err := c.convertMetric(metric, pcommon.NewMap())
		require.Error(t, err)
	}
}

// errorCheckCore keeps track of logged errors
type errorCheckCore struct {
	errorMessages []string
}

func (*errorCheckCore) Enabled(zapcore.Level) bool      { return true }
func (c *errorCheckCore) With([]zap.Field) zapcore.Core { return c }
func (c *errorCheckCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}
func (c *errorCheckCore) Write(ent zapcore.Entry, _ []zapcore.Field) error {
	if ent.Level == zapcore.ErrorLevel {
		c.errorMessages = append(c.errorMessages, ent.Message)
	}
	return nil
}
func (*errorCheckCore) Sync() error { return nil }

func TestCollectMetricsLabelSanitize(t *testing.T) {
	metric := pmetric.NewMetric()
	metric.SetName("test_metric")
	metric.SetDataType(pmetric.MetricDataTypeGauge)
	metric.SetDescription("test description")
	dp := metric.Gauge().DataPoints().AppendEmpty()
	dp.SetIntVal(42)
	dp.Attributes().InsertString("label.1", "1")
	dp.Attributes().InsertString("label/2", "2")
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	loggerCore := errorCheckCore{}
	c := collector{
		namespace: "test_space",
		accumulator: &mockAccumulator{
			[]pmetric.Metric{metric},
			pcommon.NewMap(),
		},
		sendTimestamps: false,
		logger:         zap.New(&loggerCore),
	}

	ch := make(chan prometheus.Metric, 1)
	go func() {
		c.Collect(ch)
		close(ch)
	}()

	for m := range ch {
		require.Contains(t, m.Desc().String(), "fqName: \"test_space_test_metric\"")
		require.Contains(t, m.Desc().String(), "variableLabels: [label_1 label_2]")

		pbMetric := io_prometheus_client.Metric{}
		require.NoError(t, m.Write(&pbMetric))

		labelsKeys := map[string]string{"label_1": "1", "label_2": "2"}
		for _, l := range pbMetric.Label {
			require.Equal(t, labelsKeys[*l.Name], *l.Value)
		}
	}

	require.Empty(t, loggerCore.errorMessages, "labels were not sanitized properly")
}

func TestCollectMetrics(t *testing.T) {
	tests := []struct {
		name       string
		metric     func(time.Time) pmetric.Metric
		metricType prometheus.ValueType
		value      float64
	}{
		{
			name:       "IntGauge",
			metricType: prometheus.GaugeValue,
			value:      42.0,
			metric: func(ts time.Time) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeGauge)
				metric.SetDescription("test description")
				dp := metric.Gauge().DataPoints().AppendEmpty()
				dp.SetIntVal(42)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))

				return
			},
		},
		{
			name:       "Gauge",
			metricType: prometheus.GaugeValue,
			value:      42.42,
			metric: func(ts time.Time) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeGauge)
				metric.SetDescription("test description")
				dp := metric.Gauge().DataPoints().AppendEmpty()
				dp.SetDoubleVal(42.42)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))

				return
			},
		},
		{
			name:       "IntSum",
			metricType: prometheus.GaugeValue,
			value:      42.0,
			metric: func(ts time.Time) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.Sum().SetIsMonotonic(false)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetIntVal(42)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))

				return
			},
		},
		{
			name:       "Sum",
			metricType: prometheus.GaugeValue,
			value:      42.42,
			metric: func(ts time.Time) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.Sum().SetIsMonotonic(false)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleVal(42.42)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))

				return
			},
		},
		{
			name:       "MonotonicIntSum",
			metricType: prometheus.CounterValue,
			value:      42.0,
			metric: func(ts time.Time) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.Sum().SetIsMonotonic(true)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetIntVal(42)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))

				return
			},
		},
		{
			name:       "MonotonicSum",
			metricType: prometheus.CounterValue,
			value:      42.42,
			metric: func(ts time.Time) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.Sum().SetIsMonotonic(true)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleVal(42.42)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))

				return
			},
		},
	}

	for _, tt := range tests {
		for _, sendTimestamp := range []bool{true, false} {
			name := tt.name
			if sendTimestamp {
				name += "/WithTimestamp"
			}

			rAttrs := pcommon.NewMap()
			rAttrs.InsertString(conventions.AttributeServiceInstanceID, "localhost:9090")
			rAttrs.InsertString(conventions.AttributeServiceName, "testapp")
			rAttrs.InsertString(conventions.AttributeServiceNamespace, "prod")

			t.Run(name, func(t *testing.T) {
				ts := time.Now()
				metric := tt.metric(ts)
				c := collector{
					namespace: "test_space",
					accumulator: &mockAccumulator{
						[]pmetric.Metric{metric},
						rAttrs,
					},
					sendTimestamps: sendTimestamp,
					logger:         zap.NewNop(),
				}

				ch := make(chan prometheus.Metric, 1)
				go func() {
					c.Collect(ch)
					close(ch)
				}()

				j := 0
				for m := range ch {
					j++
					require.Contains(t, m.Desc().String(), "fqName: \"test_space_test_metric\"")
					require.Contains(t, m.Desc().String(), "variableLabels: [label_1 label_2 job instance]")

					pbMetric := io_prometheus_client.Metric{}
					require.NoError(t, m.Write(&pbMetric))

					labelsKeys := map[string]string{"label_1": "1", "label_2": "2", "job": "prod/testapp", "instance": "localhost:9090"}
					for _, l := range pbMetric.Label {
						require.Equal(t, labelsKeys[*l.Name], *l.Value)
					}

					if sendTimestamp {
						require.Equal(t, ts.UnixNano()/1e6, *(pbMetric.TimestampMs))
					} else {
						require.Nil(t, pbMetric.TimestampMs)
					}

					switch tt.metricType {
					case prometheus.CounterValue:
						require.Equal(t, tt.value, *pbMetric.Counter.Value)
						require.Nil(t, pbMetric.Gauge)
						require.Nil(t, pbMetric.Histogram)
						require.Nil(t, pbMetric.Summary)
					case prometheus.GaugeValue:
						require.Equal(t, tt.value, *pbMetric.Gauge.Value)
						require.Nil(t, pbMetric.Counter)
						require.Nil(t, pbMetric.Histogram)
						require.Nil(t, pbMetric.Summary)
					}
				}
				require.Equal(t, 1, j)
			})
		}
	}
}

func TestAccumulateHistograms(t *testing.T) {
	tests := []struct {
		name   string
		metric func(time.Time) pmetric.Metric

		histogramPoints map[float64]uint64
		histogramSum    float64
		histogramCount  uint64
	}{
		{
			name: "Histogram",
			histogramPoints: map[float64]uint64{
				3.5:  5,
				10.0: 7,
			},
			histogramSum:   42.42,
			histogramCount: 7,
			metric: func(ts time.Time) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeHistogram)
				metric.Histogram().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{5, 2}))
				dp.SetCount(7)
				dp.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{3.5, 10.0}))
				dp.SetSum(42.42)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return
			},
		},
	}

	for _, tt := range tests {
		for _, sendTimestamp := range []bool{true, false} {
			name := tt.name
			if sendTimestamp {
				name += "/WithTimestamp"
			}
			t.Run(name, func(t *testing.T) {
				ts := time.Now()
				metric := tt.metric(ts)
				c := collector{
					accumulator: &mockAccumulator{
						[]pmetric.Metric{metric},
						pcommon.NewMap(),
					},
					sendTimestamps: sendTimestamp,
					logger:         zap.NewNop(),
				}

				ch := make(chan prometheus.Metric, 1)
				go func() {
					c.Collect(ch)
					close(ch)
				}()

				n := 0
				for m := range ch {
					n++
					require.Contains(t, m.Desc().String(), "fqName: \"test_metric\"")
					require.Contains(t, m.Desc().String(), "variableLabels: [label_1 label_2]")

					pbMetric := io_prometheus_client.Metric{}
					require.NoError(t, m.Write(&pbMetric))

					labelsKeys := map[string]string{"label_1": "1", "label_2": "2"}
					for _, l := range pbMetric.Label {
						require.Equal(t, labelsKeys[*l.Name], *l.Value)
					}

					if sendTimestamp {
						require.Equal(t, ts.UnixNano()/1e6, *(pbMetric.TimestampMs))
					} else {
						require.Nil(t, pbMetric.TimestampMs)
					}

					require.Nil(t, pbMetric.Gauge)
					require.Nil(t, pbMetric.Counter)

					h := *pbMetric.Histogram
					require.Equal(t, tt.histogramCount, h.GetSampleCount())
					require.Equal(t, tt.histogramSum, h.GetSampleSum())
					require.Equal(t, len(tt.histogramPoints), len(h.Bucket))

					for _, b := range h.Bucket {
						require.Equal(t, tt.histogramPoints[(*b).GetUpperBound()], b.GetCumulativeCount())
					}
				}
				require.Equal(t, 1, n)
			})
		}
	}
}

func TestAccumulateSummary(t *testing.T) {
	fillQuantileValue := func(pN, value float64, dest pmetric.ValueAtQuantile) {
		dest.SetQuantile(pN)
		dest.SetValue(value)
	}
	tests := []struct {
		name          string
		metric        func(time.Time) pmetric.Metric
		wantSum       float64
		wantCount     uint64
		wantQuantiles map[float64]float64
	}{
		{
			name:      "Summary with single point",
			wantSum:   0.012,
			wantCount: 10,
			wantQuantiles: map[float64]float64{
				0.50: 190,
				0.99: 817,
			},
			metric: func(ts time.Time) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeSummary)
				metric.SetDescription("test description")
				sp := metric.Summary().DataPoints().AppendEmpty()
				sp.SetCount(10)
				sp.SetSum(0.012)
				sp.SetCount(10)
				sp.Attributes().InsertString("label_1", "1")
				sp.Attributes().InsertString("label_2", "2")
				sp.SetTimestamp(pcommon.NewTimestampFromTime(ts))

				fillQuantileValue(0.50, 190, sp.QuantileValues().AppendEmpty())
				fillQuantileValue(0.99, 817, sp.QuantileValues().AppendEmpty())

				return
			},
		},
	}

	for _, tt := range tests {
		for _, sendTimestamp := range []bool{true, false} {
			name := tt.name
			if sendTimestamp {
				name += "/WithTimestamp"
			}
			t.Run(name, func(t *testing.T) {
				ts := time.Now()
				metric := tt.metric(ts)
				c := collector{
					accumulator: &mockAccumulator{
						[]pmetric.Metric{metric},
						pcommon.NewMap(),
					},
					sendTimestamps: sendTimestamp,
					logger:         zap.NewNop(),
				}

				ch := make(chan prometheus.Metric, 1)
				go func() {
					c.Collect(ch)
					close(ch)
				}()

				n := 0
				for m := range ch {
					n++
					require.Contains(t, m.Desc().String(), "fqName: \"test_metric\"")
					require.Contains(t, m.Desc().String(), "variableLabels: [label_1 label_2]")

					pbMetric := io_prometheus_client.Metric{}
					require.NoError(t, m.Write(&pbMetric))

					labelsKeys := map[string]string{"label_1": "1", "label_2": "2"}
					for _, l := range pbMetric.Label {
						require.Equal(t, labelsKeys[*l.Name], *l.Value)
					}

					if sendTimestamp {
						require.Equal(t, ts.UnixNano()/1e6, *(pbMetric.TimestampMs))
					} else {
						require.Nil(t, pbMetric.TimestampMs)
					}

					require.Nil(t, pbMetric.Gauge)
					require.Nil(t, pbMetric.Counter)
					require.Nil(t, pbMetric.Histogram)

					s := *pbMetric.Summary
					require.Equal(t, tt.wantCount, *s.SampleCount)
					require.Equal(t, tt.wantSum, *s.SampleSum)
					// To ensure that we can compare quantiles, we need to just extract their values.
					gotQuantiles := make(map[float64]float64)
					for _, q := range s.Quantile {
						gotQuantiles[q.GetQuantile()] = q.GetValue()
					}
					require.Equal(t, tt.wantQuantiles, gotQuantiles)
				}
				require.Equal(t, 1, n)
			})
		}
	}
}
