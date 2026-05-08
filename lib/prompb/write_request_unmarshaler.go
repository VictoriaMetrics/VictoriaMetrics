package prompb

import (
	"fmt"
	"math"
	"sync"

	"github.com/VictoriaMetrics/easyproto"
)

// GetWriteRequestUnmarshaler returns WriteRequestUnmarshaler from the pool.
//
// Return the WriteRequestUnmarshaler to the pool when it is no longer needed via PutWriteRequestUnmarshaler call.
func GetWriteRequestUnmarshaler() *WriteRequestUnmarshaler {
	v := wruPool.Get()
	if v == nil {
		return &WriteRequestUnmarshaler{}
	}
	return v.(*WriteRequestUnmarshaler)
}

// PutWriteRequestUnmarshaler returns wru to the pool.
//
// The caller mustn't access wru fields after returning wru to the pool.
func PutWriteRequestUnmarshaler(wru *WriteRequestUnmarshaler) {
	wru.Reset()
	wruPool.Put(wru)
}

var wruPool sync.Pool

// WriteRequestUnmarshaler is reusable unmarshaler for WriteRequest protobuf messages.
//
// It maintains internal pools for labels and samples to reduce memory allocations.
// See UnmarshalProtobuf for details on how to use it.
type WriteRequestUnmarshaler struct {
	wr WriteRequest

	labelsPool  []Label
	samplesPool []Sample
	fb          fmtBuffer
}

// Reset resets wru, so it could be re-used.
func (wru *WriteRequestUnmarshaler) Reset() {
	wru.wr.Reset()

	clear(wru.labelsPool)
	wru.labelsPool = wru.labelsPool[:0]

	clear(wru.samplesPool)
	wru.samplesPool = wru.samplesPool[:0]

	wru.fb.reset()
}

// UnmarshalProtobuf parses the given Protobuf-encoded `src` into an internal WriteRequest instance and returns a pointer to it.
//
// This method avoids allocations by reusing preallocated slices and pools.
//
// Notes:
//   - The `src` slice must remain unchanged for the lifetime of the returned WriteRequest,
//     as the WriteRequest retain references to it.
//   - The returned WriteRequest is only valid until the next call to UnmarshalProtobuf,
//     which reuses internal buffers and structs.
func (wru *WriteRequestUnmarshaler) UnmarshalProtobuf(src []byte) (*WriteRequest, error) {
	wru.Reset()

	var err error

	// message WriteRequest {
	//    repeated TimeSeries timeseries = 1;
	//    reserved 2;
	//    repeated Metadata metadata = 3;
	// }
	tss := wru.wr.Timeseries
	mds := wru.wr.Metadata
	labelsPool := wru.labelsPool
	samplesPool := wru.samplesPool
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return nil, fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return nil, fmt.Errorf("cannot read timeseries data")
			}
			tss, labelsPool, samplesPool, err = unmarshalTimeSeries(data, tss, labelsPool, samplesPool, &wru.fb)
			if err != nil {
				return nil, fmt.Errorf("cannot unmarshal timeseries: %w", err)
			}
		case 3:
			data, ok := fc.MessageData()
			if !ok {
				return nil, fmt.Errorf("cannot read metricMetadata data")
			}
			if len(mds) < cap(mds) {
				mds = mds[:len(mds)+1]
			} else {
				mds = append(mds, MetricMetadata{})
			}
			md := &mds[len(mds)-1]
			if err := md.unmarshalProtobuf(data); err != nil {
				return nil, fmt.Errorf("cannot unmarshal metricMetadata: %w", err)
			}

		}
	}
	wru.wr.Timeseries = tss
	wru.wr.Metadata = mds
	wru.labelsPool = labelsPool
	wru.samplesPool = samplesPool
	return &wru.wr, nil
}

// unmarshalTimeSeries unmarshals TimeSeries messages, which can specify either samples or native histogram samples, but not both.
// See https://github.com/prometheus/prometheus/blob/9a3ac8910b0476d0d73a5c36a54c55baec5829b6/prompb/types.proto#L133
func unmarshalTimeSeries(src []byte, tss []TimeSeries, labelsPool []Label, samplesPool []Sample, fb *fmtBuffer) ([]TimeSeries, []Label, []Sample, error) {
	labelsPoolLen := len(labelsPool)
	samplesPoolLen := len(samplesPool)

	var histograms [][]byte
	var fc easyproto.FieldContext
	var err error

	// message TimeSeries {
	//   repeated Label labels   = 1;
	//   repeated Sample samples = 2;
	//   repeated Histogram histograms = 4
	// }
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return tss, labelsPool, samplesPool, fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot read label data")
			}
			if len(labelsPool) < cap(labelsPool) {
				labelsPool = labelsPool[:len(labelsPool)+1]
			} else {
				labelsPool = append(labelsPool, Label{})
			}
			label := &labelsPool[len(labelsPool)-1]
			if err := label.unmarshalProtobuf(data); err != nil {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot unmarshal label: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot read sample data")
			}
			if len(samplesPool) < cap(samplesPool) {
				samplesPool = samplesPool[:len(samplesPool)+1]
			} else {
				samplesPool = append(samplesPool, Sample{})
			}
			sample := &samplesPool[len(samplesPool)-1]
			if err := sample.unmarshalProtobuf(data); err != nil {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot unmarshal sample: %w", err)
			}
		case 4:
			data, ok := fc.MessageData()
			if !ok {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot read native histogram data")
			}
			histograms = append(histograms, data)
		}
	}

	baseLabels := labelsPool[labelsPoolLen:len(labelsPool):len(labelsPool)]
	samples := samplesPool[samplesPoolLen:len(samplesPool):len(samplesPool)]

	if len(samples) > 0 && len(histograms) > 0 {
		return tss, labelsPool, samplesPool, fmt.Errorf("cannot have both samples and native histograms in the same TimeSeries")
	}

	// classic series with normal samples
	if len(samples) > 0 {
		tss = appendTimeSeries(tss, baseLabels, samples)
		return tss, labelsPool, samplesPool, nil
	}

	for _, hdata := range histograms {
		tss, labelsPool, samplesPool, err = unmarshalHistogram(hdata, tss, labelsPool, samplesPool, baseLabels, fb)
		if err != nil {
			return tss, labelsPool, samplesPool, fmt.Errorf("failed to unmarshal native histogram: %w", err)
		}
	}

	return tss, labelsPool, samplesPool, nil
}

func appendTimeSeries(tss []TimeSeries, labels []Label, samples []Sample) []TimeSeries {
	if len(tss) < cap(tss) {
		tss = tss[:len(tss)+1]
	} else {
		tss = append(tss, TimeSeries{})
	}
	ts := &tss[len(tss)-1]
	ts.Labels = labels
	ts.Samples = samples
	return tss
}

func unmarshalHistogram(src []byte, tss []TimeSeries, labelsPool []Label, samplesPool []Sample, baseLabels []Label, fb *fmtBuffer) ([]TimeSeries, []Label, []Sample, error) {
	// see https://github.com/prometheus/prometheus/blob/9a3ac8910b0476d0d73a5c36a54c55baec5829b6/prompb/types.proto#L57
	// message Histogram {
	//   oneof count { // Count of observations in the histogram.
	//     uint64 count_int   = 1;
	//     double count_float = 2;
	//   }
	//   double sum = 3; // Sum of observations in the histogram.
	//   sint32 schema             = 4;
	//   double zero_threshold     = 5; // Breadth of the zero bucket.
	//   oneof zero_count { // Count in zero bucket.
	//     uint64 zero_count_int     = 6;
	//     double zero_count_float   = 7;
	//   }

	//   repeated BucketSpan negative_spans =  8 [(gogoproto.nullable) = false];
	//   repeated sint64 negative_deltas    =  9; // Count delta of each bucket compared to previous one (or to zero for 1st bucket).
	//   repeated double negative_counts    = 10; // Absolute count of each bucket.

	//   repeated BucketSpan positive_spans = 11 [(gogoproto.nullable) = false];
	//   repeated sint64 positive_deltas    = 12; // Count delta of each bucket compared to previous one (or to zero for 1st bucket).
	//   repeated double positive_counts    = 13; // Absolute count of each bucket.

	//   ResetHint reset_hint               = 14;
	//   int64 timestamp = 15;

	//   repeated double custom_values = 16;
	// }
	nhctx := getNativeHistogramContext()
	defer putNativeHistogramContext(nhctx)

	var err error
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return tss, labelsPool, samplesPool, fmt.Errorf("cannot read next field: %w", err)
		}
		var ok bool
		switch fc.FieldNum {
		case 1:
			nhctx.countInt, ok = fc.Uint64()
			if !ok {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot read count_int")
			}
		case 2:
			nhctx.countFloat, ok = fc.Double()
			if !ok {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot read count_float")
			}
			nhctx.isCountFloat = true
		case 3:
			nhctx.sum, ok = fc.Double()
			if !ok {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot read sum")
			}
		case 4:
			nhctx.schema, ok = fc.Sint32()
			if !ok {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot read schema")
			}
		case 5:
			nhctx.zeroThreshold, ok = fc.Double()
			if !ok {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot read zero_threshold")
			}
		case 6:
			nhctx.zeroCountInt, ok = fc.Uint64()
			if !ok {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot read zero_count_int")
			}
		case 7:
			nhctx.zeroCountFloat, ok = fc.Double()
			if !ok {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot read zero_count_float")
			}
			nhctx.isZeroCountFloat = true
		case 8:
			data, ok := fc.MessageData()
			if !ok {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot read negative_spans")
			}
			nhctx.negativeSpans, err = appendBucketSpan(nhctx.negativeSpans, data)
			if err != nil {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot decode negative_spans: %w", err)
			}
		case 9:
			nhctx.negativeDeltas, ok = fc.UnpackSint64s(nhctx.negativeDeltas)
			if !ok {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot read negative_deltas")
			}
		case 10:
			nhctx.negativeCounts, ok = fc.UnpackDoubles(nhctx.negativeCounts)
			if !ok {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot read negative_counts")
			}
		case 11:
			data, ok := fc.MessageData()
			if !ok {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot read positive_spans")
			}
			nhctx.positiveSpans, err = appendBucketSpan(nhctx.positiveSpans, data)
			if err != nil {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot decode positive_spans: %w", err)
			}
		case 12:
			nhctx.positiveDeltas, ok = fc.UnpackSint64s(nhctx.positiveDeltas)
			if !ok {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot read positive_deltas")
			}
		case 13:
			nhctx.positiveCounts, ok = fc.UnpackDoubles(nhctx.positiveCounts)
			if !ok {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot read positive_counts")
			}
		// case 14: reset_hint exposes extra reset info for query
		case 15:
			nhctx.timestamp, ok = fc.Int64()
			if !ok {
				return tss, labelsPool, samplesPool, fmt.Errorf("cannot read timestamp")
			}
			// case 16: custom_values — internal OTel→Prom only, skip
		}
	}
	tss, labelsPool, samplesPool = nhctx.appendTimeSeries(tss, baseLabels, labelsPool, samplesPool, fb)

	return tss, labelsPool, samplesPool, nil
}

func appendBucketSpan(spans []bucketSpan, src []byte) ([]bucketSpan, error) {
	//	message BucketSpan {
	//	  sint32 offset = 1; // gap to previous span, or index of first bucket for the first span
	//	  uint32 length = 2; // number of consecutive buckets in this span
	//	}
	if len(spans) < cap(spans) {
		spans = spans[:len(spans)+1]
	} else {
		spans = append(spans, bucketSpan{})
	}
	span := &spans[len(spans)-1]
	var err error
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return spans, fmt.Errorf("cannot read next field: %w", err)
		}
		var ok bool
		switch fc.FieldNum {
		case 1:
			span.offset, ok = fc.Sint32()
			if !ok {
				return spans, fmt.Errorf("cannot read offset")
			}
		case 2:
			span.length, ok = fc.Uint32()
			if !ok {
				return spans, fmt.Errorf("cannot read length")
			}
		}
	}
	return spans, nil
}

// appendTimeSeries converts the parsed native histogram into _count, _sum and _bucket
// TimeSeries and appends them to tss.
// See https://prometheus.io/docs/specs/native_histograms/#data-model
func (nhctx *nativeHistogramContext) appendTimeSeries(tss []TimeSeries, baseLabels []Label, labelsPool []Label, samplesPool []Sample, fb *fmtBuffer) ([]TimeSeries, []Label, []Sample) {
	tsMillis := nhctx.timestamp

	count := float64(nhctx.countInt)
	if nhctx.isCountFloat {
		count = nhctx.countFloat
	}

	var baseName string
	var nameValueP *string
	for i := range baseLabels {
		if baseLabels[i].Name == "__name__" {
			baseName = baseLabels[i].Value
			nameValueP = &baseLabels[i].Value
			break
		}
	}
	// metric have no name, skip it
	if baseName == "" {
		return tss, labelsPool, samplesPool
	}

	*nameValueP = fb.formatName(baseName, "_count")
	tss, labelsPool, samplesPool = appendHistogramSeries(tss, labelsPool, samplesPool, baseLabels, "", tsMillis, count)
	*nameValueP = fb.formatName(baseName, "_sum")
	tss, labelsPool, samplesPool = appendHistogramSeries(tss, labelsPool, samplesPool, baseLabels, "", tsMillis, nhctx.sum)

	*nameValueP = fb.formatName(baseName, "_bucket")
	zeroCount := float64(nhctx.zeroCountInt)
	if nhctx.isZeroCountFloat {
		zeroCount = nhctx.zeroCountFloat
	}
	if zeroCount > 0 {
		vmrange := fb.formatVmrange(-nhctx.zeroThreshold, nhctx.zeroThreshold)
		tss, labelsPool, samplesPool = appendHistogramSeries(tss, labelsPool, samplesPool, baseLabels, vmrange, tsMillis, zeroCount)
	}

	ratio := math.Pow(2, -float64(nhctx.schema))
	base := math.Pow(2, ratio)

	tss, labelsPool, samplesPool = appendSpanBuckets(tss, labelsPool, samplesPool, baseLabels, fb, nhctx.positiveSpans, nhctx.positiveDeltas, nhctx.positiveCounts, base, false, tsMillis)
	tss, labelsPool, samplesPool = appendSpanBuckets(tss, labelsPool, samplesPool, baseLabels, fb, nhctx.negativeSpans, nhctx.negativeDeltas, nhctx.negativeCounts, base, true, tsMillis)

	return tss, labelsPool, samplesPool
}

// Bucket counts are stored either in deltas or floatCounts.
// deltas is used for regular histograms with integer counts, storing cumulative deltas;
// floatCounts is used for float histograms, storing absolute counts.
func appendSpanBuckets(
	tss []TimeSeries,
	labelsPool []Label,
	samplesPool []Sample,
	baseLabels []Label,
	fb *fmtBuffer,
	spans []bucketSpan,
	deltas []int64,
	floatCounts []float64,
	base float64,
	negative bool,
	tsMillis int64,
) ([]TimeSeries, []Label, []Sample) {
	useFloatCounts := len(floatCounts) > 0
	var bucketIdx int32
	var deltaIdx, floatIdx int
	var cumDelta int64

	for _, span := range spans {
		bucketIdx += span.offset
		for i := uint32(0); i < span.length; i++ {
			var bucketCount float64
			if useFloatCounts {
				if floatIdx >= len(floatCounts) {
					return tss, labelsPool, samplesPool
				}
				bucketCount = floatCounts[floatIdx]
				floatIdx++
			} else {
				if deltaIdx >= len(deltas) {
					return tss, labelsPool, samplesPool
				}
				cumDelta += deltas[deltaIdx]
				deltaIdx++
				bucketCount = float64(cumDelta)
			}

			if bucketCount > 0 {
				upper := math.Pow(base, float64(bucketIdx))
				lower := upper / base
				if negative {
					lower, upper = -upper, -lower
				}
				vmrange := fb.formatVmrange(lower, upper)
				tss, labelsPool, samplesPool = appendHistogramSeries(tss, labelsPool, samplesPool, baseLabels, vmrange, tsMillis, bucketCount)
			}
			bucketIdx++
		}
	}
	return tss, labelsPool, samplesPool
}

func appendHistogramSeries(tss []TimeSeries, labelsPool []Label, samplesPool []Sample, baseLabels []Label, vmrange string,
	tsMillis int64, value float64,
) ([]TimeSeries, []Label, []Sample) {
	labelsStart := len(labelsPool)
	for _, l := range baseLabels {
		if len(labelsPool) < cap(labelsPool) {
			labelsPool = labelsPool[:len(labelsPool)+1]
		} else {
			labelsPool = append(labelsPool, Label{})
		}
		labelsPool[len(labelsPool)-1] = l
	}
	if vmrange != "" {
		if len(labelsPool) < cap(labelsPool) {
			labelsPool = labelsPool[:len(labelsPool)+1]
		} else {
			labelsPool = append(labelsPool, Label{})
		}
		l := &labelsPool[len(labelsPool)-1]
		l.Name = "vmrange"
		l.Value = vmrange
	}
	labels := labelsPool[labelsStart:len(labelsPool):len(labelsPool)]

	if len(samplesPool) < cap(samplesPool) {
		samplesPool = samplesPool[:len(samplesPool)+1]
	} else {
		samplesPool = append(samplesPool, Sample{})
	}
	s := &samplesPool[len(samplesPool)-1]
	s.Value = value
	s.Timestamp = tsMillis
	samples := samplesPool[len(samplesPool)-1 : len(samplesPool) : len(samplesPool)]

	return appendTimeSeries(tss, labels, samples), labelsPool, samplesPool
}

type bucketSpan struct {
	offset int32
	length uint32
}

type nativeHistogramContext struct {
	isCountFloat     bool
	countInt         uint64
	countFloat       float64
	sum              float64
	schema           int32
	zeroThreshold    float64
	isZeroCountFloat bool
	zeroCountInt     uint64
	zeroCountFloat   float64
	timestamp        int64
	negativeSpans    []bucketSpan
	negativeDeltas   []int64
	negativeCounts   []float64
	positiveSpans    []bucketSpan
	positiveDeltas   []int64
	positiveCounts   []float64
}

func (nhctx *nativeHistogramContext) reset() {
	nhctx.isCountFloat = false
	nhctx.countInt = 0
	nhctx.countFloat = 0
	nhctx.sum = 0
	nhctx.schema = 0
	nhctx.zeroThreshold = 0
	nhctx.isZeroCountFloat = false
	nhctx.zeroCountInt = 0
	nhctx.zeroCountFloat = 0
	nhctx.timestamp = 0
	nhctx.negativeSpans = nhctx.negativeSpans[:0]
	nhctx.negativeDeltas = nhctx.negativeDeltas[:0]
	nhctx.negativeCounts = nhctx.negativeCounts[:0]
	nhctx.positiveSpans = nhctx.positiveSpans[:0]
	nhctx.positiveDeltas = nhctx.positiveDeltas[:0]
	nhctx.positiveCounts = nhctx.positiveCounts[:0]
}

func getNativeHistogramContext() *nativeHistogramContext {
	v := nhctxPool.Get()
	if v == nil {
		return &nativeHistogramContext{}
	}
	return v.(*nativeHistogramContext)
}

func putNativeHistogramContext(nhctx *nativeHistogramContext) {
	nhctx.reset()
	nhctxPool.Put(nhctx)
}

var nhctxPool sync.Pool

func (lbl *Label) unmarshalProtobuf(src []byte) (err error) {
	// message Label {
	//   string name  = 1;
	//   string value = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			name, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read label name")
			}
			lbl.Name = name
		case 2:
			value, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read label value")
			}
			lbl.Value = value
		}
	}
	return nil
}

func (s *Sample) unmarshalProtobuf(src []byte) (err error) {
	// message Sample {
	//   double value    = 1;
	//   int64 timestamp = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			value, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read sample value")
			}
			s.Value = value
		case 2:
			timestamp, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot read sample timestamp")
			}
			s.Timestamp = timestamp
		}
	}
	return nil
}

func (mm *MetricMetadata) unmarshalProtobuf(src []byte) (err error) {
	// message MetricMetadata {
	//   enum MetricType {
	//     UNKNOWN = 0;
	//     COUNTER = 1;
	//     GAUGE = 2;
	//     HISTOGRAM = 3;
	//     GAUGEHISTOGRAM = 4;
	//     SUMMARY = 5;
	//     INFO = 6;
	//     STATESET = 7;
	//   }
	//
	//   MetricType type = 1;
	//   string metric_family_name = 2;
	//   string help = 4;
	//   string unit = 5;
	//
	//   uint32 AccountID = 11;
	//   uint32 ProjectID = 12;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			value, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read metric type")
			}
			mm.Type = MetricType(value)
		case 2:
			value, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read metric family name")
			}
			mm.MetricFamilyName = value
		case 4:
			value, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read help")
			}
			mm.Help = value
		case 5:
			value, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read unit")
			}
			mm.Unit = value
		case 11:
			value, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read AccountID")
			}
			mm.AccountID = value
		case 12:
			value, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read ProjectID")
			}
			mm.ProjectID = value
		}
	}
	return nil
}
