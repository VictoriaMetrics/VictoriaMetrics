package metrics

import (
	"fmt"
	"io"
	"math"
	"sync"
	"time"
)

const (
	e10Min            = -9
	e10Max            = 18
	decimalMultiplier = 2
	bucketSize        = 9 * decimalMultiplier
	bucketsCount      = e10Max - e10Min
	decimalPrecision  = 1e-12
)

// Histogram is a histogram for non-negative values with automatically created buckets.
//
// See https://medium.com/@valyala/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350
//
// Each bucket contains a counter for values in the given range.
// Each non-empty bucket is exposed via the following metric:
//
//     <metric_name>_bucket{<optional_tags>,vmrange="<start>...<end>"} <counter>
//
// Where:
//
//     - <metric_name> is the metric name passed to NewHistogram
//     - <optional_tags> is optional tags for the <metric_name>, which are passed to NewHistogram
//     - <start> and <end> - start and end values for the given bucket
//     - <counter> - the number of hits to the given bucket during Update* calls
//
// Histogram buckets can be converted to Prometheus-like buckets with `le` labels
// with `prometheus_buckets(<metric_name>_bucket)` function from PromQL extensions in VictoriaMetrics.
// (see https://github.com/VictoriaMetrics/VictoriaMetrics/wiki/MetricsQL ):
//
//     prometheus_buckets(request_duration_bucket)
//
// Time series produced by the Histogram have better compression ratio comparing to
// Prometheus histogram buckets with `le` labels, since they don't include counters
// for all the previous buckets.
//
// Zero histogram is usable.
type Histogram struct {
	// Mu gurantees synchronous update for all the counters and sum.
	mu sync.Mutex

	buckets [bucketsCount]*histogramBucket

	zeros uint64
	lower uint64
	upper uint64

	sum float64
}

// Reset resets the given histogram.
func (h *Histogram) Reset() {
	h.mu.Lock()
	h.resetLocked()
	h.mu.Unlock()
}

func (h *Histogram) resetLocked() {
	for _, hb := range h.buckets[:] {
		if hb == nil {
			continue
		}
		for offset := range hb.counts[:] {
			hb.counts[offset] = 0
		}
	}
	h.zeros = 0
	h.lower = 0
	h.upper = 0
}

// Update updates h with v.
//
// Negative values and NaNs are ignored.
func (h *Histogram) Update(v float64) {
	if math.IsNaN(v) || v < 0 {
		// Skip NaNs and negative values.
		return
	}
	bucketIdx, offset := getBucketIdxAndOffset(v)
	h.mu.Lock()
	h.updateLocked(v, bucketIdx, offset)
	h.mu.Unlock()
}

func (h *Histogram) updateLocked(v float64, bucketIdx int, offset uint) {
	h.sum += v
	if bucketIdx < 0 {
		// Special cases for zero, too small or too big value
		if offset == 0 {
			h.zeros++
		} else if offset == 1 {
			h.lower++
		} else {
			h.upper++
		}
		return
	}
	hb := h.buckets[bucketIdx]
	if hb == nil {
		hb = &histogramBucket{}
		h.buckets[bucketIdx] = hb
	}
	hb.counts[offset]++
}

// VisitNonZeroBuckets calls f for all buckets with non-zero counters.
//
// vmrange contains "<start>...<end>" string with bucket bounds. The lower bound
// isn't included in the bucket, while the upper bound is included.
// This is required to be compatible with Prometheus-style histogram buckets
// with `le` (less or equal) labels.
func (h *Histogram) VisitNonZeroBuckets(f func(vmrange string, count uint64)) {
	h.mu.Lock()
	h.visitNonZeroBucketsLocked(f)
	h.mu.Unlock()
}

func (h *Histogram) visitNonZeroBucketsLocked(f func(vmrange string, count uint64)) {
	if h.zeros > 0 {
		vmrange := getVMRange(-1, 0)
		f(vmrange, h.zeros)
	}
	if h.lower > 0 {
		vmrange := getVMRange(-1, 1)
		f(vmrange, h.lower)
	}
	for bucketIdx, hb := range h.buckets[:] {
		if hb == nil {
			continue
		}
		for offset, count := range hb.counts[:] {
			if count > 0 {
				vmrange := getVMRange(bucketIdx, uint(offset))
				f(vmrange, count)
			}
		}
	}
	if h.upper > 0 {
		vmrange := getVMRange(-1, 2)
		f(vmrange, h.upper)
	}
}

type histogramBucket struct {
	counts [bucketSize]uint64
}

// NewHistogram creates and returns new histogram with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// The returned histogram is safe to use from concurrent goroutines.
func NewHistogram(name string) *Histogram {
	return defaultSet.NewHistogram(name)
}

// GetOrCreateHistogram returns registered histogram with the given name
// or creates new histogram if the registry doesn't contain histogram with
// the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// The returned histogram is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewHistogram instead of GetOrCreateHistogram.
func GetOrCreateHistogram(name string) *Histogram {
	return defaultSet.GetOrCreateHistogram(name)
}

// UpdateDuration updates request duration based on the given startTime.
func (h *Histogram) UpdateDuration(startTime time.Time) {
	d := time.Since(startTime).Seconds()
	h.Update(d)
}

func getVMRange(bucketIdx int, offset uint) string {
	bucketRangesOnce.Do(initBucketRanges)
	if bucketIdx < 0 {
		if offset > 2 {
			panic(fmt.Errorf("BUG: offset must be in range [0...2] for negative bucketIdx; got %d", offset))
		}
		return bucketRanges[offset]
	}
	idx := 3 + uint(bucketIdx)*bucketSize + offset
	return bucketRanges[idx]
}

func initBucketRanges() {
	bucketRanges[0] = "0...0"
	bucketRanges[1] = fmt.Sprintf("0...%.1fe%d", 1.0, e10Min)
	bucketRanges[2] = fmt.Sprintf("%.1fe%d...+Inf", 1.0, e10Max)
	idx := 3
	start := fmt.Sprintf("%.1fe%d", 1.0, e10Min)
	for bucketIdx := 0; bucketIdx < bucketsCount; bucketIdx++ {
		for offset := 0; offset < bucketSize; offset++ {
			e10 := e10Min + bucketIdx
			m := 1 + float64(offset+1)/decimalMultiplier
			if math.Abs(m-10) < decimalPrecision {
				m = 1
				e10++
			}
			end := fmt.Sprintf("%.1fe%d", m, e10)
			bucketRanges[idx] = start + "..." + end
			idx++
			start = end
		}
	}
}

var (
	// 3 additional buckets for zero, lower and upper.
	bucketRanges     [3 + bucketsCount*bucketSize]string
	bucketRangesOnce sync.Once
)

func (h *Histogram) marshalTo(prefix string, w io.Writer) {
	countTotal := uint64(0)
	h.VisitNonZeroBuckets(func(vmrange string, count uint64) {
		tag := fmt.Sprintf("vmrange=%q", vmrange)
		metricName := addTag(prefix, tag)
		name, filters := splitMetricName(metricName)
		fmt.Fprintf(w, "%s_bucket%s %d\n", name, filters, count)
		countTotal += count
	})
	if countTotal == 0 {
		return
	}
	name, filters := splitMetricName(prefix)
	sum := h.getSum()
	if float64(int64(sum)) == sum {
		fmt.Fprintf(w, "%s_sum%s %d\n", name, filters, int64(sum))
	} else {
		fmt.Fprintf(w, "%s_sum%s %g\n", name, filters, sum)
	}
	fmt.Fprintf(w, "%s_count%s %d\n", name, filters, countTotal)
}

func (h *Histogram) getSum() float64 {
	h.mu.Lock()
	sum := h.sum
	h.mu.Unlock()
	return sum
}

func getBucketIdxAndOffset(v float64) (int, uint) {
	if v < 0 {
		panic(fmt.Errorf("BUG: v must be positive; got %g", v))
	}
	if v == 0 {
		return -1, 0
	}
	if math.IsInf(v, 1) {
		return -1, 2
	}
	e10 := int(math.Floor(math.Log10(v)))
	bucketIdx := e10 - e10Min
	if bucketIdx < 0 {
		return -1, 1
	}
	if bucketIdx >= bucketsCount {
		if bucketIdx == bucketsCount && math.Abs(math.Pow10(e10)-v) < decimalPrecision {
			// Adjust m to be on par with Prometheus 'le' buckets (aka 'less or equal')
			return bucketsCount - 1, bucketSize - 1
		}
		return -1, 2
	}
	m := ((v / math.Pow10(e10)) - 1) * decimalMultiplier
	offset := int(m)
	if offset < 0 {
		offset = 0
	} else if offset >= bucketSize {
		offset = bucketSize - 1
	}
	if math.Abs(float64(offset)-m) < decimalPrecision {
		// Adjust offset to be on par with Prometheus 'le' buckets (aka 'less or equal')
		offset--
		if offset < 0 {
			bucketIdx--
			offset = bucketSize - 1
			if bucketIdx < 0 {
				return -1, 1
			}
		}
	}
	return bucketIdx, uint(offset)
}
