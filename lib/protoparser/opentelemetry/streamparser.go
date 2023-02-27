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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/VictoriaMetrics/metrics"
	"github.com/golang/protobuf/proto"
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
	for _, rm := range wr.req.ResourceMetrics {
		wr.baseLabels = wr.baseLabels[:0]
		// add base labels
		// victoria metrics keeps last label value
		// it's ok to have duplicates
		// it allows labels hierarchy
		if rm.Resource == nil {
			continue
		}
		for j := range rm.Resource.Attributes {
			at := rm.Resource.Attributes[j]
			wr.baseLabels = append(wr.baseLabels, prompb.Label{Name: []byte(at.Key), Value: []byte(at.Value.FormatString())})
		}
		for _, sc := range rm.ScopeMetrics {
			for _, metricData := range sc.Metrics {
				if len(metricData.Name) == 0 {
					// fast path metric without name
					continue
				}
				switch metric := metricData.Data.(type) {
				case *pb.Metric_Gauge:
					for _, p := range metric.Gauge.DataPoints {
						ts := tsFromNumericPoint(metricData.Name, p, wr.baseLabels)
						wr.tss = append(wr.tss, ts)
					}

				case *pb.Metric_Sum:
					if metric.Sum.AggregationTemporality != pb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE {
						rowsDroppedUnsupportedSum.Inc()
						continue
					}
					for _, p := range metric.Sum.DataPoints {
						ts := tsFromNumericPoint(metricData.Name, p, wr.baseLabels)
						wr.tss = append(wr.tss, ts)
					}

				case *pb.Metric_Histogram:
					if metric.Histogram.AggregationTemporality != pb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE {
						rowsDroppedUnsupportedHistogram.Inc()
						continue
					}
					for _, p := range metric.Histogram.DataPoints {
						wr.tss = append(wr.tss, tssFromHistogramPoint(metricData.Name, p, wr.baseLabels)...)
					}
				case *pb.Metric_Summary:
					for _, p := range metric.Summary.DataPoints {
						wr.tss = append(wr.tss, tssFromSummaryPoint(metricData.Name, p, wr.baseLabels)...)
					}
				default:
					rowsDroppedUnsupportedMetricType.Inc()
					logger.Warnf("unsupported type: %s for metric name: %s", metric, metricData.Name)
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
func tsFromNumericPoint(metricName string, point *pb.NumberDataPoint, baseLabels []prompb.Label) prompb.TimeSeries {
	var value float64
	switch v := point.Value.(type) {
	case *pb.NumberDataPoint_AsInt:
		value = float64(v.AsInt)
	case *pb.NumberDataPoint_AsDouble:
		value = v.AsDouble
	}
	pointLabels := append([]prompb.Label{}, baseLabels...)
	for i := range point.Attributes {
		at := point.Attributes[i]
		pointLabels = append(pointLabels, prompb.Label{Name: []byte(at.Key), Value: []byte(at.Value.FormatString())})
	}

	isStale := (point.Flags)&uint32(1) != 0

	return newPromPBTs(metricName, int64(point.TimeUnixNano/1e6), value, isStale, pointLabels...)
}

// convert openTelemetry summary data point to prometheus summary
// creates multiple timeseries with sum, count and quantile labels
func tssFromSummaryPoint(metricName string, point *pb.SummaryDataPoint, baseLabels []prompb.Label) []prompb.TimeSeries {
	var tss []prompb.TimeSeries
	pointLabels := append([]prompb.Label{}, baseLabels...)
	for i := range point.Attributes {
		at := point.Attributes[i]
		pointLabels = append(pointLabels, prompb.Label{Name: []byte(at.Key), Value: []byte(at.Value.FormatString())})
	}
	t := int64(point.TimeUnixNano / 1e6)
	isStale := (point.Flags)&uint32(1) != 0

	sumTs := newPromPBTs(metricName+"_sum", t, point.Sum, isStale, pointLabels...)
	countTs := newPromPBTs(metricName+"_count", t, float64(point.Count), isStale, pointLabels...)
	tss = append(tss, sumTs, countTs)
	for _, q := range point.QuantileValues {
		qValue := []byte(strconv.FormatFloat(q.Quantile, 'f', -1, 64))
		ts := newPromPBTs(metricName, t, q.Value, isStale, pointLabels...)
		ts.Labels = append(ts.Labels, prompb.Label{Name: quantileLabel, Value: qValue})
		tss = append(tss, ts)
	}

	return tss
}

// converts openTelemetry histogram to prometheus format
// adds additional sum and count timeseries
// it supports only cumulative histograms
func tssFromHistogramPoint(metricName string, point *pb.HistogramDataPoint, baseLabels []prompb.Label) []prompb.TimeSeries {
	var tss []prompb.TimeSeries
	if len(point.BucketCounts) == 0 {
		// fast path
		return tss
	}
	if len(point.BucketCounts) != len(point.ExplicitBounds)+1 {
		// fast path, broken data format
		logger.Warnf("opentelemetry bad histogram format: %q, size of buckets: %d, size of bounds: %d", metricName, len(point.BucketCounts), len(point.ExplicitBounds))
		return nil
	}
	pointLabels := make([]prompb.Label, 0, len(baseLabels))
	pointLabels = append(pointLabels, baseLabels...)
	for i := range point.Attributes {
		at := point.Attributes[i]
		pointLabels = append(pointLabels, prompb.Label{Name: []byte(at.Key), Value: []byte(at.Value.FormatString())})
	}
	t := int64(point.TimeUnixNano / 1e6)
	isStale := (point.Flags)&uint32(1) != 0

	sumTs := newPromPBTs(metricName+"_sum", t, *point.Sum, isStale, pointLabels...)
	countTs := newPromPBTs(metricName+"_count", t, float64(point.Count), isStale, pointLabels...)

	tss = append(tss, countTs, sumTs)
	var cumulative uint64
	for index, bound := range point.ExplicitBounds {
		cumulative += point.BucketCounts[index]
		boundLabelValue := []byte(strconv.FormatFloat(bound, 'f', -1, 64))
		ts := newPromPBTs(metricName+"_bucket", t, float64(cumulative), isStale, pointLabels...)
		ts.Labels = append(ts.Labels, prompb.Label{Name: []byte(`le`), Value: boundLabelValue})
		tss = append(tss, ts)
	}
	// adds last bucket value as +Inf
	cumulative += point.BucketCounts[len(point.BucketCounts)-1]
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
	req        pb.ExportMetricsServiceRequest
	tss        []prompb.TimeSeries
	baseLabels []prompb.Label
}

func (wr *writeContext) unpackFrom(r io.Reader, isJSON bool) error {
	if _, err := wr.bb.ReadFrom(r); err != nil {
		return err
	}
	parseFunc := func(buf []byte, m *pb.ExportMetricsServiceRequest) error {
		return proto.Unmarshal(buf, m)
	}
	if isJSON {
		parseFunc = pb.UnmarshalJSONExportMetricsServiceRequest
	}
	return parseFunc(wr.bb.B, &wr.req)
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
	wr.req.Reset()
}

var wrPool sync.Pool

func getWriteContext() *writeContext {
	wr := wrPool.Get()
	if wr == nil {
		wr = &writeContext{}
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
