package prompbmarshal

import (
	"fmt"
	"github.com/prometheus/prometheus/model/histogram"
)

// MarshalProtobuf marshals wr to dst and returns the result.
func (wr *WriteRequest) MarshalProtobuf(dst []byte) []byte {
	size := wr.Size()
	dstLen := len(dst)
	if n := size - (cap(dst) - dstLen); n > 0 {
		dst = append(dst[:cap(dst)], make([]byte, n)...)
	}
	dst = dst[:dstLen+size]
	n, err := wr.MarshalToSizedBuffer(dst[dstLen:])
	if err != nil {
		panic(fmt.Errorf("BUG: unexpected error when marshaling WriteRequest: %w", err))
	}
	return dst[:dstLen+n]
}

// Reset resets wr.
func (wr *WriteRequest) Reset() {
	wr.Timeseries = ResetTimeSeries(wr.Timeseries)
}

// ResetTimeSeries clears all the GC references from tss and returns an empty tss ready for further use.
func ResetTimeSeries(tss []TimeSeries) []TimeSeries {
	clear(tss)
	return tss[:0]
}

// Ref: https://github.com/prometheus/prometheus/blob/0f70fc76872ad4cccc56aa51d904ce9221133b64/model/histogram/histogram.go#L318
func (h *Histogram) ToFloatHistogram() *histogram.FloatHistogram {
	fh := &histogram.FloatHistogram{}
	fh.CounterResetHint = histogram.CounterResetHint(h.ResetHint)
	fh.Schema = h.Schema
	fh.ZeroThreshold = h.ZeroThreshold
	fh.ZeroCount = float64(h.ZeroCount)
	fh.Count = float64(h.Count)
	fh.Sum = h.Sum

	fh.PositiveSpans = resize(fh.PositiveSpans, len(h.PositiveSpans))
	for i, hs := range h.PositiveSpans {
		fh.PositiveSpans[i] = histogram.Span{
			Offset: hs.Offset,
			Length: hs.Length,
		}
	}

	fh.NegativeSpans = resize(fh.NegativeSpans, len(h.NegativeSpans))
	for i, hs := range h.NegativeSpans {
		fh.NegativeSpans[i] = histogram.Span{
			Offset: hs.Offset,
			Length: hs.Length,
		}
	}

	fh.PositiveBuckets = resize(fh.PositiveBuckets, len(h.PositiveDeltas))
	var currentPositive float64
	for i, b := range h.PositiveDeltas {
		currentPositive += float64(b)
		fh.PositiveBuckets[i] = currentPositive
	}

	fh.NegativeBuckets = resize(fh.NegativeBuckets, len(h.NegativeDeltas))
	var currentNegative float64
	for i, b := range h.NegativeDeltas {
		currentNegative += float64(b)
		fh.NegativeBuckets[i] = currentNegative
	}

	return fh
}

// Reverse of ToFloatHistogram, convert cumulative bucket counts to deltas
func FromFloatHistogram(fh *histogram.FloatHistogram) *Histogram {
	h := &Histogram{}
	h.ResetHint = ResetHint(fh.CounterResetHint)
	h.Schema = fh.Schema
	h.ZeroThreshold = fh.ZeroThreshold
	h.ZeroCount = uint64(fh.ZeroCount)
	h.Count = uint64(fh.Count)
	h.Sum = fh.Sum

	h.PositiveSpans = resize(h.PositiveSpans, len(fh.PositiveSpans))
	for i, fs := range fh.PositiveSpans {
		h.PositiveSpans[i] = BucketSpan{
			Offset: fs.Offset,
			Length: fs.Length,
		}
	}

	h.NegativeSpans = resize(h.NegativeSpans, len(fh.NegativeSpans))
	for i, fs := range fh.NegativeSpans {
		h.NegativeSpans[i] = BucketSpan{
			Offset: fs.Offset,
			Length: fs.Length,
		}
	}

	h.PositiveDeltas = resize(h.PositiveDeltas, len(fh.PositiveBuckets))
	var previousPositive int64
	for i, b := range fh.PositiveBuckets {
		h.PositiveDeltas[i] = int64(b) - previousPositive
		previousPositive = int64(b)
	}

	h.NegativeDeltas = resize(h.NegativeDeltas, len(fh.NegativeBuckets))
	var previousNegative int64
	for i, b := range fh.NegativeBuckets {
		h.NegativeDeltas[i] = int64(b) - previousNegative
		previousNegative = int64(b)
	}

	return h
}

func resize[T any](items []T, n int) []T {
	if cap(items) < n {
		return make([]T, n)
	}
	return items[:n]
}
