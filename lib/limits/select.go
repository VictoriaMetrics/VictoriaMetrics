package limits

import (
	"flag"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
)

var (
	maxUniqueTimeseries = flag.Int("search.maxUniqueTimeseries", 0, "The maximum number of unique time series, which can be scanned during every query. "+
		"This allows protecting against heavy queries, which select unexpectedly high number of series. When set to zero, the limit is automatically "+
		"calculated based on -search.maxConcurrentRequests (inversely proportional) and memory available to the process (proportional). "+
		"See also -search.max* command-line flags at vmselect")
	maxLabelNames = flag.Int("search.maxTagKeys", 100e3, "The maximum number of tag keys returned per search. "+
		"See also -search.maxLabelsAPISeries and -search.maxLabelsAPIDuration")
	maxLabelValues = flag.Int("search.maxTagValues", 100e3, "The maximum number of tag values returned per search. "+
		"See also -search.maxLabelsAPISeries and -search.maxLabelsAPIDuration")
	maxTagValueSuffixes   = flag.Int("search.maxTagValueSuffixesPerSearch", 100e3, "The maximum number of tag value suffixes returned from /metrics/find")
	maxConcurrentRequests = flag.Int("search.maxConcurrentRequests", defaultMaxConcurrentRequests(), "The maximum number of concurrent search requests. "+
		"It shouldn't be high, since a single request can saturate all the CPU cores, while many concurrently executed requests may require high amounts of memory. "+
		"See also -search.maxQueueDuration and -search.maxMemoryPerQuery")
	maxQueueDuration = flag.Duration("search.maxQueueDuration", 10*time.Second, "The maximum time the incoming vmselect request waits for execution "+
		"when -search.maxConcurrentRequests limit is reached")
)

func defaultMaxConcurrentRequests() int {
	// A single request can saturate all the CPU cores, so there is no sense
	// in allowing higher number of concurrent requests - they will just contend
	// for unavailable CPU time.
	n := min(cgroup.AvailableCPUs()*2, 16)
	return n
}

// MaxConcurrentRequests returns the maximum number of concurrent requests
// a server can process.
//
// The remaining requests wait for up to MaxQueueDuration for their execution.
func MaxConcurrentRequests() int {
	return *maxConcurrentRequests
}

// MaxConcurrentRequestsFlagName returns the name of the flag used for
// configuring max number of concurrent search requests.
func MaxConcurrentRequestsFlagName() string {
	return "search.maxConcurrentRequests"
}

// MaxQueueDuration returns the maximum duration to wait if
// MaxConcurrentRequests are executed.
func MaxQueueDuration() time.Duration {
	return *maxQueueDuration
}

// MaxQueueDurationFlagName returns the name of the flag used for configuring
// the max time duration during which a search request may remain in queue.
func MaxQueueDurationFlagName() string {
	return "search.maxQueueDuration"
}

// MaxMetrics calculates the max number of metric names a single query is
// allowed to return based on the limit from the search query and
// -search.maxUniqueTimeseries flag value.
func MaxMetrics(searchQueryLimit int) int {
	if searchQueryLimit <= 0 {
		return MaxUniqueTimeseries()
	}
	// searchQueryLimit cannot exceed `-search.maxUniqueTimeseries`
	if *maxUniqueTimeseries != 0 && searchQueryLimit > *maxUniqueTimeseries {
		searchQueryLimit = *maxUniqueTimeseries
	}
	return searchQueryLimit
}

// MaxLabelNames calculates the max number of label names a single query is
// allowed to return based on the limit from the search query and
// -search.maxTagKeys flag value.
func MaxLabelNames(searchQueryLimit int) int {
	return calculateLimit(searchQueryLimit, *maxLabelNames)
}

// MaxLabelValues calculates the max number of label values a single query is
// allowed to return based on the limit from the search query and
// -search.maxTagValues flag value.
func MaxLabelValues(searchQueryLimit int) int {
	return calculateLimit(searchQueryLimit, *maxLabelValues)
}

// MaxTagValueSuffixes calculates the max number of tag value suffixes a single
// query is allowed to return based on the limit from the search query and
// -search.maxTagValueSuffixesPerSearch flag value.
func MaxTagValueSuffixes(searchQueryLimit int) int {
	return calculateLimit(searchQueryLimit, *maxTagValueSuffixes)
}

func calculateLimit(searchQueryLimit, flagValue int) int {
	if 0 < searchQueryLimit && searchQueryLimit < flagValue {
		return searchQueryLimit
	}
	return flagValue
}

var (
	maxUniqueTimeseriesValue     int
	maxUniqueTimeseriesValueOnce sync.Once
)

// MaxUniqueTimeseries returns `-search.maxUniqueTimeseries` or the
// auto-calculated value based on available resources.
func MaxUniqueTimeseries() int {
	maxUniqueTimeseriesValueOnce.Do(func() {
		maxUniqueTimeseriesValue = *maxUniqueTimeseries
		if maxUniqueTimeseriesValue <= 0 {
			maxUniqueTimeseriesValue = calculateMaxUniqueTimeseries(*maxConcurrentRequests, memory.Remaining())
		}
	})
	return maxUniqueTimeseriesValue
}

// calculateMaxUniqueTimeseries calculates the maxUniqueTimeseries limit based
// on available resources.
func calculateMaxUniqueTimeseries(maxConcurrentRequests, remainingMemory int) int {
	if maxConcurrentRequests <= 0 {
		// This line should NOT be reached unless the user has set an incorrect
		// `-search.maxConcurrentRequests`. In such cases, fallback to
		// unlimited.
		logger.Warnf("limiting -search.maxUniqueTimeseries to %v because -search.maxConcurrentRequests=%d.", 2e9, maxConcurrentRequests)
		return 2e9
	}

	// Calculate the maxUniqueTimeseries limit for a single request in the
	// worst-case concurrent scenario. The approximate size of 1 unique series
	// that could occupy in vmstorage is 200 bytes.
	mts := remainingMemory / 200 / maxConcurrentRequests
	logger.Infof("limiting -search.maxUniqueTimeseries to %d according to -search.maxConcurrentRequests=%d and remaining memory=%d bytes. "+
		"To increase the limit, reduce -search.maxConcurrentRequests or increase memory available to the process.", mts, maxConcurrentRequests, remainingMemory)
	return mts
}
