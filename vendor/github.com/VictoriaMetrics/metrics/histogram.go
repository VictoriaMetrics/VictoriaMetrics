package metrics

import (
	"fmt"
	"io"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// Histogram is a histogram that covers values with the following buckets:
//
//     0
//     (0...1e-9]
//     (1e-9...2e-9]
//     (2e-9...3e-9]
//     ...
//     (9e-9...1e-8]
//     (1e-8...2e-8]
//     ...
//     (1e11...2e11]
//     (2e11...3e11]
//     ...
//     (9e11...1e12]
//     (1e12...Inf]
//
// Each bucket contains a counter for values in the given range.
// Each non-zero bucket is exposed with the following name:
//
//     <metric_name>_vmbucket{<optional_tags>,vmrange="<start>...<end>"} <counter>
//
// Where:
//
//     - <metric_name> is the metric name passed to NewHistogram
//     - <optional_tags> is optional tags for the <metric_name>, which are passed to NewHistogram
//     - <start> and <end> - start and end values for the given bucket
//     - <counter> - the number of hits to the given bucket during Update* calls.
//
// Only non-zero buckets are exposed.
//
// Histogram buckets can be converted to Prometheus-like buckets in VictoriaMetrics
// with `prometheus_buckets(<metric_name>_vmbucket)`:
//
//     histogram_quantile(0.95, prometheus_buckets(rate(request_duration_vmbucket[5m])))
//
// Histogram cannot be used for negative values.
type Histogram struct {
	buckets [bucketsCount]uint64

	sumMu sync.Mutex
	sum   float64
	count uint64
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

// Update updates h with v.
//
// v cannot be negative.
func (h *Histogram) Update(v float64) {
	idx := getBucketIdx(v)
	if idx >= uint(len(h.buckets)) {
		panic(fmt.Errorf("BUG: idx cannot exceed %d; got %d", len(h.buckets), idx))
	}
	atomic.AddUint64(&h.buckets[idx], 1)
	atomic.AddUint64(&h.count, 1)
	h.sumMu.Lock()
	h.sum += v
	h.sumMu.Unlock()
}

// UpdateDuration updates request duration based on the given startTime.
func (h *Histogram) UpdateDuration(startTime time.Time) {
	d := time.Since(startTime).Seconds()
	h.Update(d)
}

func (h *Histogram) marshalTo(prefix string, w io.Writer) {
	count := atomic.LoadUint64(&h.count)
	if count == 0 {
		return
	}
	for i := range h.buckets[:] {
		h.marshalBucket(prefix, w, i)
	}
	// Marshal `_sum` and `_count` metrics.
	name, filters := splitMetricName(prefix)
	h.sumMu.Lock()
	sum := h.sum
	h.sumMu.Unlock()
	if float64(int64(sum)) == sum {
		fmt.Fprintf(w, "%s_sum%s %d\n", name, filters, int64(sum))
	} else {
		fmt.Fprintf(w, "%s_sum%s %g\n", name, filters, sum)
	}
	fmt.Fprintf(w, "%s_count%s %d\n", name, filters, count)
}

func (h *Histogram) marshalBucket(prefix string, w io.Writer, idx int) {
	v := h.buckets[idx]
	if v == 0 {
		return
	}
	start := "0"
	if idx > 0 {
		start = getRangeEndFromBucketIdx(uint(idx - 1))
	}
	end := getRangeEndFromBucketIdx(uint(idx))
	tag := fmt.Sprintf(`vmrange="%s...%s"`, start, end)
	prefix = addTag(prefix, tag)
	name, filters := splitMetricName(prefix)
	fmt.Fprintf(w, "%s_vmbucket%s %d\n", name, filters, v)
}

func getBucketIdx(v float64) uint {
	if v < 0 {
		panic(fmt.Errorf("BUG: v cannot be negative; got %v", v))
	}
	if v == 0 {
		// Fast path for zero.
		return 0
	}
	if math.IsInf(v, 1) {
		return bucketsCount - 1
	}
	e10 := int(math.Floor(math.Log10(v)))
	if e10 < e10Min {
		return 1
	}
	if e10 > e10Max {
		if e10 == e10Max+1 && math.Pow10(e10) == v {
			// Adjust m to be on par with Prometheus 'le' buckets (aka 'less or equal')
			return bucketsCount - 2
		}
		return bucketsCount - 1
	}
	mf := v / math.Pow10(e10)
	m := uint(mf)
	// Handle possible rounding errors
	if m < 1 {
		m = 1
	} else if m > 9 {
		m = 9
	}
	if float64(m) == mf {
		// Adjust m to be on par with Prometheus 'le' buckets (aka 'less or equal')
		m--
	}
	return 1 + m + uint(e10-e10Min)*9
}

func getRangeEndFromBucketIdx(idx uint) string {
	if idx == 0 {
		return "0"
	}
	if idx == 1 {
		return fmt.Sprintf("1e%d", e10Min)
	}
	if idx >= bucketsCount-1 {
		return "+Inf"
	}
	idx -= 2
	e10 := e10Min + int(idx/9)
	m := 2 + (idx % 9)
	if m == 10 {
		e10++
		m = 1
	}
	if e10 == 0 {
		return fmt.Sprintf("%d", m)
	}
	return fmt.Sprintf("%de%d", m, e10)
}

// Each range (10^n..10^(n+1)] for e10Min<=n<=e10Max is split into 9 equal sub-ranges, plus 3 additional buckets:
// - a bucket for zeros
// - a bucket for the range (0..10^e10Min]
// - a bucket for the range (10^(e10Max+1)..Inf]
const bucketsCount = 3 + 9*(1+e10Max-e10Min)

const e10Min = -9
const e10Max = 11
