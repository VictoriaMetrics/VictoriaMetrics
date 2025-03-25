package stream

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
)

var maxRequestSize = flagutil.NewBytes("opentelemetry.maxRequestSize", 64*1024*1024, "The maximum size in bytes of a single OpenTelemetry request")

// ParseStream parses OpenTelemetry protobuf or json data from r and calls callback for the parsed rows.
//
// callback shouldn't hold tss items after returning.
//
// optional processBody can be used for pre-processing the read request body from r before parsing it in OpenTelemetry format.
func ParseStream(r io.Reader, encoding string, processBody func(data []byte) ([]byte, error), callback func(tss []prompbmarshal.TimeSeries) error) error {
	err := protoparserutil.ReadUncompressedData(r, encoding, maxRequestSize, func(data []byte) error {
		if processBody != nil {
			dataNew, err := processBody(data)
			if err != nil {
				return fmt.Errorf("cannot process request body: %w", err)
			}
			data = dataNew
		}
		return parseData(data, callback)
	})
	if err != nil {
		return fmt.Errorf("cannot decode OpenTelemetry protocol data: %w", err)
	}
	return nil
}

func parseData(data []byte, callback func(tss []prompbmarshal.TimeSeries) error) error {
	var req pb.ExportMetricsServiceRequest
	if err := req.UnmarshalProtobuf(data); err != nil {
		return fmt.Errorf("cannot unmarshal request from %d bytes: %w", len(data), err)
	}

	wr := getWriteContext()
	defer putWriteContext(wr)

	wr.parseRequestToTss(&req)

	if err := callback(wr.tss); err != nil {
		return fmt.Errorf("error when processing OpenTelemetry samples: %w", err)
	}

	return nil
}

var skippedSampleLogger = logger.WithThrottler("otlp_skipped_sample", 5*time.Second)

func (wr *writeContext) appendSamplesFromScopeMetrics(sc *pb.ScopeMetrics) {
	for _, m := range sc.Metrics {
		if len(m.Name) == 0 {
			// skip metrics without names
			continue
		}
		metricName := sanitizeMetricName(m)
		switch {
		case m.Gauge != nil:
			for _, p := range m.Gauge.DataPoints {
				wr.appendSampleFromNumericPoint(metricName, p)
			}
		case m.Sum != nil:
			if m.Sum.AggregationTemporality != pb.AggregationTemporalityCumulative {
				rowsDroppedUnsupportedSum.Inc()
				skippedSampleLogger.Warnf("unsupported delta temporality for %q ('sum'): skipping it", metricName)
				continue
			}
			for _, p := range m.Sum.DataPoints {
				wr.appendSampleFromNumericPoint(metricName, p)
			}
		case m.Summary != nil:
			for _, p := range m.Summary.DataPoints {
				wr.appendSamplesFromSummary(metricName, p)
			}
		case m.Histogram != nil:
			if m.Histogram.AggregationTemporality != pb.AggregationTemporalityCumulative {
				rowsDroppedUnsupportedHistogram.Inc()
				skippedSampleLogger.Warnf("unsupported delta temporality for %q ('histogram'): skipping it", metricName)
				continue
			}
			for _, p := range m.Histogram.DataPoints {
				wr.appendSamplesFromHistogram(metricName, p)
			}
		case m.ExponentialHistogram != nil:
			if m.ExponentialHistogram.AggregationTemporality != pb.AggregationTemporalityCumulative {
				rowsDroppedUnsupportedExponentialHistogram.Inc()
				skippedSampleLogger.Warnf("unsupported delta temporality for %q ('exponential histogram'): skipping it", metricName)
				continue
			}
			for _, p := range m.ExponentialHistogram.DataPoints {
				wr.appendSamplesFromExponentialHistogram(metricName, p)
			}
		default:
			rowsDroppedUnsupportedMetricType.Inc()
			skippedSampleLogger.Warnf("unsupported type for metric %q", metricName)
		}
	}
}

// appendSampleFromNumericPoint appends p to wr.tss
func (wr *writeContext) appendSampleFromNumericPoint(metricName string, p *pb.NumberDataPoint) {
	var v float64
	switch {
	case p.IntValue != nil:
		v = float64(*p.IntValue)
	case p.DoubleValue != nil:
		v = *p.DoubleValue
	}

	t := int64(p.TimeUnixNano / 1e6)
	isStale := (p.Flags)&uint32(1) != 0
	wr.pointLabels = appendAttributesToPromLabels(wr.pointLabels[:0], p.Attributes)

	wr.appendSample(metricName, t, v, isStale)
}

// appendSamplesFromSummary appends summary p to wr.tss
func (wr *writeContext) appendSamplesFromSummary(metricName string, p *pb.SummaryDataPoint) {
	t := int64(p.TimeUnixNano / 1e6)
	isStale := (p.Flags)&uint32(1) != 0
	wr.pointLabels = appendAttributesToPromLabels(wr.pointLabels[:0], p.Attributes)

	wr.appendSample(metricName+"_sum", t, p.Sum, isStale)
	wr.appendSample(metricName+"_count", t, float64(p.Count), isStale)
	for _, q := range p.QuantileValues {
		qValue := strconv.FormatFloat(q.Quantile, 'f', -1, 64)
		wr.appendSampleWithExtraLabel(metricName, "quantile", qValue, t, q.Value, isStale)
	}
}

// appendSamplesFromHistogram appends histogram p to wr.tss
// histograms are processed according to spec at https://github.com/OpenObservability/OpenMetrics/blob/main/specification/OpenMetrics.md#histogram
func (wr *writeContext) appendSamplesFromHistogram(metricName string, p *pb.HistogramDataPoint) {
	if len(p.BucketCounts) == 0 {
		// nothing to append
		return
	}
	if len(p.BucketCounts) != len(p.ExplicitBounds)+1 {
		// fast path, broken data format
		skippedSampleLogger.Warnf("opentelemetry bad histogram format: %q, size of buckets: %d, size of bounds: %d", metricName, len(p.BucketCounts), len(p.ExplicitBounds))
		return
	}

	t := int64(p.TimeUnixNano / 1e6)
	isStale := (p.Flags)&uint32(1) != 0
	wr.pointLabels = appendAttributesToPromLabels(wr.pointLabels[:0], p.Attributes)
	wr.appendSample(metricName+"_count", t, float64(p.Count), isStale)
	if p.Sum != nil {
		// A Histogram MetricPoint SHOULD contain Sum
		wr.appendSample(metricName+"_sum", t, *p.Sum, isStale)
	}

	var cumulative uint64
	for index, bound := range p.ExplicitBounds {
		cumulative += p.BucketCounts[index]
		boundLabelValue := strconv.FormatFloat(bound, 'f', -1, 64)
		wr.appendSampleWithExtraLabel(metricName+"_bucket", "le", boundLabelValue, t, float64(cumulative), isStale)
	}
	cumulative += p.BucketCounts[len(p.BucketCounts)-1]
	wr.appendSampleWithExtraLabel(metricName+"_bucket", "le", "+Inf", t, float64(cumulative), isStale)
}

// appendSamplesFromExponentialHistogram appends histogram p to wr.tss
func (wr *writeContext) appendSamplesFromExponentialHistogram(metricName string, p *pb.ExponentialHistogramDataPoint) {
	t := int64(p.TimeUnixNano / 1e6)
	isStale := (p.Flags)&uint32(1) != 0
	wr.pointLabels = appendAttributesToPromLabels(wr.pointLabels[:0], p.Attributes)
	wr.appendSample(metricName+"_count", t, float64(p.Count), isStale)
	if p.Sum == nil {
		// fast path, convert metric as simple counter.
		// given buckets cannot be used for histogram functions.
		// Negative threshold buckets MAY be used, but then the Histogram MetricPoint MUST NOT contain a sum value as it would no longer be a counter semantically.
		// https://github.com/OpenObservability/OpenMetrics/blob/main/specification/OpenMetrics.md#histogram
		return
	}

	wr.appendSample(metricName+"_sum", t, *p.Sum, isStale)
	if p.ZeroCount > 0 {
		vmRange := fmt.Sprintf("%.3e...%.3e", 0.0, p.ZeroThreshold)
		wr.appendSampleWithExtraLabel(metricName+"_bucket", "vmrange", vmRange, t, float64(p.ZeroCount), isStale)
	}
	ratio := math.Pow(2, -float64(p.Scale))
	base := math.Pow(2, ratio)
	if p.Positive != nil {
		bound := math.Pow(2, float64(p.Positive.Offset)*ratio)
		for i, s := range p.Positive.BucketCounts {
			if s > 0 {
				lowerBound := bound * math.Pow(base, float64(i))
				upperBound := lowerBound * base
				vmRange := fmt.Sprintf("%.3e...%.3e", lowerBound, upperBound)
				wr.appendSampleWithExtraLabel(metricName+"_bucket", "vmrange", vmRange, t, float64(s), isStale)
			}
		}
	}
	if p.Negative != nil {
		bound := math.Pow(2, -float64(p.Negative.Offset)*ratio)
		for i, s := range p.Negative.BucketCounts {
			if s > 0 {
				upperBound := bound * math.Pow(base, float64(i))
				lowerBound := upperBound / base
				vmRange := fmt.Sprintf("%.3e...%.3e", lowerBound, upperBound)
				wr.appendSampleWithExtraLabel(metricName+"_bucket", "vmrange", vmRange, t, float64(s), isStale)
			}
		}
	}
}

// appendSample appends sample with the given metricName to wr.tss
func (wr *writeContext) appendSample(metricName string, t int64, v float64, isStale bool) {
	wr.appendSampleWithExtraLabel(metricName, "", "", t, v, isStale)
}

// appendSampleWithExtraLabel appends sample with the given metricName and the given (labelName=labelValue) extra label to wr.tss
func (wr *writeContext) appendSampleWithExtraLabel(metricName, labelName, labelValue string, t int64, v float64, isStale bool) {
	if isStale {
		v = decimal.StaleNaN
	}
	if t <= 0 {
		// Set the current timestamp if t isn't set.
		t = int64(fasttime.UnixTimestamp()) * 1000
	}

	labelsPool := wr.labelsPool
	labelsLen := len(labelsPool)
	labelsPool = append(labelsPool, prompbmarshal.Label{
		Name:  "__name__",
		Value: metricName,
	})
	labelsPool = append(labelsPool, wr.baseLabels...)
	labelsPool = append(labelsPool, wr.pointLabels...)
	if labelName != "" && labelValue != "" {
		labelsPool = append(labelsPool, prompbmarshal.Label{
			Name:  labelName,
			Value: labelValue,
		})
	}

	samplesPool := wr.samplesPool
	samplesLen := len(samplesPool)
	samplesPool = append(samplesPool, prompbmarshal.Sample{
		Timestamp: t,
		Value:     v,
	})

	wr.tss = append(wr.tss, prompbmarshal.TimeSeries{
		Labels:  labelsPool[labelsLen:],
		Samples: samplesPool[samplesLen:],
	})

	wr.labelsPool = labelsPool
	wr.samplesPool = samplesPool

	rowsRead.Inc()
}

// appendAttributesToPromLabels appends attributes to dst and returns the result.
func appendAttributesToPromLabels(dst []prompbmarshal.Label, attributes []*pb.KeyValue) []prompbmarshal.Label {
	for _, at := range attributes {
		dst = append(dst, prompbmarshal.Label{
			Name:  sanitizeLabelName(at.Key),
			Value: at.Value.FormatString(true),
		})
	}
	return dst
}

type writeContext struct {
	// tss holds parsed time series
	tss []prompbmarshal.TimeSeries

	// baseLabels are labels, which must be added to all the ingested samples
	baseLabels []prompbmarshal.Label

	// pointLabels are labels, which must be added to the ingested OpenTelemetry points
	pointLabels []prompbmarshal.Label

	// pools are used for reducing memory allocations when parsing time series
	labelsPool  []prompbmarshal.Label
	samplesPool []prompbmarshal.Sample
}

func (wr *writeContext) reset() {
	clear(wr.tss)
	wr.tss = wr.tss[:0]

	wr.baseLabels = resetLabels(wr.baseLabels)
	wr.pointLabels = resetLabels(wr.pointLabels)

	wr.labelsPool = resetLabels(wr.labelsPool)
	wr.samplesPool = wr.samplesPool[:0]
}

func resetLabels(labels []prompbmarshal.Label) []prompbmarshal.Label {
	clear(labels)
	return labels[:0]
}

func (wr *writeContext) parseRequestToTss(req *pb.ExportMetricsServiceRequest) {
	for _, rm := range req.ResourceMetrics {
		var attributes []*pb.KeyValue
		if rm.Resource != nil {
			attributes = rm.Resource.Attributes
		}
		wr.baseLabels = appendAttributesToPromLabels(wr.baseLabels[:0], attributes)
		for _, sc := range rm.ScopeMetrics {
			wr.appendSamplesFromScopeMetrics(sc)
		}
	}
}

var wrPool sync.Pool

func getWriteContext() *writeContext {
	v := wrPool.Get()
	if v == nil {
		return &writeContext{}
	}
	return v.(*writeContext)
}

func putWriteContext(wr *writeContext) {
	wr.reset()
	wrPool.Put(wr)
}

var (
	rowsRead                                   = metrics.NewCounter(`vm_protoparser_rows_read_total{type="opentelemetry"}`)
	rowsDroppedUnsupportedHistogram            = metrics.NewCounter(`vm_protoparser_rows_dropped_total{type="opentelemetry",reason="unsupported_histogram_aggregation"}`)
	rowsDroppedUnsupportedExponentialHistogram = metrics.NewCounter(`vm_protoparser_rows_dropped_total{type="opentelemetry",reason="unsupported_exponential_histogram_aggregation"}`)
	rowsDroppedUnsupportedSum                  = metrics.NewCounter(`vm_protoparser_rows_dropped_total{type="opentelemetry",reason="unsupported_sum_aggregation"}`)
	rowsDroppedUnsupportedMetricType           = metrics.NewCounter(`vm_protoparser_rows_dropped_total{type="opentelemetry",reason="unsupported_metric_type"}`)
)
