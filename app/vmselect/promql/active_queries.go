package promql

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/stringsutil"
)

// ActiveQueriesHandler returns response to /api/v1/status/active_queries
//
// It writes a JSON with active queries to w.
//
// If at is nil, then all the active queries across all the tenants are written.
func ActiveQueriesHandler(at *auth.Token, w http.ResponseWriter, _ *http.Request) {
	aqes := activeQueriesV.GetAll()

	// Filter out queries, which do not belong to at.
	// if at is nil, then all the queries are returned for multi-tenant request
	dst := aqes[:0]
	for _, aqe := range aqes {
		if at == nil || (aqe.accountID == at.AccountID && aqe.projectID == at.ProjectID) {
			dst = append(dst, aqe)
		}
	}
	aqes = dst
	writeActiveQueries(w, aqes)
}

func writeActiveQueries(w http.ResponseWriter, aqes []activeQueryEntry) {
	w.Header().Set("Content-Type", "application/json")

	sort.Slice(aqes, func(i, j int) bool {
		return aqes[i].startTime.Sub(aqes[j].startTime) < 0
	})
	now := time.Now()
	fmt.Fprintf(w, `{"status":"ok","data":[`)
	for i, aqe := range aqes {
		d := now.Sub(aqe.startTime)
		fmt.Fprintf(w, `{"duration":"%.3fs","id":"%016X","remote_addr":%s,"account_id":"%d","project_id":"%d","query":%s,"start":%d,"end":%d,"step":%d,"is_multitenant":%v}`,
			d.Seconds(), aqe.qid, aqe.quotedRemoteAddr, aqe.accountID, aqe.projectID, stringsutil.JSONString(aqe.q), aqe.start, aqe.end, aqe.step, aqe.isMultitenant)
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
	accountID        uint32
	projectID        uint32
	start            int64
	end              int64
	step             int64
	qid              uint64
	quotedRemoteAddr string
	q                string
	startTime        time.Time
	isMultitenant    bool
}

func newActiveQueries() *activeQueries {
	return &activeQueries{
		m: make(map[uint64]activeQueryEntry),
	}
}

func (aq *activeQueries) Add(ec *EvalConfig, q string) uint64 {
	var aqe activeQueryEntry
	if ec.IsMultiTenant {
		aqe.isMultitenant = true
	} else {
		aqe.accountID = ec.AuthTokens[0].AccountID
		aqe.projectID = ec.AuthTokens[0].ProjectID
	}
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
