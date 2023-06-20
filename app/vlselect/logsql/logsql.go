package logsql

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bufferedwriter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
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
	w.Header().Set("Content-Type", "application/stream+json; charset=utf-8")

	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)

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
		// Do not check for error here, since the only valid error is when the client
		// closes the connection during Write() call. There is no need in logging this error,
		// since it may be too verbose and it doesn't give any actionable info.
		_, _ = bw.Write(bb.B)
		blockResultPool.Put(bb)
	})
	_ = bw.Flush()
}

var blockResultPool bytesutil.ByteBufferPool
