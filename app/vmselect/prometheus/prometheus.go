package prometheus

import (
	"flag"
	"fmt"
	"math"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
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
	maxQueryLen = flagutil.NewBytes("search.maxQueryLen", 16*1024, "The maximum search query length in bytes")
	maxLookback = flag.Duration("search.maxLookback", 0, "Synonim to -search.lookback-delta from Prometheus. "+
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
	ct := startTime.UnixNano() / 1e6
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
	start, err := searchutils.GetTime(r, "start", ct-lookbackDelta)
	if err != nil {
		return err
	}
	end, err := searchutils.GetTime(r, "end", ct)
	if err != nil {
		return err
	}
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
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
	ct := startTime.UnixNano() / 1e6
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
	start, err := searchutils.GetTime(r, "start", 0)
	if err != nil {
		return err
	}
	end, err := searchutils.GetTime(r, "end", ct)
	if err != nil {
		return err
	}
	format := r.FormValue("format")
	maxRowsPerLine := int(fastfloat.ParseInt64BestEffort(r.FormValue("max_rows_per_line")))
	deadline := searchutils.GetDeadlineForExport(r, startTime)
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
	writeLineFunc := func(rs *netstorage.Result, resultsCh chan<- *quicktemplate.ByteBuffer) {
		bb := quicktemplate.AcquireByteBuffer()
		WriteExportJSONLine(bb, rs)
		resultsCh <- bb
	}
	if maxRowsPerLine > 0 {
		writeLineFunc = func(rs *netstorage.Result, resultsCh chan<- *quicktemplate.ByteBuffer) {
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
				bb := quicktemplate.AcquireByteBuffer()
				WriteExportJSONLine(bb, rs)
				resultsCh <- bb
			}
			rs.Values = valuesOrig
			rs.Timestamps = timestampsOrig
		}
	}
	contentType := "application/stream+json"
	if format == "prometheus" {
		contentType = "text/plain"
		writeLineFunc = func(rs *netstorage.Result, resultsCh chan<- *quicktemplate.ByteBuffer) {
			bb := quicktemplate.AcquireByteBuffer()
			WriteExportPrometheusLine(bb, rs)
			resultsCh <- bb
		}
	} else if format == "promapi" {
		writeResponseFunc = WriteExportPromAPIResponse
		writeLineFunc = func(rs *netstorage.Result, resultsCh chan<- *quicktemplate.ByteBuffer) {
			bb := quicktemplate.AcquireByteBuffer()
			WriteExportPromAPILine(bb, rs)
			resultsCh <- bb
		}
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
			writeLineFunc(rs, resultsCh)
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
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
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
		ct := startTime.UnixNano() / 1e6
		end, err := searchutils.GetTime(r, "end", ct)
		if err != nil {
			return err
		}
		start, err := searchutils.GetTime(r, "start", end-defaultStep)
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
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
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
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
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
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
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
		ct := startTime.UnixNano() / 1e6
		end, err := searchutils.GetTime(r, "end", ct)
		if err != nil {
			return err
		}
		start, err := searchutils.GetTime(r, "start", end-defaultStep)
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
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
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
	ct := startTime.UnixNano() / 1e6
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	matches := r.Form["match[]"]
	if len(matches) == 0 {
		return fmt.Errorf("missing `match[]` arg")
	}
	end, err := searchutils.GetTime(r, "end", ct)
	if err != nil {
		return err
	}
	// Do not set start to searchutils.minTimeMsecs by default as Prometheus does,
	// since this leads to fetching and scanning all the data from the storage,
	// which can take a lot of time for big storages.
	// It is better setting start as end-defaultStep by default.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/91
	start, err := searchutils.GetTime(r, "start", end-defaultStep)
	if err != nil {
		return err
	}
	deadline := searchutils.GetDeadlineForQuery(r, startTime)

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
	ct := startTime.UnixNano() / 1e6
	query := r.FormValue("query")
	if len(query) == 0 {
		return fmt.Errorf("missing `query` arg")
	}
	start, err := searchutils.GetTime(r, "time", ct)
	if err != nil {
		return err
	}
	lookbackDelta, err := getMaxLookback(r)
	if err != nil {
		return err
	}
	step, err := searchutils.GetDuration(r, "step", lookbackDelta)
	if err != nil {
		return err
	}
	if step <= 0 {
		step = defaultStep
	}
	deadline := searchutils.GetDeadlineForQuery(r, startTime)

	if len(query) > maxQueryLen.N {
		return fmt.Errorf("too long query; got %d bytes; mustn't exceed `-search.maxQueryLen=%d` bytes", len(query), maxQueryLen.N)
	}
	queryOffset := getLatencyOffsetMilliseconds()
	if !searchutils.GetBool(r, "nocache") && ct-start < queryOffset {
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
		if err := queryRangeHandler(startTime, w, childQuery, start, end, step, r, ct); err != nil {
			return fmt.Errorf("error when executing query=%q on the time range (start=%d, end=%d, step=%d): %w", childQuery, start, end, step, err)
		}
		queryDuration.UpdateDuration(startTime)
		return nil
	}

	ec := promql.EvalConfig{
		Start:            start,
		End:              start,
		Step:             step,
		QuotedRemoteAddr: httpserver.GetQuotedRemoteAddr(r),
		Deadline:         deadline,
		LookbackDelta:    lookbackDelta,
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
	ct := startTime.UnixNano() / 1e6
	query := r.FormValue("query")
	if len(query) == 0 {
		return fmt.Errorf("missing `query` arg")
	}
	start, err := searchutils.GetTime(r, "start", ct-defaultStep)
	if err != nil {
		return err
	}
	end, err := searchutils.GetTime(r, "end", ct)
	if err != nil {
		return err
	}
	step, err := searchutils.GetDuration(r, "step", defaultStep)
	if err != nil {
		return err
	}
	if err := queryRangeHandler(startTime, w, query, start, end, step, r, ct); err != nil {
		return fmt.Errorf("error when executing query=%q on the time range (start=%d, end=%d, step=%d): %w", query, start, end, step, err)
	}
	queryRangeDuration.UpdateDuration(startTime)
	return nil
}

func queryRangeHandler(startTime time.Time, w http.ResponseWriter, query string, start, end, step int64, r *http.Request, ct int64) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	mayCache := !searchutils.GetBool(r, "nocache")
	lookbackDelta, err := getMaxLookback(r)
	if err != nil {
		return err
	}

	// Validate input args.
	if len(query) > maxQueryLen.N {
		return fmt.Errorf("too long query; got %d bytes; mustn't exceed `-search.maxQueryLen=%d` bytes", len(query), maxQueryLen.N)
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
		Start:            start,
		End:              end,
		Step:             step,
		QuotedRemoteAddr: httpserver.GetQuotedRemoteAddr(r),
		Deadline:         deadline,
		MayCache:         mayCache,
		LookbackDelta:    lookbackDelta,
	}
	result, err := promql.Exec(&ec, query, false)
	if err != nil {
		return fmt.Errorf("cannot execute query: %w", err)
	}
	queryOffset := getLatencyOffsetMilliseconds()
	if ct-queryOffset < end {
		result = adjustLastPoints(result, ct-queryOffset, ct+step)
	}

	// Remove NaN values as Prometheus does.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/153
	result = removeEmptyValuesAndTimeseries(result)

	w.Header().Set("Content-Type", "application/json")
	WriteQueryRangeResponse(w, result)
	return nil
}

func removeEmptyValuesAndTimeseries(tss []netstorage.Result) []netstorage.Result {
	dst := tss[:0]
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
			if len(ts.Values) > 0 {
				dst = append(dst, *ts)
			}
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
		if len(ts.Values) > 0 {
			dst = append(dst, *ts)
		}
	}
	return dst
}

var queryRangeDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/query_range"}`)

var nan = math.NaN()

// adjustLastPoints substitutes the last point values on the time range (start..end]
// with the previous point values, since these points may contain incomplete values.
func adjustLastPoints(tss []netstorage.Result, start, end int64) []netstorage.Result {
	for i := range tss {
		ts := &tss[i]
		values := ts.Values
		timestamps := ts.Timestamps
		j := len(timestamps) - 1
		if j >= 0 && timestamps[j] > end {
			// It looks like the `offset` is used in the query, which shifts time range beyond the `end`.
			// Leave such a time series as is, since it is unclear which points may be incomplete in it.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/625
			continue
		}
		for j >= 0 && timestamps[j] > start {
			j--
		}
		j++
		lastValue := nan
		if j > 0 {
			lastValue = values[j-1]
		}
		for j < len(timestamps) && timestamps[j] <= end {
			values[j] = lastValue
			j++
		}
	}
	return tss
}

func getMaxLookback(r *http.Request) (int64, error) {
	d := maxLookback.Milliseconds()
	if d == 0 {
		d = maxStalenessInterval.Milliseconds()
	}
	return searchutils.GetDuration(r, "max_lookback", d)
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
