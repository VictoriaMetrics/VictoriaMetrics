package metrics

import (
	"fmt"
	"io"
	"math"
	"sync"
	"time"
)

// PrometheusHistogramDefaultBuckets is a list of the default bucket upper
// bounds. Those default buckets are quite generic, and it is recommended to
// pick custom buckets for improved accuracy.
var PrometheusHistogramDefaultBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

// PrometheusHistogram is a histogram for non-negative values with pre-defined buckets
//
// Each bucket contains a counter for values in the given range.
// Each bucket is exposed via the following metric:
//
// <metric_name>_bucket{<optional_tags>,le="upper_bound"} <counter>
//
// Where:
//
//   - <metric_name> is the metric name passed to NewPrometheusHistogram
//   - <optional_tags> is optional tags for the <metric_name>, which are passed to NewPrometheusHistogram
//   - <upper_bound> - upper bound of the current bucket. all samples <= upper_bound are in that bucket
//   - <counter> - the number of hits to the given bucket during Update* calls
//
// Next to the bucket metrics, two additional metrics track the total number of
// samples (_count) and the total sum (_sum) of all samples:
//
//   - <metric_name>_sum{<optional_tags>} <counter>
//   - <metric_name>_count{<optional_tags>} <counter>
type PrometheusHistogram struct {
	// mu guarantees synchronous update for all the counters.
	//
	// Do not use sync.RWMutex, since it has zero sense from performance PoV.
	// It only complicates the code.
	mu sync.Mutex

	// upperBounds and buckets are aligned by element position:
	// upperBounds[i] defines the upper bound for buckets[i].
	// buckets[i] contains the count of elements <= upperBounds[i]
	upperBounds []float64
	buckets     []uint64

	// count is the counter for all observations on this histogram
	count uint64

	// sum is the sum of all the values put into Histogram
	sum float64
}

// Reset resets previous observations in h.
func (h *PrometheusHistogram) Reset() {
	h.mu.Lock()
	for i := range h.buckets {
		h.buckets[i] = 0
	}
	h.sum = 0
	h.count = 0
	h.mu.Unlock()
}

// Update updates h with v.
//
// Negative values and NaNs are ignored.
func (h *PrometheusHistogram) Update(v float64) {
	if math.IsNaN(v) || v < 0 {
		// Skip NaNs and negative values.
		return
	}
	bucketIdx := -1
	for i, ub := range h.upperBounds {
		if v <= ub {
			bucketIdx = i
			break
		}
	}
	h.mu.Lock()
	h.sum += v
	h.count++
	if bucketIdx == -1 {
		// +Inf, nothing to do, already accounted for in the total sum
		h.mu.Unlock()
		return
	}
	h.buckets[bucketIdx]++
	h.mu.Unlock()
}

// UpdateDuration updates request duration based on the given startTime.
func (h *PrometheusHistogram) UpdateDuration(startTime time.Time) {
	d := time.Since(startTime).Seconds()
	h.Update(d)
}

// NewPrometheusHistogram creates and returns new PrometheusHistogram with the given name
// and PrometheusHistogramDefaultBuckets.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned histogram is safe to use from concurrent goroutines.
func NewPrometheusHistogram(name string) *PrometheusHistogram {
	return defaultSet.NewPrometheusHistogram(name)
}

// NewPrometheusHistogramExt creates and returns new PrometheusHistogram with the given name
// and given upperBounds.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned histogram is safe to use from concurrent goroutines.
func NewPrometheusHistogramExt(name string, upperBounds []float64) *PrometheusHistogram {
	return defaultSet.NewPrometheusHistogramExt(name, upperBounds)
}

// GetOrCreatePrometheusHistogram returns registered PrometheusHistogram with the given name
// or creates a new PrometheusHistogram if the registry doesn't contain histogram with
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
// Performance tip: prefer NewPrometheusHistogram instead of GetOrCreatePrometheusHistogram.
func GetOrCreatePrometheusHistogram(name string) *PrometheusHistogram {
	return defaultSet.GetOrCreatePrometheusHistogram(name)
}

// GetOrCreatePrometheusHistogramExt returns registered PrometheusHistogram with the given name and
// upperBounds or creates new PrometheusHistogram if the registry doesn't contain histogram
// with the given name.
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
// Performance tip: prefer NewPrometheusHistogramExt instead of GetOrCreatePrometheusHistogramExt.
func GetOrCreatePrometheusHistogramExt(name string, upperBounds []float64) *PrometheusHistogram {
	return defaultSet.GetOrCreatePrometheusHistogramExt(name, upperBounds)
}

func newPrometheusHistogram(upperBounds []float64) *PrometheusHistogram {
	mustValidateBuckets(upperBounds)
	last := len(upperBounds) - 1
	if math.IsInf(upperBounds[last], +1) {
		upperBounds = upperBounds[:last] // ignore +Inf bucket as it is covered anyways
	}
	h := PrometheusHistogram{
		upperBounds: upperBounds,
		buckets:     make([]uint64, len(upperBounds)),
	}

	return &h
}

func mustValidateBuckets(upperBounds []float64) {
	if err := ValidateBuckets(upperBounds); err != nil {
		panic(err)
	}
}

// ValidateBuckets validates the given upperBounds and returns an error
// if validation failed.
func ValidateBuckets(upperBounds []float64) error {
	if len(upperBounds) == 0 {
		return fmt.Errorf("upperBounds can't be empty")
	}
	for i := 0; i < len(upperBounds)-1; i++ {
		if upperBounds[i] >= upperBounds[i+1] {
			return fmt.Errorf("upper bounds for the buckets must be strictly increasing")
		}
	}
	return nil
}

// LinearBuckets returns a list of upperBounds for PrometheusHistogram,
// and whose distribution is as follows:
//
//	[start, start + width, start + 2 * width, ... start + (count-1) * width]
//
// Panics if given start, width and count produce negative buckets or none buckets at all.
func LinearBuckets(start, width float64, count int) []float64 {
	if count < 1 {
		panic("LinearBuckets: count can't be less than 1")
	}
	upperBounds := make([]float64, count)
	for i := range upperBounds {
		upperBounds[i] = start
		start += width
	}
	mustValidateBuckets(upperBounds)
	return upperBounds
}

// ExponentialBuckets returns a list of upperBounds for PrometheusHistogram,
// and whose distribution is as follows:
//
//	[start, start * factor pow 1, start * factor pow 2, ... start * factor pow (count-1)]
//
// Panics if given start, width and count produce negative buckets or none buckets at all.
func ExponentialBuckets(start, factor float64, count int) []float64 {
	if count < 1 {
		panic("ExponentialBuckets: count can't be less than 1")
	}
	if factor <= 1 {
		panic("ExponentialBuckets: factor must be greater than 1")
	}
	if start <= 0 {
		panic("ExponentialBuckets: start can't be less than 0")
	}
	upperBounds := make([]float64, count)
	for i := range upperBounds {
		upperBounds[i] = start
		start *= factor
	}
	mustValidateBuckets(upperBounds)
	return upperBounds
}

func (h *PrometheusHistogram) marshalTo(prefix string, w io.Writer) {
	cumulativeSum := uint64(0)
	h.mu.Lock()
	count := h.count
	sum := h.sum
	for i, ub := range h.upperBounds {
		cumulativeSum += h.buckets[i]
		tag := fmt.Sprintf(`le="%v"`, ub)
		metricName := addTag(prefix, tag)
		name, labels := splitMetricName(metricName)
		fmt.Fprintf(w, "%s_bucket%s %d\n", name, labels, cumulativeSum)
	}
	h.mu.Unlock()

	tag := fmt.Sprintf("le=%q", "+Inf")
	metricName := addTag(prefix, tag)
	name, labels := splitMetricName(metricName)
	fmt.Fprintf(w, "%s_bucket%s %d\n", name, labels, count)

	name, labels = splitMetricName(prefix)
	if float64(int64(sum)) == sum {
		fmt.Fprintf(w, "%s_sum%s %d\n", name, labels, int64(sum))
	} else {
		fmt.Fprintf(w, "%s_sum%s %g\n", name, labels, sum)
	}
	fmt.Fprintf(w, "%s_count%s %d\n", name, labels, count)
}

func (h *PrometheusHistogram) metricType() string {
	return "histogram"
}
