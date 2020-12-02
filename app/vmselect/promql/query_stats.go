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
)

var (
	maxQueryStatsRecordLifeTime    = flag.Duration("search.MaxQueryStatsRecordLifeTime", 5*time.Minute, "Limit maximum lifetime for query stats record. With minimum 10 seconds")
	maxQueryStatsTrackerItemsCount = flag.Int("search.MaxQueryStatsItems", 100, "Limits count for distinct query stat records, keyed by query name and query time range. "+
		"With Maximum 1000 records. Zero value disables query stats recording")
)

var (
	globalQueryStatsTrack *queryStatsTracker
	gqlOnce               sync.Once
)

// InsertQueryStat - inserts query stats record to global query stats tracker
// with given query name, query time-range, execution time and its duration.
func InsertQueryStat(query string, tr int64, execTime time.Time, duration time.Duration) {
	gqlOnce.Do(func() {
		initQL()
	})
	globalQueryStatsTrack.insertQuery(query, tr, execTime, duration)
}

// WriteQueryStatsResponse - writes query stats to given writer in json format.
func WriteQueryStatsResponse(w io.Writer, topN int) {
	gqlOnce.Do(func() {
		initQL()
	})
	writeJSONQueryStats(w, globalQueryStatsTrack, topN)
}

// queryStatsTracker - tracks queryStatRecords execution time,
// query name and query range is a group key.
type queryStatsTracker struct {
	queryStatsLocker      sync.Mutex
	maxQueryLogRecordTime time.Duration
	limit                 int
	s                     []queryStats
}

type queryStats struct {
	query            string
	queryRange       int64
	queryLastSeen    int64
	queryStatRecords []queryStatRecord
}

type queryStatRecord struct {
	duration time.Duration
	// in seconds as unix_ts.
	execTime int64
}

func initQL() {
	limit := *maxQueryStatsTrackerItemsCount
	if limit > 1000 {
		limit = 1000
	}
	qlt := *maxQueryStatsRecordLifeTime
	if qlt == 0 {
		qlt = time.Second * 10
	}
	logger.Infof("enabled search query stats tracker, max records count: %d, max query record duration: %s", limit, qlt)
	qst := queryStatsTracker{
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
		fmt.Fprintf(&s, `"range":  %q,`, time.Duration(q.queryRange*1e6))
		fmt.Fprintf(&s, `"duration":  %q,`, q.duration())
		if len(q.queryStatRecords) > 0 {
			fmt.Fprintf(&s, `"avg_duration": %q,`, q.duration()/time.Duration(len(q.queryStatRecords)))
		}
		fmt.Fprintf(&s, `"count": "%d"`, len(q.queryStatRecords))
		s.WriteString(`}`)
		if i != len(queries)-1 {
			s.WriteString(`,`)
		}

	}
	return s.String()
}

func writeJSONQueryStats(w io.Writer, ql *queryStatsTracker, topN int) {
	fmt.Fprint(w, `{"status": "ok",`)
	fmt.Fprintf(w, `"top_n": "%d",`, topN)
	fmt.Fprint(w, `"top_by_duration": [`)
	fmt.Fprint(w, formatJSONQueryStats(getTopNQueriesByDuration(ql, topN)))
	fmt.Fprint(w, `],`)
	fmt.Fprint(w, `"top_by_count": [`)
	fmt.Fprint(w, formatJSONQueryStats(getTopNQueriesByRecordCount(ql, topN)))
	fmt.Fprint(w, `],`)
	fmt.Fprint(w, `"top_by_avg_duration": [`)
	fmt.Fprint(w, formatJSONQueryStats(getTopNQueriesByAvgDuration(ql, topN)))
	fmt.Fprint(w, `]`)
	fmt.Fprint(w, `}`)
}

// drops query stats records less then given time.
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
func (qs *queryStats) duration() time.Duration {
	var cnt time.Duration
	for _, v := range qs.queryStatRecords {
		cnt += v.duration
	}
	return cnt
}

// must be called with mutex,
// shrinks slice by last added query log records.
func (qst *queryStatsTracker) shrink() {
	sort.Slice(qst.s, func(i, j int) bool {
		return qst.s[i].queryLastSeen < qst.s[j].queryLastSeen
	})
	var ts []int64
	for _, v := range qst.s {
		ts = append(ts, v.queryLastSeen)
	}
	mustShrinkItems := 10
	// that's odd and seems like bug if len < limit.
	if len(qst.s) < mustShrinkItems {
		return
	}
	qst.s = qst.s[mustShrinkItems:]
}

// drop old keys.
func (qst *queryStatsTracker) shrinkOldStats() {
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

func (qst *queryStatsTracker) insertQuery(query string, tr int64, execTime time.Time, duration time.Duration) {
	qst.queryStatsLocker.Lock()
	defer qst.queryStatsLocker.Unlock()
	if len(qst.s) > qst.limit+10 {
		qst.shrink()
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
func getTopNQueriesByAvgDuration(qst *queryStatsTracker, top int) []queryStats {
	return getTopNQueryStatsItemsWithFilter(qst, top, func(i, j int) bool {
		lenI := len(qst.s[i].queryStatRecords)
		lenJ := len(qst.s[j].queryStatRecords)
		if lenI == 0 || lenJ == 0 {
			return false
		}
		return int(qst.s[i].duration())/lenI > int(qst.s[j].duration())/lenJ
	})
}

func getTopNQueriesByRecordCount(qst *queryStatsTracker, top int) []queryStats {
	return getTopNQueryStatsItemsWithFilter(qst, top, func(i, j int) bool {
		return len(qst.s[i].queryStatRecords) > len(qst.s[j].queryStatRecords)
	})
}

func getTopNQueriesByDuration(qst *queryStatsTracker, top int) []queryStats {
	return getTopNQueryStatsItemsWithFilter(qst, top, func(i, j int) bool {
		return qst.s[i].duration() > qst.s[j].duration()
	})
}

func getTopNQueryStatsItemsWithFilter(qst *queryStatsTracker, top int, filterFunc func(i, j int) bool) []queryStats {
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
