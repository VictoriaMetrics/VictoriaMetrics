package opentelemetry

import (
	"fmt"
	"io"
	"math/bits"
	"strconv"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
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
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)
	r = wcr

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
		// edge case
		if rm.Resource == nil {
			continue
		}
		wr.baseLabels = attributesToPromLabels(wr, rm.Resource.Attributes)
		for _, sc := range rm.ScopeMetrics {
			scopedMetricToTimeSeries(wr, sc)
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

func scopedMetricToTimeSeries(wr *writeContext, sc *pb.ScopeMetrics) {
	for _, metricData := range sc.Metrics {
		if len(metricData.Name) == 0 {
			// fast path metric without name
			continue
		}
		switch metric := metricData.Data.(type) {
		case *pb.Metric_Gauge:
			for _, p := range metric.Gauge.DataPoints {
				tsFromNumericPoint(wr, metricData.Name, p)
			}

		case *pb.Metric_Sum:
			if metric.Sum.AggregationTemporality != pb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE {
				rowsDroppedUnsupportedSum.Inc()
				continue
			}
			for _, p := range metric.Sum.DataPoints {
				tsFromNumericPoint(wr, metricData.Name, p)
			}

		case *pb.Metric_Histogram:
			if metric.Histogram.AggregationTemporality != pb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE {
				rowsDroppedUnsupportedHistogram.Inc()
				continue
			}
			for _, p := range metric.Histogram.DataPoints {
				addHistogramTss(wr, metricData.Name, p)
			}
		case *pb.Metric_Summary:
			for _, p := range metric.Summary.DataPoints {
				tssFromSummaryPoint(wr, metricData.Name, p)
			}
		default:
			rowsDroppedUnsupportedMetricType.Inc()
			logger.Warnf("unsupported type: %s for metric name: %s", metric, metricData.Name)
		}
	}
}

// converts single datapoint into prompb.Series
func tsFromNumericPoint(wr *writeContext, metricName string, point *pb.NumberDataPoint) {
	var value float64
	switch v := point.Value.(type) {
	case *pb.NumberDataPoint_AsInt:
		value = float64(v.AsInt)
	case *pb.NumberDataPoint_AsDouble:
		value = v.AsDouble
	}
	pointLabels := attributesToPromLabels(wr, point.Attributes)
	isStale := (point.Flags)&uint32(1) != 0

	appendTs(wr, metricName, int64(point.TimeUnixNano/1e6), value, isStale, pointLabels)
}

// convert openTelemetry summary data point to prometheus summary
// creates multiple timeseries with sum, count and quantile labels
func tssFromSummaryPoint(wr *writeContext, metricName string, point *pb.SummaryDataPoint) {
	pointLabels := attributesToPromLabels(wr, point.Attributes)
	t := int64(point.TimeUnixNano / 1e6)
	isStale := (point.Flags)&uint32(1) != 0
	appendTs(wr, metricName+"_sum", t, point.Sum, isStale, pointLabels)
	appendTs(wr, metricName+"_count", t, float64(point.Count), isStale, pointLabels)
	for _, q := range point.QuantileValues {
		qValue := []byte(strconv.FormatFloat(q.Quantile, 'f', -1, 64))
		appendTsWithExtraLabel(wr, metricName, quantileLabel, qValue, t, q.Value, isStale, pointLabels)
	}
}

// attributesToPromLabels converts otlp attributes to prompb labels format
// reuses label pool for reducing memory allocations
func attributesToPromLabels(wr *writeContext, attributes []*pb.KeyValue) []prompb.Label {
	poolStart := len(wr.labelsPool)
	wr.labelsPool = extendSliceWithCopy(wr.labelsPool, len(attributes))
	for i := range attributes {
		at := attributes[i]
		l := &wr.labelsPool[len(wr.labelsPool)-1-i]
		l.Name = bytesutil.ToUnsafeBytes(at.Key)
		l.Value = bytesutil.ToUnsafeBytes(at.Value.FormatString())
	}
	wr.labelsPool = extendSliceWithCopy(wr.labelsPool, len(wr.baseLabels))
	for i := range wr.baseLabels {
		l := &wr.labelsPool[len(wr.labelsPool)-1-i]
		l.Name = wr.baseLabels[i].Name
		l.Value = wr.baseLabels[i].Value
	}

	return wr.labelsPool[poolStart:len(wr.labelsPool):len(wr.labelsPool)]
}

// converts openTelemetry histogram to prometheus format
// adds additional sum and count timeseries
// it supports only cumulative histograms
func addHistogramTss(wr *writeContext, metricName string, point *pb.HistogramDataPoint) {
	if len(point.BucketCounts) == 0 {
		// fast path
		return
	}
	if len(point.BucketCounts) != len(point.ExplicitBounds)+1 {
		// fast path, broken data format
		logger.Warnf("opentelemetry bad histogram format: %q, size of buckets: %d, size of bounds: %d", metricName, len(point.BucketCounts), len(point.ExplicitBounds))
		return
	}
	pointLabels := attributesToPromLabels(wr, point.Attributes)
	t := int64(point.TimeUnixNano / 1e6)
	isStale := (point.Flags)&uint32(1) != 0
	appendTs(wr, metricName+"_sum", t, *point.Sum, isStale, pointLabels)
	appendTs(wr, metricName+"_count", t, float64(point.Count), isStale, pointLabels)

	var cumulative uint64
	for index, bound := range point.ExplicitBounds {
		cumulative += point.BucketCounts[index]
		boundLabelValue := []byte(strconv.FormatFloat(bound, 'f', -1, 64))
		appendTsWithExtraLabel(wr, metricName+"_bucket", boundLabel, boundLabelValue, t, float64(cumulative), isStale, pointLabels)
	}
	// adds last bucket value as +Inf
	cumulative += point.BucketCounts[len(point.BucketCounts)-1]
	appendTsWithExtraLabel(wr, metricName+"_bucket", boundLabel, infLabelValue, t, float64(cumulative), isStale, pointLabels)
}

func appendTs(wr *writeContext, metricName string, t int64, value float64, isStale bool, pointLabels []prompb.Label) {
	if isStale {
		value = decimal.StaleNaN
	}
	if t <= 0 {
		// Set the current timestamp if t isn't set.
		t = int64(fasttime.UnixTimestamp()) * 1000
	}
	labelsStart := len(wr.labelsPool)
	// take in account name label
	pointLabelsLen := len(pointLabels) + 1

	wr.labelsPool = extendSliceWithCopy(wr.labelsPool, pointLabelsLen)
	for idx := range pointLabels {
		p := &wr.labelsPool[len(wr.labelsPool)-1-idx]
		p.Name = pointLabels[idx].Name
		p.Value = pointLabels[idx].Value
	}
	nameLabel := &wr.labelsPool[len(wr.labelsPool)-pointLabelsLen]
	nameLabel.Name = metricNameLabel
	nameLabel.Value = bytesutil.ToUnsafeBytes(metricName)

	wr.samplesPool = extendSliceWithCopy(wr.samplesPool, 1)
	sample := &wr.samplesPool[len(wr.samplesPool)-1]
	sample.Timestamp = t
	sample.Value = value

	wr.tss = extendSliceWithCopy(wr.tss, 1)
	ts := &wr.tss[len(wr.tss)-1]
	tsLabels := wr.labelsPool[labelsStart:]
	ts.Labels = tsLabels[:len(tsLabels):len(tsLabels)]
	ts.Samples = wr.samplesPool[len(wr.samplesPool)-1 : len(wr.samplesPool) : len(wr.samplesPool)]
}

func appendTsWithExtraLabel(wr *writeContext, metricName string, labelName, labelValue []byte, t int64, value float64, isStale bool, pointLabels []prompb.Label) {
	if isStale {
		value = decimal.StaleNaN
	}
	if t <= 0 {
		// Set the current timestamp if t isn't set.
		t = int64(fasttime.UnixTimestamp()) * 1000
	}
	labelsStart := len(wr.labelsPool)
	// take in account name + extra label
	pointLabelsLen := len(pointLabels) + 2
	wr.labelsPool = extendSliceWithCopy(wr.labelsPool, pointLabelsLen)
	for idx := range pointLabels {
		p := &wr.labelsPool[len(wr.labelsPool)-1-idx]
		p.Name = pointLabels[idx].Name
		p.Value = pointLabels[idx].Value
	}
	nameLabel := &wr.labelsPool[len(wr.labelsPool)-pointLabelsLen]
	nameLabel.Name = metricNameLabel
	nameLabel.Value = bytesutil.ToUnsafeBytes(metricName)
	extraLabel := &wr.labelsPool[len(wr.labelsPool)-pointLabelsLen+1]
	extraLabel.Name = labelName
	extraLabel.Value = labelValue

	wr.samplesPool = extendSliceWithCopy(wr.samplesPool, 1)
	sample := &wr.samplesPool[len(wr.samplesPool)-1]
	sample.Timestamp = t
	sample.Value = value

	wr.tss = extendSliceWithCopy(wr.tss, 1)
	ts := &wr.tss[len(wr.tss)-1]
	tsLabels := wr.labelsPool[labelsStart:]
	ts.Labels = tsLabels[:len(tsLabels):len(tsLabels)]
	ts.Samples = wr.samplesPool[len(wr.samplesPool)-1 : len(wr.samplesPool) : len(wr.samplesPool)]
}

type writeContext struct {
	bb         bytesutil.ByteBuffer
	req        pb.ExportMetricsServiceRequest
	tss        []prompb.TimeSeries
	baseLabels []prompb.Label
	// pools for reducing memory allocations
	labelsPool  []prompb.Label
	samplesPool []prompb.Sample
}

func (wr *writeContext) unpackFrom(r io.Reader, isJSON bool) error {
	if _, err := wr.bb.ReadFrom(r); err != nil {
		return err
	}
	parseFunc := func(buf []byte, m *pb.ExportMetricsServiceRequest) error {
		return m.UnmarshalVT(buf)
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
	wr.baseLabels = wr.baseLabels[:0]
	wr.labelsPool = wr.labelsPool[:0]
	wr.samplesPool = wr.samplesPool[:0]
	wr.bb.Reset()
	wr.req.Reset()
}

var wrPool sync.Pool

func getWriteContext() *writeContext {
	wr := wrPool.Get()
	if wr == nil {
		wr = &writeContext{
			tss:         make([]prompb.TimeSeries, 0, 8),
			labelsPool:  make([]prompb.Label, 0, 16),
			samplesPool: make([]prompb.Sample, 0, 8),
		}
	}
	return wr.(*writeContext)
}

func putWriteContext(wr *writeContext) {
	wr.reset()
	wr.req.Reset()
	wrPool.Put(wr)
}

// yes, it's generic function.
func extendSliceWithCopy[T any](dst []T, n int) []T {
	if n == 0 {
		return dst
	}
	baseLen := len(dst)
	if cap(dst) <= len(dst)+n {
		nNew := roundToNearestPow2(n)
		bNew := make([]T, nNew)
		dst = append(dst, bNew...)
	}
	dst = dst[:baseLen+n]
	return dst
}

func roundToNearestPow2(n int) int {
	pow2 := uint8(bits.Len(uint(n - 1)))
	return 1 << pow2
}

var (
	rowsRead                         = metrics.NewCounter(`vm_protoparser_rows_read_total{type="opentelemetry"}`)
	rowsDroppedUnsupportedHistogram  = metrics.NewCounter(`vm_protoparser_rows_dropped_total{type="opentelemetry",reason="unsupported_histogram_aggregation"}`)
	rowsDroppedUnsupportedSum        = metrics.NewCounter(`vm_protoparser_rows_dropped_total{type="opentelemetry",reason="unsupported_sum_aggregation"}`)
	rowsDroppedUnsupportedMetricType = metrics.NewCounter(`vm_protoparser_rows_dropped_total{type="opentelemetry",reason="unsupported_metric_type"}`)
)
