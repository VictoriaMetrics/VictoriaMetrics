package logsql

import (
	"net/http"

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
func ProcessQueryRequest(w http.ResponseWriter, r *http.Request, stopCh <-chan struct{}, cancel func()) {
	// Extract tenantID
	tenantID, err := logstorage.GetTenantIDFromRequest(r)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	limit, err := httputils.GetInt(r, "limit")
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
	w.Header().Set("Content-Type", "application/stream+json; charset=utf-8")

	sw := getSortWriter()
	sw.Init(w, maxSortBufferSize.IntN(), limit)
	tenantIDs := []logstorage.TenantID{tenantID}
	vlstorage.RunQuery(tenantIDs, q, stopCh, func(columns []logstorage.BlockColumn) {
		if len(columns) == 0 {
			return
		}
		rowsCount := len(columns[0].Values)

		bb := blockResultPool.Get()
		for rowIdx := 0; rowIdx < rowsCount; rowIdx++ {
			WriteJSONRow(bb, columns, rowIdx)
		}

		if !sw.TryWrite(bb.B) {
			cancel()
		}

		blockResultPool.Put(bb)
	})
	sw.FinalFlush()
	putSortWriter(sw)
}

var blockResultPool bytesutil.ByteBufferPool
