package vmselect

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/graphite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/metrics"
)

var (
	deleteAuthKey         = flag.String("deleteAuthKey", "", "authKey for metrics' deletion via /api/v1/admin/tsdb/delete_series and /tags/delSeries")
	maxConcurrentRequests = flag.Int("search.maxConcurrentRequests", getDefaultMaxConcurrentRequests(), "The maximum number of concurrent search requests. "+
		"It shouldn't be high, since a single request can saturate all the CPU cores. See also -search.maxQueueDuration")
	maxQueueDuration = flag.Duration("search.maxQueueDuration", 10*time.Second, "The maximum time the request waits for execution when -search.maxConcurrentRequests "+
		"limit is reached; see also -search.maxQueryDuration")
	resetCacheAuthKey = flag.String("search.resetCacheAuthKey", "", "Optional authKey for resetting rollup cache via /internal/resetRollupResultCache call")
)

func getDefaultMaxConcurrentRequests() int {
	n := cgroup.AvailableCPUs()
	if n <= 4 {
		n *= 2
	}
	if n > 16 {
		// A single request can saturate all the CPU cores, so there is no sense
		// in allowing higher number of concurrent requests - they will just contend
		// for unavailable CPU time.
		n = 16
	}
	return n
}

// Init initializes vmselect
func Init() {
	tmpDirPath := *vmstorage.DataPath + "/tmp"
	fs.RemoveDirContents(tmpDirPath)
	netstorage.InitTmpBlocksDir(tmpDirPath)
	promql.InitRollupResultCache(*vmstorage.DataPath + "/cache/rollupResult")

	concurrencyCh = make(chan struct{}, *maxConcurrentRequests)
}

// Stop stops vmselect
func Stop() {
	promql.StopRollupResultCache()
}

var concurrencyCh chan struct{}

var (
	concurrencyLimitReached = metrics.NewCounter(`vm_concurrent_select_limit_reached_total`)
	concurrencyLimitTimeout = metrics.NewCounter(`vm_concurrent_select_limit_timeout_total`)

	_ = metrics.NewGauge(`vm_concurrent_select_capacity`, func() float64 {
		return float64(cap(concurrencyCh))
	})
	_ = metrics.NewGauge(`vm_concurrent_select_current`, func() float64 {
		return float64(len(concurrencyCh))
	})
)

// RequestHandler handles remote read API requests for Prometheus
func RequestHandler(w http.ResponseWriter, r *http.Request) bool {
	startTime := time.Now()
	// Limit the number of concurrent queries.
	select {
	case concurrencyCh <- struct{}{}:
		defer func() { <-concurrencyCh }()
	default:
		// Sleep for a while until giving up. This should resolve short bursts in requests.
		concurrencyLimitReached.Inc()
		d := searchutils.GetMaxQueryDuration(r)
		if d > *maxQueueDuration {
			d = *maxQueueDuration
		}
		t := timerpool.Get(d)
		select {
		case concurrencyCh <- struct{}{}:
			timerpool.Put(t)
			defer func() { <-concurrencyCh }()
		case <-t.C:
			timerpool.Put(t)
			concurrencyLimitTimeout.Inc()
			err := &httpserver.ErrorWithStatusCode{
				Err: fmt.Errorf("cannot handle more than %d concurrent search requests during %s; possible solutions: "+
					"increase `-search.maxQueueDuration`; increase `-search.maxQueryDuration`; increase `-search.maxConcurrentRequests`; "+
					"increase server capacity",
					*maxConcurrentRequests, d),
				StatusCode: http.StatusServiceUnavailable,
			}
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
	}

	path := strings.Replace(r.URL.Path, "//", "/", -1)
	if path == "/internal/resetRollupResultCache" {
		if len(*resetCacheAuthKey) > 0 && r.FormValue("authKey") != *resetCacheAuthKey {
			sendPrometheusError(w, r, fmt.Errorf("invalid authKey=%q for %q", r.FormValue("authKey"), path))
			return true
		}
		promql.ResetRollupResultCache()
		return true
	}

	if strings.HasPrefix(path, "/api/v1/label/") {
		s := r.URL.Path[len("/api/v1/label/"):]
		if strings.HasSuffix(s, "/values") {
			labelValuesRequests.Inc()
			labelName := s[:len(s)-len("/values")]
			httpserver.EnableCORS(w, r)
			if err := prometheus.LabelValuesHandler(startTime, labelName, w, r); err != nil {
				labelValuesErrors.Inc()
				sendPrometheusError(w, r, err)
				return true
			}
			return true
		}
	}
	if strings.HasPrefix(path, "/tags/") && !isGraphiteTagsPath(path) {
		tagName := r.URL.Path[len("/tags/"):]
		graphiteTagValuesRequests.Inc()
		if err := graphite.TagValuesHandler(startTime, tagName, w, r); err != nil {
			graphiteTagValuesErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	}

	switch path {
	case "/api/v1/query":
		queryRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.QueryHandler(startTime, w, r); err != nil {
			queryErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/query_range":
		queryRangeRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.QueryRangeHandler(startTime, w, r); err != nil {
			queryRangeErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/series":
		seriesRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.SeriesHandler(startTime, w, r); err != nil {
			seriesErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/series/count":
		seriesCountRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.SeriesCountHandler(startTime, w, r); err != nil {
			seriesCountErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/labels":
		labelsRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.LabelsHandler(startTime, w, r); err != nil {
			labelsErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/labels/count":
		labelsCountRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.LabelsCountHandler(startTime, w, r); err != nil {
			labelsCountErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/status/top_queries":
		topQueriesRequests.Inc()
		if err := prometheus.QueryStatsHandler(startTime, w, r); err != nil {
			topQueriesErrors.Inc()
			sendPrometheusError(w, r, fmt.Errorf("cannot query status endpoint: %w", err))
			return true
		}
		return true
	case "/api/v1/status/tsdb":
		statusTSDBRequests.Inc()
		if err := prometheus.TSDBStatusHandler(startTime, w, r); err != nil {
			statusTSDBErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/status/active_queries":
		statusActiveQueriesRequests.Inc()
		promql.WriteActiveQueries(w)
		return true
	case "/api/v1/export":
		exportRequests.Inc()
		if err := prometheus.ExportHandler(startTime, w, r); err != nil {
			exportErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/api/v1/export/csv":
		exportCSVRequests.Inc()
		if err := prometheus.ExportCSVHandler(startTime, w, r); err != nil {
			exportCSVErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/api/v1/export/native":
		exportNativeRequests.Inc()
		if err := prometheus.ExportNativeHandler(startTime, w, r); err != nil {
			exportNativeErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/federate":
		federateRequests.Inc()
		if err := prometheus.FederateHandler(startTime, w, r); err != nil {
			federateErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/metrics/find", "/metrics/find/":
		graphiteMetricsFindRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := graphite.MetricsFindHandler(startTime, w, r); err != nil {
			graphiteMetricsFindErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/metrics/expand", "/metrics/expand/":
		graphiteMetricsExpandRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := graphite.MetricsExpandHandler(startTime, w, r); err != nil {
			graphiteMetricsExpandErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/metrics/index.json", "/metrics/index.json/":
		graphiteMetricsIndexRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := graphite.MetricsIndexHandler(startTime, w, r); err != nil {
			graphiteMetricsIndexErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/tags/tagSeries":
		graphiteTagsTagSeriesRequests.Inc()
		if err := graphite.TagsTagSeriesHandler(startTime, w, r); err != nil {
			graphiteTagsTagSeriesErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/tags/tagMultiSeries":
		graphiteTagsTagMultiSeriesRequests.Inc()
		if err := graphite.TagsTagMultiSeriesHandler(startTime, w, r); err != nil {
			graphiteTagsTagMultiSeriesErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/tags":
		graphiteTagsRequests.Inc()
		if err := graphite.TagsHandler(startTime, w, r); err != nil {
			graphiteTagsErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/tags/findSeries":
		graphiteTagsFindSeriesRequests.Inc()
		if err := graphite.TagsFindSeriesHandler(startTime, w, r); err != nil {
			graphiteTagsFindSeriesErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/tags/autoComplete/tags":
		graphiteTagsAutoCompleteTagsRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := graphite.TagsAutoCompleteTagsHandler(startTime, w, r); err != nil {
			graphiteTagsAutoCompleteTagsErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/tags/autoComplete/values":
		graphiteTagsAutoCompleteValuesRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := graphite.TagsAutoCompleteValuesHandler(startTime, w, r); err != nil {
			graphiteTagsAutoCompleteValuesErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/tags/delSeries":
		graphiteTagsDelSeriesRequests.Inc()
		authKey := r.FormValue("authKey")
		if authKey != *deleteAuthKey {
			httpserver.Errorf(w, r, "invalid authKey %q. It must match the value from -deleteAuthKey command line flag", authKey)
			return true
		}
		if err := graphite.TagsDelSeriesHandler(startTime, w, r); err != nil {
			graphiteTagsDelSeriesErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/api/v1/rules":
		// Return dumb placeholder
		rulesRequests.Inc()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprintf(w, "%s", `{"status":"success","data":{"groups":[]}}`)
		return true
	case "/api/v1/alerts":
		// Return dumb placehloder
		alertsRequests.Inc()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprintf(w, "%s", `{"status":"success","data":{"alerts":[]}}`)
		return true
	case "/api/v1/metadata":
		// Return dumb placeholder
		metadataRequests.Inc()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprintf(w, "%s", `{"status":"success","data":{}}`)
		return true
	case "/api/v1/admin/tsdb/delete_series":
		deleteRequests.Inc()
		authKey := r.FormValue("authKey")
		if authKey != *deleteAuthKey {
			httpserver.Errorf(w, r, "invalid authKey %q. It must match the value from -deleteAuthKey command line flag", authKey)
			return true
		}
		if err := prometheus.DeleteHandler(startTime, r); err != nil {
			deleteErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	default:
		return false
	}
}

func isGraphiteTagsPath(path string) bool {
	switch path {
	// See https://graphite.readthedocs.io/en/stable/tags.html for a list of Graphite Tags API paths.
	// Do not include `/tags/<tag_name>` here, since this will fool the caller.
	case "/tags/tagSeries", "/tags/tagMultiSeries", "/tags/findSeries",
		"/tags/autoComplete/tags", "/tags/autoComplete/values", "/tags/delSeries":
		return true
	default:
		return false
	}
}

func sendPrometheusError(w http.ResponseWriter, r *http.Request, err error) {
	logger.Warnf("error in %q: %s", r.RequestURI, err)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	statusCode := http.StatusUnprocessableEntity
	var esc *httpserver.ErrorWithStatusCode
	if errors.As(err, &esc) {
		statusCode = esc.StatusCode
	}
	w.WriteHeader(statusCode)
	prometheus.WriteErrorResponse(w, statusCode, err)
}

var (
	labelValuesRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/label/{}/values"}`)
	labelValuesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/label/{}/values"}`)

	queryRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/query"}`)
	queryErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/query"}`)

	queryRangeRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/query_range"}`)
	queryRangeErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/query_range"}`)

	seriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/series"}`)
	seriesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/series"}`)

	seriesCountRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/series/count"}`)
	seriesCountErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/series/count"}`)

	labelsRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/labels"}`)
	labelsErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/labels"}`)

	labelsCountRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/labels/count"}`)
	labelsCountErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/labels/count"}`)

	topQueriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/status/top_queries"}`)
	topQueriesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/status/top_queries"}`)

	statusTSDBRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/status/tsdb"}`)
	statusTSDBErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/status/tsdb"}`)

	statusActiveQueriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/status/active_queries"}`)

	deleteRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/admin/tsdb/delete_series"}`)
	deleteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/admin/tsdb/delete_series"}`)

	exportRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/export"}`)
	exportErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/export"}`)

	exportCSVRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/export/csv"}`)
	exportCSVErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/export/csv"}`)

	exportNativeRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/export/native"}`)
	exportNativeErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/export/native"}`)

	federateRequests = metrics.NewCounter(`vm_http_requests_total{path="/federate"}`)
	federateErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/federate"}`)

	graphiteMetricsFindRequests = metrics.NewCounter(`vm_http_requests_total{path="/metrics/find"}`)
	graphiteMetricsFindErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/metrics/find"}`)

	graphiteMetricsExpandRequests = metrics.NewCounter(`vm_http_requests_total{path="/metrics/expand"}`)
	graphiteMetricsExpandErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/metrics/expand"}`)

	graphiteMetricsIndexRequests = metrics.NewCounter(`vm_http_requests_total{path="/metrics/index.json"}`)
	graphiteMetricsIndexErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/metrics/index.json"}`)

	graphiteTagsTagSeriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/tags/tagSeries"}`)
	graphiteTagsTagSeriesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/tags/tagSeries"}`)

	graphiteTagsTagMultiSeriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/tags/tagMultiSeries"}`)
	graphiteTagsTagMultiSeriesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/tags/tagMultiSeries"}`)

	graphiteTagsRequests = metrics.NewCounter(`vm_http_requests_total{path="/tags"}`)
	graphiteTagsErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/tags"}`)

	graphiteTagValuesRequests = metrics.NewCounter(`vm_http_requests_total{path="/tags/<tag_name>"}`)
	graphiteTagValuesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/tags/<tag_name>"}`)

	graphiteTagsFindSeriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/tags/findSeries"}`)
	graphiteTagsFindSeriesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/tags/findSeries"}`)

	graphiteTagsAutoCompleteTagsRequests = metrics.NewCounter(`vm_http_requests_total{path="/tags/autoComplete/tags"}`)
	graphiteTagsAutoCompleteTagsErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/tags/autoComplete/tags"}`)

	graphiteTagsAutoCompleteValuesRequests = metrics.NewCounter(`vm_http_requests_total{path="/tags/autoComplete/values"}`)
	graphiteTagsAutoCompleteValuesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/tags/autoComplete/values"}`)

	graphiteTagsDelSeriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/tags/delSeries"}`)
	graphiteTagsDelSeriesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/tags/delSeries"}`)

	rulesRequests    = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/rules"}`)
	alertsRequests   = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/alerts"}`)
	metadataRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/metadata"}`)
)
