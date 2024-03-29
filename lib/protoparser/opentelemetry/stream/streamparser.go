package stream

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
)

var (
	// sanitizeMetrics controls sanitizing metric and label names ingested via OpenTelemetry protocol.
	sanitizeMetrics = flag.Bool("opentelemetry.sanitizeMetrics", false, "Sanitize metric and label names for the ingested OpenTelemetry data")
)

// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/b8655058501bed61a06bb660869051491f46840b/pkg/translator/prometheus/normalize_name.go#L19
var unitMap = []struct {
	prefix string
	units  map[string]string
}{
	{
		units: map[string]string{
			// Time
			"d":   "days",
			"h":   "hours",
			"min": "minutes",
			"s":   "seconds",
			"ms":  "milliseconds",
			"us":  "microseconds",
			"ns":  "nanoseconds",

			// Bytes
			"By":   "bytes",
			"KiBy": "kibibytes",
			"MiBy": "mebibytes",
			"GiBy": "gibibytes",
			"TiBy": "tibibytes",
			"KBy":  "kilobytes",
			"MBy":  "megabytes",
			"GBy":  "gigabytes",
			"TBy":  "terabytes",

			// SI
			"m": "meters",
			"V": "volts",
			"A": "amperes",
			"J": "joules",
			"W": "watts",
			"g": "grams",

			// Misc
			"Cel": "celsius",
			"Hz":  "hertz",
			"1":   "",
			"%":   "percent",
		},
	}, {
		prefix: "per",
		units: map[string]string{
			"s":  "second",
			"m":  "minute",
			"h":  "hour",
			"d":  "day",
			"w":  "week",
			"mo": "month",
			"y":  "year",
		},
	},
}

// ParseStream parses OpenTelemetry protobuf or json data from r and calls callback for the parsed rows.
//
// callback shouldn't hold tss items after returning.
//
// optional processBody can be used for pre-processing the read request body from r before parsing it in OpenTelemetry format.
func ParseStream(r io.Reader, isGzipped bool, processBody func([]byte) ([]byte, error), callback func(tss []prompbmarshal.TimeSeries) error) error {
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

	wr := getWriteContext()
	defer putWriteContext(wr)
	req, err := wr.readAndUnpackRequest(r, processBody)
	if err != nil {
		return fmt.Errorf("cannot unpack OpenTelemetry metrics: %w", err)
	}
	wr.parseRequestToTss(req)

	if err := callback(wr.tss); err != nil {
		return fmt.Errorf("error when processing OpenTelemetry samples: %w", err)
	}

	return nil
}

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
func (wr *writeContext) appendSamplesFromHistogram(metricName string, p *pb.HistogramDataPoint) {
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
			Value: at.Value.FormatString(),
		})
	}
	return dst
}

type writeContext struct {
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

func (wr *writeContext) reset() {
	wr.bb.Reset()

	tss := wr.tss
	for i := range tss {
		ts := &tss[i]
		ts.Labels = nil
		ts.Samples = nil
	}
	wr.tss = tss[:0]

	wr.baseLabels = resetLabels(wr.baseLabels)
	wr.pointLabels = resetLabels(wr.pointLabels)

	wr.labelsPool = resetLabels(wr.labelsPool)
	wr.samplesPool = wr.samplesPool[:0]
}

func resetLabels(labels []prompbmarshal.Label) []prompbmarshal.Label {
	for i := range labels {
		labels[i] = prompbmarshal.Label{}
	}
	return labels[:0]
}

func (wr *writeContext) readAndUnpackRequest(r io.Reader, processBody func([]byte) ([]byte, error)) (*pb.ExportMetricsServiceRequest, error) {
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
	if err := req.UnmarshalProtobuf(wr.bb.B); err != nil {
		return nil, fmt.Errorf("cannot unmarshal request from %d bytes: %w", len(wr.bb.B), err)
	}
	return &req, nil
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

// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/b8655058501bed61a06bb660869051491f46840b/pkg/translator/prometheus/normalize_label.go#L26
func sanitizeLabelName(labelName string) string {
	if !*sanitizeMetrics {
		return labelName
	}
	if len(labelName) == 0 {
		return labelName
	}
	labelName = promrelabel.SanitizeLabelName(labelName)
	if unicode.IsDigit(rune(labelName[0])) {
		return "key_" + labelName
	} else if strings.HasPrefix(labelName, "_") && !strings.HasPrefix(labelName, "__") {
		return "key" + labelName
	}
	return labelName
}

// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/b8655058501bed61a06bb660869051491f46840b/pkg/translator/prometheus/normalize_name.go#L83
func sanitizeMetricName(metric *pb.Metric) string {
	if !*sanitizeMetrics {
		return metric.Name
	}
	nameTokens := promrelabel.SanitizeLabelNameParts(metric.Name)
	unitTokens := strings.SplitN(metric.Unit, "/", len(unitMap))
	for i, u := range unitTokens {
		unitToken := strings.TrimSpace(u)
		if unitToken == "" || strings.ContainsAny(unitToken, "{}") {
			continue
		}
		if unit, ok := unitMap[i].units[unitToken]; ok {
			unitToken = unit
		}
		if unitToken != "" && !containsToken(nameTokens, unitToken) {
			unitPrefix := unitMap[i].prefix
			if unitPrefix != "" {
				nameTokens = append(nameTokens, unitPrefix, unitToken)
			} else {
				nameTokens = append(nameTokens, unitToken)
			}
		}
	}
	if metric.Sum != nil && metric.Sum.IsMonotonic {
		nameTokens = moveOrAppend(nameTokens, "total")
	} else if metric.Unit == "1" && metric.Gauge != nil {
		nameTokens = moveOrAppend(nameTokens, "ratio")
	}
	return strings.Join(nameTokens, "_")
}

func containsToken(tokens []string, value string) bool {
	for _, token := range tokens {
		if token == value {
			return true
		}
	}
	return false
}

func moveOrAppend(tokens []string, value string) []string {
	for t := range tokens {
		if tokens[t] == value {
			tokens = append(tokens[:t], tokens[t+1:]...)
			break
		}
	}
	return append(tokens, value)
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
	rowsRead                         = metrics.NewCounter(`vm_protoparser_rows_read_total{type="opentelemetry"}`)
	rowsDroppedUnsupportedHistogram  = metrics.NewCounter(`vm_protoparser_rows_dropped_total{type="opentelemetry",reason="unsupported_histogram_aggregation"}`)
	rowsDroppedUnsupportedSum        = metrics.NewCounter(`vm_protoparser_rows_dropped_total{type="opentelemetry",reason="unsupported_sum_aggregation"}`)
	rowsDroppedUnsupportedMetricType = metrics.NewCounter(`vm_protoparser_rows_dropped_total{type="opentelemetry",reason="unsupported_metric_type"}`)
)
