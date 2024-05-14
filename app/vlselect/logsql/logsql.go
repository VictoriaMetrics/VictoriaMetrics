package logsql

import (
	"context"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
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

	if limit > 0 {
		q.AddPipeLimit(uint64(limit))
	}

	tenantIDs := []logstorage.TenantID{tenantID}

	bw := getBufferedWriter(w)

	writeBlock := func(_ uint, timestamps []int64, columns []logstorage.BlockColumn) {
		if len(columns) == 0 {
			return
		}

		bb := blockResultPool.Get()
		for i := range timestamps {
			WriteJSONRow(bb, columns, i)
		}
		bw.WriteIgnoreErrors(bb.B)
		blockResultPool.Put(bb)
	}

	err = vlstorage.RunQuery(ctx, tenantIDs, q, writeBlock)

	bw.FlushIgnoreErrors()
	putBufferedWriter(bw)

	if err != nil {
		httpserver.Errorf(w, r, "cannot execute query [%s]: %s", qStr, err)
	}

}

var blockResultPool bytesutil.ByteBufferPool
