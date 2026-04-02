package querystats

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/stringsutil"
)

var (
	lastQueriesCount = flag.Int("search.queryStats.lastQueriesCount", 20000, "Query stats for /api/v1/status/top_queries is tracked on this number of last queries. "+
		"Zero value disables query stats tracking")
	minQueryDuration    = flag.Duration("search.queryStats.minQueryDuration", time.Millisecond, "The minimum duration for queries to track in query stats at /api/v1/status/top_queries. Queries with lower duration are ignored in query stats")
	minQueryMemoryUsage = flagutil.NewBytes("search.queryStats.minQueryMemoryUsage", 1024, "The minimum memory bytes consumption for queries to track in query stats at /api/v1/status/top_queries. Queries with lower memory bytes consumption are ignored in query stats")
)

var (
	qsTracker *queryStatsTracker
	initOnce  sync.Once
)

// Enabled returns true of query stats tracking is enabled.
func Enabled() bool {
	return *lastQueriesCount > 0
}

// RegisterQuery registers the query on the given timeRangeMsecs, which has been started at startTime.
//
// RegisterQuery must be called when the query is finished.
func RegisterQuery(query string, timeRangeMsecs int64, startTime time.Time, memoryUsage int64) {
	initOnce.Do(initQueryStats)
	qsTracker.registerQuery(query, timeRangeMsecs, startTime, memoryUsage)
}

// WriteJSONQueryStats writes query stats to given writer in json format.
func WriteJSONQueryStats(w io.Writer, topN int, maxLifetime time.Duration) {
	initOnce.Do(initQueryStats)
	qsTracker.writeJSONQueryStats(w, topN, maxLifetime)
}

// queryStatsTracker holds statistics for queries
type queryStatsTracker struct {
	mu      sync.Mutex
	a       []queryStatRecord
	nextIdx uint
}

type queryStatRecord struct {
	query         string
	timeRangeSecs int64
	registerTime  time.Time
	duration      time.Duration
	memoryUsage   int64
}

type queryStatKey struct {
	query         string
	timeRangeSecs int64
}

func initQueryStats() {
	recordsCount := *lastQueriesCount
	if recordsCount <= 0 {
		recordsCount = 1
	} else {
		logger.Infof("enabled query stats tracking at `/api/v1/status/top_queries` with -search.queryStats.lastQueriesCount=%d, -search.queryStats.minQueryDuration=%s, -search.queryStats.minQueryMemoryUsage=%s",
			*lastQueriesCount, *minQueryDuration, minQueryMemoryUsage)
	}
	qsTracker = &queryStatsTracker{
		a: make([]queryStatRecord, recordsCount),
	}
}

func (qst *queryStatsTracker) writeJSONQueryStats(w io.Writer, topN int, maxLifetime time.Duration) {
	fmt.Fprintf(w, `{"topN":"%d","maxLifetime":"%s",`, topN, maxLifetime)
	fmt.Fprintf(w, `"search.queryStats.lastQueriesCount":%d,`, *lastQueriesCount)
	fmt.Fprintf(w, `"search.queryStats.minQueryDuration":"%s",`, *minQueryDuration)
	fmt.Fprintf(w, `"search.queryStats.minQueryMemoryUsage":"%s",`, minQueryMemoryUsage)
	fmt.Fprintf(w, `"topByCount":[`)
	topByCount := qst.getTopByCount(topN, maxLifetime)
	for i, r := range topByCount {
		fmt.Fprintf(w, `{"query":%s,"timeRangeSeconds":%d,"count":%d}`, stringsutil.JSONString(r.query), r.timeRangeSecs, r.count)
		if i+1 < len(topByCount) {
			fmt.Fprintf(w, `,`)
		}
	}
	fmt.Fprintf(w, `],"topByAvgDuration":[`)
	topByAvgDuration := qst.getTopByAvgDuration(topN, maxLifetime)
	for i, r := range topByAvgDuration {
		fmt.Fprintf(w, `{"query":%s,"timeRangeSeconds":%d,"avgDurationSeconds":%.3f,"count":%d}`, stringsutil.JSONString(r.query), r.timeRangeSecs, r.duration.Seconds(), r.count)
		if i+1 < len(topByAvgDuration) {
			fmt.Fprintf(w, `,`)
		}
	}
	fmt.Fprintf(w, `],"topBySumDuration":[`)
	topBySumDuration := qst.getTopBySumDuration(topN, maxLifetime)
	for i, r := range topBySumDuration {
		fmt.Fprintf(w, `{"query":%s,"timeRangeSeconds":%d,"sumDurationSeconds":%.3f,"count":%d}`, stringsutil.JSONString(r.query), r.timeRangeSecs, r.duration.Seconds(), r.count)
		if i+1 < len(topBySumDuration) {
			fmt.Fprintf(w, `,`)
		}
	}

	fmt.Fprintf(w, `],"topByAvgMemoryUsage":[`)
	topByAvgMemoryConsumption := qst.getTopByAvgMemoryUsage(topN, maxLifetime)
	for i, r := range topByAvgMemoryConsumption {
		fmt.Fprintf(w, `{"query":%s,"timeRangeSeconds":%d,"avgMemoryBytes":%d,"count":%d}`, stringsutil.JSONString(r.query), r.timeRangeSecs, r.memoryUsage, r.count)
		if i+1 < len(topByAvgMemoryConsumption) {
			fmt.Fprintf(w, `,`)
		}
	}

	fmt.Fprintf(w, `]}`)
}

func (qst *queryStatsTracker) registerQuery(query string, timeRangeMsecs int64, startTime time.Time, memoryUsage int64) {
	registerTime := time.Now()
	duration := registerTime.Sub(startTime)
	if duration < *minQueryDuration {
		return
	}
	if memoryUsage < int64(minQueryMemoryUsage.IntN()) {
		return
	}

	qst.mu.Lock()
	defer qst.mu.Unlock()

	a := qst.a
	idx := qst.nextIdx
	if idx >= uint(len(a)) {
		idx = 0
	}
	qst.nextIdx = idx + 1
	r := &a[idx]
	r.query = query
	r.timeRangeSecs = timeRangeMsecs / 1000
	r.registerTime = registerTime
	r.duration = duration
	r.memoryUsage = memoryUsage
}

func (r *queryStatRecord) matches(currentTime time.Time, maxLifetime time.Duration) bool {
	if r.query == "" || currentTime.Sub(r.registerTime) > maxLifetime {
		return false
	}
	return true
}

func (r *queryStatRecord) key() queryStatKey {
	return queryStatKey{
		query:         r.query,
		timeRangeSecs: r.timeRangeSecs,
	}
}

func (qst *queryStatsTracker) getTopByCount(topN int, maxLifetime time.Duration) []queryStatByCount {
	currentTime := time.Now()
	qst.mu.Lock()
	m := make(map[queryStatKey]int)
	for _, r := range qst.a {
		if r.matches(currentTime, maxLifetime) {
			k := r.key()
			m[k] = m[k] + 1
		}
	}
	qst.mu.Unlock()

	var a []queryStatByCount
	for k, count := range m {
		a = append(a, queryStatByCount{
			query:         k.query,
			timeRangeSecs: k.timeRangeSecs,
			count:         count,
		})
	}
	sort.Slice(a, func(i, j int) bool {
		return a[i].count > a[j].count
	})
	if len(a) > topN {
		a = a[:topN]
	}
	return a
}

type queryStatByCount struct {
	query         string
	timeRangeSecs int64
	count         int
}

func (qst *queryStatsTracker) getTopByAvgDuration(topN int, maxLifetime time.Duration) []queryStatByDuration {
	currentTime := time.Now()
	qst.mu.Lock()
	type countSum struct {
		count int
		sum   time.Duration
	}
	m := make(map[queryStatKey]countSum)
	for _, r := range qst.a {
		if r.matches(currentTime, maxLifetime) {
			k := r.key()
			ks := m[k]
			ks.count++
			ks.sum += r.duration
			m[k] = ks
		}
	}
	qst.mu.Unlock()

	var a []queryStatByDuration
	for k, ks := range m {
		a = append(a, queryStatByDuration{
			query:         k.query,
			timeRangeSecs: k.timeRangeSecs,
			duration:      ks.sum / time.Duration(ks.count),
			count:         ks.count,
		})
	}
	sort.Slice(a, func(i, j int) bool {
		return a[i].duration > a[j].duration
	})
	if len(a) > topN {
		a = a[:topN]
	}
	return a
}

type queryStatByDuration struct {
	query         string
	timeRangeSecs int64
	duration      time.Duration
	count         int
}

func (qst *queryStatsTracker) getTopBySumDuration(topN int, maxLifetime time.Duration) []queryStatByDuration {
	currentTime := time.Now()
	qst.mu.Lock()
	type countDuration struct {
		count int
		sum   time.Duration
	}
	m := make(map[queryStatKey]countDuration)
	for _, r := range qst.a {
		if r.matches(currentTime, maxLifetime) {
			k := r.key()
			kd := m[k]
			kd.count++
			kd.sum += r.duration
			m[k] = kd
		}
	}
	qst.mu.Unlock()

	var a []queryStatByDuration
	for k, kd := range m {
		a = append(a, queryStatByDuration{
			query:         k.query,
			timeRangeSecs: k.timeRangeSecs,
			duration:      kd.sum,
			count:         kd.count,
		})
	}
	sort.Slice(a, func(i, j int) bool {
		return a[i].duration > a[j].duration
	})
	if len(a) > topN {
		a = a[:topN]
	}
	return a
}

type queryStatByMemory struct {
	query         string
	timeRangeSecs int64
	memoryUsage   int64
	count         int
}

func (qst *queryStatsTracker) getTopByAvgMemoryUsage(topN int, maxLifetime time.Duration) []queryStatByMemory {
	currentTime := time.Now()
	qst.mu.Lock()
	type countSum struct {
		count int
		sum   int64
	}
	m := make(map[queryStatKey]countSum)
	for _, r := range qst.a {
		if r.matches(currentTime, maxLifetime) {
			k := r.key()
			ks := m[k]
			ks.count++
			ks.sum += r.memoryUsage
			m[k] = ks
		}
	}
	qst.mu.Unlock()

	var a []queryStatByMemory
	for k, ks := range m {
		a = append(a, queryStatByMemory{
			query:         k.query,
			timeRangeSecs: k.timeRangeSecs,
			memoryUsage:   ks.sum / int64(ks.count),
			count:         ks.count,
		})
	}
	sort.Slice(a, func(i, j int) bool {
		return a[i].memoryUsage > a[j].memoryUsage
	})
	if len(a) > topN {
		a = a[:topN]
	}
	return a
}
