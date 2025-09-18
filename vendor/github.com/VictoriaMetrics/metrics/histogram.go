package metrics

import (
	"fmt"
	"io"
	"math"
	"sync"
	"time"
)

const (
	e10Min              = -9
	e10Max              = 18
	bucketsPerDecimal   = 18
	decimalBucketsCount = e10Max - e10Min
	bucketsCount        = decimalBucketsCount * bucketsPerDecimal
)

var bucketMultiplier = math.Pow(10, 1.0/bucketsPerDecimal)

// Histogram is a histogram for non-negative values with automatically created buckets.
//
// See https://medium.com/@valyala/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350
//
// Each bucket contains a counter for values in the given range.
// Each non-empty bucket is exposed via the following metric:
//
//	<metric_name>_bucket{<optional_tags>,vmrange="<start>...<end>"} <counter>
//
// Where:
//
//   - <metric_name> is the metric name passed to NewHistogram
//   - <optional_tags> is optional tags for the <metric_name>, which are passed to NewHistogram
//   - <start> and <end> - start and end values for the given bucket
//   - <counter> - the number of hits to the given bucket during Update* calls
//
// Histogram buckets can be converted to Prometheus-like buckets with `le` labels
// with `prometheus_buckets(<metric_name>_bucket)` function from PromQL extensions in VictoriaMetrics.
// (see https://docs.victoriametrics.com/victoriametrics/metricsql/ ):
//
//	prometheus_buckets(request_duration_bucket)
//
// Time series produced by the Histogram have better compression ratio comparing to
// Prometheus histogram buckets with `le` labels, since they don't include counters
// for all the previous buckets.
//
// Zero histogram is usable.
type Histogram struct {
	// Mu guarantees synchronous update for all the counters and sum.
	//
	// Do not use sync.RWMutex, since it has zero sense from performance PoV.
	// It only complicates the code.
	mu sync.Mutex

	// decimalBuckets contains counters for histogram buckets
	decimalBuckets [decimalBucketsCount]*[bucketsPerDecimal]uint64

	// lower is the number of values, which hit the lower bucket
	lower uint64

	// upper is the number of values, which hit the upper bucket
	upper uint64

	// sum is the sum of all the values put into Histogram
	sum float64
}

// Reset resets the given histogram.
func (h *Histogram) Reset() {
	h.mu.Lock()
	for _, db := range h.decimalBuckets[:] {
		if db == nil {
			continue
		}
		for i := range db[:] {
			db[i] = 0
		}
	}
	h.lower = 0
	h.upper = 0
	h.sum = 0
	h.mu.Unlock()
}

// Update updates h with v.
//
// Negative values and NaNs are ignored.
func (h *Histogram) Update(v float64) {
	if math.IsNaN(v) || v < 0 {
		// Skip NaNs and negative values.
		return
	}
	bucketIdx := (math.Log10(v) - e10Min) * bucketsPerDecimal
	h.mu.Lock()
	h.sum += v
	if bucketIdx < 0 {
		h.lower++
	} else if bucketIdx >= bucketsCount {
		h.upper++
	} else {
		idx := uint(bucketIdx)
		if bucketIdx == float64(idx) && idx > 0 {
			// Edge case for 10^n values, which must go to the lower bucket
			// according to Prometheus logic for `le`-based histograms.
			idx--
		}
		decimalBucketIdx := idx / bucketsPerDecimal
		offset := idx % bucketsPerDecimal
		db := h.decimalBuckets[decimalBucketIdx]
		if db == nil {
			var b [bucketsPerDecimal]uint64
			db = &b
			h.decimalBuckets[decimalBucketIdx] = db
		}
		db[offset]++
	}
	h.mu.Unlock()
}

// Merge merges src to h
func (h *Histogram) Merge(src *Histogram) {
	h.mu.Lock()
	defer h.mu.Unlock()

	src.mu.Lock()
	defer src.mu.Unlock()

	h.lower += src.lower
	h.upper += src.upper
	h.sum += src.sum

	for i, dbSrc := range src.decimalBuckets {
		if dbSrc == nil {
			continue
		}
		dbDst := h.decimalBuckets[i]
		if dbDst == nil {
			var b [bucketsPerDecimal]uint64
			dbDst = &b
			h.decimalBuckets[i] = dbDst
		}
		for j := range dbSrc {
			dbDst[j] += dbSrc[j]
		}
	}
}

// VisitNonZeroBuckets calls f for all buckets with non-zero counters.
//
// vmrange contains "<start>...<end>" string with bucket bounds. The lower bound
// isn't included in the bucket, while the upper bound is included.
// This is required to be compatible with Prometheus-style histogram buckets
// with `le` (less or equal) labels.
func (h *Histogram) VisitNonZeroBuckets(f func(vmrange string, count uint64)) {
	h.mu.Lock()
	if h.lower > 0 {
		f(lowerBucketRange, h.lower)
	}
	for decimalBucketIdx, db := range h.decimalBuckets[:] {
		if db == nil {
			continue
		}
		for offset, count := range db[:] {
			if count > 0 {
				bucketIdx := decimalBucketIdx*bucketsPerDecimal + offset
				vmrange := getVMRange(bucketIdx)
				f(vmrange, count)
			}
		}
	}
	if h.upper > 0 {
		f(upperBucketRange, h.upper)
	}
	h.mu.Unlock()
}

// NewHistogram creates and returns new histogram with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
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
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
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

func getVMRange(bucketIdx int) string {
	bucketRangesOnce.Do(initBucketRanges)
	return bucketRanges[bucketIdx]
}

func initBucketRanges() {
	v := math.Pow10(e10Min)
	start := fmt.Sprintf("%.3e", v)
	for i := 0; i < bucketsCount; i++ {
		v *= bucketMultiplier
		end := fmt.Sprintf("%.3e", v)
		bucketRanges[i] = start + "..." + end
		start = end
	}
}

var (
	lowerBucketRange = fmt.Sprintf("0...%.3e", math.Pow10(e10Min))
	upperBucketRange = fmt.Sprintf("%.3e...+Inf", math.Pow10(e10Max))

	bucketRanges     [bucketsCount]string
	bucketRangesOnce sync.Once
)

func (h *Histogram) marshalTo(prefix string, w io.Writer) {
	countTotal := uint64(0)
	h.VisitNonZeroBuckets(func(vmrange string, count uint64) {
		tag := fmt.Sprintf("vmrange=%q", vmrange)
		metricName := addTag(prefix, tag)
		name, labels := splitMetricName(metricName)
		fmt.Fprintf(w, "%s_bucket%s %d\n", name, labels, count)
		countTotal += count
	})
	if countTotal == 0 {
		return
	}
	name, labels := splitMetricName(prefix)
	sum := h.getSum()
	if float64(int64(sum)) == sum {
		fmt.Fprintf(w, "%s_sum%s %d\n", name, labels, int64(sum))
	} else {
		fmt.Fprintf(w, "%s_sum%s %g\n", name, labels, sum)
	}
	fmt.Fprintf(w, "%s_count%s %d\n", name, labels, countTotal)
}

func (h *Histogram) getSum() float64 {
	h.mu.Lock()
	sum := h.sum
	h.mu.Unlock()
	return sum
}

func (h *Histogram) metricType() string {
	return "histogram"
}
