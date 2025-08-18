package servers

import (
	"flag"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

var (
	maxUniqueTimeseries = flag.Int("search.maxUniqueTimeseries", 0, "The maximum number of unique time series, which can be scanned during every query. "+
		"This allows protecting against heavy queries, which select unexpectedly high number of series. When set to zero, the limit is automatically calculated based on -search.maxConcurrentRequests (inversely proportional) and memory available to the process (proportional). See also -search.max* command-line flags at vmselect")
	maxTagKeys = flag.Int("search.maxTagKeys", 100e3, "The maximum number of tag keys returned per search. "+
		"See also -search.maxLabelsAPISeries and -search.maxLabelsAPIDuration")
	maxTagValues = flag.Int("search.maxTagValues", 100e3, "The maximum number of tag values returned per search. "+
		"See also -search.maxLabelsAPISeries and -search.maxLabelsAPIDuration")
	maxTagValueSuffixesPerSearch = flag.Int("search.maxTagValueSuffixesPerSearch", 100e3, "The maximum number of tag value suffixes returned from /metrics/find")
	maxConcurrentRequests        = flag.Int("search.maxConcurrentRequests", getDefaultMaxConcurrentRequests(), "The maximum number of concurrent vmselect requests "+
		"the vmstorage can process at -vmselectAddr. It shouldn't be high, since a single request usually saturates a CPU core, and many concurrently executed requests "+
		"may require high amounts of memory. See also -search.maxQueueDuration")
	maxQueueDuration = flag.Duration("search.maxQueueDuration", 10*time.Second, "The maximum time the incoming vmselect request waits for execution "+
		"when -search.maxConcurrentRequests limit is reached")

	denyQueriesOutsideRetention = flag.Bool("denyQueriesOutsideRetention", false, "Whether to deny queries outside of the configured -retentionPeriod. "+
		"When set, then /api/v1/query_range would return '503 Service Unavailable' error for queries with 'from' value outside -retentionPeriod. "+
		"This may be useful when multiple data sources with distinct retentions are hidden behind query-tee")
)

var (
	maxUniqueTimeseriesValue     int
	maxUniqueTimeseriesValueOnce sync.Once
)

func getDefaultMaxConcurrentRequests() int {
	n := cgroup.AvailableCPUs() * 2
	if n > 16 {
		// A single request can saturate all the CPU cores, so there is no sense
		// in allowing higher number of concurrent requests - they will just contend
		// for unavailable CPU time.
		n = 16
	}
	return n
}

// GetMaxTagKeys returns value of `-search.maxTagKeys` flag
func GetMaxTagKeys() int {
	return *maxTagKeys
}

// GetMaxTagValues returns value of `-search.maxTagValues` flag
func GetMaxTagValues() int {
	return *maxTagValues
}

// GetMaxTagValueSuffixesPerSearch returns value of `-search.maxTagValueSuffixesPerSearch` flag
func GetMaxTagValueSuffixesPerSearch() int {
	return *maxTagValueSuffixesPerSearch
}

// GetMaxConcurrentRequests returns value of `-search.maxConcurrentRequests` flag.
func GetMaxConcurrentRequests() int {
	return *maxConcurrentRequests
}

// GetMaxQueueDuration returns value of `-search.maxQueueDuration` flag.
func GetMaxQueueDuration() time.Duration {
	return *maxQueueDuration
}

// GetMaxUniqueTimeSeries returns `-search.maxUniqueTimeseries` or the auto-calculated value based on available resources.
// The calculation is split into calculateMaxUniqueTimeSeriesForResource for unit testing.
func GetMaxUniqueTimeSeries() int {
	maxUniqueTimeseriesValueOnce.Do(func() {
		maxUniqueTimeseriesValue = *maxUniqueTimeseries
		if maxUniqueTimeseriesValue <= 0 {
			maxUniqueTimeseriesValue = calculateMaxUniqueTimeSeriesForResource(*maxConcurrentRequests, memory.Remaining())
		}
	})
	return maxUniqueTimeseriesValue
}

// calculateMaxUniqueTimeSeriesForResource calculate the max metrics limit calculated by available resources.
func calculateMaxUniqueTimeSeriesForResource(maxConcurrentRequests, remainingMemory int) int {
	if maxConcurrentRequests <= 0 {
		// This line should NOT be reached unless the user has set an incorrect `search.maxConcurrentRequests`.
		// In such cases, fallback to unlimited.
		logger.Warnf("limiting -search.maxUniqueTimeseries to %v because -search.maxConcurrentRequests=%d.", 2e9, maxConcurrentRequests)
		return 2e9
	}

	// Calculate the max metrics limit for a single request in the worst-case concurrent scenario.
	// The approximate size of 1 unique series that could occupy in the vmstorage is 200 bytes.
	mts := remainingMemory / 200 / maxConcurrentRequests
	logger.Infof("limiting -search.maxUniqueTimeseries to %d according to -search.maxConcurrentRequests=%d and remaining memory=%d bytes. To increase the limit, reduce -search.maxConcurrentRequests or increase memory available to the process.", mts, maxConcurrentRequests, remainingMemory)
	return mts
}

// CheckTimeRange returns true if the given tr is denied for querying.
func CheckTimeRange(s *storage.Storage, tr storage.TimeRange) error {
	if !*denyQueriesOutsideRetention {
		return nil
	}
	retentionMsecs := s.RetentionMsecs()
	minAllowedTimestamp := int64(fasttime.UnixTimestamp()*1000) - retentionMsecs
	if tr.MinTimestamp > minAllowedTimestamp {
		return nil
	}
	return &httpserver.ErrorWithStatusCode{
		Err: fmt.Errorf("the given time range %s is outside the allowed retention %.3f days according to -denyQueriesOutsideRetention",
			&tr, float64(retentionMsecs)/(24*3600*1000)),
		StatusCode: http.StatusServiceUnavailable,
	}
}
