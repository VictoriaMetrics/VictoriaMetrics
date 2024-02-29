package promql

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ActiveQueriesHandler returns response to /api/v1/status/active_queries
//
// It writes a JSON with active queries to w.
func ActiveQueriesHandler(w http.ResponseWriter, _ *http.Request) {
	aqes := activeQueriesV.GetAll()

	w.Header().Set("Content-Type", "application/json")
	sort.Slice(aqes, func(i, j int) bool {
		return aqes[i].startTime.Sub(aqes[j].startTime) < 0
	})
	now := time.Now()
	fmt.Fprintf(w, `{"status":"ok","data":[`)
	for i, aqe := range aqes {
		d := now.Sub(aqe.startTime)
		fmt.Fprintf(w, `{"duration":"%.3fs","id":"%016X","remote_addr":%s,"query":%q,"start":%d,"end":%d,"step":%d}`,
			d.Seconds(), aqe.qid, aqe.quotedRemoteAddr, aqe.q, aqe.start, aqe.end, aqe.step)
		if i+1 < len(aqes) {
			fmt.Fprintf(w, `,`)
		}
	}
	fmt.Fprintf(w, `]}`)
}

var activeQueriesV = newActiveQueries()

type activeQueries struct {
	mu sync.Mutex
	m  map[uint64]activeQueryEntry
}

type activeQueryEntry struct {
	start            int64
	end              int64
	step             int64
	qid              uint64
	quotedRemoteAddr string
	q                string
	startTime        time.Time
}

func newActiveQueries() *activeQueries {
	return &activeQueries{
		m: make(map[uint64]activeQueryEntry),
	}
}

func (aq *activeQueries) Add(ec *EvalConfig, q string) uint64 {
	var aqe activeQueryEntry
	aqe.start = ec.Start
	aqe.end = ec.End
	aqe.step = ec.Step
	aqe.qid = nextActiveQueryID.Add(1)
	aqe.quotedRemoteAddr = ec.QuotedRemoteAddr
	aqe.q = q
	aqe.startTime = time.Now()

	aq.mu.Lock()
	aq.m[aqe.qid] = aqe
	aq.mu.Unlock()
	return aqe.qid
}

func (aq *activeQueries) Remove(qid uint64) {
	aq.mu.Lock()
	delete(aq.m, qid)
	aq.mu.Unlock()
}

func (aq *activeQueries) GetAll() []activeQueryEntry {
	aq.mu.Lock()
	aqes := make([]activeQueryEntry, 0, len(aq.m))
	for _, aqe := range aq.m {
		aqes = append(aqes, aqe)
	}
	aq.mu.Unlock()
	return aqes
}

var nextActiveQueryID = func() *atomic.Uint64 {
	var x atomic.Uint64
	x.Store(uint64(time.Now().UnixNano()))
	return &x
}()
