package stream

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
)

// ParseMetricsStream parses OpenTelemetry protobuf or json data from r and calls callback for the parsed rows.
//
// callback shouldn't hold tss items after returning.
//
// optional processBody can be used for pre-processing the read request body from r before parsing it in OpenTelemetry format.
func ParseMetricsStream(r io.Reader, contentType string, isGzipped bool, processBody func([]byte) ([]byte, error), callback func(tss []prompbmarshal.TimeSeries) error) error {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)
	r = wcr

	if isGzipped {
		zr, err := common.GetGzipReader(r)
		if err != nil {
			return fmt.Errorf("cannot read gzip-compressed OpenTelemetry protocol data: %w", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	}

	wr := getWriteMetricsContext()
	defer putWriteMetricsContext(wr)
	req, err := wr.readAndUnpackMetricsRequest(r, contentType, processBody)
	if err != nil {
		return fmt.Errorf("cannot unpack OpenTelemetry metrics: %w", err)
	}
	wr.parseRequestToTss(req)

	if err := callback(wr.tss); err != nil {
		return fmt.Errorf("error when processing OpenTelemetry samples: %w", err)
	}

	return nil
}

func (wr *writeMetricsContext) appendSamplesFromScopeMetrics(sc *pb.ScopeMetrics) {
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
				continue
			}
			for _, p := range m.Histogram.DataPoints {
				wr.appendSamplesFromHistogram(metricName, p)
			}
		default:
			rowsDroppedUnsupportedMetricType.Inc()
			logger.Warnf("unsupported type for metric %q", metricName)
		}
	}
}

// appendSampleFromNumericPoint appends p to wr.tss
func (wr *writeMetricsContext) appendSampleFromNumericPoint(metricName string, p *pb.NumberDataPoint) {
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
func (wr *writeMetricsContext) appendSamplesFromSummary(metricName string, p *pb.SummaryDataPoint) {
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
func (wr *writeMetricsContext) appendSamplesFromHistogram(metricName string, p *pb.HistogramDataPoint) {
	if len(p.BucketCounts) == 0 {
		// nothing to append
		return
	}
	if len(p.BucketCounts) != len(p.ExplicitBounds)+1 {
		// fast path, broken data format
		logger.Warnf("opentelemetry bad histogram format: %q, size of buckets: %d, size of bounds: %d", metricName, len(p.BucketCounts), len(p.ExplicitBounds))
		return
	}

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

	var cumulative uint64
	for index, bound := range p.ExplicitBounds {
		cumulative += p.BucketCounts[index]
		boundLabelValue := strconv.FormatFloat(bound, 'f', -1, 64)
		wr.appendSampleWithExtraLabel(metricName+"_bucket", "le", boundLabelValue, t, float64(cumulative), isStale)
	}
	cumulative += p.BucketCounts[len(p.BucketCounts)-1]
	wr.appendSampleWithExtraLabel(metricName+"_bucket", "le", "+Inf", t, float64(cumulative), isStale)
}

// appendSample appends sample with the given metricName to wr.tss
func (wr *writeMetricsContext) appendSample(metricName string, t int64, v float64, isStale bool) {
	wr.appendSampleWithExtraLabel(metricName, "", "", t, v, isStale)
}

// appendSampleWithExtraLabel appends sample with the given metricName and the given (labelName=labelValue) extra label to wr.tss
func (wr *writeMetricsContext) appendSampleWithExtraLabel(metricName, labelName, labelValue string, t int64, v float64, isStale bool) {
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
			Value: at.Value.FormatString(),
		})
	}
	return dst
}

type writeMetricsContext struct {
	// bb holds the original data (json or protobuf), which must be parsed.
	bb bytesutil.ByteBuffer

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

func (wr *writeMetricsContext) reset() {
	wr.bb.Reset()

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

func (wr *writeMetricsContext) readAndUnpackMetricsRequest(r io.Reader, contentType string, processBody func([]byte) ([]byte, error)) (*pb.ExportMetricsServiceRequest, error) {
	if _, err := wr.bb.ReadFrom(r); err != nil {
		return nil, fmt.Errorf("cannot read request: %w", err)
	}
	var req pb.ExportMetricsServiceRequest
	if processBody != nil {
		data, err := processBody(wr.bb.B)
		if err != nil {
			return nil, fmt.Errorf("cannot process request body: %w", err)
		}
		wr.bb.B = append(wr.bb.B[:0], data...)
	}
	if contentType == "application/json" {
		if err := json.Unmarshal(wr.bb.B, &req); err != nil {
			return nil, fmt.Errorf("cannot unmarshal request from %d bytes: %w", len(wr.bb.B), err)
		}
	} else {
		if err := req.UnmarshalProtobuf(wr.bb.B); err != nil {
			return nil, fmt.Errorf("cannot unmarshal request from %d bytes: %w", len(wr.bb.B), err)
		}
	}
	return &req, nil
}

func (wr *writeMetricsContext) parseRequestToTss(req *pb.ExportMetricsServiceRequest) {
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

var wrMetricsPool sync.Pool

func getWriteMetricsContext() *writeMetricsContext {
	v := wrMetricsPool.Get()
	if v == nil {
		return &writeMetricsContext{}
	}
	return v.(*writeMetricsContext)
}

func putWriteMetricsContext(wr *writeMetricsContext) {
	wr.reset()
	wrMetricsPool.Put(wr)
}

var (
	rowsRead                         = metrics.NewCounter(`vm_protoparser_rows_read_total{type="opentelemetry"}`)
	rowsDroppedUnsupportedHistogram  = metrics.NewCounter(`vm_protoparser_rows_dropped_total{type="opentelemetry",reason="unsupported_histogram_aggregation"}`)
	rowsDroppedUnsupportedSum        = metrics.NewCounter(`vm_protoparser_rows_dropped_total{type="opentelemetry",reason="unsupported_sum_aggregation"}`)
	rowsDroppedUnsupportedMetricType = metrics.NewCounter(`vm_protoparser_rows_dropped_total{type="opentelemetry",reason="unsupported_metric_type"}`)
)
