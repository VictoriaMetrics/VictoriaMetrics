package promql

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var (
	maxQueryStatsRecordLifeTime    = flag.Duration("search.MaxQueryStatsRecordLifeTime", 5*time.Minute, "Limits maximum lifetime for query stats record. With minimum 10 seconds")
	maxQueryStatsTrackerItemsCount = flag.Int("search.MaxQueryStatsItems", 1000, "Limits count for distinct query stat records, keyed by query name and query time range. "+
		"With Maximum 5000 records. Zero value disables query stats recording")
)

var (
	shrinkQueryStatsCalls = metrics.NewCounter(`vm_query_stats_shrink_calls_total`)
	globalQueryStatsTrack *queryStatsTrack
	gQSTOnce              sync.Once
)

// InsertQueryStat - inserts query stats record to global query stats tracker
// with given query name, query time-range, execution time and its duration.
func InsertQueryStat(query string, tr int64, execTime time.Time, duration time.Duration) {
	gQSTOnce.Do(func() {
		initQL()
	})
	globalQueryStatsTrack.insertQueryStat(query, tr, execTime, duration)
}

// WriteQueryStatsResponse - writes query stats to given writer in json format with given aggregate key.
func WriteQueryStatsResponse(w io.Writer, topN int, aggregateBy string) {
	gQSTOnce.Do(func() {
		initQL()
	})
	writeJSONQueryStats(w, globalQueryStatsTrack, topN, aggregateBy)
}

// queryStatsTrack - hold queries statistics
// query name and query range is a group key.
type queryStatsTrack struct {
	maxQueryLogRecordTime time.Duration
	limit                 int
	queryStatsLocker      sync.Mutex
	s                     []queryStats
}

// queryStats - represent single query
type queryStats struct {
	query            string
	queryRange       int64
	queryLastSeen    int64
	queryStatRecords []queryStatRecord
}

type queryStatRecord struct {
	// end-start
	duration time.Duration
	// in seconds as unix_ts.
	execTime int64
}

func initQL() {
	limit := *maxQueryStatsTrackerItemsCount
	if limit > 5000 {
		limit = 5000
	}
	qlt := *maxQueryStatsRecordLifeTime
	if qlt == 0 {
		qlt = time.Second * 10
	}
	logger.Infof("enabled query stats tracking, max records count: %d, max query record lifetime: %s", limit, qlt)
	qst := queryStatsTrack{
		limit:                 limit,
		maxQueryLogRecordTime: qlt,
	}
	go func() {
		for {
			time.Sleep(time.Second * 10)
			qst.shrinkOldStats()
		}
	}()
	globalQueryStatsTrack = &qst
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

func writeJSONQueryStats(w io.Writer, ql *queryStatsTrack, topN int, aggregateBy string) {
	fmt.Fprint(w, `{"status": "ok",`)
	fmt.Fprintf(w, `"top_n": "%d",`, topN)
	fmt.Fprintf(w, `"stats_since": %q,`, time.Now().Add(-*maxQueryStatsRecordLifeTime))
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
		panic("invalid aggregation key")
	}
	fmt.Fprint(w, `]`)
	fmt.Fprint(w, `}`)
}

// drops query stats records less then given time in seconds.
// no need to sort
// its added in chronological order.
// must be called with mutex.
func (qs *queryStats) shrinkOldStatsRecords(t int64) {
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
// shrinks slice by last added query log records with given shrinkSize.
func (qst *queryStatsTrack) shrink(shrinkSize int) {
	if len(qst.s) < shrinkSize {
		return
	}
	sort.Slice(qst.s, func(i, j int) bool {
		return qst.s[i].queryLastSeen < qst.s[j].queryLastSeen
	})
	qst.s = qst.s[shrinkSize:]
}

// drop old keys.
func (qst *queryStatsTrack) shrinkOldStats() {
	qst.queryStatsLocker.Lock()
	defer qst.queryStatsLocker.Unlock()
	t := time.Now().Add(-qst.maxQueryLogRecordTime).Unix()
	qlCopy := make([]queryStats, 0, len(qst.s))
	for _, v := range qst.s {
		v.shrinkOldStatsRecords(t)
		if len(v.queryStatRecords) == 0 {
			continue
		}
		qlCopy = append(qlCopy, v)
	}
	qst.s = qlCopy
}

func (qst *queryStatsTrack) insertQueryStat(query string, tr int64, execTime time.Time, duration time.Duration) {
	qst.queryStatsLocker.Lock()
	defer qst.queryStatsLocker.Unlock()
	if len(qst.s) > qst.limit {
		shrinkQueryStatsCalls.Inc()
		qst.shrink(1)
	}
	for i, v := range qst.s {
		// add record to exist stats, keyed by query string and time-range.
		if v.query == query && v.queryRange == tr {
			v.queryLastSeen = execTime.Unix()
			v.queryStatRecords = append(v.queryStatRecords, queryStatRecord{execTime: execTime.Unix(), duration: duration})
			qst.s[i] = v
			return
		}
	}
	qst.s = append(qst.s, queryStats{
		queryStatRecords: []queryStatRecord{{execTime: execTime.Unix(), duration: duration}},
		queryLastSeen:    execTime.Unix(),
		query:            query,
		queryRange:       tr,
	})

}

// returns
func getTopNQueriesByAvgDuration(qst *queryStatsTrack, top int) []queryStats {
	return getTopNQueryStatsItemsWithFilter(qst, top, func(i, j int) bool {
		lenI := len(qst.s[i].queryStatRecords)
		lenJ := len(qst.s[j].queryStatRecords)
		if lenI == 0 || lenJ == 0 {
			return false
		}
		return int(qst.s[i].Duration())/lenI > int(qst.s[j].Duration())/lenJ
	})
}

func getTopNQueriesByRecordCount(qst *queryStatsTrack, top int) []queryStats {
	return getTopNQueryStatsItemsWithFilter(qst, top, func(i, j int) bool {
		return len(qst.s[i].queryStatRecords) > len(qst.s[j].queryStatRecords)
	})
}

func getTopNQueriesByDuration(qst *queryStatsTrack, top int) []queryStats {
	return getTopNQueryStatsItemsWithFilter(qst, top, func(i, j int) bool {
		return qst.s[i].Duration() > qst.s[j].Duration()
	})
}

func getTopNQueryStatsItemsWithFilter(qst *queryStatsTrack, top int, filterFunc func(i, j int) bool) []queryStats {
	qst.queryStatsLocker.Lock()
	defer qst.queryStatsLocker.Unlock()
	if top > len(qst.s) {
		top = len(qst.s)
	}
	sort.Slice(qst.s, filterFunc)
	result := make([]queryStats, 0, top)
	result = append(result, qst.s[:top]...)
	return result
}
