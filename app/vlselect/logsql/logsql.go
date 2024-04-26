package logsql

import (
	"context"
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

// ProcessQueryRequest handles /select/logsql/query request.
func ProcessQueryRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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

	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	writeBlock := func(_ uint, timestamps []int64, columns []logstorage.BlockColumn) {
		if len(columns) == 0 {
			return
		}

		bb := blockResultPool.Get()
		for i := range timestamps {
			WriteJSONRow(bb, columns, i)
		}

		if !sw.TryWrite(bb.B) {
			cancel()
		}

		blockResultPool.Put(bb)
	}

	vlstorage.RunQuery(ctxWithCancel, tenantIDs, q, writeBlock)

	sw.FinalFlush()
	putSortWriter(sw)
}

var blockResultPool bytesutil.ByteBufferPool
