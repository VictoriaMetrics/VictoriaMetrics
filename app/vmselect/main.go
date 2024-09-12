package vmselect

import (
	"embed"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/graphite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/metrics"
)

var (
	deleteAuthKey         = flagutil.NewPassword("deleteAuthKey", "authKey for metrics' deletion via /api/v1/admin/tsdb/delete_series and /tags/delSeries. It overrides -httpAuth.*")
	maxConcurrentRequests = flag.Int("search.maxConcurrentRequests", getDefaultMaxConcurrentRequests(), "The maximum number of concurrent search requests. "+
		"It shouldn't be high, since a single request can saturate all the CPU cores, while many concurrently executed requests may require high amounts of memory. "+
		"See also -search.maxQueueDuration and -search.maxMemoryPerQuery")
	maxQueueDuration = flag.Duration("search.maxQueueDuration", 10*time.Second, "The maximum time the request waits for execution when -search.maxConcurrentRequests "+
		"limit is reached; see also -search.maxQueryDuration")
	resetCacheAuthKey    = flagutil.NewPassword("search.resetCacheAuthKey", "Optional authKey for resetting rollup cache via /internal/resetRollupResultCache call. It overrides -httpAuth.*")
	logSlowQueryDuration = flag.Duration("search.logSlowQueryDuration", 5*time.Second, "Log queries with execution time exceeding this value. Zero disables slow query logging. "+
		"See also -search.logQueryMemoryUsage")
	vmalertProxyURL = flag.String("vmalert.proxyURL", "", "Optional URL for proxying requests to vmalert. For example, if -vmalert.proxyURL=http://vmalert:8880 , then alerting API requests such as /api/v1/rules from Grafana will be proxied to http://vmalert:8880/api/v1/rules")
)

var slowQueries = metrics.NewCounter(`vm_slow_queries_total`)

func getDefaultMaxConcurrentRequests() int {
	n := cgroup.AvailableCPUs() * 2
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

	concurrencyLimitCh = make(chan struct{}, *maxConcurrentRequests)
	initVMAlertProxy()
}

// Stop stops vmselect
func Stop() {
	promql.StopRollupResultCache()
}

var concurrencyLimitCh chan struct{}

var (
	concurrencyLimitReached = metrics.NewCounter(`vm_concurrent_select_limit_reached_total`)
	concurrencyLimitTimeout = metrics.NewCounter(`vm_concurrent_select_limit_timeout_total`)

	_ = metrics.NewGauge(`vm_concurrent_select_capacity`, func() float64 {
		return float64(cap(concurrencyLimitCh))
	})
	_ = metrics.NewGauge(`vm_concurrent_select_current`, func() float64 {
		return float64(len(concurrencyLimitCh))
	})
)

//go:embed vmui
var vmuiFiles embed.FS

var vmuiFileServer = http.FileServer(http.FS(vmuiFiles))

// RequestHandler handles remote read API requests
func RequestHandler(w http.ResponseWriter, r *http.Request) bool {
	path := strings.Replace(r.URL.Path, "//", "/", -1)

	// Strip /prometheus and /graphite prefixes in order to provide path compatibility with cluster version
	//
	// See https://docs.victoriametrics.com/cluster-victoriametrics/#url-format
	switch {
	case strings.HasPrefix(path, "/prometheus/"):
		path = path[len("/prometheus"):]
	case strings.HasPrefix(path, "/graphite/"):
		path = path[len("/graphite"):]
	}

	if handleStaticAndSimpleRequests(w, r, path) {
		return true
	}

	// Handle non-trivial dynamic requests, which may take big amounts of time and resources.
	startTime := time.Now()
	defer requestDuration.UpdateDuration(startTime)
	tracerEnabled := httputils.GetBool(r, "trace")
	qt := querytracer.New(tracerEnabled, "%s", r.URL.Path)

	// Limit the number of concurrent queries.
	select {
	case concurrencyLimitCh <- struct{}{}:
		defer func() { <-concurrencyLimitCh }()
	default:
		// Sleep for a while until giving up. This should resolve short bursts in requests.
		concurrencyLimitReached.Inc()
		d := searchutils.GetMaxQueryDuration(r)
		if d > *maxQueueDuration {
			d = *maxQueueDuration
		}
		t := timerpool.Get(d)
		select {
		case concurrencyLimitCh <- struct{}{}:
			timerpool.Put(t)
			qt.Printf("wait in queue because -search.maxConcurrentRequests=%d concurrent requests are executed", *maxConcurrentRequests)
			defer func() { <-concurrencyLimitCh }()
		case <-r.Context().Done():
			timerpool.Put(t)
			remoteAddr := httpserver.GetQuotedRemoteAddr(r)
			requestURI := httpserver.GetRequestURI(r)
			logger.Infof("client has canceled the request after %.3f seconds: remoteAddr=%s, requestURI: %q",
				time.Since(startTime).Seconds(), remoteAddr, requestURI)
			return true
		case <-t.C:
			timerpool.Put(t)
			concurrencyLimitTimeout.Inc()
			err := &httpserver.ErrorWithStatusCode{
				Err: fmt.Errorf("couldn't start executing the request in %.3f seconds, since -search.maxConcurrentRequests=%d concurrent requests "+
					"are executed. Possible solutions: to reduce query load; to add more compute resources to the server; "+
					"to increase -search.maxQueueDuration=%s; to increase -search.maxQueryDuration; to increase -search.maxConcurrentRequests",
					d.Seconds(), *maxConcurrentRequests, maxQueueDuration),
				StatusCode: http.StatusTooManyRequests,
			}
			w.Header().Add("Retry-After", "10")
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
	}

	if *logSlowQueryDuration > 0 {
		actualStartTime := time.Now()
		defer func() {
			d := time.Since(actualStartTime)
			if d >= *logSlowQueryDuration {
				remoteAddr := httpserver.GetQuotedRemoteAddr(r)
				requestURI := httpserver.GetRequestURI(r)
				logger.Warnf("slow query according to -search.logSlowQueryDuration=%s: remoteAddr=%s, duration=%.3f seconds; requestURI: %q",
					*logSlowQueryDuration, remoteAddr, d.Seconds(), requestURI)
				slowQueries.Inc()
			}
		}()
	}

	if path == "/internal/resetRollupResultCache" {
		if !httpserver.CheckAuthFlag(w, r, resetCacheAuthKey) {
			return true
		}
		promql.ResetRollupResultCache()
		return true
	}

	if strings.HasPrefix(path, "/api/v1/label/") {
		s := path[len("/api/v1/label/"):]
		if strings.HasSuffix(s, "/values") {
			labelValuesRequests.Inc()
			labelName := s[:len(s)-len("/values")]
			httpserver.EnableCORS(w, r)
			if err := prometheus.LabelValuesHandler(qt, startTime, labelName, w, r); err != nil {
				labelValuesErrors.Inc()
				httpserver.SendPrometheusError(w, r, err)
				return true
			}
			return true
		}
	}
	if strings.HasPrefix(path, "/tags/") && !isGraphiteTagsPath(path) {
		tagName := path[len("/tags/"):]
		graphiteTagValuesRequests.Inc()
		if err := graphite.TagValuesHandler(startTime, tagName, w, r); err != nil {
			graphiteTagValuesErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	}

	switch path {
	case "/api/v1/query":
		queryRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.QueryHandler(qt, startTime, w, r); err != nil {
			queryErrors.Inc()
			httpserver.SendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/query_range":
		queryRangeRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.QueryRangeHandler(qt, startTime, w, r); err != nil {
			queryRangeErrors.Inc()
			httpserver.SendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/series":
		seriesRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.SeriesHandler(qt, startTime, w, r); err != nil {
			seriesErrors.Inc()
			httpserver.SendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/series/count":
		seriesCountRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.SeriesCountHandler(startTime, w, r); err != nil {
			seriesCountErrors.Inc()
			httpserver.SendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/labels":
		labelsRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.LabelsHandler(qt, startTime, w, r); err != nil {
			labelsErrors.Inc()
			httpserver.SendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/status/tsdb":
		statusTSDBRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.TSDBStatusHandler(qt, startTime, w, r); err != nil {
			statusTSDBErrors.Inc()
			httpserver.SendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/export":
		exportRequests.Inc()
		if err := prometheus.ExportHandler(startTime, w, r); err != nil {
			exportErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "/api/v1/export/csv":
		exportCSVRequests.Inc()
		if err := prometheus.ExportCSVHandler(startTime, w, r); err != nil {
			exportCSVErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "/api/v1/export/native":
		exportNativeRequests.Inc()
		if err := prometheus.ExportNativeHandler(startTime, w, r); err != nil {
			exportNativeErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "/federate":
		federateRequests.Inc()
		if err := prometheus.FederateHandler(startTime, w, r); err != nil {
			federateErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "/metrics/find", "/metrics/find/":
		graphiteMetricsFindRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := graphite.MetricsFindHandler(startTime, w, r); err != nil {
			graphiteMetricsFindErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "/metrics/expand", "/metrics/expand/":
		graphiteMetricsExpandRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := graphite.MetricsExpandHandler(startTime, w, r); err != nil {
			graphiteMetricsExpandErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "/metrics/index.json", "/metrics/index.json/":
		graphiteMetricsIndexRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := graphite.MetricsIndexHandler(startTime, w, r); err != nil {
			graphiteMetricsIndexErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "/tags/tagSeries":
		graphiteTagsTagSeriesRequests.Inc()
		if err := graphite.TagsTagSeriesHandler(startTime, w, r); err != nil {
			graphiteTagsTagSeriesErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "/tags/tagMultiSeries":
		graphiteTagsTagMultiSeriesRequests.Inc()
		if err := graphite.TagsTagMultiSeriesHandler(startTime, w, r); err != nil {
			graphiteTagsTagMultiSeriesErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "/tags":
		graphiteTagsRequests.Inc()
		if err := graphite.TagsHandler(startTime, w, r); err != nil {
			graphiteTagsErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "/tags/findSeries":
		graphiteTagsFindSeriesRequests.Inc()
		if err := graphite.TagsFindSeriesHandler(startTime, w, r); err != nil {
			graphiteTagsFindSeriesErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "/tags/autoComplete/tags":
		graphiteTagsAutoCompleteTagsRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := graphite.TagsAutoCompleteTagsHandler(startTime, w, r); err != nil {
			graphiteTagsAutoCompleteTagsErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "/tags/autoComplete/values":
		graphiteTagsAutoCompleteValuesRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := graphite.TagsAutoCompleteValuesHandler(startTime, w, r); err != nil {
			graphiteTagsAutoCompleteValuesErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "/tags/delSeries":
		if !httpserver.CheckAuthFlag(w, r, deleteAuthKey) {
			return true
		}
		graphiteTagsDelSeriesRequests.Inc()
		if err := graphite.TagsDelSeriesHandler(startTime, w, r); err != nil {
			graphiteTagsDelSeriesErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "/render":
		graphiteRenderRequests.Inc()
		if err := graphite.RenderHandler(startTime, w, r); err != nil {
			graphiteRenderErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/api/v1/admin/tsdb/delete_series":
		if !httpserver.CheckAuthFlag(w, r, deleteAuthKey) {
			return true
		}
		deleteRequests.Inc()
		if err := prometheus.DeleteHandler(startTime, r); err != nil {
			deleteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	default:
		return false
	}
}

func handleStaticAndSimpleRequests(w http.ResponseWriter, r *http.Request, path string) bool {
	// vmui access.
	if path == "/vmui" || path == "/graph" {
		// VMUI access via incomplete url without `/` in the end. Redirect to complete url.
		// Use relative redirect, since the hostname and path prefix may be incorrect if VictoriaMetrics
		// is hidden behind vmauth or similar proxy.
		_ = r.ParseForm()
		path = strings.TrimPrefix(path, "/")
		newURL := path + "/?" + r.Form.Encode()
		httpserver.Redirect(w, newURL)
		return true
	}
	if strings.HasPrefix(path, "/graph/") {
		// This is needed for serving /graph URLs from Prometheus datasource in Grafana.
		path = strings.Replace(path, "/graph/", "/vmui/", 1)
	}
	if path == "/vmui/custom-dashboards" {
		if err := handleVMUICustomDashboards(w); err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	}
	if path == "/vmui/timezone" {
		httpserver.EnableCORS(w, r)
		if err := handleVMUITimezone(w); err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	}
	if strings.HasPrefix(path, "/vmui/") {
		if strings.HasPrefix(path, "/vmui/static/") {
			// Allow clients caching static contents for long period of time, since it shouldn't change over time.
			// Path to static contents (such as js and css) must be changed whenever its contents is changed.
			// See https://developer.chrome.com/docs/lighthouse/performance/uses-long-cache-ttl/
			w.Header().Set("Cache-Control", "max-age=31536000")
		}
		r.URL.Path = path
		vmuiFileServer.ServeHTTP(w, r)
		return true
	}

	if strings.HasPrefix(path, "/functions") {
		funcName := path[len("/functions"):]
		funcName = strings.TrimPrefix(funcName, "/")
		if funcName == "" {
			graphiteFunctionsRequests.Inc()
			if err := graphite.FunctionsHandler(w, r); err != nil {
				graphiteFunctionsErrors.Inc()
				httpserver.Errorf(w, r, "%s", err)
				return true
			}
			return true
		}
		graphiteFunctionDetailsRequests.Inc()
		if err := graphite.FunctionDetailsHandler(funcName, w, r); err != nil {
			graphiteFunctionDetailsErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	}

	if path == "/vmalert" {
		// vmalert access via incomplete url without `/` in the end. Redirect to complete url.
		// Use relative redirect, since the hostname and path prefix may be incorrect if VictoriaMetrics
		// is hidden behind vmauth or similar proxy.
		httpserver.Redirect(w, "vmalert/")
		return true
	}
	if strings.HasPrefix(path, "/vmalert/") {
		vmalertRequests.Inc()
		if len(*vmalertProxyURL) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, "%s", `{"status":"error","msg":"for accessing vmalert flag '-vmalert.proxyURL' must be configured"}`)
			return true
		}
		proxyVMAlertRequests(w, r)
		return true
	}

	switch path {
	case "/api/v1/status/active_queries":
		statusActiveQueriesRequests.Inc()
		httpserver.EnableCORS(w, r)
		promql.ActiveQueriesHandler(w, r)
		return true
	case "/api/v1/status/top_queries":
		topQueriesRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.QueryStatsHandler(w, r); err != nil {
			topQueriesErrors.Inc()
			httpserver.SendPrometheusError(w, r, fmt.Errorf("cannot query status endpoint: %w", err))
			return true
		}
		return true
	case "/metric-relabel-debug":
		promscrapeMetricRelabelDebugRequests.Inc()
		promscrape.WriteMetricRelabelDebug(w, r)
		return true
	case "/target-relabel-debug":
		promscrapeTargetRelabelDebugRequests.Inc()
		promscrape.WriteTargetRelabelDebug(w, r)
		return true
	case "/expand-with-exprs":
		expandWithExprsRequests.Inc()
		prometheus.ExpandWithExprs(w, r)
		return true
	case "/prettify-query":
		prettifyQueryRequests.Inc()
		prometheus.PrettifyQuery(w, r)
		return true
	case "/api/v1/rules", "/rules":
		rulesRequests.Inc()
		if len(*vmalertProxyURL) > 0 {
			proxyVMAlertRequests(w, r)
			return true
		}
		// Return dumb placeholder for https://prometheus.io/docs/prometheus/latest/querying/api/#rules
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"groups":[]}}`)
		return true
	case "/api/v1/alerts", "/alerts":
		alertsRequests.Inc()
		if len(*vmalertProxyURL) > 0 {
			proxyVMAlertRequests(w, r)
			return true
		}
		// Return dumb placeholder for https://prometheus.io/docs/prometheus/latest/querying/api/#alerts
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"alerts":[]}}`)
		return true
	case "/api/v1/metadata":
		// Return dumb placeholder for https://prometheus.io/docs/prometheus/latest/querying/api/#querying-metric-metadata
		metadataRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%s", `{"status":"success","data":{}}`)
		return true
	case "/api/v1/status/buildinfo":
		buildInfoRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		// prometheus version is used here, which affects what API Grafana uses when retrieving label values.
		// as new Grafana features are added that are customized for the Prometheus version, maybe the version will need to be increased.
		// see this issue for more info: https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5370
		fmt.Fprintf(w, "%s", `{"status":"success","data":{"version":"2.24.0"}}`)
		return true
	case "/api/v1/query_exemplars":
		// Return dumb placeholder for https://prometheus.io/docs/prometheus/latest/querying/api/#querying-exemplars
		queryExemplarsRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%s", `{"status":"success","data":[]}`)
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

var (
	requestDuration = metrics.NewHistogram(`vmselect_request_duration_seconds`)

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

	statusTSDBRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/status/tsdb"}`)
	statusTSDBErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/status/tsdb"}`)

	statusActiveQueriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/status/active_queries"}`)

	topQueriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/status/top_queries"}`)
	topQueriesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/status/top_queries"}`)

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

	graphiteRenderRequests = metrics.NewCounter(`vm_http_requests_total{path="/render"}`)
	graphiteRenderErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/render"}`)

	promscrapeMetricRelabelDebugRequests = metrics.NewCounter(`vm_http_requests_total{path="/metric-relabel-debug"}`)
	promscrapeTargetRelabelDebugRequests = metrics.NewCounter(`vm_http_requests_total{path="/target-relabel-debug"}`)

	graphiteFunctionsRequests = metrics.NewCounter(`vm_http_requests_total{path="/functions"}`)
	graphiteFunctionsErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/functions"}`)

	graphiteFunctionDetailsRequests = metrics.NewCounter(`vm_http_requests_total{path="/functions/<func_name>"}`)
	graphiteFunctionDetailsErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/functions/<func_name>"}`)

	expandWithExprsRequests = metrics.NewCounter(`vm_http_requests_total{path="/expand-with-exprs"}`)
	prettifyQueryRequests   = metrics.NewCounter(`vm_http_requests_total{path="/prettify-query"}`)

	vmalertRequests = metrics.NewCounter(`vm_http_requests_total{path="/vmalert"}`)
	rulesRequests   = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/rules"}`)
	alertsRequests  = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/alerts"}`)

	metadataRequests       = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/metadata"}`)
	buildInfoRequests      = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/buildinfo"}`)
	queryExemplarsRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/query_exemplars"}`)
)

func proxyVMAlertRequests(w http.ResponseWriter, r *http.Request) {
	defer func() {
		err := recover()
		if err == nil || err == http.ErrAbortHandler {
			// Suppress http.ErrAbortHandler panic.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1353
			return
		}
		// Forward other panics to the caller.
		panic(err)
	}()
	r.Host = vmalertProxyHost
	vmalertProxy.ServeHTTP(w, r)
}

var (
	vmalertProxyHost string
	vmalertProxy     *httputil.ReverseProxy
)

// initVMAlertProxy must be called after flag.Parse(), since it uses command-line flags.
func initVMAlertProxy() {
	if len(*vmalertProxyURL) == 0 {
		return
	}
	proxyURL, err := url.Parse(*vmalertProxyURL)
	if err != nil {
		logger.Fatalf("cannot parse -vmalert.proxyURL=%q: %s", *vmalertProxyURL, err)
	}
	vmalertProxyHost = proxyURL.Host
	vmalertProxy = httputil.NewSingleHostReverseProxy(proxyURL)
}
