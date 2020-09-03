package promql

import (
	"fmt"
	"io"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// WriteActiveQueries writes active queries to w.
//
// The written active queries are sorted in descending order of their exeuction duration.
func WriteActiveQueries(w io.Writer) {
	aqes := activeQueriesV.GetAll()
	sort.Slice(aqes, func(i, j int) bool {
		return aqes[i].startTime.Sub(aqes[j].startTime) < 0
	})
	now := time.Now()
	for _, aqe := range aqes {
		d := now.Sub(aqe.startTime)
		fmt.Fprintf(w, "\tduration: %.3fs, id=%016X, remote_addr=%s, accountID=%d, projectID=%d, query=%q, start=%d, end=%d, step=%d\n",
			d.Seconds(), aqe.qid, aqe.quotedRemoteAddr, aqe.accountID, aqe.projectID, aqe.q, aqe.start, aqe.end, aqe.step)
	}
}

var activeQueriesV = newActiveQueries()

type activeQueries struct {
	mu sync.Mutex
	m  map[uint64]activeQueryEntry
}

type activeQueryEntry struct {
	accountID        uint32
	projectID        uint32
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
	aqe.accountID = ec.AuthToken.AccountID
	aqe.projectID = ec.AuthToken.ProjectID
	aqe.start = ec.Start
	aqe.end = ec.End
	aqe.step = ec.Step
	aqe.qid = atomic.AddUint64(&nextActiveQueryID, 1)
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

var nextActiveQueryID = uint64(time.Now().UnixNano())
