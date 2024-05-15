package logsql

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

// ProcessQueryRequest handles /select/logsql/query request.
func ProcessQueryRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// Extract tenantID
	tenantID, err := logstorage.GetTenantIDFromRequest(r)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	// Parse query
	qStr := r.FormValue("query")
	q, err := logstorage.ParseQuery(qStr)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse query [%s]: %s", qStr, err)
		return
	}

	// Parse optional start and end args
	start, okStart, err := getTimeNsec(r, "start")
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	end, okEnd, err := getTimeNsec(r, "end")
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	if okStart || okEnd {
		if !okStart {
			start = math.MinInt64
		}
		if !okEnd {
			end = math.MaxInt64
		}
		q.AddTimeFilter(start, end)
	}

	// Parse limit query arg
	limit, err := httputils.GetInt(r, "limit")
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	if limit > 0 {
		q.AddPipeLimit(uint64(limit))
	}
	q.Optimize()

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

	w.Header().Set("Content-Type", "application/stream+json; charset=utf-8")
	err = vlstorage.RunQuery(ctx, tenantIDs, q, writeBlock)

	bw.FlushIgnoreErrors()
	putBufferedWriter(bw)

	if err != nil {
		httpserver.Errorf(w, r, "cannot execute query [%s]: %s", qStr, err)
	}

}

var blockResultPool bytesutil.ByteBufferPool

func getTimeNsec(r *http.Request, argName string) (int64, bool, error) {
	s := r.FormValue(argName)
	if s == "" {
		return 0, false, nil
	}
	currentTimestamp := float64(time.Now().UnixNano()) / 1e9
	secs, err := promutils.ParseTimeAt(s, currentTimestamp)
	if err != nil {
		return 0, false, fmt.Errorf("cannot parse %s=%s: %w", argName, s, err)
	}
	return int64(secs * 1e9), true, nil
}
