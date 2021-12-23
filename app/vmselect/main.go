package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/graphite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/metrics"
)

var (
	httpListenAddr        = flag.String("httpListenAddr", ":8481", "Address to listen for http connections")
	cacheDataPath         = flag.String("cacheDataPath", "", "Path to directory for cache files. Cache isn't saved if empty")
	maxConcurrentRequests = flag.Int("search.maxConcurrentRequests", getDefaultMaxConcurrentRequests(), "The maximum number of concurrent search requests. "+
		"It shouldn't be high, since a single request can saturate all the CPU cores. See also -search.maxQueueDuration")
	maxQueueDuration = flag.Duration("search.maxQueueDuration", 10*time.Second, "The maximum time the request waits for execution when -search.maxConcurrentRequests "+
		"limit is reached; see also -search.maxQueryDuration")
	minScrapeInterval = flag.Duration("dedup.minScrapeInterval", 0, "Leave only the first sample in every time series per each discrete interval "+
		"equal to -dedup.minScrapeInterval > 0. See https://docs.victoriametrics.com/#deduplication for details")
	resetCacheAuthKey    = flag.String("search.resetCacheAuthKey", "", "Optional authKey for resetting rollup cache via /internal/resetRollupResultCache call")
	logSlowQueryDuration = flag.Duration("search.logSlowQueryDuration", 5*time.Second, "Log queries with execution time exceeding this value. Zero disables slow query logging")
	storageNodes         = flagutil.NewArray("storageNode", "Comma-separated addresses of vmstorage nodes; usage: -storageNode=vmstorage-host1,...,vmstorage-hostN")
)

var slowQueries = metrics.NewCounter(`vm_slow_queries_total`)

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

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	buildinfo.Init()
	logger.Init()

	logger.Infof("starting netstorage at storageNodes %s", *storageNodes)
	startTime := time.Now()
	storage.SetDedupInterval(*minScrapeInterval)
	if len(*storageNodes) == 0 {
		logger.Fatalf("missing -storageNode arg")
	}
	netstorage.InitStorageNodes(*storageNodes)
	logger.Infof("started netstorage in %.3f seconds", time.Since(startTime).Seconds())

	if len(*cacheDataPath) > 0 {
		tmpDataPath := *cacheDataPath + "/tmp"
		fs.RemoveDirContents(tmpDataPath)
		netstorage.InitTmpBlocksDir(tmpDataPath)
		promql.InitRollupResultCache(*cacheDataPath + "/rollupResult")
	} else {
		netstorage.InitTmpBlocksDir("")
		promql.InitRollupResultCache("")
	}
	concurrencyCh = make(chan struct{}, *maxConcurrentRequests)

	go func() {
		httpserver.Serve(*httpListenAddr, requestHandler)
	}()

	sig := procutil.WaitForSigterm()
	logger.Infof("service received signal %s", sig)

	logger.Infof("gracefully shutting down http service at %q", *httpListenAddr)
	startTime = time.Now()
	if err := httpserver.Stop(*httpListenAddr); err != nil {
		logger.Fatalf("cannot stop http service: %s", err)
	}
	logger.Infof("successfully shut down http service in %.3f seconds", time.Since(startTime).Seconds())

	logger.Infof("shutting down neststorage...")
	startTime = time.Now()
	netstorage.Stop()
	if len(*cacheDataPath) > 0 {
		promql.StopRollupResultCache()
	}
	logger.Infof("successfully stopped netstorage in %.3f seconds", time.Since(startTime).Seconds())

	fs.MustStopDirRemover()

	logger.Infof("the vmselect has been stopped")
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

func requestHandler(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/" {
		if r.Method != "GET" {
			return false
		}
		fmt.Fprintf(w, `vmselect - a component of VictoriaMetrics cluster<br/>
<a href="https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html">docs</a><br>
`)
		return true
	}

	startTime := time.Now()
	defer requestDuration.UpdateDuration(startTime)

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

	path := strings.Replace(r.URL.Path, "//", "/", -1)
	if path == "/internal/resetRollupResultCache" {
		if len(*resetCacheAuthKey) > 0 && r.FormValue("authKey") != *resetCacheAuthKey {
			sendPrometheusError(w, r, fmt.Errorf("invalid authKey=%q for %q", r.FormValue("authKey"), path))
			return true
		}
		promql.ResetRollupResultCache()
		return true
	}
	if path == "/api/v1/status/top_queries" {
		globalTopQueriesRequests.Inc()
		if err := prometheus.QueryStatsHandler(startTime, nil, w, r); err != nil {
			globalTopQueriesErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	}

	p, err := httpserver.ParsePath(path)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse path %q: %s", path, err)
		return true
	}
	at, err := auth.NewToken(p.AuthToken)
	if err != nil {
		httpserver.Errorf(w, r, "auth error: %s", err)
		return true
	}
	switch p.Prefix {
	case "select":
		return selectHandler(startTime, w, r, p, at)
	case "delete":
		return deleteHandler(startTime, w, r, p, at)
	default:
		// This is not our link
		return false
	}
}

//go:embed vmui
var vmuiFiles embed.FS

var vmuiFileServer = http.FileServer(http.FS(vmuiFiles))

func selectHandler(startTime time.Time, w http.ResponseWriter, r *http.Request, p *httpserver.Path, at *auth.Token) bool {
	defer func() {
		// Count per-tenant cumulative durations and total requests
		httpRequests.Get(at).Inc()
		httpRequestsDuration.Get(at).Add(int(time.Since(startTime).Milliseconds()))
	}()
	if p.Suffix == "" {
		if r.Method != "GET" {
			return false
		}
		fmt.Fprintf(w, "<h2>VictoriaMetrics cluster - vmselect</h2></br>")
		fmt.Fprintf(w, "See <a href='https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format'>docs</a></br>")
		fmt.Fprintf(w, "Useful endpoints:</br>")
		fmt.Fprintf(w, `<a href="vmui">Web UI</a><br>`)
		fmt.Fprintf(w, `<a href="prometheus/api/v1/status/tsdb">tsdb status page</a><br>`)
		fmt.Fprintf(w, `<a href="prometheus/api/v1/status/top_queries">top queries</a><br>`)
		fmt.Fprintf(w, `<a href="prometheus/api/v1/status/active_queries">active queries</a><br>`)
		return true
	}
	if strings.HasPrefix(p.Suffix, "vmui") || strings.HasPrefix(p.Suffix, "prometheus/vmui") {
		// vmui access.
		prefix := strings.Join([]string{"", p.Prefix, p.AuthToken}, "/")
		r.URL.Path = strings.Replace(r.URL.Path, "/prometheus/vmui", "/vmui", 1)
		http.StripPrefix(prefix, vmuiFileServer).ServeHTTP(w, r)
		return true
	}
	if p.Suffix == "graph" || p.Suffix == "prometheus/graph" {
		// Redirect to /graph/, otherwise vmui redirects to /vmui/, which can be inaccessible in user env.
		// Use relative redirect, since, since the hostname and path prefix may be incorrect if VictoriaMetrics
		// is hidden behind vmauth or similar proxy.
		_ = r.ParseForm()
		newURL := "graph/?" + r.Form.Encode()
		http.Redirect(w, r, newURL, http.StatusFound)
		return true
	}
	if strings.HasPrefix(p.Suffix, "graph/") || strings.HasPrefix(p.Suffix, "prometheus/graph/") {
		// This is needed for serving /graph URLs from Prometheus datasource in Grafana.
		prefix := strings.Join([]string{"", p.Prefix, p.AuthToken}, "/")
		if strings.HasPrefix(p.Suffix, "prometheus/graph/") {
			r.URL.Path = strings.Replace(r.URL.Path, "/prometheus/graph/", "/vmui/", 1)
		} else {
			r.URL.Path = strings.Replace(r.URL.Path, "/graph/", "/vmui/", 1)
		}
		http.StripPrefix(prefix, vmuiFileServer).ServeHTTP(w, r)
		return true
	}
	if strings.HasPrefix(p.Suffix, "prometheus/api/v1/label/") {
		s := p.Suffix[len("prometheus/api/v1/label/"):]
		if strings.HasSuffix(s, "/values") {
			labelValuesRequests.Inc()
			labelName := s[:len(s)-len("/values")]
			httpserver.EnableCORS(w, r)
			if err := prometheus.LabelValuesHandler(startTime, at, labelName, w, r); err != nil {
				labelValuesErrors.Inc()
				sendPrometheusError(w, r, err)
				return true
			}
			return true
		}
	}
	if strings.HasPrefix(p.Suffix, "graphite/tags/") && !isGraphiteTagsPath(p.Suffix[len("graphite"):]) {
		tagName := p.Suffix[len("graphite/tags/"):]
		graphiteTagValuesRequests.Inc()
		if err := graphite.TagValuesHandler(startTime, at, tagName, w, r); err != nil {
			graphiteTagValuesErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	}
	if strings.HasPrefix(p.Suffix, "graphite/functions") {
		graphiteFunctionsRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%s", `{}`)
		return true
	}

	switch p.Suffix {
	case "prometheus/api/v1/query":
		queryRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.QueryHandler(startTime, at, w, r); err != nil {
			queryErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "prometheus/api/v1/query_range":
		queryRangeRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.QueryRangeHandler(startTime, at, w, r); err != nil {
			queryRangeErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "prometheus/api/v1/series":
		seriesRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.SeriesHandler(startTime, at, w, r); err != nil {
			seriesErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "prometheus/api/v1/series/count":
		seriesCountRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.SeriesCountHandler(startTime, at, w, r); err != nil {
			seriesCountErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "prometheus/api/v1/labels":
		labelsRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.LabelsHandler(startTime, at, w, r); err != nil {
			labelsErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "prometheus/api/v1/labels/count":
		labelsCountRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.LabelsCountHandler(startTime, at, w, r); err != nil {
			labelsCountErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "prometheus/api/v1/status/tsdb":
		statusTSDBRequests.Inc()
		if err := prometheus.TSDBStatusHandler(startTime, at, w, r); err != nil {
			statusTSDBErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "prometheus/api/v1/status/active_queries":
		statusActiveQueriesRequests.Inc()
		promql.WriteActiveQueries(w)
		return true
	case "prometheus/api/v1/status/top_queries":
		topQueriesRequests.Inc()
		if err := prometheus.QueryStatsHandler(startTime, at, w, r); err != nil {
			topQueriesErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "prometheus/api/v1/export":
		exportRequests.Inc()
		if err := prometheus.ExportHandler(startTime, at, w, r); err != nil {
			exportErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "prometheus/api/v1/export/native":
		exportNativeRequests.Inc()
		if err := prometheus.ExportNativeHandler(startTime, at, w, r); err != nil {
			exportNativeErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "prometheus/api/v1/export/csv":
		exportCSVRequests.Inc()
		if err := prometheus.ExportCSVHandler(startTime, at, w, r); err != nil {
			exportCSVErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "prometheus/federate":
		federateRequests.Inc()
		if err := prometheus.FederateHandler(startTime, at, w, r); err != nil {
			federateErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "graphite/metrics/find", "graphite/metrics/find/":
		graphiteMetricsFindRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := graphite.MetricsFindHandler(startTime, at, w, r); err != nil {
			graphiteMetricsFindErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "graphite/metrics/expand", "graphite/metrics/expand/":
		graphiteMetricsExpandRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := graphite.MetricsExpandHandler(startTime, at, w, r); err != nil {
			graphiteMetricsExpandErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "graphite/metrics/index.json", "graphite/metrics/index.json/":
		graphiteMetricsIndexRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := graphite.MetricsIndexHandler(startTime, at, w, r); err != nil {
			graphiteMetricsIndexErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "graphite/tags/tagSeries":
		graphiteTagsTagSeriesRequests.Inc()
		if err := graphite.TagsTagSeriesHandler(startTime, at, w, r); err != nil {
			graphiteTagsTagSeriesErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "graphite/tags/tagMultiSeries":
		graphiteTagsTagMultiSeriesRequests.Inc()
		if err := graphite.TagsTagMultiSeriesHandler(startTime, at, w, r); err != nil {
			graphiteTagsTagMultiSeriesErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "graphite/tags":
		graphiteTagsRequests.Inc()
		if err := graphite.TagsHandler(startTime, at, w, r); err != nil {
			graphiteTagsErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "graphite/tags/findSeries":
		graphiteTagsFindSeriesRequests.Inc()
		if err := graphite.TagsFindSeriesHandler(startTime, at, w, r); err != nil {
			graphiteTagsFindSeriesErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "graphite/tags/autoComplete/tags":
		graphiteTagsAutoCompleteTagsRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := graphite.TagsAutoCompleteTagsHandler(startTime, at, w, r); err != nil {
			graphiteTagsAutoCompleteTagsErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "graphite/tags/autoComplete/values":
		graphiteTagsAutoCompleteValuesRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := graphite.TagsAutoCompleteValuesHandler(startTime, at, w, r); err != nil {
			graphiteTagsAutoCompleteValuesErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "graphite/tags/delSeries":
		graphiteTagsDelSeriesRequests.Inc()
		if err := graphite.TagsDelSeriesHandler(startTime, at, w, r); err != nil {
			graphiteTagsDelSeriesErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "prometheus/api/v1/rules", "prometheus/rules":
		// Return dumb placeholder for https://prometheus.io/docs/prometheus/latest/querying/api/#rules
		rulesRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%s", `{"status":"success","data":{"groups":[]}}`)
		return true
	case "prometheus/api/v1/alerts", "prometheus/alerts":
		// Return dumb placeholder for https://prometheus.io/docs/prometheus/latest/querying/api/#alerts
		alertsRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%s", `{"status":"success","data":{"alerts":[]}}`)
		return true
	case "prometheus/api/v1/metadata":
		// Return dumb placeholder for https://prometheus.io/docs/prometheus/latest/querying/api/#querying-metric-metadata
		metadataRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%s", `{"status":"success","data":{}}`)
		return true
	case "prometheus/api/v1/query_exemplars":
		// Return dumb placeholder for https://prometheus.io/docs/prometheus/latest/querying/api/#querying-exemplars
		queryExemplarsRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%s", `{"status":"success","data":[]}`)
		return true
	default:
		return false
	}
}

func deleteHandler(startTime time.Time, w http.ResponseWriter, r *http.Request, p *httpserver.Path, at *auth.Token) bool {
	switch p.Suffix {
	case "prometheus/api/v1/admin/tsdb/delete_series":
		deleteRequests.Inc()
		if err := prometheus.DeleteHandler(startTime, at, r); err != nil {
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
	logger.Warnf("error in %q: %s", httpserver.GetRequestURI(r), err)

	w.Header().Set("Content-Type", "application/json")
	statusCode := http.StatusUnprocessableEntity
	var esc *httpserver.ErrorWithStatusCode
	if errors.As(err, &esc) {
		statusCode = esc.StatusCode
	}
	w.WriteHeader(statusCode)
	prometheus.WriteErrorResponse(w, statusCode, err)
}

var (
	requestDuration = metrics.NewHistogram(`vmselect_request_duration_seconds`)

	labelValuesRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/label/{}/values"}`)
	labelValuesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/prometheus/api/v1/label/{}/values"}`)

	queryRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/query"}`)
	queryErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/prometheus/api/v1/query"}`)

	queryRangeRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/query_range"}`)
	queryRangeErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/prometheus/api/v1/query_range"}`)

	seriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/series"}`)
	seriesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/prometheus/api/v1/series"}`)

	seriesCountRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/series/count"}`)
	seriesCountErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/prometheus/api/v1/series/count"}`)

	labelsRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/labels"}`)
	labelsErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/prometheus/api/v1/labels"}`)

	labelsCountRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/labels/count"}`)
	labelsCountErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/prometheus/api/v1/labels/count"}`)

	statusTSDBRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/status/tsdb"}`)
	statusTSDBErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/prometheus/api/v1/status/tsdb"}`)

	statusActiveQueriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}prometheus/api/v1/status/active_queries"}`)

	topQueriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/status/top_queries"}`)
	topQueriesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/prometheus/api/v1/status/top_queries"}`)

	globalTopQueriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/status/top_queries"}`)
	globalTopQueriesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/status/top_queries"}`)

	deleteRequests = metrics.NewCounter(`vm_http_requests_total{path="/delete/{}/prometheus/api/v1/admin/tsdb/delete_series"}`)
	deleteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/delete/{}/prometheus/api/v1/admin/tsdb/delete_series"}`)

	exportRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/export"}`)
	exportErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/prometheus/api/v1/export"}`)

	exportNativeRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/export/native"}`)
	exportNativeErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/prometheus/api/v1/export/native"}`)

	exportCSVRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/export/csv"}`)
	exportCSVErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/prometheus/api/v1/export/csv"}`)

	federateRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/federate"}`)
	federateErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/prometheus/federate"}`)

	graphiteMetricsFindRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/graphite/metrics/find"}`)
	graphiteMetricsFindErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/graphite/metrics/find"}`)

	graphiteMetricsExpandRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/graphite/metrics/expand"}`)
	graphiteMetricsExpandErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/graphite/metrics/expand"}`)

	graphiteMetricsIndexRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/graphite/metrics/index.json"}`)
	graphiteMetricsIndexErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/graphite/metrics/index.json"}`)

	graphiteTagsTagSeriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/graphite/tags/tagSeries"}`)
	graphiteTagsTagSeriesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/graphite/tags/tagSeries"}`)

	graphiteTagsTagMultiSeriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/graphite/tags/tagMultiSeries"}`)
	graphiteTagsTagMultiSeriesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/graphite/tags/tagMultiSeries"}`)

	graphiteTagsRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/graphite/tags"}`)
	graphiteTagsErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/graphite/tags"}`)

	graphiteTagValuesRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/graphite/tags/<tag_name>"}`)
	graphiteTagValuesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/graphite/tags/<tag_name>"}`)

	graphiteTagsFindSeriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/graphite/tags/findSeries"}`)
	graphiteTagsFindSeriesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/graphite/tags/findSeries"}`)

	graphiteTagsAutoCompleteTagsRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/graphite/tags/autoComplete/tags"}`)
	graphiteTagsAutoCompleteTagsErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/graphite/tags/autoComplete/tags"}`)

	graphiteTagsAutoCompleteValuesRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/graphite/tags/autoComplete/values"}`)
	graphiteTagsAutoCompleteValuesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/graphite/tags/autoComplete/values"}`)

	graphiteTagsDelSeriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/graphite/tags/delSeries"}`)
	graphiteTagsDelSeriesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/graphite/tags/delSeries"}`)

	graphiteFunctionsRequests = metrics.NewCounter(`vm_http_request_total{path="/select/{}/graphite/functions"}`)

	rulesRequests          = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/rules"}`)
	alertsRequests         = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/alerts"}`)
	metadataRequests       = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/metadata"}`)
	queryExemplarsRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/query_exemplars"}`)

	httpRequests         = tenantmetrics.NewCounterMap(`vm_tenant_select_requests_total`)
	httpRequestsDuration = tenantmetrics.NewCounterMap(`vm_tenant_select_requests_duration_ms_total`)
)

func usage() {
	const s = `
vmselect processes incoming queries by fetching the requested data from vmstorage nodes configured via -storageNode.

See the docs at https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html .
`
	flagutil.Usage(s)
}
