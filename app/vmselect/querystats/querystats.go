package querystats

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var (
	lastQueriesCount = flag.Int("search.queryStats.lastQueriesCount", 20000, "Query stats for /api/v1/status/top_queries is tracked on this number of last queries. "+
		"Zero value disables query stats tracking")
	minQueryDuration = flag.Duration("search.queryStats.minQueryDuration", time.Millisecond, "The minimum duration for queries to track in query stats at /api/v1/status/top_queries. Queries with lower duration are ignored in query stats")
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
func RegisterQuery(query string, timeRangeMsecs int64, startTime time.Time) {
	initOnce.Do(initQueryStats)
	qsTracker.registerQuery(query, timeRangeMsecs, startTime)
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
		logger.Infof("enabled query stats tracking at `/api/v1/status/top_queries` with -search.queryStats.lastQueriesCount=%d, -search.queryStats.minQueryDuration=%s",
			*lastQueriesCount, *minQueryDuration)
	}
	qsTracker = &queryStatsTracker{
		a: make([]queryStatRecord, recordsCount),
	}
}

func (qst *queryStatsTracker) writeJSONQueryStats(w io.Writer, topN int, maxLifetime time.Duration) {
	fmt.Fprintf(w, `{"topN":"%d","maxLifetime":%q,`, topN, maxLifetime)
	fmt.Fprintf(w, `"search.queryStats.lastQueriesCount":%d,`, *lastQueriesCount)
	fmt.Fprintf(w, `"search.queryStats.minQueryDuration":%q,`, *minQueryDuration)
	fmt.Fprintf(w, `"topByCount":[`)
	topByCount := qst.getTopByCount(topN, maxLifetime)
	for i, r := range topByCount {
		fmt.Fprintf(w, `{"query":%q,"timeRangeSeconds":%d,"count":%d}`, r.query, r.timeRangeSecs, r.count)
		if i+1 < len(topByCount) {
			fmt.Fprintf(w, `,`)
		}
	}
	fmt.Fprintf(w, `],"topByAvgDuration":[`)
	topByAvgDuration := qst.getTopByAvgDuration(topN, maxLifetime)
	for i, r := range topByAvgDuration {
		fmt.Fprintf(w, `{"query":%q,"timeRangeSeconds":%d,"avgDurationSeconds":%.3f,"count":%d}`, r.query, r.timeRangeSecs, r.duration.Seconds(), r.count)
		if i+1 < len(topByAvgDuration) {
			fmt.Fprintf(w, `,`)
		}
	}
	fmt.Fprintf(w, `],"topBySumDuration":[`)
	topBySumDuration := qst.getTopBySumDuration(topN, maxLifetime)
	for i, r := range topBySumDuration {
		fmt.Fprintf(w, `{"query":%q,"timeRangeSeconds":%d,"sumDurationSeconds":%.3f,"count":%d}`, r.query, r.timeRangeSecs, r.duration.Seconds(), r.count)
		if i+1 < len(topBySumDuration) {
			fmt.Fprintf(w, `,`)
		}
	}
	fmt.Fprintf(w, `]}`)
}

func (qst *queryStatsTracker) registerQuery(query string, timeRangeMsecs int64, startTime time.Time) {
	registerTime := time.Now()
	duration := registerTime.Sub(startTime)
	if duration < *minQueryDuration {
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
