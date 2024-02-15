package logsql

import (
	"net/http"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

var (
	maxSortBufferSize = flagutil.NewBytes("select.maxSortBufferSize", 1024*1024, "Query results from /select/logsql/query are automatically sorted by _time "+
		"if their summary size doesn't exceed this value; otherwise, query results are streamed in the response without sorting; "+
		"too big value for this flag may result in high memory usage since the sorting is performed in memory")
)

// ProcessQueryRequest handles /select/logsql/query request
func ProcessQueryRequest(w http.ResponseWriter, r *http.Request, stopCh <-chan struct{}) {
	// Extract tenantID
	tenantID, err := logstorage.GetTenantIDFromRequest(r)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	qStr := r.FormValue("query")
	q, err := logstorage.ParseQuery(qStr)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse query [%s]: %s", qStr, err)
		return
	}
	limit, err := httputils.GetInt(r, "limit")
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse limit from the request: %s", err)
		return
	}
	w.Header().Set("Content-Type", "application/stream+json; charset=utf-8")
	sw := getSortWriter()
	sw.Init(w, maxSortBufferSize.IntN())
	tenantIDs := []logstorage.TenantID{tenantID}

	var mx sync.Mutex
	vlstorage.RunQuery(tenantIDs, q, stopCh, func(columns []logstorage.BlockColumn) bool {
		if len(columns) == 0 {
			return true
		}
		rowsCount := len(columns[0].Values)
		mx.Lock()
		if rowsCount > limit {
			rowsCount = limit
		}
		limit = limit - rowsCount
		mx.Unlock()
		bb := blockResultPool.Get()
		for rowIdx := 0; rowIdx < rowsCount; rowIdx++ {
			WriteJSONRow(bb, columns, rowIdx)
		}
		sw.MustWrite(bb.B)
		blockResultPool.Put(bb)

		return limit == 0
	})
	sw.FinalFlush()
	putSortWriter(sw)
}

var blockResultPool bytesutil.ByteBufferPool
