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

// ProcessFieldValuesRequest handles /select/logsql/field_values request.
func ProcessFieldValuesRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	q, tenantIDs, err := parseCommonArgs(r)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	// Parse fieldName query arg
	fieldName := r.FormValue("field_name")
	if fieldName == "" {
		httpserver.Errorf(w, r, "missing 'field_name' query arg")
		return
	}

	// Parse limit query arg
	limit, err := httputils.GetInt(r, "limit")
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	if limit < 0 {
		limit = 0
	}

	// Obtain unique values for the given field
	q.Optimize()
	values, err := vlstorage.GetUniqueFieldValues(ctx, tenantIDs, q, fieldName, uint64(limit))
	if err != nil {
		httpserver.Errorf(w, r, "cannot obtain values for field %q: %s", fieldName, err)
		return
	}

	// Write results
	w.Header().Set("Content-Type", "application/json")
	WriteFieldValuesResponse(w, values)
}

// ProcessQueryRequest handles /select/logsql/query request.
//
// See https://docs.victoriametrics.com/victorialogs/querying/#http-api
func ProcessQueryRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	q, tenantIDs, err := parseCommonArgs(r)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
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

	w.Header().Set("Content-Type", "application/stream+json")
	q.Optimize()
	err = vlstorage.RunQuery(ctx, tenantIDs, q, writeBlock)

	bw.FlushIgnoreErrors()
	putBufferedWriter(bw)

	if err != nil {
		httpserver.Errorf(w, r, "cannot execute query [%s]: %s", q, err)
	}
}

var blockResultPool bytesutil.ByteBufferPool

func parseCommonArgs(r *http.Request) (*logstorage.Query, []logstorage.TenantID, error) {
	// Extract tenantID
	tenantID, err := logstorage.GetTenantIDFromRequest(r)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot obtain tenanID: %w", err)
	}
	tenantIDs := []logstorage.TenantID{tenantID}

	// Parse query
	qStr := r.FormValue("query")
	q, err := logstorage.ParseQuery(qStr)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}

	// Parse optional start and end args
	start, okStart, err := getTimeNsec(r, "start")
	if err != nil {
		return nil, nil, err
	}
	end, okEnd, err := getTimeNsec(r, "end")
	if err != nil {
		return nil, nil, err
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

	return q, tenantIDs, nil
}

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
