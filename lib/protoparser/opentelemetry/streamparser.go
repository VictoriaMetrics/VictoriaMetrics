package opentelemetry

import (
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/metrics"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
)

var (
	metricNameLabel = []byte(`__name__`)
	boundLabel      = []byte(`le`)
	quantileLabel   = []byte(`quantile`)
	infLabelValue   = []byte(`+Inf`)
)

// ParseStream parses OpenTelemetry protobuf or json data from r and calls callback for the parsed rows.
//
// The callback can be called concurrently multiple times for streamed data from r.
//
// callback shouldn't hold rows after returning.
func ParseStream(r io.Reader, isJSON, isGzipped bool, callback func(tss []prompb.TimeSeries) error) error {

	if isGzipped {
		zr, err := common.GetGzipReader(r)
		if err != nil {
			return fmt.Errorf("cannot read opentelemetry protocol data: %w", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	}
	wr := getWriteContext()
	defer putWriteContext(wr)
	if err := wr.unpackFrom(r, isJSON); err != nil {
		return fmt.Errorf("cannot unpack opentelemetry metrics: %w", err)
	}
	rms := wr.req.Metrics().ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		wr.baseLabels = wr.baseLabels[:0]

		rm := rms.At(i)
		// add base labels
		// victoria metrics keeps last label value
		// it's ok to have duplicates
		// it allows labels hierarchy
		rm.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
			wr.baseLabels = append(wr.baseLabels, prompb.Label{Name: []byte(k), Value: []byte(v.AsString())})
			return true
		})
		scopedMetrics := rm.ScopeMetrics()
		for j := 0; j < scopedMetrics.Len(); j++ {
			metricSlice := scopedMetrics.At(j).Metrics()

			for k := 0; k < metricSlice.Len(); k++ {
				metric := metricSlice.At(k)
				if len(metric.Name()) == 0 {
					// fast path metric without name
					continue
				}
				switch metric.DataType() {
				case pmetric.MetricDataTypeGauge:
					points := metric.Gauge().DataPoints()
					for p := 0; p < points.Len(); p++ {
						point := points.At(p)
						ts := tsFromNumericPoint(metric.Name(), point, wr.baseLabels)
						wr.tss = append(wr.tss, ts)
					}
				case pmetric.MetricDataTypeSum:
					if metric.Sum().AggregationTemporality() != pmetric.MetricAggregationTemporalityCumulative {
						rowsDroppedUnsupportedSum.Inc()
						continue
					}
					points := metric.Sum().DataPoints()
					for p := 0; p < points.Len(); p++ {
						point := points.At(p)
						ts := tsFromNumericPoint(metric.Name(), point, wr.baseLabels)
						wr.tss = append(wr.tss, ts)
					}
				case pmetric.MetricDataTypeHistogram:
					if metric.Histogram().AggregationTemporality() != pmetric.MetricAggregationTemporalityCumulative {
						rowsDroppedUnsupportedHistogram.Inc()
						continue
					}
					points := metric.Histogram().DataPoints()
					for p := 0; p < points.Len(); p++ {
						point := points.At(p)
						wr.tss = append(wr.tss, tssFromHistogramPoint(metric.Name(), &point, wr.baseLabels)...)
					}
				case pmetric.MetricDataTypeSummary:
					points := metric.Summary().DataPoints()
					for p := 0; p < points.Len(); p++ {
						point := points.At(p)
						wr.tss = append(wr.tss, tssFromSummaryPoint(metric.Name(), &point, wr.baseLabels)...)
					}
				default:
					rowsDroppedUnsupportedMetricType.Inc()
					logger.Warnf("unsupported type: %s for metric name: %s", metric.DataType(), metric.Name())
				}
			}
		}
	}
	tss := wr.tss

	rows := 0
	for i := range tss {
		rows += len(tss[i].Samples)
	}
	rowsRead.Add(rows)

	if err := callback(tss); err != nil {
		return fmt.Errorf("error when processing imported data: %w", err)
	}
	return nil
}

// converts single datapoint into prompb.Series
func tsFromNumericPoint(metricName string, point pmetric.NumberDataPoint, baseLabels []prompb.Label) prompb.TimeSeries {
	var value float64
	switch point.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		value = float64(point.IntVal())
	case pmetric.NumberDataPointValueTypeDouble:
		value = point.DoubleVal()
	}
	pointLabels := append([]prompb.Label{}, baseLabels...)
	point.Attributes().Range(func(k string, v pcommon.Value) bool {
		pointLabels = append(pointLabels, prompb.Label{Name: []byte(k), Value: []byte(v.AsString())})
		return true
	})
	isStale := point.Flags().HasFlag(pmetric.MetricDataPointFlagNoRecordedValue)

	return newPromPBTs(metricName, point.Timestamp().AsTime().UnixMilli(), value, isStale, pointLabels...)
}

// convert openTelemetry summary data point to prometheus summary
// creates multiple timeseries with sum, count and quantile labels
func tssFromSummaryPoint(metricName string, point *pmetric.SummaryDataPoint, baseLabels []prompb.Label) []prompb.TimeSeries {
	var tss []prompb.TimeSeries
	pointLabels := append([]prompb.Label{}, baseLabels...)
	point.Attributes().Range(func(k string, v pcommon.Value) bool {
		pointLabels = append(pointLabels, prompb.Label{Name: []byte(k), Value: []byte(v.AsString())})
		return true
	})
	t := point.Timestamp().AsTime().UnixMilli()
	isStale := point.Flags().HasFlag(pmetric.MetricDataPointFlagNoRecordedValue)

	sumTs := newPromPBTs(metricName+"_sum", t, point.Sum(), isStale, pointLabels...)
	countTs := newPromPBTs(metricName+"_count", t, float64(point.Count()), isStale, pointLabels...)
	tss = append(tss, sumTs, countTs)
	for i := 0; i < point.QuantileValues().Len(); i++ {
		quantile := point.QuantileValues().At(i)
		qValue := []byte(strconv.FormatFloat(quantile.Quantile(), 'f', -1, 64))
		ts := newPromPBTs(metricName, t, quantile.Value(), isStale, pointLabels...)
		ts.Labels = append(ts.Labels, prompb.Label{Name: quantileLabel, Value: qValue})
		tss = append(tss, ts)
	}
	return tss
}

// converts openTelemetry histogram to prometheus format
// adds additional sum and count timeseries
// it supports only cumulative histograms
func tssFromHistogramPoint(metricName string, point *pmetric.HistogramDataPoint, baseLabels []prompb.Label) []prompb.TimeSeries {
	var tss []prompb.TimeSeries
	if len(point.BucketCounts()) == 0 {
		// fast path
		return tss
	}
	if len(point.BucketCounts()) != len(point.ExplicitBounds())+1 {
		// fast path, broken data format
		logger.Warnf("open telemetry bad histogram format: %q, size of buckets: %d, size of bounds: %d", metricName, len(point.BucketCounts()), len(point.ExplicitBounds()))
		return tss
	}
	pointLabels := make([]prompb.Label, 0, len(baseLabels))
	pointLabels = append(pointLabels, baseLabels...)
	point.Attributes().Range(func(k string, v pcommon.Value) bool {
		pointLabels = append(pointLabels, prompb.Label{Name: []byte(k), Value: []byte(v.AsString())})
		return true
	})
	t := point.Timestamp().AsTime().UnixMilli()
	isStale := point.Flags().HasFlag(pmetric.MetricDataPointFlagNoRecordedValue)

	// add base prometheus histogram sum and count metrics
	sumTs := newPromPBTs(metricName+"_sum", t, point.Sum(), isStale, pointLabels...)
	countTs := newPromPBTs(metricName+"_count", t, float64(point.Count()), isStale, pointLabels...)

	tss = append(tss, countTs, sumTs)
	var cumulative uint64
	for index, bound := range point.ExplicitBounds() {
		cumulative += point.BucketCounts()[index]
		boundLabelValue := []byte(strconv.FormatFloat(bound, 'f', -1, 64))
		ts := newPromPBTs(metricName+"_bucket", t, float64(cumulative), isStale, pointLabels...)
		ts.Labels = append(ts.Labels, prompb.Label{Name: []byte(`le`), Value: boundLabelValue})
		tss = append(tss, ts)
	}
	// adds last bucket value as +Inf
	cumulative += point.BucketCounts()[len(point.BucketCounts())-1]
	infTs := newPromPBTs(metricName+"_bucket", t, float64(cumulative), isStale, pointLabels...)
	infTs.Labels = append(infTs.Labels, prompb.Label{Name: boundLabel, Value: infLabelValue})
	tss = append(tss, infTs)
	return tss
}

func newPromPBTs(metricName string, t int64, value float64, isStale bool, extraLabels ...prompb.Label) prompb.TimeSeries {
	if isStale {
		value = decimal.StaleNaN
	}
	ts := prompb.TimeSeries{
		Labels: []prompb.Label{
			{
				Name:  metricNameLabel,
				Value: []byte(metricName),
			},
		},
		Samples: []prompb.Sample{
			{
				Value:     value,
				Timestamp: t,
			},
		},
	}
	ts.Labels = append(ts.Labels, extraLabels...)
	return ts
}

type writeContext struct {
	bb         bytesutil.ByteBuffer
	req        pmetricotlp.Request
	tss        []prompb.TimeSeries
	baseLabels []prompb.Label
}

func (wr *writeContext) unpackFrom(r io.Reader, isJSON bool) error {
	if _, err := wr.bb.ReadFrom(r); err != nil {
		return err
	}
	parseFunc := wr.req.UnmarshalProto
	if isJSON {
		parseFunc = wr.req.UnmarshalJSON
	}
	return parseFunc(wr.bb.B)
}

func (wr *writeContext) reset() {
	for i := range wr.tss {
		ts := &wr.tss[i]
		ts.Labels = nil
		ts.Samples = nil
	}
	wr.tss = wr.tss[:0]
	for i := range wr.baseLabels {
		label := &wr.baseLabels[i]
		label.Name = nil
		label.Value = nil
	}
	wr.baseLabels = wr.baseLabels[:0]
	wr.bb.Reset()
	wr.req.SetMetrics(pmetric.NewMetrics())
}

var wrPool sync.Pool

func getWriteContext() *writeContext {
	wr := wrPool.Get()
	if wr == nil {
		wr = &writeContext{
			req: pmetricotlp.NewRequest(),
		}
	}
	return wr.(*writeContext)
}

func putWriteContext(wr *writeContext) {
	wr.reset()
	wrPool.Put(wr)
}

var (
	rowsRead                         = metrics.NewCounter(`vm_protoparser_rows_read_total{type="opentelemetry"}`)
	rowsDroppedUnsupportedHistogram  = metrics.NewCounter(`vm_protoparser_rows_dropped_total{type="opentelemetry",reason="unsupported_histogram_aggregation"}`)
	rowsDroppedUnsupportedSum        = metrics.NewCounter(`vm_protoparser_rows_dropped_total{type="opentelemetry",reason="unsupported_sum_aggregation"}`)
	rowsDroppedUnsupportedMetricType = metrics.NewCounter(`vm_protoparser_rows_dropped_total{type="opentelemetry",reason="unsupported_metric_type"}`)
)
