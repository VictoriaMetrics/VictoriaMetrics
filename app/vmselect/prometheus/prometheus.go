package prometheus

import (
	"flag"
	"fmt"
	"math"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/quicktemplate"
)

var (
	maxQueryDuration = flag.Duration("search.maxQueryDuration", time.Second*30, "The maximum time for search query execution")
	maxQueryLen      = flag.Int("search.maxQueryLen", 16*1024, "The maximum search query length in bytes")

	selectNodes flagutil.Array
)

func init() {
	flag.Var(&selectNodes, "selectNode", "vmselect address, usage -selectNode=vmselect-host1:8481 -selectNode=vmselect-host2:8481")
}

// Default step used if not set.
const defaultStep = 5 * 60 * 1000

// Latency for data processing pipeline, i.e. the time between data is ignested
// into the system and the time it becomes visible to search.
const latencyOffset = 60 * 1000

// FederateHandler implements /federate . See https://prometheus.io/docs/prometheus/latest/federation/
func FederateHandler(at *auth.Token, w http.ResponseWriter, r *http.Request) error {
	startTime := time.Now()
	ct := currentTime()
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse request form values: %s", err)
	}
	matches := r.Form["match[]"]
	maxLookback := getDuration(r, "max_lookback", defaultStep)
	start := getTime(r, "start", ct-maxLookback)
	end := getTime(r, "end", ct)
	deadline := getDeadline(r)
	if start >= end {
		start = end - defaultStep
	}
	tagFilterss, err := getTagFilterssFromMatches(matches)
	if err != nil {
		return err
	}
	sq := &storage.SearchQuery{
		AccountID:    at.AccountID,
		ProjectID:    at.ProjectID,
		MinTimestamp: start,
		MaxTimestamp: end,
		TagFilterss:  tagFilterss,
	}
	rss, _, err := netstorage.ProcessSearchQuery(at, sq, deadline)
	if err != nil {
		return fmt.Errorf("cannot fetch data for %q: %s", sq, err)
	}

	resultsCh := make(chan *quicktemplate.ByteBuffer)
	doneCh := make(chan error)
	go func() {
		err := rss.RunParallel(func(rs *netstorage.Result) {
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
		return fmt.Errorf("error during data fetching: %s", err)
	}
	federateDuration.UpdateDuration(startTime)
	return nil
}

var federateDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/federate"}`)

// ExportHandler exports data in raw format from /api/v1/export.
func ExportHandler(at *auth.Token, w http.ResponseWriter, r *http.Request) error {
	startTime := time.Now()
	ct := currentTime()
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse request form values: %s", err)
	}
	matches := r.Form["match[]"]
	if len(matches) == 0 {
		// Maintain backwards compatibility
		match := r.FormValue("match")
		matches = []string{match}
	}
	start := getTime(r, "start", 0)
	end := getTime(r, "end", ct)
	format := r.FormValue("format")
	deadline := getDeadline(r)
	if start >= end {
		start = end - defaultStep
	}
	if err := exportHandler(at, w, matches, start, end, format, deadline); err != nil {
		return err
	}
	exportDuration.UpdateDuration(startTime)
	return nil
}

var exportDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/export"}`)

func exportHandler(at *auth.Token, w http.ResponseWriter, matches []string, start, end int64, format string, deadline netstorage.Deadline) error {
	writeResponseFunc := WriteExportStdResponse
	writeLineFunc := WriteExportJSONLine
	contentType := "application/json"
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
		AccountID:    at.AccountID,
		ProjectID:    at.ProjectID,
		MinTimestamp: start,
		MaxTimestamp: end,
		TagFilterss:  tagFilterss,
	}
	rss, isPartial, err := netstorage.ProcessSearchQuery(at, sq, deadline)
	if err != nil {
		return fmt.Errorf("cannot fetch data for %q: %s", sq, err)
	}
	if isPartial {
		rss.Cancel()
		return fmt.Errorf("some of the storage nodes are unavailable at the moment")
	}

	resultsCh := make(chan *quicktemplate.ByteBuffer, runtime.GOMAXPROCS(-1))
	doneCh := make(chan error)
	go func() {
		err := rss.RunParallel(func(rs *netstorage.Result) {
			bb := quicktemplate.AcquireByteBuffer()
			writeLineFunc(bb, rs)
			resultsCh <- bb
		})
		close(resultsCh)
		doneCh <- err
	}()

	w.Header().Set("Content-Type", contentType)
	writeResponseFunc(w, resultsCh)

	err = <-doneCh
	if err != nil {
		return fmt.Errorf("error during data fetching: %s", err)
	}
	return nil
}

// DeleteHandler processes /api/v1/admin/tsdb/delete_series prometheus API request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#delete-series
func DeleteHandler(at *auth.Token, r *http.Request) error {
	startTime := time.Now()
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse request form values: %s", err)
	}
	if r.FormValue("start") != "" || r.FormValue("end") != "" {
		return fmt.Errorf("start and end aren't supported. Remove these args from the query in order to delete all the matching metrics")
	}
	matches := r.Form["match[]"]
	deadline := getDeadline(r)
	tagFilterss, err := getTagFilterssFromMatches(matches)
	if err != nil {
		return err
	}
	sq := &storage.SearchQuery{
		AccountID:   at.AccountID,
		ProjectID:   at.ProjectID,
		TagFilterss: tagFilterss,
	}
	deletedCount, err := netstorage.DeleteSeries(at, sq, deadline)
	if err != nil {
		return fmt.Errorf("cannot delete time series matching %q: %s", matches, err)
	}
	if deletedCount > 0 {
		// Reset rollup result cache on all the vmselect nodes,
		// since the cache may contain deleted data.
		// TODO: reset only cache for (account, project)
		resetRollupResultCaches()
	}
	deleteDuration.UpdateDuration(startTime)
	return nil
}

var deleteDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/admin/tsdb/delete_series"}`)

func resetRollupResultCaches() {
	if len(selectNodes) == 0 {
		logger.Panicf("BUG: missing -selectNode flag")
	}
	for _, selectNode := range selectNodes {
		callURL := fmt.Sprintf("http://%s/internal/resetRollupResultCache", selectNode)
		resp, err := httpClient.Get(callURL)
		if err != nil {
			logger.Errorf("error when accessing %q: %s", callURL, err)
			resetRollupResultCacheErrors.Inc()
			continue
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			logger.Errorf("unexpected status code at %q; got %d; want %d", callURL, resp.StatusCode, http.StatusOK)
			resetRollupResultCacheErrors.Inc()
			continue
		}
		_ = resp.Body.Close()
	}
	resetRollupResultCacheCalls.Inc()
}

var (
	resetRollupResultCacheErrors = metrics.NewCounter("vm_reset_rollup_result_cache_errors_total")
	resetRollupResultCacheCalls  = metrics.NewCounter("vm_reset_rollup_result_cache_calls_total")
)

var httpClient = &http.Client{
	Timeout: time.Second * 5,
}

// LabelValuesHandler processes /api/v1/label/<labelName>/values request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#querying-label-values
func LabelValuesHandler(at *auth.Token, labelName string, w http.ResponseWriter, r *http.Request) error {
	startTime := time.Now()
	deadline := getDeadline(r)
	labelValues, _, err := netstorage.GetLabelValues(at, labelName, deadline)
	if err != nil {
		return fmt.Errorf(`cannot obtain label values for %q: %s`, labelName, err)
	}

	w.Header().Set("Content-Type", "application/json")
	WriteLabelValuesResponse(w, labelValues)
	labelValuesDuration.UpdateDuration(startTime)
	return nil
}

var labelValuesDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/label/{}/values"}`)

// LabelsHandler processes /api/v1/labels request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#getting-label-names
func LabelsHandler(at *auth.Token, w http.ResponseWriter, r *http.Request) error {
	startTime := time.Now()
	deadline := getDeadline(r)
	labels, _, err := netstorage.GetLabels(at, deadline)
	if err != nil {
		return fmt.Errorf("cannot obtain labels: %s", err)
	}

	w.Header().Set("Content-Type", "application/json")
	WriteLabelsResponse(w, labels)
	labelsDuration.UpdateDuration(startTime)
	return nil
}

var labelsDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/labels"}`)

// SeriesCountHandler processes /api/v1/series/count request.
func SeriesCountHandler(at *auth.Token, w http.ResponseWriter, r *http.Request) error {
	startTime := time.Now()
	deadline := getDeadline(r)
	n, _, err := netstorage.GetSeriesCount(at, deadline)
	if err != nil {
		return fmt.Errorf("cannot obtain series count: %s", err)
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
func SeriesHandler(at *auth.Token, w http.ResponseWriter, r *http.Request) error {
	startTime := time.Now()
	ct := currentTime()

	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %s", err)
	}
	matches := r.Form["match[]"]
	start := getTime(r, "start", ct-defaultStep)
	end := getTime(r, "end", ct)
	deadline := getDeadline(r)

	tagFilterss, err := getTagFilterssFromMatches(matches)
	if err != nil {
		return err
	}
	if start >= end {
		start = end - defaultStep
	}
	sq := &storage.SearchQuery{
		AccountID:    at.AccountID,
		ProjectID:    at.ProjectID,
		MinTimestamp: start,
		MaxTimestamp: end,
		TagFilterss:  tagFilterss,
	}
	rss, _, err := netstorage.ProcessSearchQuery(at, sq, deadline)
	if err != nil {
		return fmt.Errorf("cannot fetch data for %q: %s", sq, err)
	}

	resultsCh := make(chan *quicktemplate.ByteBuffer)
	doneCh := make(chan error)
	go func() {
		err := rss.RunParallel(func(rs *netstorage.Result) {
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
	// fail to consume all the data.
	for bb := range resultsCh {
		quicktemplate.ReleaseByteBuffer(bb)
	}

	err = <-doneCh
	if err != nil {
		return fmt.Errorf("error during data fetching: %s", err)
	}
	seriesDuration.UpdateDuration(startTime)
	return nil
}

var seriesDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/series"}`)

// QueryHandler processes /api/v1/query request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#instant-queries
func QueryHandler(at *auth.Token, w http.ResponseWriter, r *http.Request) error {
	startTime := time.Now()
	ct := currentTime()

	query := r.FormValue("query")
	start := getTime(r, "time", ct)
	step := getDuration(r, "step", latencyOffset)
	deadline := getDeadline(r)

	if len(query) > *maxQueryLen {
		return fmt.Errorf(`too long query; got %d bytes; mustn't exceed %d bytes`, len(query), *maxQueryLen)
	}
	if ct-start < latencyOffset {
		start -= latencyOffset
	}
	if childQuery, windowStr, offsetStr := promql.IsMetricSelectorWithRollup(query); childQuery != "" {
		var window int64
		if len(windowStr) > 0 {
			var err error
			window, err = promql.DurationValue(windowStr, step)
			if err != nil {
				return err
			}
		}
		var offset int64
		if len(offsetStr) > 0 {
			var err error
			offset, err = promql.DurationValue(offsetStr, step)
			if err != nil {
				return err
			}
		}
		start -= offset
		end := start
		start = end - window
		if err := exportHandler(at, w, []string{childQuery}, start, end, "promapi", deadline); err != nil {
			return err
		}
		queryDuration.UpdateDuration(startTime)
		return nil
	}

	ec := promql.EvalConfig{
		AuthToken: at,
		Start:     start,
		End:       start,
		Step:      step,
		Deadline:  deadline,
	}
	result, err := promql.Exec(&ec, query)
	if err != nil {
		return fmt.Errorf("cannot execute %q: %s", query, err)
	}

	w.Header().Set("Content-Type", "application/json")
	WriteQueryResponse(w, result)
	queryDuration.UpdateDuration(startTime)
	return nil
}

var queryDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/query"}`)

// QueryRangeHandler processes /api/v1/query_range request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries
func QueryRangeHandler(at *auth.Token, w http.ResponseWriter, r *http.Request) error {
	startTime := time.Now()
	ct := currentTime()

	query := r.FormValue("query")
	start := getTime(r, "start", ct-defaultStep)
	end := getTime(r, "end", ct)
	step := getDuration(r, "step", defaultStep)
	deadline := getDeadline(r)
	mayCache := !getBool(r, "nocache")

	// Validate input args.
	if len(query) > *maxQueryLen {
		return fmt.Errorf(`too long query; got %d bytes; mustn't exceed %d bytes`, len(query), *maxQueryLen)
	}
	if start > end {
		start = end
	}
	if err := promql.ValidateMaxPointsPerTimeseries(start, end, step); err != nil {
		return err
	}
	start, end = promql.AdjustStartEnd(start, end, step)

	ec := promql.EvalConfig{
		AuthToken: at,
		Start:     start,
		End:       end,
		Step:      step,
		Deadline:  deadline,
		MayCache:  mayCache,
	}
	result, err := promql.Exec(&ec, query)
	if err != nil {
		return fmt.Errorf("cannot execute %q: %s", query, err)
	}
	if ct-end < latencyOffset {
		adjustLastPoints(result)
	}

	w.Header().Set("Content-Type", "application/json")
	WriteQueryRangeResponse(w, result)
	queryRangeDuration.UpdateDuration(startTime)
	return nil
}

var queryRangeDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/query_range"}`)

// adjustLastPoints substitutes the last point values with the previous
// point values, since the last points may contain garbage.
func adjustLastPoints(tss []netstorage.Result) {
	if len(tss) == 0 {
		return
	}

	// Search for the last non-NaN value across all the timeseries.
	lastNonNaNIdx := -1
	for i := range tss {
		r := &tss[i]
		j := len(r.Values) - 1
		for j >= 0 && math.IsNaN(r.Values[j]) {
			j--
		}
		if j > lastNonNaNIdx {
			lastNonNaNIdx = j
		}
	}
	if lastNonNaNIdx == -1 {
		// All timeseries contain only NaNs.
		return
	}

	// Substitute last three values starting from lastNonNaNIdx
	// with the previous values for each timeseries.
	for i := range tss {
		r := &tss[i]
		for j := 0; j < 3; j++ {
			idx := lastNonNaNIdx + j
			if idx <= 0 || idx >= len(r.Values) {
				continue
			}
			r.Values[idx] = r.Values[idx-1]
		}
	}
}

func getTime(r *http.Request, argKey string, defaultValue int64) int64 {
	argValue := r.FormValue(argKey)
	if len(argValue) == 0 {
		return defaultValue
	}
	secs, err := strconv.ParseFloat(argValue, 64)
	if err != nil {
		// Try parsing string format
		t, err := time.Parse(time.RFC3339, argValue)
		if err != nil {
			return defaultValue
		}
		secs = float64(t.UnixNano()) / 1e9
	}
	msecs := int64(secs * 1e3)
	if msecs < minTimeMsecs || msecs > maxTimeMsecs {
		return defaultValue
	}
	return msecs
}

const (
	// These values prevent from overflow when storing msec-precision time in int64.
	minTimeMsecs = int64(-1<<63) / 1e6
	maxTimeMsecs = int64(1<<63-1) / 1e6
)

func getDuration(r *http.Request, argKey string, defaultValue int64) int64 {
	argValue := r.FormValue(argKey)
	if len(argValue) == 0 {
		return defaultValue
	}
	secs, err := strconv.ParseFloat(argValue, 64)
	if err != nil {
		// Try parsing string format
		d, err := time.ParseDuration(argValue)
		if err != nil {
			return defaultValue
		}
		secs = d.Seconds()
	}
	msecs := int64(secs * 1e3)
	if msecs <= 0 || msecs > maxDurationMsecs {
		return defaultValue
	}
	return msecs
}

const maxDurationMsecs = 100 * 365 * 24 * 3600 * 1000

func getDeadline(r *http.Request) netstorage.Deadline {
	d := getDuration(r, "timeout", 0)
	dMax := int64(maxQueryDuration.Seconds() * 1e3)
	if d <= 0 || d > dMax {
		d = dMax
	}
	timeout := time.Duration(d) * time.Millisecond
	return netstorage.NewDeadline(timeout)
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
	return int64(time.Now().UTC().Unix()) * 1e3
}

func getTagFilterssFromMatches(matches []string) ([][]storage.TagFilter, error) {
	tagFilterss := make([][]storage.TagFilter, 0, len(matches))
	for _, match := range matches {
		tagFilters, err := promql.ParseMetricSelector(match)
		if err != nil {
			return nil, fmt.Errorf("cannot parse %q: %s", match, err)
		}
		tagFilterss = append(tagFilterss, tagFilters)
	}
	return tagFilterss, nil
}
