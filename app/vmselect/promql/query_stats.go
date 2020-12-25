package promql

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

var (
	maxQueryStatsRecordLifeTime    = flag.Duration("search.MaxQueryStatsRecordLifeTime", 10*time.Minute, "Limits maximum lifetime for query stats record. With minimum 10 seconds")
	maxQueryStatsTrackerItemsCount = flag.Int("search.MaxQueryStatsItems", 1000, "Limits count for distinct query stat records, keyed by query name and query time range. "+
		"With Maximum 5000 records. Zero value disables query stats recording")
)

var (
	shrinkQueryStatsCalls   = metrics.NewCounter(`vm_query_stats_shrink_calls_total`)
	globalQueryStatsTracker *queryStatsTracker
	gQSTOnce                sync.Once
)

// InsertQueryStat - inserts query stats record to global query stats tracker
// with given query name, query time-range, execution time and its duration.
func InsertQueryStat(query string, tr int64, execTime time.Time, duration time.Duration) {
	gQSTOnce.Do(func() {
		initQueryStatsTracker()
	})
	globalQueryStatsTracker.insertQueryStat(query, tr, execTime, duration)
}

// WriteQueryStatsResponse - writes query stats to given writer in json format with given aggregate key.
func WriteQueryStatsResponse(w io.Writer, topN int, aggregateBy string) {
	gQSTOnce.Do(func() {
		initQueryStatsTracker()
	})
	writeJSONQueryStats(w, globalQueryStatsTracker, topN, aggregateBy)
}

// queryStatsTracker - hold statistics for all queries,
// query name and query range is a group key.
type queryStatsTracker struct {
	maxQueryLogRecordTime time.Duration
	limit                 int
	queryStatsLocker      sync.Mutex
	qs                    []queryStats
}

// queryStats - represent single query statistic.
type queryStats struct {
	query            string
	queryRange       int64
	queryLastSeen    int64
	queryStatRecords []queryStatRecord
}

// queryStatRecord - one record of query stat.
type queryStatRecord struct {
	// end-start
	duration time.Duration
	// in seconds as unix_ts.
	execTime int64
}

func initQueryStatsTracker() {
	limit := *maxQueryStatsTrackerItemsCount
	if limit > 5000 {
		limit = 5000
	}
	qlt := *maxQueryStatsRecordLifeTime
	if qlt == 0 {
		qlt = time.Second * 10
	}
	logger.Infof("enabled query stats tracking, max records count: %d, max query record lifetime: %s", limit, qlt)
	qst := queryStatsTracker{
		limit:                 limit,
		maxQueryLogRecordTime: qlt,
	}
	go func() {
		for {
			time.Sleep(time.Second * 10)
			qst.dropOutdatedRecords()
		}
	}()
	globalQueryStatsTracker = &qst
}

func formatJSONQueryStats(queries []queryStats) string {
	var s strings.Builder
	for i, q := range queries {
		fmt.Fprintf(&s, `{"query":  %q,`, q.query)
		fmt.Fprintf(&s, `"query_time_range":  %q,`, time.Duration(q.queryRange*1e6))
		fmt.Fprintf(&s, `"cumalative_duration":  %q,`, q.Duration())
		if len(q.queryStatRecords) > 0 {
			fmt.Fprintf(&s, `"avg_duration": %q,`, q.Duration()/time.Duration(len(q.queryStatRecords)))
		}
		fmt.Fprintf(&s, `"requests_count": "%d"`, len(q.queryStatRecords))
		s.WriteString(`}`)
		if i != len(queries)-1 {
			s.WriteString(`,`)
		}

	}
	return s.String()
}

func writeJSONQueryStats(w io.Writer, ql *queryStatsTracker, topN int, aggregateBy string) {
	fmt.Fprintf(w, `{"top_n": "%d",`, topN)
	fmt.Fprintf(w, `"stats_max_duration": %q,`, maxQueryStatsRecordLifeTime.String())
	fmt.Fprint(w, `"top": [`)
	switch aggregateBy {
	case "frequency":
		fmt.Fprint(w, formatJSONQueryStats(getTopNQueriesByRecordCount(ql, topN)))
	case "duration":
		fmt.Fprint(w, formatJSONQueryStats(getTopNQueriesByDuration(ql, topN)))
	case "avg_duration":
		fmt.Fprint(w, formatJSONQueryStats(getTopNQueriesByAvgDuration(ql, topN)))
	default:
		logger.Errorf("invalid aggregation key=%q, report bug", aggregateBy)
		fmt.Fprintf(w, `{"error": "invalid aggregateBy value=%s"}`, aggregateBy)
	}
	fmt.Fprint(w, `]`)
	fmt.Fprint(w, `}`)
}

// drops query stats records less then given time in seconds.
// no need to sort
// its added in chronological order.
// must be called with mutex.
func (qs *queryStats) dropOutDatedRecords(t int64) {
	// fast path
	// compare time with last elem.
	if len(qs.queryStatRecords) > 0 && qs.queryStatRecords[len(qs.queryStatRecords)-1].execTime < t {
		qs.queryStatRecords = qs.queryStatRecords[:0]
		return
	}
	// remove all elements by default.
	shrinkIndex := len(qs.queryStatRecords)
	for i, v := range qs.queryStatRecords {
		if t < v.execTime {
			shrinkIndex = i
			break
		}
	}
	if shrinkIndex > 0 {
		qs.queryStatRecords = qs.queryStatRecords[shrinkIndex:]
	}
}

// calculates cumulative duration for query.
func (qs *queryStats) Duration() time.Duration {
	var cnt time.Duration
	for _, v := range qs.queryStatRecords {
		cnt += v.duration
	}
	return cnt
}

// must be called with mutex,
// shrinks slice by the last added query with given shrinkSize.
func (qst *queryStatsTracker) shrink(shrinkSize int) {
	if len(qst.qs) < shrinkSize {
		return
	}
	sort.Slice(qst.qs, func(i, j int) bool {
		return qst.qs[i].queryLastSeen < qst.qs[j].queryLastSeen
	})
	qst.qs = qst.qs[shrinkSize:]
}

// drop outdated keys.
func (qst *queryStatsTracker) dropOutdatedRecords() {
	qst.queryStatsLocker.Lock()
	defer qst.queryStatsLocker.Unlock()
	t := time.Now().Add(-qst.maxQueryLogRecordTime).Unix()
	var i int
	for _, v := range qst.qs {
		v.dropOutDatedRecords(t)
		if len(v.queryStatRecords) > 0 {
			qst.qs[i] = v
			i++
		}
	}
	if i == len(qst.qs) {
		return
	}
	qst.qs = qst.qs[:i]
}

func (qst *queryStatsTracker) insertQueryStat(query string, tr int64, execTime time.Time, duration time.Duration) {
	qst.queryStatsLocker.Lock()
	defer qst.queryStatsLocker.Unlock()
	// shrink old queries.
	if len(qst.qs) > qst.limit {
		shrinkQueryStatsCalls.Inc()
		qst.shrink(1)
	}
	// add record to exist stats, keyed by query string and time-range.
	for i, v := range qst.qs {
		if v.query == query && v.queryRange == tr {
			v.queryLastSeen = execTime.Unix()
			v.queryStatRecords = append(v.queryStatRecords, queryStatRecord{execTime: execTime.Unix(), duration: duration})
			qst.qs[i] = v
			return
		}
	}
	qst.qs = append(qst.qs, queryStats{
		queryStatRecords: []queryStatRecord{{execTime: execTime.Unix(), duration: duration}},
		queryLastSeen:    execTime.Unix(),
		query:            query,
		queryRange:       tr,
	})

}

func getTopNQueriesByAvgDuration(qst *queryStatsTracker, top int) []queryStats {
	return getTopNQueryStatsItemsWithFilter(qst, top, func(i, j int) bool {
		lenI := len(qst.qs[i].queryStatRecords)
		lenJ := len(qst.qs[j].queryStatRecords)
		if lenI == 0 || lenJ == 0 {
			return false
		}
		return qst.qs[i].Duration()/time.Duration(lenI) > qst.qs[j].Duration()/time.Duration(lenJ)
	})
}

func getTopNQueriesByRecordCount(qst *queryStatsTracker, top int) []queryStats {
	return getTopNQueryStatsItemsWithFilter(qst, top, func(i, j int) bool {
		return len(qst.qs[i].queryStatRecords) > len(qst.qs[j].queryStatRecords)
	})
}

func getTopNQueriesByDuration(qst *queryStatsTracker, top int) []queryStats {
	return getTopNQueryStatsItemsWithFilter(qst, top, func(i, j int) bool {
		return qst.qs[i].Duration() > qst.qs[j].Duration()
	})
}

func getTopNQueryStatsItemsWithFilter(qst *queryStatsTracker, top int, filterFunc func(i, j int) bool) []queryStats {
	qst.queryStatsLocker.Lock()
	defer qst.queryStatsLocker.Unlock()
	if top > len(qst.qs) {
		top = len(qst.qs)
	}
	sort.Slice(qst.qs, filterFunc)
	result := make([]queryStats, 0, top)
	result = append(result, qst.qs[:top]...)
	return result
}
