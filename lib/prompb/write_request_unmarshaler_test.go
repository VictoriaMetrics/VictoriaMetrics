package prompb

import (
	"encoding/binary"
	"math"
	"reflect"
	"testing"
)

func TestUnmarshalTimeSeries(t *testing.T) {
	f := func(src []byte, wantTSS []TimeSeries) {
		t.Helper()

		var tss []TimeSeries
		var err error

		tss, _, _, _, err = unmarshalTimeSeries(src, tss, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !reflect.DeepEqual(tss, wantTSS) {
			t.Fatalf("unexpected result\ngot:\n%v\nwant:\n%v", tss, wantTSS)
		}
	}

	// classic time series with samples, no histogram
	{
		src := encodeTimeSeries(
			[]Label{{Name: "__name__", Value: "rpc_latency_seconds"}, {Name: "job", Value: "node-exporter"}},
			[]Sample{{Value: 1.5, Timestamp: 5000}},
			nil,
		)
		f(src, []TimeSeries{
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds"}, {Name: "job", Value: "node-exporter"}},
				Samples: []Sample{{Value: 1.5, Timestamp: 5000}},
			},
		})
	}

	// basic positive histogram
	{
		nativeHistogramC := nativeHistogramContext{
			countInt:       13,
			sum:            175.5,
			schema:         0,
			zeroThreshold:  0.00001,
			zeroCountInt:   2,
			positiveSpans:  []bucketSpan{{offset: 0, length: 4}, {offset: 2, length: 1}},
			positiveDeltas: []int64{2, -1, 2, -1, 1},
			timestamp:      1000,
		}
		histogram := encodeHistogram(nativeHistogramC)
		src := encodeTimeSeries(
			[]Label{{Name: "__name__", Value: "rpc_latency_seconds"}, {Name: "job", Value: "node-exporter"}},
			nil,
			[][]byte{histogram},
		)
		f(src, []TimeSeries{
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_count"}, {Name: "job", Value: "node-exporter"}},
				Samples: []Sample{{Value: 13, Timestamp: 1000}},
			},
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_sum"}, {Name: "job", Value: "node-exporter"}},
				Samples: []Sample{{Value: 175.5, Timestamp: 1000}},
			},
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_bucket"}, {Name: "job", Value: "node-exporter"}, {Name: "vmrange", Value: appendVmrangeHelper(-0.00001, 0.00001)}},
				Samples: []Sample{{Value: 2, Timestamp: 1000}},
			},
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_bucket"}, {Name: "job", Value: "node-exporter"}, {Name: "vmrange", Value: appendVmrangeHelper(0.5, 1)}},
				Samples: []Sample{{Value: 2, Timestamp: 1000}},
			},
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_bucket"}, {Name: "job", Value: "node-exporter"}, {Name: "vmrange", Value: appendVmrangeHelper(1, 2)}},
				Samples: []Sample{{Value: 1, Timestamp: 1000}},
			},
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_bucket"}, {Name: "job", Value: "node-exporter"}, {Name: "vmrange", Value: appendVmrangeHelper(2, 4)}},
				Samples: []Sample{{Value: 3, Timestamp: 1000}},
			},
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_bucket"}, {Name: "job", Value: "node-exporter"}, {Name: "vmrange", Value: appendVmrangeHelper(4, 8)}},
				Samples: []Sample{{Value: 2, Timestamp: 1000}},
			},
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_bucket"}, {Name: "job", Value: "node-exporter"}, {Name: "vmrange", Value: appendVmrangeHelper(32, 64)}},
				Samples: []Sample{{Value: 3, Timestamp: 1000}},
			},
		})
	}

	// basic negative histogram
	{
		nativeHistogramC := nativeHistogramContext{
			countInt:       7,
			sum:            -15.0,
			schema:         0,
			timestamp:      2000,
			negativeSpans:  []bucketSpan{{offset: 1, length: 2}},
			negativeDeltas: []int64{3, 1},
		}
		histogram := encodeHistogram(nativeHistogramC)
		src := encodeTimeSeries(
			[]Label{{Name: "__name__", Value: "rpc_latency_seconds"}},
			nil,
			[][]byte{histogram},
		)
		f(src, []TimeSeries{
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_count"}},
				Samples: []Sample{{Value: 7, Timestamp: 2000}},
			},
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_sum"}},
				Samples: []Sample{{Value: -15.0, Timestamp: 2000}},
			},
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_bucket"}, {Name: "vmrange", Value: appendVmrangeHelper(-2, -1)}},
				Samples: []Sample{{Value: 3, Timestamp: 2000}},
			},
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_bucket"}, {Name: "vmrange", Value: appendVmrangeHelper(-4, -2)}},
				Samples: []Sample{{Value: 4, Timestamp: 2000}},
			},
		})
	}

	// float histogram
	{
		nativeHistogramC := nativeHistogramContext{
			countInt:         0,
			isCountFloat:     true,
			countFloat:       2.5,
			sum:              1.0,
			schema:           1,
			zeroThreshold:    0.00001,
			isZeroCountFloat: true,
			zeroCountFloat:   0.5,
			timestamp:        3000,
			positiveSpans:    []bucketSpan{{offset: 0, length: 2}},
			positiveCounts:   []float64{1.5, 1.0},
		}
		histogram := encodeHistogram(nativeHistogramC)
		src := encodeTimeSeries(
			[]Label{{Name: "__name__", Value: "rpc_latency_seconds"}},
			nil,
			[][]byte{histogram},
		)
		f(src, []TimeSeries{
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_count"}},
				Samples: []Sample{{Value: 2.5, Timestamp: 3000}},
			},
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_sum"}},
				Samples: []Sample{{Value: 1.0, Timestamp: 3000}},
			},
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_bucket"}, {Name: "vmrange", Value: appendVmrangeHelper(-0.00001, 0.00001)}},
				Samples: []Sample{{Value: 0.5, Timestamp: 3000}},
			},
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_bucket"}, {Name: "vmrange", Value: appendVmrangeHelper(0.7071, 1)}},
				Samples: []Sample{{Value: 1.5, Timestamp: 3000}},
			},
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_bucket"}, {Name: "vmrange", Value: appendVmrangeHelper(1, 1.414)}},
				Samples: []Sample{{Value: 1.0, Timestamp: 3000}},
			},
		})
	}

	// count-only histogram: no buckets, just count and sum
	{
		histogram := encodeHistogram(nativeHistogramContext{
			countInt:  10,
			sum:       42.0,
			schema:    3,
			timestamp: 4000,
		})
		src := encodeTimeSeries(
			[]Label{{Name: "__name__", Value: "rpc_latency_seconds"}},
			nil,
			[][]byte{histogram},
		)
		f(src, []TimeSeries{
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_count"}},
				Samples: []Sample{{Value: 10, Timestamp: 4000}},
			},
			{
				Labels:  []Label{{Name: "__name__", Value: "rpc_latency_seconds_sum"}},
				Samples: []Sample{{Value: 42.0, Timestamp: 4000}},
			},
		})
	}
}

func encodeTimeSeries(labels []Label, samples []Sample, histograms [][]byte) []byte {
	var dst []byte
	for _, l := range labels {
		msg := pbEncodeLabel(l.Name, l.Value)
		dst = pbAppendBytes(dst, 1, msg)
	}
	for _, s := range samples {
		msg := pbEncodeSample(s.Value, s.Timestamp)
		dst = pbAppendBytes(dst, 2, msg)
	}
	for _, h := range histograms {
		dst = pbAppendBytes(dst, 4, h)
	}
	return dst
}

func encodeHistogram(h nativeHistogramContext) []byte {
	var dst []byte

	dst = pbAppendVarint(dst, 1, h.countInt)
	if h.isCountFloat {
		dst = pbAppendDouble(dst, 2, h.countFloat)
	}
	if h.sum != 0 {
		dst = pbAppendDouble(dst, 3, h.sum)
	}
	if h.schema != 0 {
		dst = pbAppendSint32(dst, 4, h.schema)
	}
	if h.zeroThreshold != 0 {
		dst = pbAppendDouble(dst, 5, h.zeroThreshold)
	}
	dst = pbAppendVarint(dst, 6, h.zeroCountInt)
	if h.isZeroCountFloat {
		dst = pbAppendDouble(dst, 7, h.zeroCountFloat)
	}
	for _, span := range h.negativeSpans {
		dst = pbAppendBytes(dst, 8, pbEncodeBucketSpan(span))
	}
	if len(h.negativeDeltas) > 0 {
		dst = pbAppendBytes(dst, 9, pbPackSint64s(h.negativeDeltas))
	}
	if len(h.negativeCounts) > 0 {
		dst = pbAppendBytes(dst, 10, pbPackDoubles(h.negativeCounts))
	}
	for _, span := range h.positiveSpans {
		dst = pbAppendBytes(dst, 11, pbEncodeBucketSpan(span))
	}
	if len(h.positiveDeltas) > 0 {
		dst = pbAppendBytes(dst, 12, pbPackSint64s(h.positiveDeltas))
	}
	if len(h.positiveCounts) > 0 {
		dst = pbAppendBytes(dst, 13, pbPackDoubles(h.positiveCounts))
	}
	if h.timestamp != 0 {
		dst = pbAppendVarint(dst, 15, uint64(h.timestamp))
	}
	return dst
}

func pbEncodeLabel(name, value string) []byte {
	var dst []byte
	dst = pbAppendBytes(dst, 1, []byte(name))
	dst = pbAppendBytes(dst, 2, []byte(value))
	return dst
}

func pbEncodeSample(value float64, timestamp int64) []byte {
	var dst []byte
	if value != 0 {
		dst = pbAppendDouble(dst, 1, value)
	}
	if timestamp != 0 {
		dst = pbAppendVarint(dst, 2, uint64(timestamp))
	}
	return dst
}

func pbEncodeBucketSpan(span bucketSpan) []byte {
	var dst []byte
	if span.offset != 0 {
		dst = pbAppendSint32(dst, 1, span.offset)
	}
	if span.length != 0 {
		dst = pbAppendVarint(dst, 2, uint64(span.length))
	}
	return dst
}

func pbAppendVarint(dst []byte, field uint32, v uint64) []byte {
	dst = appendProtoVarint(dst, uint64(field<<3))
	dst = appendProtoVarint(dst, v)
	return dst
}

func pbAppendDouble(dst []byte, field uint32, v float64) []byte {
	dst = appendProtoVarint(dst, uint64(field<<3|1))
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], math.Float64bits(v))
	dst = append(dst, buf[:]...)
	return dst
}

func pbAppendSint32(dst []byte, field uint32, v int32) []byte {
	dst = appendProtoVarint(dst, uint64(field<<3))
	dst = appendProtoVarint(dst, uint64((uint32(v)<<1)^uint32(v>>31)))
	return dst
}

func pbAppendBytes(dst []byte, field uint32, data []byte) []byte {
	dst = appendProtoVarint(dst, uint64(field<<3|2))
	dst = appendProtoVarint(dst, uint64(len(data)))
	dst = append(dst, data...)
	return dst
}

func pbPackSint64s(values []int64) []byte {
	var dst []byte
	for _, v := range values {
		dst = appendProtoVarint(dst, uint64((uint64(v)<<1)^uint64(v>>63)))
	}
	return dst
}

func pbPackDoubles(values []float64) []byte {
	var dst []byte
	for _, v := range values {
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], math.Float64bits(v))
		dst = append(dst, buf[:]...)
	}
	return dst
}

func appendProtoVarint(dst []byte, v uint64) []byte {
	for v >= 0x80 {
		dst = append(dst, byte(v)|0x80)
		v >>= 7
	}
	dst = append(dst, byte(v))
	return dst
}

func appendVmrangeHelper(lower float64, upper float64) string {
	var vmrange string
	_, vmrange = appendVmrange(nil, lower, upper)
	return vmrange
}
