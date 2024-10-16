package logsql

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
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
		return
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

	// Obtain field entries
	fields := r.Form["field"]

	// Obtain limit on the number of top fields entries.
	fieldsLimit, err := httputils.GetInt(r, "fields_limit")
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	if fieldsLimit < 0 {
		fieldsLimit = 0
	}

	// Prepare the query for hits count.
	q.Optimize()
	q.DropAllPipes()
	q.AddCountByTimePipe(int64(step), int64(offset), fields)

	var mLock sync.Mutex
	m := make(map[string]*hitsSeries)
	writeBlock := func(_ uint, timestamps []int64, columns []logstorage.BlockColumn) {
		if len(columns) == 0 || len(columns[0].Values) == 0 {
			return
		}

		timestampValues := columns[0].Values
		hitsValues := columns[len(columns)-1].Values
		columns = columns[1 : len(columns)-1]

		bb := blockResultPool.Get()
		for i := range timestamps {
			timestampStr := strings.Clone(timestampValues[i])
			hitsStr := strings.Clone(hitsValues[i])
			hits, err := strconv.ParseUint(hitsStr, 10, 64)
			if err != nil {
				logger.Panicf("BUG: cannot parse hitsStr=%q: %s", hitsStr, err)
			}

			bb.Reset()
			WriteFieldsForHits(bb, columns, i)

			mLock.Lock()
			hs, ok := m[string(bb.B)]
			if !ok {
				k := string(bb.B)
				hs = &hitsSeries{}
				m[k] = hs
			}
			hs.timestamps = append(hs.timestamps, timestampStr)
			hs.hits = append(hs.hits, hits)
			hs.hitsTotal += hits
			mLock.Unlock()
		}
		blockResultPool.Put(bb)
	}

	// Execute the query
	if err := vlstorage.RunQuery(ctx, tenantIDs, q, writeBlock); err != nil {
		httpserver.Errorf(w, r, "cannot execute query [%s]: %s", q, err)
		return
	}

	m = getTopHitsSeries(m, fieldsLimit)

	// Write response
	w.Header().Set("Content-Type", "application/json")
	WriteHitsSeries(w, m)
}

func getTopHitsSeries(m map[string]*hitsSeries, fieldsLimit int) map[string]*hitsSeries {
	if fieldsLimit <= 0 || fieldsLimit >= len(m) {
		return m
	}

	type fieldsHits struct {
		fieldsStr string
		hs        *hitsSeries
	}
	a := make([]fieldsHits, 0, len(m))
	for fieldsStr, hs := range m {
		a = append(a, fieldsHits{
			fieldsStr: fieldsStr,
			hs:        hs,
		})
	}
	sort.Slice(a, func(i, j int) bool {
		return a[i].hs.hitsTotal > a[j].hs.hitsTotal
	})

	hitsOther := make(map[string]uint64)
	for _, x := range a[fieldsLimit:] {
		for i, timestampStr := range x.hs.timestamps {
			hitsOther[timestampStr] += x.hs.hits[i]
		}
	}
	var hsOther hitsSeries
	for timestampStr, hits := range hitsOther {
		hsOther.timestamps = append(hsOther.timestamps, timestampStr)
		hsOther.hits = append(hsOther.hits, hits)
		hsOther.hitsTotal += hits
	}

	mNew := make(map[string]*hitsSeries, fieldsLimit+1)
	for _, x := range a[:fieldsLimit] {
		mNew[x.fieldsStr] = x.hs
	}
	mNew["{}"] = &hsOther

	return mNew
}

type hitsSeries struct {
	hitsTotal  uint64
	timestamps []string
	hits       []uint64
}

func (hs *hitsSeries) sort() {
	sort.Sort(hs)
}

func (hs *hitsSeries) Len() int {
	return len(hs.timestamps)
}

func (hs *hitsSeries) Swap(i, j int) {
	hs.timestamps[i], hs.timestamps[j] = hs.timestamps[j], hs.timestamps[i]
	hs.hits[i], hs.hits[j] = hs.hits[j], hs.hits[i]
}

func (hs *hitsSeries) Less(i, j int) bool {
	return hs.timestamps[i] < hs.timestamps[j]
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

	// Write results
	w.Header().Set("Content-Type", "application/json")
	WriteValuesWithHitsJSON(w, fieldNames)
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
	fieldName := r.FormValue("field")
	if fieldName == "" {
		httpserver.Errorf(w, r, "missing 'field' query arg")
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

	// Write results
	w.Header().Set("Content-Type", "application/json")
	WriteValuesWithHitsJSON(w, values)
}

// ProcessStreamFieldNamesRequest processes /select/logsql/stream_field_names request.
//
// See https://docs.victoriametrics.com/victorialogs/querying/#querying-stream-field-names
func ProcessStreamFieldNamesRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	q, tenantIDs, err := parseCommonArgs(r)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	// Obtain stream field names for the given query
	q.Optimize()
	names, err := vlstorage.GetStreamFieldNames(ctx, tenantIDs, q)
	if err != nil {
		httpserver.Errorf(w, r, "cannot obtain stream field names: %s", err)
	}

	// Write results
	w.Header().Set("Content-Type", "application/json")
	WriteValuesWithHitsJSON(w, names)
}

// ProcessStreamFieldValuesRequest processes /select/logsql/stream_field_values request.
//
// See https://docs.victoriametrics.com/victorialogs/querying/#querying-stream-field-values
func ProcessStreamFieldValuesRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	q, tenantIDs, err := parseCommonArgs(r)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	// Parse fieldName query arg
	fieldName := r.FormValue("field")
	if fieldName == "" {
		httpserver.Errorf(w, r, "missing 'field' query arg")
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

	// Obtain stream field values for the given query and the given fieldName
	q.Optimize()
	values, err := vlstorage.GetStreamFieldValues(ctx, tenantIDs, q, fieldName, uint64(limit))
	if err != nil {
		httpserver.Errorf(w, r, "cannot obtain stream field values: %s", err)
	}

	// Write results
	w.Header().Set("Content-Type", "application/json")
	WriteValuesWithHitsJSON(w, values)
}

// ProcessStreamIDsRequest processes /select/logsql/stream_ids request.
//
// See https://docs.victoriametrics.com/victorialogs/querying/#querying-stream_ids
func ProcessStreamIDsRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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
	if limit < 0 {
		limit = 0
	}

	// Obtain streamIDs for the given query
	q.Optimize()
	streamIDs, err := vlstorage.GetStreamIDs(ctx, tenantIDs, q, uint64(limit))
	if err != nil {
		httpserver.Errorf(w, r, "cannot obtain stream_ids: %s", err)
	}

	// Write results
	w.Header().Set("Content-Type", "application/json")
	WriteValuesWithHitsJSON(w, streamIDs)
}

// ProcessStreamsRequest processes /select/logsql/streams request.
//
// See https://docs.victoriametrics.com/victorialogs/querying/#querying-streams
func ProcessStreamsRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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
	if limit < 0 {
		limit = 0
	}

	// Obtain streams for the given query
	q.Optimize()
	streams, err := vlstorage.GetStreams(ctx, tenantIDs, q, uint64(limit))
	if err != nil {
		httpserver.Errorf(w, r, "cannot obtain streams: %s", err)
	}

	// Write results
	w.Header().Set("Content-Type", "application/json")
	WriteValuesWithHitsJSON(w, streams)
}

// ProcessLiveTailRequest processes live tailing request to /select/logsq/tail
//
// See https://docs.victoriametrics.com/victorialogs/querying/#live-tailing
func ProcessLiveTailRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	liveTailRequests.Inc()
	defer liveTailRequests.Dec()

	q, tenantIDs, err := parseCommonArgs(r)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	if !q.CanLiveTail() {
		httpserver.Errorf(w, r, "the query [%s] cannot be used in live tailing; "+
			"see https://docs.victoriametrics.com/victorialogs/querying/#live-tailing for details", q)
		return
	}
	q.Optimize()

	refreshIntervalMsecs, err := httputils.GetDuration(r, "refresh_interval", 1000)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	refreshInterval := time.Millisecond * time.Duration(refreshIntervalMsecs)

	ctxWithCancel, cancel := context.WithCancel(ctx)
	tp := newTailProcessor(cancel)

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	end := time.Now().UnixNano()
	doneCh := ctxWithCancel.Done()
	flusher, ok := w.(http.Flusher)
	if !ok {
		logger.Panicf("BUG: it is expected that http.ResponseWriter (%T) supports http.Flusher interface", w)
	}
	qOrig := q
	for {
		start := end - tailOffsetNsecs
		end = time.Now().UnixNano()

		q = qOrig.Clone(end)
		q.AddTimeFilter(start, end)
		// q.Optimize() call is needed for converting '*' into filterNoop.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6785#issuecomment-2358547733
		q.Optimize()
		if err := vlstorage.RunQuery(ctxWithCancel, tenantIDs, q, tp.writeBlock); err != nil {
			httpserver.Errorf(w, r, "cannot execute tail query [%s]: %s", q, err)
			return
		}
		resultRows, err := tp.getTailRows()
		if err != nil {
			httpserver.Errorf(w, r, "cannot get tail results for query [%q]: %s", q, err)
			return
		}
		if len(resultRows) > 0 {
			WriteJSONRows(w, resultRows)
			flusher.Flush()
		}

		select {
		case <-doneCh:
			return
		case <-ticker.C:
		}
	}
}

var liveTailRequests = metrics.NewCounter(`vl_live_tailing_requests`)

const tailOffsetNsecs = 5e9

type logRow struct {
	timestamp int64
	fields    []logstorage.Field
}

func sortLogRows(rows []logRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].timestamp < rows[j].timestamp
	})
}

type tailProcessor struct {
	cancel func()

	mu sync.Mutex

	perStreamRows  map[string][]logRow
	lastTimestamps map[string]int64

	err error
}

func newTailProcessor(cancel func()) *tailProcessor {
	return &tailProcessor{
		cancel: cancel,

		perStreamRows:  make(map[string][]logRow),
		lastTimestamps: make(map[string]int64),
	}
}

func (tp *tailProcessor) writeBlock(_ uint, timestamps []int64, columns []logstorage.BlockColumn) {
	if len(timestamps) == 0 {
		return
	}

	tp.mu.Lock()
	defer tp.mu.Unlock()

	if tp.err != nil {
		return
	}

	// Make sure columns contain _time field, since it is needed for proper tail work.
	hasTime := false
	for _, c := range columns {
		if c.Name == "_time" {
			hasTime = true
			break
		}
	}
	if !hasTime {
		tp.err = fmt.Errorf("missing _time field")
		tp.cancel()
		return
	}

	// Copy block rows to tp.perStreamRows
	for i, timestamp := range timestamps {
		streamID := ""
		fields := make([]logstorage.Field, len(columns))
		for j, c := range columns {
			name := strings.Clone(c.Name)
			value := strings.Clone(c.Values[i])

			fields[j] = logstorage.Field{
				Name:  name,
				Value: value,
			}

			if name == "_stream_id" {
				streamID = value
			}
		}

		tp.perStreamRows[streamID] = append(tp.perStreamRows[streamID], logRow{
			timestamp: timestamp,
			fields:    fields,
		})
	}
}

func (tp *tailProcessor) getTailRows() ([][]logstorage.Field, error) {
	if tp.err != nil {
		return nil, tp.err
	}

	var resultRows []logRow
	for streamID, rows := range tp.perStreamRows {
		sortLogRows(rows)

		lastTimestamp, ok := tp.lastTimestamps[streamID]
		if ok {
			// Skip already written rows
			for len(rows) > 0 && rows[0].timestamp <= lastTimestamp {
				rows = rows[1:]
			}
		}
		if len(rows) > 0 {
			resultRows = append(resultRows, rows...)
			tp.lastTimestamps[streamID] = rows[len(rows)-1].timestamp
		}
	}
	clear(tp.perStreamRows)

	sortLogRows(resultRows)

	tailRows := make([][]logstorage.Field, len(resultRows))
	for i, row := range resultRows {
		tailRows[i] = row.fields
	}

	return tailRows, nil
}

// ProcessStatsQueryRangeRequest handles /select/logsql/stats_query_range request.
//
// See https://docs.victoriametrics.com/victorialogs/querying/#querying-log-range-stats
func ProcessStatsQueryRangeRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	q, tenantIDs, err := parseCommonArgs(r)
	if err != nil {
		httpserver.SendPrometheusError(w, r, err)
		return
	}

	// Obtain step
	stepStr := r.FormValue("step")
	if stepStr == "" {
		stepStr = "1d"
	}
	step, err := promutils.ParseDuration(stepStr)
	if err != nil {
		err = fmt.Errorf("cannot parse 'step' arg: %s", err)
		httpserver.SendPrometheusError(w, r, err)
		return
	}
	if step <= 0 {
		err := fmt.Errorf("'step' must be bigger than zero")
		httpserver.SendPrometheusError(w, r, err)
		return
	}

	// Obtain `by(...)` fields from the last `| stats` pipe in q.
	// Add `_time:step` to the `by(...)` list.
	byFields, err := q.GetStatsByFieldsAddGroupingByTime(int64(step))
	if err != nil {
		httpserver.SendPrometheusError(w, r, err)
		return
	}

	q.Optimize()

	m := make(map[string]*statsSeries)
	var mLock sync.Mutex

	writeBlock := func(_ uint, timestamps []int64, columns []logstorage.BlockColumn) {
		clonedColumnNames := make([]string, len(columns))
		for i, c := range columns {
			clonedColumnNames[i] = strings.Clone(c.Name)
		}
		for i := range timestamps {
			timestamp := q.GetTimestamp()
			labels := make([]logstorage.Field, 0, len(byFields))
			for j, c := range columns {
				if c.Name == "_time" {
					nsec, ok := logstorage.TryParseTimestampRFC3339Nano(c.Values[i])
					if ok {
						timestamp = nsec
						continue
					}
				}
				if slices.Contains(byFields, c.Name) {
					labels = append(labels, logstorage.Field{
						Name:  clonedColumnNames[j],
						Value: strings.Clone(c.Values[i]),
					})
				}
			}

			var dst []byte
			for j, c := range columns {
				if !slices.Contains(byFields, c.Name) {
					name := clonedColumnNames[j]
					dst = dst[:0]
					dst = append(dst, name...)
					dst = logstorage.MarshalFieldsToJSON(dst, labels)
					key := string(dst)
					p := statsPoint{
						Timestamp: timestamp,
						Value:     strings.Clone(c.Values[i]),
					}

					mLock.Lock()
					ss := m[key]
					if ss == nil {
						ss = &statsSeries{
							key:    key,
							Name:   name,
							Labels: labels,
						}
						m[key] = ss
					}
					ss.Points = append(ss.Points, p)
					mLock.Unlock()
				}
			}
		}
	}

	if err := vlstorage.RunQuery(ctx, tenantIDs, q, writeBlock); err != nil {
		err = fmt.Errorf("cannot execute query [%s]: %s", q, err)
		httpserver.SendPrometheusError(w, r, err)
		return
	}

	// Sort the collected stats by time
	rows := make([]*statsSeries, 0, len(m))
	for _, ss := range m {
		points := ss.Points
		sort.Slice(points, func(i, j int) bool {
			return points[i].Timestamp < points[j].Timestamp
		})
		rows = append(rows, ss)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].key < rows[j].key
	})

	w.Header().Set("Content-Type", "application/json")
	WriteStatsQueryRangeResponse(w, rows)
}

type statsSeries struct {
	key string

	Name   string
	Labels []logstorage.Field
	Points []statsPoint
}

type statsPoint struct {
	Timestamp int64
	Value     string
}

// ProcessStatsQueryRequest handles /select/logsql/stats_query request.
//
// See https://docs.victoriametrics.com/victorialogs/querying/#querying-log-stats
func ProcessStatsQueryRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	q, tenantIDs, err := parseCommonArgs(r)
	if err != nil {
		httpserver.SendPrometheusError(w, r, err)
		return
	}

	// Obtain `by(...)` fields from the last `| stats` pipe in q.
	byFields, err := q.GetStatsByFields()
	if err != nil {
		httpserver.SendPrometheusError(w, r, err)
		return
	}

	q.Optimize()

	var rows []statsRow
	var rowsLock sync.Mutex

	timestamp := q.GetTimestamp()
	writeBlock := func(_ uint, timestamps []int64, columns []logstorage.BlockColumn) {
		clonedColumnNames := make([]string, len(columns))
		for i, c := range columns {
			clonedColumnNames[i] = strings.Clone(c.Name)
		}
		for i := range timestamps {
			labels := make([]logstorage.Field, 0, len(byFields))
			for j, c := range columns {
				if slices.Contains(byFields, c.Name) {
					labels = append(labels, logstorage.Field{
						Name:  clonedColumnNames[j],
						Value: strings.Clone(c.Values[i]),
					})
				}
			}

			for j, c := range columns {
				if !slices.Contains(byFields, c.Name) {
					r := statsRow{
						Name:      clonedColumnNames[j],
						Labels:    labels,
						Timestamp: timestamp,
						Value:     strings.Clone(c.Values[i]),
					}

					rowsLock.Lock()
					rows = append(rows, r)
					rowsLock.Unlock()
				}
			}
		}
	}

	if err := vlstorage.RunQuery(ctx, tenantIDs, q, writeBlock); err != nil {
		err = fmt.Errorf("cannot execute query [%s]: %s", q, err)
		httpserver.SendPrometheusError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	WriteStatsQueryResponse(w, rows)
}

type statsRow struct {
	Name      string
	Labels    []logstorage.Field
	Timestamp int64
	Value     string
}

// ProcessQueryRequest handles /select/logsql/query request.
//
// See https://docs.victoriametrics.com/victorialogs/querying/#querying-logs
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

	bw := getBufferedWriter(w)
	defer func() {
		bw.FlushIgnoreErrors()
		putBufferedWriter(bw)
	}()
	w.Header().Set("Content-Type", "application/stream+json")

	if limit > 0 {
		if q.CanReturnLastNResults() {
			rows, err := getLastNQueryResults(ctx, tenantIDs, q, limit)
			if err != nil {
				httpserver.Errorf(w, r, "%s", err)
				return
			}
			bb := blockResultPool.Get()
			b := bb.B
			for i := range rows {
				b = logstorage.MarshalFieldsToJSON(b[:0], rows[i].fields)
				b = append(b, '\n')
				bw.WriteIgnoreErrors(b)
			}
			bb.B = b
			blockResultPool.Put(bb)
			return
		}

		q.AddPipeLimit(uint64(limit))
	}
	q.Optimize()

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

	if err := vlstorage.RunQuery(ctx, tenantIDs, q, writeBlock); err != nil {
		httpserver.Errorf(w, r, "cannot execute query [%s]: %s", q, err)
		return
	}
}

var blockResultPool bytesutil.ByteBufferPool

type row struct {
	timestamp int64
	fields    []logstorage.Field
}

func getLastNQueryResults(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, limit int) ([]row, error) {
	limitUpper := 2 * limit
	q.AddPipeLimit(uint64(limitUpper))
	q.Optimize()

	rows, err := getQueryResultsWithLimit(ctx, tenantIDs, q, limitUpper)
	if err != nil {
		return nil, err
	}
	if len(rows) < limitUpper {
		// Fast path - the requested time range contains up to limitUpper rows.
		rows = getLastNRows(rows, limit)
		return rows, nil
	}

	// Slow path - adjust time range for selecting up to limitUpper rows
	start, end := q.GetFilterTimeRange()
	d := end/2 - start/2
	start += d

	qOrig := q
	for {
		timestamp := qOrig.GetTimestamp()
		q = qOrig.Clone(timestamp)
		q.AddTimeFilter(start, end)
		// q.Optimize() call is needed for converting '*' into filterNoop.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6785#issuecomment-2358547733
		q.Optimize()
		rows, err := getQueryResultsWithLimit(ctx, tenantIDs, q, limitUpper)
		if err != nil {
			return nil, err
		}

		if d == 0 || start >= end {
			// The [start ... end] time range equals one nanosecond.
			// Just return up to limit rows.
			if len(rows) > limit {
				rows = rows[:limit]
			}
			return rows, nil
		}

		dLastBit := d & 1
		d /= 2

		if len(rows) >= limitUpper {
			// The number of found rows on the [start ... end] time range exceeds limitUpper,
			// so reduce the time range to [start+d ... end].
			start += d
			continue
		}
		if len(rows) >= limit {
			// The number of found rows is in the range [limit ... limitUpper).
			// This means that found rows contains the needed limit rows with the biggest timestamps.
			rows = getLastNRows(rows, limit)
			return rows, nil
		}

		// The number of found rows on [start ... end] time range is below the limit.
		// This means the time range doesn't cover the needed logs, so it must be extended.

		if len(rows) == 0 {
			// The [start ... end] time range doesn't contain any rows, so change it to [start-d ... start).
			end = start - 1
			start -= d + dLastBit
			continue
		}

		// The number of found rows on [start ... end] time range is bigger than 0 but smaller than limit.
		// Increase the time range to [start-d ... end].
		start -= d + dLastBit
	}
}

func getLastNRows(rows []row, limit int) []row {
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].timestamp < rows[j].timestamp
	})
	if len(rows) > limit {
		rows = rows[len(rows)-limit:]
	}
	return rows
}

func getQueryResultsWithLimit(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, limit int) ([]row, error) {
	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	var rows []row
	var rowsLock sync.Mutex
	writeBlock := func(_ uint, timestamps []int64, columns []logstorage.BlockColumn) {
		clonedColumnNames := make([]string, len(columns))
		for i, c := range columns {
			clonedColumnNames[i] = strings.Clone(c.Name)
		}

		for i, timestamp := range timestamps {
			fields := make([]logstorage.Field, len(columns))
			for j := range columns {
				f := &fields[j]
				f.Name = clonedColumnNames[j]
				f.Value = strings.Clone(columns[j].Values[i])
			}

			rowsLock.Lock()
			rows = append(rows, row{
				timestamp: timestamp,
				fields:    fields,
			})
			rowsLock.Unlock()
		}

		if len(rows) >= limit {
			cancel()
		}
	}
	if err := vlstorage.RunQuery(ctxWithCancel, tenantIDs, q, writeBlock); err != nil {
		return nil, err
	}

	return rows, nil
}

func parseCommonArgs(r *http.Request) (*logstorage.Query, []logstorage.TenantID, error) {
	// Extract tenantID
	tenantID, err := logstorage.GetTenantIDFromRequest(r)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot obtain tenanID: %w", err)
	}
	tenantIDs := []logstorage.TenantID{tenantID}

	// Parse optional time arg
	timestamp, okTime, err := getTimeNsec(r, "time")
	if err != nil {
		return nil, nil, err
	}
	if !okTime {
		// If time arg is missing, then evaluate query at the current timestamp
		timestamp = time.Now().UnixNano()
	}

	// decrease timestamp by one nanosecond in order to avoid capturing logs belonging
	// to the first nanosecond at the next period of time (month, week, day, hour, etc.)
	timestamp--

	// Parse query
	qStr := r.FormValue("query")
	q, err := logstorage.ParseQueryAtTimestamp(qStr, timestamp)
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
	currentTimestamp := time.Now().UnixNano()
	nsecs, err := promutils.ParseTimeAt(s, currentTimestamp)
	if err != nil {
		return 0, false, fmt.Errorf("cannot parse %s=%s: %w", argName, s, err)
	}
	return nsecs, true, nil
}
