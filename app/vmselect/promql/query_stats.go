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
	maxQueryLogRecordTime = flag.Duration("search.logQueryRecordMaxTime", 5*time.Minute, "Limit maximum lifetime for query log record. With minimum 10seconds")
	maxQueryLogsCount     = flag.Int("search.logsQueryMaxCount", 100, "Limits count for distinct query message + time range for tracking query execution stats. With Maximum 1000 records.")
)

var (
	globalQueryLogger *queryLogger
	gqlOnce           sync.Once
)

// queryLogger - tracks queries execution time,
// query name and query range is a key for tracking.
type queryLogger struct {
	mu                    sync.Mutex
	maxQueryLogRecordTime time.Duration
	limit                 int
	s                     []queryLog
}

type queryLog struct {
	query     string
	timeRange int64
	lastSeen  int64
	queries   []queryLogRecord
}

type queryLogRecord struct {
	// stop-start in unix_nano.
	duration time.Duration
	// in seconds as unix_ts.
	execTime int64
}

func initQL() {
	limit := *maxQueryLogsCount
	if limit > 1000 {
		limit = 1000
	}
	qlt := *maxQueryLogRecordTime
	if qlt == 0 {
		qlt = time.Second * 10
	}
	logger.Infof("enabled search query stats profiler, max records count: %d, max query record duration: %s", limit, qlt)
	ql := queryLogger{
		limit:                 limit,
		maxQueryLogRecordTime: qlt,
	}
	go func() {
		for {
			time.Sleep(time.Second * 10)
			ql.dropOldQueryRecords()
		}
	}()
	globalQueryLogger = &ql
}

func formatJSONQueryLog(queries []queryLog) string {
	var s strings.Builder
	for i, q := range queries {
		fmt.Fprintf(&s, `{"query":  %q,`, q.query)
		fmt.Fprintf(&s, `"range":  %q,`, time.Duration(q.timeRange*1e6))
		fmt.Fprintf(&s, `"duration":  %q,`, q.duration())
		if len(q.queries) > 0 {
			fmt.Fprintf(&s, `"avg_duration": %q,`, q.duration()/time.Duration(len(q.queries)))
		}
		fmt.Fprintf(&s, `"count": "%d"`, len(q.queries))
		s.WriteString(`}`)
		if i != len(queries)-1 {
			s.WriteString(`,`)
		}

	}
	return s.String()
}

// WriteQueryStatsResponse - writes query stats to given writer in json format.
func WriteQueryStatsResponse(w io.Writer, topN int) {
	if globalQueryLogger == nil {
		fmt.Fprintf(w, `{"error": "query stats endpoint is disabled, according to flag value -search.logsQueryMaxCount=0"}`)
		return
	}
	writeJSONQueryStats(w, globalQueryLogger, topN)
}

func writeJSONQueryStats(w io.Writer, ql *queryLogger, topN int) {
	fmt.Fprint(w, `{"status": "ok",`)
	fmt.Fprintf(w, `"top_n": "%d",`, topN)
	fmt.Fprint(w, `"top_by_duration": [`)
	fmt.Fprint(w, formatJSONQueryLog(getQueriesByDuration(ql, topN)))
	fmt.Fprint(w, `],`)
	fmt.Fprint(w, `"top_by_count": [`)
	fmt.Fprint(w, formatJSONQueryLog(getQueriesByRecordCount(ql, topN)))
	fmt.Fprint(w, `],`)
	fmt.Fprint(w, `"top_by_avg_duration": [`)
	fmt.Fprint(w, formatJSONQueryLog(getQueriesByAvgDuration(ql, topN)))
	fmt.Fprint(w, `]`)
	fmt.Fprint(w, `}`)
}

// drop query log items less then given time.
// no need to sort
// its added in chronological order.
func (ql *queryLog) dropOldRecords(t int64) {
	// fast path
	if len(ql.queries) > 0 && ql.queries[len(ql.queries)-1].execTime < t {
		ql.queries = ql.queries[:0]
		return
	}
	shrinkIndex := len(ql.queries)
	for i, v := range ql.queries {
		if t < v.execTime {
			shrinkIndex = i
			break
		}
	}
	if shrinkIndex > 0 {
		ql.queries = ql.queries[shrinkIndex:]
	}
}

// calculates cumulative duration for query.
func (ql *queryLog) duration() time.Duration {
	var cnt time.Duration
	for _, v := range ql.queries {
		cnt += v.duration
	}
	return cnt
}

// must be called with mutex,
// shrinks slice by last added query log records.
func (ql *queryLogger) shrink() {
	logger.Infof("shrink needed")
	sort.Slice(ql.s, func(i, j int) bool {
		return ql.s[i].lastSeen < ql.s[j].lastSeen
	})
	var ts []int64
	for _, v := range ql.s {
		ts = append(ts, v.lastSeen)
	}
	logger.Infof("before shrink ts: %v", ts)
	mustShrinkItems := 10
	// that's odd and seems like bug if len < limit.
	if len(ql.s) < mustShrinkItems {
		return
	}
	ql.s = ql.s[mustShrinkItems:]
}

// drop old keys.
func (ql *queryLogger) dropOldQueryRecords() {
	ql.mu.Lock()
	defer ql.mu.Unlock()
	t := time.Now().Add(-ql.maxQueryLogRecordTime).Unix()
	qlCopy := make([]queryLog, 0, len(ql.s))
	for _, v := range ql.s {
		v.dropOldRecords(t)
		if len(v.queries) == 0 {
			continue
		}
		qlCopy = append(qlCopy, v)
	}
	ql.s = qlCopy
}

func InsertQueryStat(query string, tr int64, execTime time.Time, duration time.Duration) {
	gqlOnce.Do(func() {
		initQL()
	})
	globalQueryLogger.insertQuery(query, tr, execTime, duration)
}
func (ql *queryLogger) insertQuery(query string, tr int64, execTime time.Time, duration time.Duration) {
	ql.mu.Lock()
	defer ql.mu.Unlock()
	if len(ql.s) > ql.limit+10 {
		ql.shrink()
	}
	for i, v := range ql.s {
		if v.query == query && v.timeRange == tr {
			v.lastSeen = execTime.Unix()
			v.queries = append(v.queries, queryLogRecord{execTime: execTime.Unix(), duration: duration})
			ql.s[i] = v
			return
		}
	}
	ql.s = append(ql.s, queryLog{
		queries:   []queryLogRecord{{execTime: execTime.Unix(), duration: duration}},
		lastSeen:  execTime.Unix(),
		query:     query,
		timeRange: tr,
	})

}

func getQueriesByAvgDuration(ql *queryLogger, top int) []queryLog {
	return getQueryStatWithFilter(ql, top, func(i, j int) bool {
		lenI := len(ql.s[i].queries)
		lenJ := len(ql.s[j].queries)
		if lenI == 0 || lenJ == 0 {
			return false
		}
		return int(ql.s[i].duration())/lenI > int(ql.s[j].duration())/lenJ
	})
}

func getQueriesByRecordCount(ql *queryLogger, top int) []queryLog {
	return getQueryStatWithFilter(ql, top, func(i, j int) bool {
		return len(ql.s[i].queries) > len(ql.s[j].queries)
	})
}

func getQueriesByDuration(ql *queryLogger, top int) []queryLog {
	return getQueryStatWithFilter(ql, top, func(i, j int) bool {
		return ql.s[i].duration() > ql.s[j].duration()
	})
}

func getQueryStatWithFilter(ql *queryLogger, top int, filterFunc func(i, j int) bool) []queryLog {
	ql.mu.Lock()
	defer ql.mu.Unlock()
	if top > len(ql.s) {
		top = len(ql.s)
	}
	sort.Slice(ql.s, filterFunc)
	return ql.s[:top]
}
