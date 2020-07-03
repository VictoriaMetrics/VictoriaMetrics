package prometheus

import (
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
	"github.com/VictoriaMetrics/metricsql"
	"github.com/valyala/fastjson/fastfloat"
	"github.com/valyala/quicktemplate"
)

var (
	latencyOffset = flag.Duration("search.latencyOffset", time.Second*30, "The time when data points become visible in query results after the colection. "+
		"Too small value can result in incomplete last points for query results")
	maxExportDuration = flag.Duration("search.maxExportDuration", time.Hour*24*30, "The maximum duration for /api/v1/export call")
	maxQueryDuration  = flag.Duration("search.maxQueryDuration", time.Second*30, "The maximum duration for search query execution")
	maxQueryLen       = flag.Int("search.maxQueryLen", 16*1024, "The maximum search query length in bytes")
	maxLookback       = flag.Duration("search.maxLookback", 0, "Synonim to -search.lookback-delta from Prometheus. "+
		"The value is dynamically detected from interval between time series datapoints if not set. It can be overridden on per-query basis via max_lookback arg. "+
		"See also '-search.maxStalenessInterval' flag, which has the same meaining due to historical reasons")
	maxStalenessInterval = flag.Duration("search.maxStalenessInterval", 0, "The maximum interval for staleness calculations. "+
		"By default it is automatically calculated from the median interval between samples. This flag could be useful for tuning "+
		"Prometheus data model closer to Influx-style data model. See https://prometheus.io/docs/prometheus/latest/querying/basics/#staleness for details. "+
		"See also '-search.maxLookback' flag, which has the same meanining due to historical reasons")
)

// Default step used if not set.
const defaultStep = 5 * 60 * 1000

// FederateHandler implements /federate . See https://prometheus.io/docs/prometheus/latest/federation/
func FederateHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	ct := currentTime()
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse request form values: %w", err)
	}
	matches := r.Form["match[]"]
	if len(matches) == 0 {
		return fmt.Errorf("missing `match[]` arg")
	}
	lookbackDelta, err := getMaxLookback(r)
	if err != nil {
		return err
	}
	if lookbackDelta <= 0 {
		lookbackDelta = defaultStep
	}
	start, err := getTime(r, "start", ct-lookbackDelta)
	if err != nil {
		return err
	}
	end, err := getTime(r, "end", ct)
	if err != nil {
		return err
	}
	deadline := getDeadlineForQuery(r)
	if start >= end {
		start = end - defaultStep
	}
	tagFilterss, err := getTagFilterssFromMatches(matches)
	if err != nil {
		return err
	}
	sq := &storage.SearchQuery{
		MinTimestamp: start,
		MaxTimestamp: end,
		TagFilterss:  tagFilterss,
	}
	rss, err := netstorage.ProcessSearchQuery(sq, true, deadline)
	if err != nil {
		return fmt.Errorf("cannot fetch data for %q: %w", sq, err)
	}

	resultsCh := make(chan *quicktemplate.ByteBuffer)
	doneCh := make(chan error)
	go func() {
		err := rss.RunParallel(func(rs *netstorage.Result, workerID uint) {
			bb := quicktemplate.AcquireByteBuffer()
			WriteFederate(bb, rs)
			resultsCh <- bb
		})
		close(resultsCh)
		doneCh <- err
	}()

	w.Header().Set("Content-Type", "text/plain")
	for bb := range resultsCh {
		w.Write(bb.B)
		quicktemplate.ReleaseByteBuffer(bb)
	}

	err = <-doneCh
	if err != nil {
		return fmt.Errorf("error during data fetching: %w", err)
	}
	federateDuration.UpdateDuration(startTime)
	return nil
}

var federateDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/federate"}`)

// ExportHandler exports data in raw format from /api/v1/export.
func ExportHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	ct := currentTime()
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse request form values: %w", err)
	}
	matches := r.Form["match[]"]
	if len(matches) == 0 {
		// Maintain backwards compatibility
		match := r.FormValue("match")
		if len(match) == 0 {
			return fmt.Errorf("missing `match[]` arg")
		}
		matches = []string{match}
	}
	start, err := getTime(r, "start", 0)
	if err != nil {
		return err
	}
	end, err := getTime(r, "end", ct)
	if err != nil {
		return err
	}
	format := r.FormValue("format")
	maxRowsPerLine := int(fastfloat.ParseInt64BestEffort(r.FormValue("max_rows_per_line")))
	deadline := getDeadlineForExport(r)
	if start >= end {
		end = start + defaultStep
	}
	if err := exportHandler(w, matches, start, end, format, maxRowsPerLine, deadline); err != nil {
		return fmt.Errorf("error when exporting data for queries=%q on the time range (start=%d, end=%d): %w", matches, start, end, err)
	}
	exportDuration.UpdateDuration(startTime)
	return nil
}

var exportDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/export"}`)

func exportHandler(w http.ResponseWriter, matches []string, start, end int64, format string, maxRowsPerLine int, deadline netstorage.Deadline) error {
	writeResponseFunc := WriteExportStdResponse
	writeLineFunc := WriteExportJSONLine
	if maxRowsPerLine > 0 {
		writeLineFunc = func(w io.Writer, rs *netstorage.Result) {
			valuesOrig := rs.Values
			timestampsOrig := rs.Timestamps
			values := valuesOrig
			timestamps := timestampsOrig
			for len(values) > 0 {
				var valuesChunk []float64
				var timestampsChunk []int64
				if len(values) > maxRowsPerLine {
					valuesChunk = values[:maxRowsPerLine]
					timestampsChunk = timestamps[:maxRowsPerLine]
					values = values[maxRowsPerLine:]
					timestamps = timestamps[maxRowsPerLine:]
				} else {
					valuesChunk = values
					timestampsChunk = timestamps
					values = nil
					timestamps = nil
				}
				rs.Values = valuesChunk
				rs.Timestamps = timestampsChunk
				WriteExportJSONLine(w, rs)
			}
			rs.Values = valuesOrig
			rs.Timestamps = timestampsOrig
		}
	}
	contentType := "application/stream+json"
	if format == "prometheus" {
		contentType = "text/plain"
		writeLineFunc = WriteExportPrometheusLine
	} else if format == "promapi" {
		writeResponseFunc = WriteExportPromAPIResponse
		writeLineFunc = WriteExportPromAPILine
	}

	tagFilterss, err := getTagFilterssFromMatches(matches)
	if err != nil {
		return err
	}
	sq := &storage.SearchQuery{
		MinTimestamp: start,
		MaxTimestamp: end,
		TagFilterss:  tagFilterss,
	}
	rss, err := netstorage.ProcessSearchQuery(sq, true, deadline)
	if err != nil {
		return fmt.Errorf("cannot fetch data for %q: %w", sq, err)
	}

	resultsCh := make(chan *quicktemplate.ByteBuffer, runtime.GOMAXPROCS(-1))
	doneCh := make(chan error)
	go func() {
		err := rss.RunParallel(func(rs *netstorage.Result, workerID uint) {
			bb := quicktemplate.AcquireByteBuffer()
			writeLineFunc(bb, rs)
			resultsCh <- bb
		})
		close(resultsCh)
		doneCh <- err
	}()

	w.Header().Set("Content-Type", contentType)
	writeResponseFunc(w, resultsCh)

	// Consume all the data from resultsCh in the event writeResponseFunc
	// fails to consume all the data.
	for bb := range resultsCh {
		quicktemplate.ReleaseByteBuffer(bb)
	}
	err = <-doneCh
	if err != nil {
		return fmt.Errorf("error during data fetching: %w", err)
	}
	return nil
}

// DeleteHandler processes /api/v1/admin/tsdb/delete_series prometheus API request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#delete-series
func DeleteHandler(startTime time.Time, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse request form values: %w", err)
	}
	if r.FormValue("start") != "" || r.FormValue("end") != "" {
		return fmt.Errorf("start and end aren't supported. Remove these args from the query in order to delete all the matching metrics")
	}
	matches := r.Form["match[]"]
	if len(matches) == 0 {
		return fmt.Errorf("missing `match[]` arg")
	}
	tagFilterss, err := getTagFilterssFromMatches(matches)
	if err != nil {
		return err
	}
	sq := &storage.SearchQuery{
		TagFilterss: tagFilterss,
	}
	deletedCount, err := netstorage.DeleteSeries(sq)
	if err != nil {
		return fmt.Errorf("cannot delete time series matching %q: %w", matches, err)
	}
	if deletedCount > 0 {
		promql.ResetRollupResultCache()
	}
	deleteDuration.UpdateDuration(startTime)
	return nil
}

var deleteDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/admin/tsdb/delete_series"}`)

// LabelValuesHandler processes /api/v1/label/<labelName>/values request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#querying-label-values
func LabelValuesHandler(startTime time.Time, labelName string, w http.ResponseWriter, r *http.Request) error {
	deadline := getDeadlineForQuery(r)

	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	var labelValues []string
	if len(r.Form["match[]"]) == 0 && len(r.Form["start"]) == 0 && len(r.Form["end"]) == 0 {
		var err error
		labelValues, err = netstorage.GetLabelValues(labelName, deadline)
		if err != nil {
			return fmt.Errorf(`cannot obtain label values for %q: %w`, labelName, err)
		}
	} else {
		// Extended functionality that allows filtering by label filters and time range
		// i.e. /api/v1/label/foo/values?match[]=foobar{baz="abc"}&start=...&end=...
		// is equivalent to `label_values(foobar{baz="abc"}, foo)` call on the selected
		// time range in Grafana templating.
		matches := r.Form["match[]"]
		if len(matches) == 0 {
			matches = []string{fmt.Sprintf("{%s!=''}", labelName)}
		}
		ct := currentTime()
		end, err := getTime(r, "end", ct)
		if err != nil {
			return err
		}
		start, err := getTime(r, "start", end-defaultStep)
		if err != nil {
			return err
		}
		labelValues, err = labelValuesWithMatches(labelName, matches, start, end, deadline)
		if err != nil {
			return fmt.Errorf("cannot obtain label values for %q, match[]=%q, start=%d, end=%d: %w", labelName, matches, start, end, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	WriteLabelValuesResponse(w, labelValues)
	labelValuesDuration.UpdateDuration(startTime)
	return nil
}

func labelValuesWithMatches(labelName string, matches []string, start, end int64, deadline netstorage.Deadline) ([]string, error) {
	if len(matches) == 0 {
		logger.Panicf("BUG: matches must be non-empty")
	}
	tagFilterss, err := getTagFilterssFromMatches(matches)
	if err != nil {
		return nil, err
	}

	// Add `labelName!=''` tag filter in order to filter out series without the labelName.
	// There is no need in adding `__name__!=''` filter, since all the time series should
	// already have non-empty name.
	if labelName != "__name__" {
		key := []byte(labelName)
		for i, tfs := range tagFilterss {
			tagFilterss[i] = append(tfs, storage.TagFilter{
				Key:        key,
				IsNegative: true,
			})
		}
	}
	if start >= end {
		end = start + defaultStep
	}
	sq := &storage.SearchQuery{
		MinTimestamp: start,
		MaxTimestamp: end,
		TagFilterss:  tagFilterss,
	}
	rss, err := netstorage.ProcessSearchQuery(sq, false, deadline)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch data for %q: %w", sq, err)
	}

	m := make(map[string]struct{})
	var mLock sync.Mutex
	err = rss.RunParallel(func(rs *netstorage.Result, workerID uint) {
		labelValue := rs.MetricName.GetTagValue(labelName)
		if len(labelValue) == 0 {
			return
		}
		mLock.Lock()
		m[string(labelValue)] = struct{}{}
		mLock.Unlock()
	})
	if err != nil {
		return nil, fmt.Errorf("error when data fetching: %w", err)
	}

	labelValues := make([]string, 0, len(m))
	for labelValue := range m {
		labelValues = append(labelValues, labelValue)
	}
	sort.Strings(labelValues)
	return labelValues, nil
}

var labelValuesDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/label/{}/values"}`)

// LabelsCountHandler processes /api/v1/labels/count request.
func LabelsCountHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := getDeadlineForQuery(r)
	labelEntries, err := netstorage.GetLabelEntries(deadline)
	if err != nil {
		return fmt.Errorf(`cannot obtain label entries: %w`, err)
	}
	w.Header().Set("Content-Type", "application/json")
	WriteLabelsCountResponse(w, labelEntries)
	labelsCountDuration.UpdateDuration(startTime)
	return nil
}

var labelsCountDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/labels/count"}`)

const secsPerDay = 3600 * 24

// TSDBStatusHandler processes /api/v1/status/tsdb request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-stats
func TSDBStatusHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := getDeadlineForQuery(r)
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	date := fasttime.UnixDate()
	dateStr := r.FormValue("date")
	if len(dateStr) > 0 {
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return fmt.Errorf("cannot parse `date` arg %q: %w", dateStr, err)
		}
		date = uint64(t.Unix()) / secsPerDay
	}
	topN := 10
	topNStr := r.FormValue("topN")
	if len(topNStr) > 0 {
		n, err := strconv.Atoi(topNStr)
		if err != nil {
			return fmt.Errorf("cannot parse `topN` arg %q: %w", topNStr, err)
		}
		if n <= 0 {
			n = 1
		}
		if n > 1000 {
			n = 1000
		}
		topN = n
	}
	status, err := netstorage.GetTSDBStatusForDate(deadline, date, topN)
	if err != nil {
		return fmt.Errorf(`cannot obtain tsdb status for date=%d, topN=%d: %w`, date, topN, err)
	}
	w.Header().Set("Content-Type", "application/json")
	WriteTSDBStatusResponse(w, status)
	tsdbStatusDuration.UpdateDuration(startTime)
	return nil
}

var tsdbStatusDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/status/tsdb"}`)

// LabelsHandler processes /api/v1/labels request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#getting-label-names
func LabelsHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := getDeadlineForQuery(r)

	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	var labels []string
	if len(r.Form["match[]"]) == 0 && len(r.Form["start"]) == 0 && len(r.Form["end"]) == 0 {
		var err error
		labels, err = netstorage.GetLabels(deadline)
		if err != nil {
			return fmt.Errorf("cannot obtain labels: %w", err)
		}
	} else {
		// Extended functionality that allows filtering by label filters and time range
		// i.e. /api/v1/labels?match[]=foobar{baz="abc"}&start=...&end=...
		matches := r.Form["match[]"]
		if len(matches) == 0 {
			matches = []string{"{__name__!=''}"}
		}
		ct := currentTime()
		end, err := getTime(r, "end", ct)
		if err != nil {
			return err
		}
		start, err := getTime(r, "start", end-defaultStep)
		if err != nil {
			return err
		}
		labels, err = labelsWithMatches(matches, start, end, deadline)
		if err != nil {
			return fmt.Errorf("cannot obtain labels for match[]=%q, start=%d, end=%d: %w", matches, start, end, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	WriteLabelsResponse(w, labels)
	labelsDuration.UpdateDuration(startTime)
	return nil
}

func labelsWithMatches(matches []string, start, end int64, deadline netstorage.Deadline) ([]string, error) {
	if len(matches) == 0 {
		logger.Panicf("BUG: matches must be non-empty")
	}
	tagFilterss, err := getTagFilterssFromMatches(matches)
	if err != nil {
		return nil, err
	}
	if start >= end {
		end = start + defaultStep
	}
	sq := &storage.SearchQuery{
		MinTimestamp: start,
		MaxTimestamp: end,
		TagFilterss:  tagFilterss,
	}
	rss, err := netstorage.ProcessSearchQuery(sq, false, deadline)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch data for %q: %w", sq, err)
	}

	m := make(map[string]struct{})
	var mLock sync.Mutex
	err = rss.RunParallel(func(rs *netstorage.Result, workerID uint) {
		mLock.Lock()
		tags := rs.MetricName.Tags
		for i := range tags {
			t := &tags[i]
			m[string(t.Key)] = struct{}{}
		}
		m["__name__"] = struct{}{}
		mLock.Unlock()
	})
	if err != nil {
		return nil, fmt.Errorf("error when data fetching: %w", err)
	}

	labels := make([]string, 0, len(m))
	for label := range m {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	return labels, nil
}

var labelsDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/labels"}`)

// SeriesCountHandler processes /api/v1/series/count request.
func SeriesCountHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := getDeadlineForQuery(r)
	n, err := netstorage.GetSeriesCount(deadline)
	if err != nil {
		return fmt.Errorf("cannot obtain series count: %w", err)
	}
	w.Header().Set("Content-Type", "application/json")
	WriteSeriesCountResponse(w, n)
	seriesCountDuration.UpdateDuration(startTime)
	return nil
}

var seriesCountDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/series/count"}`)

// SeriesHandler processes /api/v1/series request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#finding-series-by-label-matchers
func SeriesHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	ct := currentTime()

	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	matches := r.Form["match[]"]
	if len(matches) == 0 {
		return fmt.Errorf("missing `match[]` arg")
	}
	end, err := getTime(r, "end", ct)
	if err != nil {
		return err
	}
	// Do not set start to minTimeMsecs by default as Prometheus does,
	// since this leads to fetching and scanning all the data from the storage,
	// which can take a lot of time for big storages.
	// It is better setting start as end-defaultStep by default.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/91
	start, err := getTime(r, "start", end-defaultStep)
	if err != nil {
		return err
	}
	deadline := getDeadlineForQuery(r)

	tagFilterss, err := getTagFilterssFromMatches(matches)
	if err != nil {
		return err
	}
	if start >= end {
		end = start + defaultStep
	}
	sq := &storage.SearchQuery{
		MinTimestamp: start,
		MaxTimestamp: end,
		TagFilterss:  tagFilterss,
	}
	rss, err := netstorage.ProcessSearchQuery(sq, false, deadline)
	if err != nil {
		return fmt.Errorf("cannot fetch data for %q: %w", sq, err)
	}

	resultsCh := make(chan *quicktemplate.ByteBuffer)
	doneCh := make(chan error)
	go func() {
		err := rss.RunParallel(func(rs *netstorage.Result, workerID uint) {
			bb := quicktemplate.AcquireByteBuffer()
			writemetricNameObject(bb, &rs.MetricName)
			resultsCh <- bb
		})
		close(resultsCh)
		doneCh <- err
	}()

	w.Header().Set("Content-Type", "application/json")
	WriteSeriesResponse(w, resultsCh)

	// Consume all the data from resultsCh in the event WriteSeriesResponse
	// fails to consume all the data.
	for bb := range resultsCh {
		quicktemplate.ReleaseByteBuffer(bb)
	}
	err = <-doneCh
	if err != nil {
		return fmt.Errorf("error during data fetching: %w", err)
	}
	seriesDuration.UpdateDuration(startTime)
	return nil
}

var seriesDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/series"}`)

// QueryHandler processes /api/v1/query request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#instant-queries
func QueryHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	ct := currentTime()

	query := r.FormValue("query")
	if len(query) == 0 {
		return fmt.Errorf("missing `query` arg")
	}
	start, err := getTime(r, "time", ct)
	if err != nil {
		return err
	}
	lookbackDelta, err := getMaxLookback(r)
	if err != nil {
		return err
	}
	step, err := getDuration(r, "step", lookbackDelta)
	if err != nil {
		return err
	}
	if step <= 0 {
		step = defaultStep
	}
	deadline := getDeadlineForQuery(r)

	if len(query) > *maxQueryLen {
		return fmt.Errorf("too long query; got %d bytes; mustn't exceed `-search.maxQueryLen=%d` bytes", len(query), *maxQueryLen)
	}
	queryOffset := getLatencyOffsetMilliseconds()
	if !getBool(r, "nocache") && ct-start < queryOffset {
		// Adjust start time only if `nocache` arg isn't set.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/241
		start = ct - queryOffset
	}
	if childQuery, windowStr, offsetStr := promql.IsMetricSelectorWithRollup(query); childQuery != "" {
		window, err := parsePositiveDuration(windowStr, step)
		if err != nil {
			return fmt.Errorf("cannot parse window: %w", err)
		}
		offset, err := parseDuration(offsetStr, step)
		if err != nil {
			return fmt.Errorf("cannot parse offset: %w", err)
		}
		start -= offset
		end := start
		start = end - window
		if err := exportHandler(w, []string{childQuery}, start, end, "promapi", 0, deadline); err != nil {
			return fmt.Errorf("error when exporting data for query=%q on the time range (start=%d, end=%d): %w", childQuery, start, end, err)
		}
		queryDuration.UpdateDuration(startTime)
		return nil
	}
	if childQuery, windowStr, stepStr, offsetStr := promql.IsRollup(query); childQuery != "" {
		newStep, err := parsePositiveDuration(stepStr, step)
		if err != nil {
			return fmt.Errorf("cannot parse step: %w", err)
		}
		if newStep > 0 {
			step = newStep
		}
		window, err := parsePositiveDuration(windowStr, step)
		if err != nil {
			return fmt.Errorf("cannot parse window: %w", err)
		}
		offset, err := parseDuration(offsetStr, step)
		if err != nil {
			return fmt.Errorf("cannot parse offset: %w", err)
		}
		start -= offset
		end := start
		start = end - window
		if err := queryRangeHandler(w, childQuery, start, end, step, r, ct); err != nil {
			return fmt.Errorf("error when executing query=%q on the time range (start=%d, end=%d, step=%d): %w", childQuery, start, end, step, err)
		}
		queryDuration.UpdateDuration(startTime)
		return nil
	}

	ec := promql.EvalConfig{
		Start:         start,
		End:           start,
		Step:          step,
		Deadline:      deadline,
		LookbackDelta: lookbackDelta,
	}
	result, err := promql.Exec(&ec, query, true)
	if err != nil {
		return fmt.Errorf("error when executing query=%q for (time=%d, step=%d): %w", query, start, step, err)
	}

	w.Header().Set("Content-Type", "application/json")
	WriteQueryResponse(w, result)
	queryDuration.UpdateDuration(startTime)
	return nil
}

var queryDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/query"}`)

func parseDuration(s string, step int64) (int64, error) {
	if len(s) == 0 {
		return 0, nil
	}
	return metricsql.DurationValue(s, step)
}

func parsePositiveDuration(s string, step int64) (int64, error) {
	if len(s) == 0 {
		return 0, nil
	}
	return metricsql.PositiveDurationValue(s, step)
}

// QueryRangeHandler processes /api/v1/query_range request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries
func QueryRangeHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	ct := currentTime()

	query := r.FormValue("query")
	if len(query) == 0 {
		return fmt.Errorf("missing `query` arg")
	}
	start, err := getTime(r, "start", ct-defaultStep)
	if err != nil {
		return err
	}
	end, err := getTime(r, "end", ct)
	if err != nil {
		return err
	}
	step, err := getDuration(r, "step", defaultStep)
	if err != nil {
		return err
	}
	if err := queryRangeHandler(w, query, start, end, step, r, ct); err != nil {
		return fmt.Errorf("error when executing query=%q on the time range (start=%d, end=%d, step=%d): %w", query, start, end, step, err)
	}
	queryRangeDuration.UpdateDuration(startTime)
	return nil
}

func queryRangeHandler(w http.ResponseWriter, query string, start, end, step int64, r *http.Request, ct int64) error {
	deadline := getDeadlineForQuery(r)
	mayCache := !getBool(r, "nocache")
	lookbackDelta, err := getMaxLookback(r)
	if err != nil {
		return err
	}

	// Validate input args.
	if len(query) > *maxQueryLen {
		return fmt.Errorf("too long query; got %d bytes; mustn't exceed `-search.maxQueryLen=%d` bytes", len(query), *maxQueryLen)
	}
	if start > end {
		end = start + defaultStep
	}
	if err := promql.ValidateMaxPointsPerTimeseries(start, end, step); err != nil {
		return err
	}
	if mayCache {
		start, end = promql.AdjustStartEnd(start, end, step)
	}

	ec := promql.EvalConfig{
		Start:         start,
		End:           end,
		Step:          step,
		Deadline:      deadline,
		MayCache:      mayCache,
		LookbackDelta: lookbackDelta,
	}
	result, err := promql.Exec(&ec, query, false)
	if err != nil {
		return fmt.Errorf("cannot execute query: %w", err)
	}
	queryOffset := getLatencyOffsetMilliseconds()
	if ct-end < queryOffset {
		result = adjustLastPoints(result)
	}

	// Remove NaN values as Prometheus does.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/153
	removeNaNValuesInplace(result)

	w.Header().Set("Content-Type", "application/json")
	WriteQueryRangeResponse(w, result)
	return nil
}

func removeNaNValuesInplace(tss []netstorage.Result) {
	for i := range tss {
		ts := &tss[i]
		hasNaNs := false
		for _, v := range ts.Values {
			if math.IsNaN(v) {
				hasNaNs = true
				break
			}
		}
		if !hasNaNs {
			// Fast path: nothing to remove.
			continue
		}

		// Slow path: remove NaNs.
		srcTimestamps := ts.Timestamps
		dstValues := ts.Values[:0]
		dstTimestamps := ts.Timestamps[:0]
		for j, v := range ts.Values {
			if math.IsNaN(v) {
				continue
			}
			dstValues = append(dstValues, v)
			dstTimestamps = append(dstTimestamps, srcTimestamps[j])
		}
		ts.Values = dstValues
		ts.Timestamps = dstTimestamps
	}
}

var queryRangeDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/query_range"}`)

// adjustLastPoints substitutes the last point values with the previous
// point values, since the last points may contain garbage.
func adjustLastPoints(tss []netstorage.Result) []netstorage.Result {
	if len(tss) == 0 {
		return nil
	}

	// Search for the last non-NaN value across all the timeseries.
	lastNonNaNIdx := -1
	for i := range tss {
		values := tss[i].Values
		j := len(values) - 1
		for j >= 0 && math.IsNaN(values[j]) {
			j--
		}
		if j > lastNonNaNIdx {
			lastNonNaNIdx = j
		}
	}
	if lastNonNaNIdx == -1 {
		// All timeseries contain only NaNs.
		return nil
	}

	// Substitute the last two values starting from lastNonNaNIdx
	// with the previous values for each timeseries.
	for i := range tss {
		values := tss[i].Values
		for j := 0; j < 2; j++ {
			idx := lastNonNaNIdx + j
			if idx <= 0 || idx >= len(values) || math.IsNaN(values[idx-1]) {
				continue
			}
			values[idx] = values[idx-1]
		}
	}
	return tss
}

func getTime(r *http.Request, argKey string, defaultValue int64) (int64, error) {
	argValue := r.FormValue(argKey)
	if len(argValue) == 0 {
		return defaultValue, nil
	}
	secs, err := strconv.ParseFloat(argValue, 64)
	if err != nil {
		// Try parsing string format
		t, err := time.Parse(time.RFC3339, argValue)
		if err != nil {
			// Handle Prometheus'-provided minTime and maxTime.
			// See https://github.com/prometheus/client_golang/issues/614
			switch argValue {
			case prometheusMinTimeFormatted:
				return minTimeMsecs, nil
			case prometheusMaxTimeFormatted:
				return maxTimeMsecs, nil
			}
			// Try parsing duration relative to the current time
			d, err1 := time.ParseDuration(argValue)
			if err1 != nil {
				return 0, fmt.Errorf("cannot parse %q=%q: %w", argKey, argValue, err)
			}
			if d > 0 {
				d = -d
			}
			t = time.Now().Add(d)
		}
		secs = float64(t.UnixNano()) / 1e9
	}
	msecs := int64(secs * 1e3)
	if msecs < minTimeMsecs {
		msecs = 0
	}
	if msecs > maxTimeMsecs {
		msecs = maxTimeMsecs
	}
	return msecs, nil
}

var (
	// These constants were obtained from https://github.com/prometheus/prometheus/blob/91d7175eaac18b00e370965f3a8186cc40bf9f55/web/api/v1/api.go#L442
	// See https://github.com/prometheus/client_golang/issues/614 for details.
	prometheusMinTimeFormatted = time.Unix(math.MinInt64/1000+62135596801, 0).UTC().Format(time.RFC3339Nano)
	prometheusMaxTimeFormatted = time.Unix(math.MaxInt64/1000-62135596801, 999999999).UTC().Format(time.RFC3339Nano)
)

const (
	// These values prevent from overflow when storing msec-precision time in int64.
	minTimeMsecs = 0 // use 0 instead of `int64(-1<<63) / 1e6` because the storage engine doesn't actually support negative time
	maxTimeMsecs = int64(1<<63-1) / 1e6
)

func getDuration(r *http.Request, argKey string, defaultValue int64) (int64, error) {
	argValue := r.FormValue(argKey)
	if len(argValue) == 0 {
		return defaultValue, nil
	}
	secs, err := strconv.ParseFloat(argValue, 64)
	if err != nil {
		// Try parsing string format
		d, err := time.ParseDuration(argValue)
		if err != nil {
			return 0, fmt.Errorf("cannot parse %q=%q: %w", argKey, argValue, err)
		}
		secs = d.Seconds()
	}
	msecs := int64(secs * 1e3)
	if msecs <= 0 || msecs > maxDurationMsecs {
		return 0, fmt.Errorf("%q=%dms is out of allowed range [%d ... %d]", argKey, msecs, 0, int64(maxDurationMsecs))
	}
	return msecs, nil
}

const maxDurationMsecs = 100 * 365 * 24 * 3600 * 1000

func getMaxLookback(r *http.Request) (int64, error) {
	d := maxLookback.Milliseconds()
	if d == 0 {
		d = maxStalenessInterval.Milliseconds()
	}
	return getDuration(r, "max_lookback", d)
}

func getDeadlineForQuery(r *http.Request) netstorage.Deadline {
	dMax := maxQueryDuration.Milliseconds()
	return getDeadlineWithMaxDuration(r, dMax, "-search.maxQueryDuration")
}

func getDeadlineForExport(r *http.Request) netstorage.Deadline {
	dMax := maxExportDuration.Milliseconds()
	return getDeadlineWithMaxDuration(r, dMax, "-search.maxExportDuration")
}

func getDeadlineWithMaxDuration(r *http.Request, dMax int64, flagHint string) netstorage.Deadline {
	d, err := getDuration(r, "timeout", 0)
	if err != nil {
		d = 0
	}
	if d <= 0 || d > dMax {
		d = dMax
	}
	timeout := time.Duration(d) * time.Millisecond
	return netstorage.NewDeadline(timeout, flagHint)
}

func getBool(r *http.Request, argKey string) bool {
	argValue := r.FormValue(argKey)
	switch strings.ToLower(argValue) {
	case "", "0", "f", "false", "no":
		return false
	default:
		return true
	}
}

func currentTime() int64 {
	return int64(fasttime.UnixTimestamp() * 1000)
}

func getTagFilterssFromMatches(matches []string) ([][]storage.TagFilter, error) {
	tagFilterss := make([][]storage.TagFilter, 0, len(matches))
	for _, match := range matches {
		tagFilters, err := promql.ParseMetricSelector(match)
		if err != nil {
			return nil, fmt.Errorf("cannot parse %q: %w", match, err)
		}
		tagFilterss = append(tagFilterss, tagFilters)
	}
	return tagFilterss, nil
}

func getLatencyOffsetMilliseconds() int64 {
	d := latencyOffset.Milliseconds()
	if d <= 1000 {
		d = 1000
	}
	return d
}
