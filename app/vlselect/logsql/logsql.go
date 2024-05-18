package logsql

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

// ProcessHitsRequest handles /select/logsql/hits request.
//
// See https://docs.victoriametrics.com/victorialogs/querying/#querying-hits-stats
func ProcessHitsRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	q, tenantIDs, err := parseCommonArgs(r)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	// Obtain step
	stepStr := r.FormValue("step")
	if stepStr == "" {
		stepStr = "1d"
	}
	step, err := promutils.ParseDuration(stepStr)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse 'step' arg: %s", err)
		return
	}
	if step <= 0 {
		httpserver.Errorf(w, r, "'step' must be bigger than zero")
	}

	// Obtain offset
	offsetStr := r.FormValue("offset")
	if offsetStr == "" {
		offsetStr = "0s"
	}
	offset, err := promutils.ParseDuration(offsetStr)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse 'offset' arg: %s", err)
		return
	}

	q.AddCountByTimePipe(int64(step), int64(offset))
	q.Optimize()

	var wLock sync.Mutex
	isFirstWrite := true
	writeBlock := func(_ uint, timestamps []int64, columns []logstorage.BlockColumn) {
		if len(columns) == 0 || len(columns[0].Values) == 0 {
			return
		}

		bb := blockResultPool.Get()
		for i := range timestamps {
			bb.B = append(bb.B, ',')
			WriteJSONRow(bb, columns, i)
			// Remove newline at the end
			bb.B = bb.B[:len(bb.B)-1]
		}
		wLock.Lock()
		buf := bb.B
		if isFirstWrite {
			buf = buf[1:]
			isFirstWrite = false
		}
		_, _ = w.Write(buf)
		wLock.Unlock()
		blockResultPool.Put(bb)
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"rows":[`)
	err = vlstorage.RunQuery(ctx, tenantIDs, q, writeBlock)
	fmt.Fprintf(w, `]}`)

	if err != nil {
		httpserver.Errorf(w, r, "cannot execute query [%s]: %s", q, err)
	}
}

// ProcessFieldNamesRequest handles /select/logsql/field_names request.
//
// See https://docs.victoriametrics.com/victorialogs/querying/#querying-field-names
func ProcessFieldNamesRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	q, tenantIDs, err := parseCommonArgs(r)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	// Obtain field names for the given query
	q.Optimize()
	fieldNames, err := vlstorage.GetFieldNames(ctx, tenantIDs, q)
	if err != nil {
		httpserver.Errorf(w, r, "cannot obtain field names: %s", err)
		return
	}

	slices.Sort(fieldNames)

	// Write results
	w.Header().Set("Content-Type", "application/json")
	WriteFieldNamesResponse(w, fieldNames)
}

// ProcessFieldValuesRequest handles /select/logsql/field_values request.
//
// See https://docs.victoriametrics.com/victorialogs/querying/#querying-field-values
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
	values, err := vlstorage.GetFieldValues(ctx, tenantIDs, q, fieldName, uint64(limit))
	if err != nil {
		httpserver.Errorf(w, r, "cannot obtain values for field %q: %s", fieldName, err)
		return
	}

	if limit == 0 || len(values) < limit {
		// Sort values only if their number is below the limit.
		// Otherwise there is little sense in sorting, since the query may return
		// different subset of values on every execution.
		slices.Sort(values)
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
		if len(columns) == 0 || len(columns[0].Values) == 0 {
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
