package logsql

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

// ProcessFacetsRequest handles /select/logsql/facets request.
//
// See https://docs.victoriametrics.com/victorialogs/querying/#querying-facets
func ProcessFacetsRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	q, tenantIDs, err := parseCommonArgs(r)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	limit, err := httputil.GetInt(r, "limit")
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	maxValuesPerField, err := httputil.GetInt(r, "max_values_per_field")
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	maxValueLen, err := httputil.GetInt(r, "max_value_len")
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	keepConstFields := httputil.GetBool(r, "keep_const_fields")

	// Pipes must be dropped, since it is expected facets are obtained
	// from the real logs stored in the database.
	q.DropAllPipes()

	q.AddFacetsPipe(limit, maxValuesPerField, maxValueLen, keepConstFields)

	var mLock sync.Mutex
	m := make(map[string][]facetEntry)
	writeBlock := func(_ uint, db *logstorage.DataBlock) {
		rowsCount := db.RowsCount()
		if rowsCount == 0 {
			return
		}

		columns := db.Columns
		if len(columns) != 3 {
			logger.Panicf("BUG: expecting 3 columns; got %d columns", len(columns))
		}

		fieldNames := columns[0].Values
		fieldValues := columns[1].Values
		hits := columns[2].Values

		bb := blockResultPool.Get()
		for i := range fieldNames {
			fieldName := strings.Clone(fieldNames[i])
			fieldValue := strings.Clone(fieldValues[i])
			hitsStr := strings.Clone(hits[i])

			mLock.Lock()
			m[fieldName] = append(m[fieldName], facetEntry{
				value: fieldValue,
				hits:  hitsStr,
			})
			mLock.Unlock()
		}
		blockResultPool.Put(bb)
	}

	// Execute the query
	if err := vlstorage.RunQuery(ctx, tenantIDs, q, writeBlock); err != nil {
		httpserver.Errorf(w, r, "cannot execute query [%s]: %s", q, err)
		return
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	WriteFacetsResponse(w, m)
}

type facetEntry struct {
	value string
	hits  string
}

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
	step, err := timeutil.ParseDuration(stepStr)
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
	offset, err := timeutil.ParseDuration(offsetStr)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse 'offset' arg: %s", err)
		return
	}

	// Obtain field entries
	fields := r.Form["field"]

	// Obtain limit on the number of top fields entries.
	fieldsLimit, err := httputil.GetInt(r, "fields_limit")
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	if fieldsLimit < 0 {
		fieldsLimit = 0
	}

	// Pipes must be dropped, since it is expected hits are obtained
	// from the real logs stored in the database.
	q.DropAllPipes()

	q.AddCountByTimePipe(int64(step), int64(offset), fields)

	var mLock sync.Mutex
	m := make(map[string]*hitsSeries)
	writeBlock := func(_ uint, db *logstorage.DataBlock) {
		rowsCount := db.RowsCount()
		if rowsCount == 0 {
			return
		}

		columns := db.Columns
		timestampValues := columns[0].Values
		hitsValues := columns[len(columns)-1].Values
		columns = columns[1 : len(columns)-1]

		bb := blockResultPool.Get()
		for i := 0; i < rowsCount; i++ {
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

var blockResultPool bytesutil.ByteBufferPool

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

	// Pipes must be dropped, since it is expected field names are obtained
	// from the real logs stored in the database.
	q.DropAllPipes()

	// Obtain field names for the given query
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
	limit, err := httputil.GetInt(r, "limit")
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	if limit < 0 {
		limit = 0
	}

	// Pipes must be dropped, since it is expected field values are obtained
	// from the real logs stored in the database.
	q.DropAllPipes()

	// Obtain unique values for the given field
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

	// Pipes must be dropped, since it is expected stream field names are obtained
	// from the real logs stored in the database.
	q.DropAllPipes()

	// Obtain stream field names for the given query
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
	limit, err := httputil.GetInt(r, "limit")
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	if limit < 0 {
		limit = 0
	}

	// Pipes must be dropped, since it is expected stream field values are obtained
	// from the real logs stored in the database.
	q.DropAllPipes()

	// Obtain stream field values for the given query and the given fieldName
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
	limit, err := httputil.GetInt(r, "limit")
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	if limit < 0 {
		limit = 0
	}

	// Pipes must be dropped, since it is expected stream ids are obtained
	// from the real logs stored in the database.
	q.DropAllPipes()

	// Obtain streamIDs for the given query
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
	limit, err := httputil.GetInt(r, "limit")
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	if limit < 0 {
		limit = 0
	}

	// Pipes must be dropped, since it is expected stream are obtained
	// from the real logs stored in the database.
	q.DropAllPipes()

	// Obtain streams for the given query
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

	refreshIntervalMsecs, err := httputil.GetDuration(r, "refresh_interval", 1000)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	refreshInterval := time.Millisecond * time.Duration(refreshIntervalMsecs)

	startOffsetMsecs, err := httputil.GetDuration(r, "start_offset", 5*1000)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	startOffset := startOffsetMsecs * 1e6

	offsetMsecs, err := httputil.GetDuration(r, "offset", 1000)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	offset := offsetMsecs * 1e6

	ctxWithCancel, cancel := context.WithCancel(ctx)
	tp := newTailProcessor(cancel)

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	end := time.Now().UnixNano() - offset
	start := end - startOffset
	doneCh := ctxWithCancel.Done()
	flusher, ok := w.(http.Flusher)
	if !ok {
		logger.Panicf("BUG: it is expected that http.ResponseWriter (%T) supports http.Flusher interface", w)
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	flusher.Flush()

	qOrig := q
	for {
		q = qOrig.CloneWithTimeFilter(end, start, end)
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
			start = end - tailOffsetNsecs
			end = time.Now().UnixNano() - offset
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

func (tp *tailProcessor) writeBlock(_ uint, db *logstorage.DataBlock) {
	if db.RowsCount() == 0 {
		return
	}

	tp.mu.Lock()
	defer tp.mu.Unlock()

	if tp.err != nil {
		return
	}

	// Make sure columns contain _time field, since it is needed for proper tail work.
	timestamps, ok := db.GetTimestamps(nil)
	if !ok {
		tp.err = fmt.Errorf("missing _time field")
		tp.cancel()
		return
	}

	// Copy block rows to tp.perStreamRows
	for i, timestamp := range timestamps {
		streamID := ""
		fields := make([]logstorage.Field, len(db.Columns))
		for j, c := range db.Columns {
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
	step, err := timeutil.ParseDuration(stepStr)
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

	m := make(map[string]*statsSeries)
	var mLock sync.Mutex

	writeBlock := func(_ uint, db *logstorage.DataBlock) {
		rowsCount := db.RowsCount()

		columns := db.Columns
		clonedColumnNames := make([]string, len(columns))
		for i, c := range columns {
			clonedColumnNames[i] = strings.Clone(c.Name)
		}
		for i := 0; i < rowsCount; i++ {
			// Do not move q.GetTimestamp() outside writeBlock, since ts
			// must be initialized to query timestamp for every processed log row.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8312
			ts := q.GetTimestamp()
			labels := make([]logstorage.Field, 0, len(byFields))
			for j, c := range columns {
				if c.Name == "_time" {
					nsec, ok := logstorage.TryParseTimestampRFC3339Nano(c.Values[i])
					if ok {
						ts = nsec
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
						Timestamp: ts,
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

	var rows []statsRow
	var rowsLock sync.Mutex

	timestamp := q.GetTimestamp()
	writeBlock := func(_ uint, db *logstorage.DataBlock) {
		rowsCount := db.RowsCount()
		columns := db.Columns
		clonedColumnNames := make([]string, len(columns))
		for i, c := range columns {
			clonedColumnNames[i] = strings.Clone(c.Name)
		}
		for i := 0; i < rowsCount; i++ {
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
	limit, err := httputil.GetInt(r, "limit")
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	sw := &syncWriter{
		w: w,
	}

	var bwShards atomicutil.Slice[bufferedWriter]
	bwShards.Init = func(shard *bufferedWriter) {
		shard.sw = sw
	}
	defer func() {
		shards := bwShards.All()
		for _, shard := range shards {
			shard.FlushIgnoreErrors()
		}
	}()

	w.Header().Set("Content-Type", "application/stream+json")

	if limit > 0 {
		if q.CanReturnLastNResults() {
			rows, err := getLastNQueryResults(ctx, tenantIDs, q, limit)
			if err != nil {
				httpserver.Errorf(w, r, "%s", err)
				return
			}
			bw := bwShards.Get(0)
			for i := range rows {
				bw.buf = logstorage.MarshalFieldsToJSON(bw.buf, rows[i].fields)
				bw.buf = append(bw.buf, '\n')
				if len(bw.buf) > 16*1024 {
					bw.FlushIgnoreErrors()
				}
			}
			return
		}

		q.AddPipeLimit(uint64(limit))
	}

	writeBlock := func(workerID uint, db *logstorage.DataBlock) {
		rowsCount := db.RowsCount()
		if rowsCount == 0 {
			return
		}
		columns := db.Columns

		bw := bwShards.Get(workerID)
		for i := 0; i < rowsCount; i++ {
			WriteJSONRow(bw, columns, i)
			if len(bw.buf) > 16*1024 {
				bw.FlushIgnoreErrors()
			}
		}
	}

	if err := vlstorage.RunQuery(ctx, tenantIDs, q, writeBlock); err != nil {
		httpserver.Errorf(w, r, "cannot execute query [%s]: %s", q, err)
		return
	}
}

type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (sw *syncWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	n, err := sw.w.Write(p)
	sw.mu.Unlock()
	return n, err
}

type bufferedWriter struct {
	buf []byte
	sw  *syncWriter
}

func (bw *bufferedWriter) Write(p []byte) (int, error) {
	bw.buf = append(bw.buf, p...)

	// Do not send bw.buf to bw.sw here, since the data at bw.buf may be incomplete (it must end with '\n')

	return len(p), nil
}

func (bw *bufferedWriter) FlushIgnoreErrors() {
	_, _ = bw.sw.Write(bw.buf)
	bw.buf = bw.buf[:0]
}

func getLastNQueryResults(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, limit int) ([]logRow, error) {
	limitUpper := 2 * limit
	q.AddPipeLimit(uint64(limitUpper))

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
		q = qOrig.CloneWithTimeFilter(timestamp, start, end)
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

func getLastNRows(rows []logRow, limit int) []logRow {
	sortLogRows(rows)
	if len(rows) > limit {
		rows = rows[len(rows)-limit:]
	}
	return rows
}

func getQueryResultsWithLimit(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, limit int) ([]logRow, error) {
	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	var missingTimeColumn atomic.Bool
	var rows []logRow
	var rowsLock sync.Mutex
	writeBlock := func(_ uint, db *logstorage.DataBlock) {
		if missingTimeColumn.Load() {
			return
		}

		columns := db.Columns
		clonedColumnNames := make([]string, len(columns))
		for i, c := range columns {
			clonedColumnNames[i] = strings.Clone(c.Name)
		}

		timestamps, ok := db.GetTimestamps(nil)
		if !ok {
			missingTimeColumn.Store(true)
			cancel()
			return
		}

		for i, timestamp := range timestamps {
			fields := make([]logstorage.Field, len(columns))
			for j := range columns {
				f := &fields[j]
				f.Name = clonedColumnNames[j]
				f.Value = strings.Clone(columns[j].Values[i])
			}

			rowsLock.Lock()
			rows = append(rows, logRow{
				timestamp: timestamp,
				fields:    fields,
			})
			rowsLock.Unlock()
		}

		if len(rows) >= limit {
			cancel()
		}
	}
	err := vlstorage.RunQuery(ctxWithCancel, tenantIDs, q, writeBlock)

	if missingTimeColumn.Load() {
		return nil, fmt.Errorf("missing _time column in the result for the query [%s]", q)
	}

	return rows, err
}

func parseCommonArgs(r *http.Request) (*logstorage.Query, []logstorage.TenantID, error) {
	// Extract tenantID
	tenantID, err := logstorage.GetTenantIDFromRequest(r)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot obtain tenanID: %w", err)
	}
	tenantIDs := []logstorage.TenantID{tenantID}

	// Parse optional start and end args
	start, okStart, err := getTimeNsec(r, "start")
	if err != nil {
		return nil, nil, err
	}
	end, okEnd, err := getTimeNsec(r, "end")
	if err != nil {
		return nil, nil, err
	}

	// Parse optional time arg
	timestamp, okTime, err := getTimeNsec(r, "time")
	if err != nil {
		return nil, nil, err
	}
	if !okTime {
		// If time arg is missing, then evaluate query either at the end timestamp (if it is set)
		// or at the current timestamp (if end query arg isn't set)
		if okEnd {
			timestamp = end
		} else {
			timestamp = time.Now().UnixNano()
		}
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

	if okStart || okEnd {
		// Add _time:[start, end] filter if start or end args were set.
		if !okStart {
			start = math.MinInt64
		}
		if !okEnd {
			end = math.MaxInt64
		}
		q.AddTimeFilter(start, end)
	}

	// Parse optional extra_filters
	for _, extraFiltersStr := range r.Form["extra_filters"] {
		extraFilters, err := parseExtraFilters(extraFiltersStr)
		if err != nil {
			return nil, nil, err
		}
		q.AddExtraFilters(extraFilters)
	}

	// Parse optional extra_stream_filters
	for _, extraStreamFiltersStr := range r.Form["extra_stream_filters"] {
		extraStreamFilters, err := parseExtraStreamFilters(extraStreamFiltersStr)
		if err != nil {
			return nil, nil, err
		}
		q.AddExtraFilters(extraStreamFilters)
	}

	return q, tenantIDs, nil
}

func getTimeNsec(r *http.Request, argName string) (int64, bool, error) {
	s := r.FormValue(argName)
	if s == "" {
		return 0, false, nil
	}
	currentTimestamp := time.Now().UnixNano()
	nsecs, err := timeutil.ParseTimeAt(s, currentTimestamp)
	if err != nil {
		return 0, false, fmt.Errorf("cannot parse %s=%s: %w", argName, s, err)
	}
	return nsecs, true, nil
}

func parseExtraFilters(s string) (*logstorage.Filter, error) {
	if s == "" {
		return nil, nil
	}
	if !strings.HasPrefix(s, `{"`) {
		return logstorage.ParseFilter(s)
	}

	// Extra filters in the form {"field":"value",...}.
	kvs, err := parseExtraFiltersJSON(s)
	if err != nil {
		return nil, err
	}

	filters := make([]string, len(kvs))
	for i, kv := range kvs {
		if len(kv.values) == 1 {
			filters[i] = fmt.Sprintf("%q:=%q", kv.key, kv.values[0])
		} else {
			orValues := make([]string, len(kv.values))
			for j, v := range kv.values {
				orValues[j] = fmt.Sprintf("%q", v)
			}
			filters[i] = fmt.Sprintf("%q:in(%s)", kv.key, strings.Join(orValues, ","))
		}
	}
	s = strings.Join(filters, " ")
	return logstorage.ParseFilter(s)
}

func parseExtraStreamFilters(s string) (*logstorage.Filter, error) {
	if s == "" {
		return nil, nil
	}
	if !strings.HasPrefix(s, `{"`) {
		return logstorage.ParseFilter(s)
	}

	// Extra stream filters in the form {"field":"value",...}.
	kvs, err := parseExtraFiltersJSON(s)
	if err != nil {
		return nil, err
	}

	filters := make([]string, len(kvs))
	for i, kv := range kvs {
		if len(kv.values) == 1 {
			filters[i] = fmt.Sprintf("%q=%q", kv.key, kv.values[0])
		} else {
			orValues := make([]string, len(kv.values))
			for j, v := range kv.values {
				orValues[j] = regexp.QuoteMeta(v)
			}
			filters[i] = fmt.Sprintf("%q=~%q", kv.key, strings.Join(orValues, "|"))
		}
	}
	s = "{" + strings.Join(filters, ",") + "}"
	return logstorage.ParseFilter(s)
}

type extraFilter struct {
	key    string
	values []string
}

func parseExtraFiltersJSON(s string) ([]extraFilter, error) {
	v, err := fastjson.Parse(s)
	if err != nil {
		return nil, err
	}
	o := v.GetObject()

	var errOuter error
	var filters []extraFilter
	o.Visit(func(k []byte, v *fastjson.Value) {
		if errOuter != nil {
			return
		}
		switch v.Type() {
		case fastjson.TypeString:
			filters = append(filters, extraFilter{
				key:    string(k),
				values: []string{string(v.GetStringBytes())},
			})
		case fastjson.TypeArray:
			a := v.GetArray()
			if len(a) == 0 {
				return
			}
			orValues := make([]string, len(a))
			for i, av := range a {
				ov, err := av.StringBytes()
				if err != nil {
					errOuter = fmt.Errorf("cannot obtain string item at the array for key %q; item: %s", k, av)
					return
				}
				orValues[i] = string(ov)
			}
			filters = append(filters, extraFilter{
				key:    string(k),
				values: orValues,
			})
		default:
			errOuter = fmt.Errorf("unexpected type of value for key %q: %s; value: %s", k, v.Type(), v)
		}
	})
	if errOuter != nil {
		return nil, errOuter
	}
	return filters, nil
}
