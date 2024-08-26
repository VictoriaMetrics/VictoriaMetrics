package main

import (
	"embed"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/clusternative"
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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pushmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vmselectapi"
)

var (
	httpListenAddrs  = flagutil.NewArrayString("httpListenAddr", "Address to listen for incoming http requests. See also -httpListenAddr.useProxyProtocol")
	useProxyProtocol = flagutil.NewArrayBool("httpListenAddr.useProxyProtocol", "Whether to use proxy protocol for connections accepted at the given -httpListenAddr . "+
		"See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . "+
		"With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing")
	cacheDataPath = flag.String("cacheDataPath", "", "Path to directory for cache files and temporary query results. "+
		"By default, the cache won't be persisted, and temporary query results will be placed under /tmp/searchResults. If set, the cache will be persisted under cacheDataPath/rollupResult, and temporary query results will be placed under cacheDataPath/tmp/searchResults.")
	maxConcurrentRequests = flag.Int("search.maxConcurrentRequests", getDefaultMaxConcurrentRequests(), "The maximum number of concurrent search requests. "+
		"It shouldn't be high, since a single request can saturate all the CPU cores, while many concurrently executed requests may require high amounts of memory. "+
		"See also -search.maxQueueDuration and -search.maxMemoryPerQuery")
	maxQueueDuration = flag.Duration("search.maxQueueDuration", 10*time.Second, "The maximum time the request waits for execution when -search.maxConcurrentRequests "+
		"limit is reached; see also -search.maxQueryDuration")
	minScrapeInterval = flag.Duration("dedup.minScrapeInterval", 0, "Leave only the last sample in every time series per each discrete interval "+
		"equal to -dedup.minScrapeInterval > 0. See https://docs.victoriametrics.com/#deduplication for details")
	deleteAuthKey        = flagutil.NewPassword("deleteAuthKey", "authKey for metrics' deletion via /prometheus/api/v1/admin/tsdb/delete_series and /graphite/tags/delSeries")
	resetCacheAuthKey    = flagutil.NewPassword("search.resetCacheAuthKey", "Optional authKey for resetting rollup cache via /internal/resetRollupResultCache call")
	logSlowQueryDuration = flag.Duration("search.logSlowQueryDuration", 5*time.Second, "Log queries with execution time exceeding this value. Zero disables slow query logging. "+
		"See also -search.logQueryMemoryUsage")
	vmalertProxyURL = flag.String("vmalert.proxyURL", "", "Optional URL for proxying requests to vmalert. For example, if -vmalert.proxyURL=http://vmalert:8880 , then alerting API requests such as /api/v1/rules from Grafana will be proxied to http://vmalert:8880/api/v1/rules")
	storageNodes    = flagutil.NewArrayString("storageNode", "Comma-separated addresses of vmstorage nodes; usage: -storageNode=vmstorage-host1,...,vmstorage-hostN . "+
		"Enterprise version of VictoriaMetrics supports automatic discovery of vmstorage addresses via DNS SRV records. For example, -storageNode=srv+vmstorage.addrs . "+
		"See https://docs.victoriametrics.com/cluster-victoriametrics/#automatic-vmstorage-discovery")

	clusternativeListenAddr = flag.String("clusternativeListenAddr", "", "TCP address to listen for requests from other vmselect nodes in multi-level cluster setup. "+
		"See https://docs.victoriametrics.com/cluster-victoriametrics/#multi-level-cluster-setup . Usually :8401 should be set to match default vmstorage port for vmselect. Disabled work if empty")
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

//go:embed static
var staticFiles embed.FS

var staticServer = http.FileServer(http.FS(staticFiles))

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
	if hasEmptyValues(*storageNodes) {
		logger.Fatalf("found empty address of storage node in the -storageNodes flag, please make sure that all -storageNode args are non-empty")
	}
	if duplicatedAddr := checkDuplicates(*storageNodes); duplicatedAddr != "" {
		logger.Fatalf("found equal addresses of storage nodes in the -storageNodes flag: %q", duplicatedAddr)
	}

	netstorage.Init(*storageNodes)
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
	concurrencyLimitCh = make(chan struct{}, *maxConcurrentRequests)
	initVMAlertProxy()
	var vmselectapiServer *vmselectapi.Server
	if *clusternativeListenAddr != "" {
		logger.Infof("starting vmselectapi server at %q", *clusternativeListenAddr)
		s, err := clusternative.NewVMSelectServer(*clusternativeListenAddr)
		if err != nil {
			logger.Fatalf("cannot initialize vmselectapi server: %s", err)
		}
		vmselectapiServer = s
		logger.Infof("started vmselectapi server at %q", *clusternativeListenAddr)
	}

	listenAddrs := *httpListenAddrs
	if len(listenAddrs) == 0 {
		listenAddrs = []string{":8481"}
	}
	go httpserver.Serve(listenAddrs, useProxyProtocol, requestHandler)

	pushmetrics.Init()
	sig := procutil.WaitForSigterm()
	logger.Infof("service received signal %s", sig)
	pushmetrics.Stop()

	logger.Infof("gracefully shutting down http service at %q", listenAddrs)
	startTime = time.Now()
	if err := httpserver.Stop(listenAddrs); err != nil {
		logger.Fatalf("cannot stop http service: %s", err)
	}
	logger.Infof("successfully shut down http service in %.3f seconds", time.Since(startTime).Seconds())

	if vmselectapiServer != nil {
		logger.Infof("stopping vmselectapi server...")
		vmselectapiServer.MustStop()
		logger.Infof("stopped vmselectapi server")
	}

	logger.Infof("shutting down neststorage...")
	startTime = time.Now()
	netstorage.MustStop()
	if len(*cacheDataPath) > 0 {
		promql.StopRollupResultCache()
	}
	logger.Infof("successfully stopped netstorage in %.3f seconds", time.Since(startTime).Seconds())

	fs.MustStopDirRemover()

	logger.Infof("the vmselect has been stopped")
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

func requestHandler(w http.ResponseWriter, r *http.Request) bool {
	path := strings.Replace(r.URL.Path, "//", "/", -1)

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
	if path == "/admin/tenants" {
		tenantsRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.Tenants(qt, startTime, w, r); err != nil {
			tenantsErrors.Inc()
			httpserver.Errorf(w, r, "error getting tenants: %s", err)
			return true
		}
		return true
	}
	p, err := httpserver.ParsePath(path)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse path %q: %s", path, err)
		return true
	}
	at, err := auth.NewTokenPossibleMultitenant(p.AuthToken)
	if err != nil {
		httpserver.Errorf(w, r, "auth error: %s", err)
		return true
	}
	switch p.Prefix {
	case "select":
		return selectHandler(qt, startTime, w, r, p, at)
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

func selectHandler(qt *querytracer.Tracer, startTime time.Time, w http.ResponseWriter, r *http.Request, p *httpserver.Path, at *auth.Token) bool {
	defer func() {
		// Count per-tenant cumulative durations and total requests
		httpRequests.Get(at).Inc()
		httpRequestsDuration.Get(at).Add(int(time.Since(startTime).Milliseconds()))
	}()
	if strings.HasPrefix(p.Suffix, "prometheus/api/v1/label/") {
		s := p.Suffix[len("prometheus/api/v1/label/"):]
		if strings.HasSuffix(s, "/values") {
			labelValuesRequests.Inc()
			labelName := s[:len(s)-len("/values")]
			httpserver.EnableCORS(w, r)
			if err := prometheus.LabelValuesHandler(qt, startTime, at, labelName, w, r); err != nil {
				labelValuesErrors.Inc()
				httpserver.SendPrometheusError(w, r, err)
				return true
			}
			return true
		}
	}
	if strings.HasPrefix(p.Suffix, "graphite/") && at == nil {
		httpserver.Errorf(w, r, "multi-tenant queries are not supported by Graphite endpoints")
		return true
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

	switch p.Suffix {
	case "prometheus/api/v1/query":
		queryRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.QueryHandler(qt, startTime, at, w, r); err != nil {
			queryErrors.Inc()
			httpserver.SendPrometheusError(w, r, err)
			return true
		}
		return true
	case "prometheus/api/v1/query_range":
		queryRangeRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.QueryRangeHandler(qt, startTime, at, w, r); err != nil {
			queryRangeErrors.Inc()
			httpserver.SendPrometheusError(w, r, err)
			return true
		}
		return true
	case "prometheus/api/v1/series":
		seriesRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.SeriesHandler(qt, startTime, at, w, r); err != nil {
			seriesErrors.Inc()
			httpserver.SendPrometheusError(w, r, err)
			return true
		}
		return true
	case "prometheus/api/v1/series/count":
		seriesCountRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.SeriesCountHandler(startTime, at, w, r); err != nil {
			seriesCountErrors.Inc()
			httpserver.SendPrometheusError(w, r, err)
			return true
		}
		return true
	case "prometheus/api/v1/labels":
		labelsRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.LabelsHandler(qt, startTime, at, w, r); err != nil {
			labelsErrors.Inc()
			httpserver.SendPrometheusError(w, r, err)
			return true
		}
		return true
	case "prometheus/api/v1/status/tsdb":
		statusTSDBRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.TSDBStatusHandler(qt, startTime, at, w, r); err != nil {
			statusTSDBErrors.Inc()
			httpserver.SendPrometheusError(w, r, err)
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
	case "prometheus/api/v1/export/csv":
		exportCSVRequests.Inc()
		if err := prometheus.ExportCSVHandler(startTime, at, w, r); err != nil {
			exportCSVErrors.Inc()
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
		if !httpserver.CheckAuthFlag(w, r, deleteAuthKey) {
			return true
		}
		graphiteTagsDelSeriesRequests.Inc()
		if err := graphite.TagsDelSeriesHandler(startTime, at, w, r); err != nil {
			graphiteTagsDelSeriesErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "graphite/render":
		graphiteRenderRequests.Inc()
		if err := graphite.RenderHandler(startTime, at, w, r); err != nil {
			graphiteRenderErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	default:
		return false
	}
}

func handleStaticAndSimpleRequests(w http.ResponseWriter, r *http.Request, path string) bool {
	if path == "/" {
		if r.Method != http.MethodGet {
			return false
		}
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `vmselect - a component of VictoriaMetrics cluster<br/>
<a href="https://docs.victoriametrics.com/cluster-victoriametrics/">docs</a><br>
`)
		return true
	}
	if path == "/api/v1/status/top_queries" {
		globalTopQueriesRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.QueryStatsHandler(nil, w, r); err != nil {
			globalTopQueriesErrors.Inc()
			httpserver.SendPrometheusError(w, r, err)
			return true
		}
		return true
	}
	if path == "/api/v1/status/active_queries" {
		globalStatusActiveQueriesRequests.Inc()
		httpserver.EnableCORS(w, r)
		promql.ActiveQueriesHandler(nil, w, r)
		return true
	}
	p, err := httpserver.ParsePath(path)
	if err != nil {
		return false
	}
	if p.Suffix == "" {
		if r.Method != http.MethodGet {
			return false
		}
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<h2>VictoriaMetrics cluster - vmselect</h2></br>")
		fmt.Fprintf(w, "See <a href='https://docs.victoriametrics.com/cluster-victoriametrics/#url-format'>docs</a></br>")
		fmt.Fprintf(w, "Useful endpoints:</br>")
		fmt.Fprintf(w, `<a href="vmui">Web UI</a><br>`)
		fmt.Fprintf(w, `<a href="metric-relabel-debug">metric-level relabel debugging</a></br>`)
		fmt.Fprintf(w, `<a href="target-relabel-debug">target-level relabel debugging</a></br>`)
		fmt.Fprintf(w, `<a href="expand-with-exprs">WITH expressions' tutorial</a></br>`)
		fmt.Fprintf(w, `<a href="prometheus/api/v1/status/tsdb">tsdb status page</a><br>`)
		fmt.Fprintf(w, `<a href="prometheus/api/v1/status/top_queries">top queries</a><br>`)
		fmt.Fprintf(w, `<a href="prometheus/api/v1/status/active_queries">active queries</a><br>`)
		return true
	}
	if strings.HasPrefix(p.Suffix, "static") {
		prefix := strings.Join([]string{"", p.Prefix, p.AuthToken}, "/")
		http.StripPrefix(prefix, staticServer).ServeHTTP(w, r)
		return true
	}
	if strings.HasPrefix(p.Suffix, "prometheus/static") {
		prefix := strings.Join([]string{"", p.Prefix, p.AuthToken}, "/")
		r.URL.Path = strings.Replace(r.URL.Path, "/prometheus/static", "/static", 1)
		http.StripPrefix(prefix, staticServer).ServeHTTP(w, r)
		return true
	}
	if p.Suffix == "vmui" || p.Suffix == "graph" || p.Suffix == "prometheus/vmui" || p.Suffix == "prometheus/graph" {
		// VMUI access via incomplete url without `/` in the end. Redirect to complete url.
		// Use relative redirect, since the hostname and path prefix may be incorrect if VictoriaMetrics
		// is hidden behind vmauth or similar proxy.
		_ = r.ParseForm()
		suffix := strings.Replace(p.Suffix, "prometheus/", "../prometheus/", 1)
		newURL := suffix + "/?" + r.Form.Encode()
		httpserver.Redirect(w, newURL)
		return true
	}
	if strings.HasPrefix(p.Suffix, "graph/") || strings.HasPrefix(p.Suffix, "prometheus/graph/") {
		// This is needed for serving /graph URLs from Prometheus datasource in Grafana.
		p.Suffix = strings.Replace(p.Suffix, "graph/", "vmui/", 1)
		r.URL.Path = strings.Replace(r.URL.Path, "/graph/", "/vmui/", 1)
	}
	if p.Suffix == "vmui/custom-dashboards" || p.Suffix == "prometheus/vmui/custom-dashboards" {
		if err := handleVMUICustomDashboards(w); err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	}
	if p.Suffix == "vmui/timezone" || p.Suffix == "prometheus/vmui/timezone" {
		httpserver.EnableCORS(w, r)
		if err := handleVMUITimezone(w); err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	}
	if strings.HasPrefix(p.Suffix, "vmui/") || strings.HasPrefix(p.Suffix, "prometheus/vmui/") {
		// vmui access.
		if strings.HasPrefix(p.Suffix, "vmui/static/") || strings.HasPrefix(p.Suffix, "prometheus/vmui/static/") {
			// Allow clients caching static contents for long period of time, since it shouldn't change over time.
			// Path to static contents (such as js and css) must be changed whenever its contents is changed.
			// See https://developer.chrome.com/docs/lighthouse/performance/uses-long-cache-ttl/
			w.Header().Set("Cache-Control", "max-age=31536000")
		}
		prefix := strings.Join([]string{"", p.Prefix, p.AuthToken}, "/")
		r.URL.Path = strings.Replace(r.URL.Path, "/prometheus/vmui/", "/vmui/", 1)
		http.StripPrefix(prefix, vmuiFileServer).ServeHTTP(w, r)
		return true
	}
	if strings.HasPrefix(p.Suffix, "graphite/functions") {
		funcName := p.Suffix[len("graphite/functions"):]
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
	if p.Suffix == "prometheus/vmalert" {
		// vmalert access via incomplete url without `/` in the end. Redirect to complete url.
		// Use relative redirect, since the hostname and path prefix may be incorrect if VictoriaMetrics
		// is hidden behind vmauth or similar proxy.
		path := "../" + p.Suffix + "/"
		httpserver.Redirect(w, path)
		return true
	}
	if strings.HasPrefix(p.Suffix, "prometheus/vmalert/") {
		vmalertRequests.Inc()
		if len(*vmalertProxyURL) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, "%s", `{"status":"error","msg":"for accessing vmalert flag '-vmalert.proxyURL' must be configured"}`)
			return true
		}
		proxyVMAlertRequests(w, r, p.Suffix)
		return true
	}
	switch p.Suffix {
	case "prometheus/api/v1/status/active_queries":
		at, err := auth.NewTokenPossibleMultitenant(p.AuthToken)
		if err != nil {
			return false
		}
		statusActiveQueriesRequests.Inc()
		httpserver.EnableCORS(w, r)
		promql.ActiveQueriesHandler(at, w, r)
		return true
	case "prometheus/api/v1/status/top_queries":
		at, err := auth.NewTokenPossibleMultitenant(p.AuthToken)
		if err != nil {
			return false
		}
		topQueriesRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.QueryStatsHandler(at, w, r); err != nil {
			topQueriesErrors.Inc()
			httpserver.SendPrometheusError(w, r, err)
			return true
		}
		return true
	case "prometheus/metric-relabel-debug", "metric-relabel-debug":
		promrelabelMetricRelabelDebugRequests.Inc()
		metric := r.FormValue("metric")
		relabelConfigs := r.FormValue("relabel_configs")
		format := r.FormValue("format")
		promrelabel.WriteMetricRelabelDebug(w, "", metric, relabelConfigs, format, nil)
		return true
	case "prometheus/target-relabel-debug", "target-relabel-debug":
		promrelabelTargetRelabelDebugRequests.Inc()
		metric := r.FormValue("metric")
		relabelConfigs := r.FormValue("relabel_configs")
		format := r.FormValue("format")
		promrelabel.WriteTargetRelabelDebug(w, "", metric, relabelConfigs, format, nil)
		return true
	case "prometheus/expand-with-exprs", "expand-with-exprs":
		expandWithExprsRequests.Inc()
		prometheus.ExpandWithExprs(w, r)
		return true
	case "prometheus/prettify-query", "prettify-query":
		prettifyQueryRequests.Inc()
		prometheus.PrettifyQuery(w, r)
		return true
	case "prometheus/api/v1/rules", "prometheus/rules":
		rulesRequests.Inc()
		if len(*vmalertProxyURL) > 0 {
			proxyVMAlertRequests(w, r, p.Suffix)
			return true
		}
		// Return dumb placeholder for https://prometheus.io/docs/prometheus/latest/querying/api/#rules
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"groups":[]}}`)
		return true
	case "prometheus/api/v1/alerts", "prometheus/alerts":
		alertsRequests.Inc()
		if len(*vmalertProxyURL) > 0 {
			proxyVMAlertRequests(w, r, p.Suffix)
			return true
		}
		// Return dumb placeholder for https://prometheus.io/docs/prometheus/latest/querying/api/#alerts
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"alerts":[]}}`)
		return true
	case "prometheus/api/v1/metadata":
		// Return dumb placeholder for https://prometheus.io/docs/prometheus/latest/querying/api/#querying-metric-metadata
		metadataRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%s", `{"status":"success","data":{}}`)
		return true
	case "prometheus/api/v1/status/buildinfo":
		buildInfoRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		// prometheus version is used here, which affects what API Grafana uses when retrieving label values.
		// as new Grafana features are added that are customized for the Prometheus version, maybe the version will need to be increased.
		// see this issue for more info: https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5370
		fmt.Fprintf(w, "%s", `{"status":"success","data":{"version":"2.24.0"}}`)
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
		if !httpserver.CheckAuthFlag(w, r, deleteAuthKey) {
			return true
		}
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

	statusTSDBRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/status/tsdb"}`)
	statusTSDBErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/prometheus/api/v1/status/tsdb"}`)

	globalStatusActiveQueriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/status/active_queries"}`)
	statusActiveQueriesRequests       = metrics.NewCounter(`vm_http_requests_total{path="/select/{}prometheus/api/v1/status/active_queries"}`)

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

	graphiteRenderRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/graphite/render"}`)
	graphiteRenderErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/graphite/render"}`)

	graphiteFunctionsRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/graphite/functions"}`)
	graphiteFunctionsErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/graphite/functions"}`)

	graphiteFunctionDetailsRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/graphite/functions/<func_name>"}`)
	graphiteFunctionDetailsErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/graphite/functions/<func_name>"}`)

	promrelabelMetricRelabelDebugRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/metric-relabel-debug"}`)
	promrelabelTargetRelabelDebugRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/target-relabel-debug"}`)

	expandWithExprsRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/expand-with-exprs"}`)
	prettifyQueryRequests   = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/prettify-query"}`)

	vmalertRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/vmalert"}`)
	rulesRequests   = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/rules"}`)
	alertsRequests  = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/alerts"}`)

	metadataRequests       = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/metadata"}`)
	buildInfoRequests      = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/buildinfo"}`)
	queryExemplarsRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/query_exemplars"}`)

	tenantsRequests = metrics.NewCounter(`vm_http_requests_total{path="/admin/tenants"}`)
	tenantsErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/admin/tenants"}`)

	httpRequests         = tenantmetrics.NewCounterMap(`vm_tenant_select_requests_total`)
	httpRequestsDuration = tenantmetrics.NewCounterMap(`vm_tenant_select_requests_duration_ms_total`)
)

func usage() {
	const s = `
vmselect processes incoming queries by fetching the requested data from vmstorage nodes configured via -storageNode.

See the docs at https://docs.victoriametrics.com/cluster-victoriametrics/ .
`
	flagutil.Usage(s)
}

func proxyVMAlertRequests(w http.ResponseWriter, r *http.Request, path string) {
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
	r.URL.Path = strings.TrimPrefix(path, "prometheus")
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

func checkDuplicates(arr []string) string {
	visited := make(map[string]struct{})
	for _, s := range arr {
		if _, ok := visited[s]; ok {
			return s
		}
		visited[s] = struct{}{}
	}
	return ""
}

func hasEmptyValues(arr []string) bool {
	for _, s := range arr {
		if s == "" {
			return true
		}
	}
	return false
}
