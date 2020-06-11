package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/metrics"
)

var (
	httpListenAddr        = flag.String("httpListenAddr", ":8481", "Address to listen for http connections")
	cacheDataPath         = flag.String("cacheDataPath", "", "Path to directory for cache files. Cache isn't saved if empty")
	maxConcurrentRequests = flag.Int("search.maxConcurrentRequests", getDefaultMaxConcurrentRequests(), "The maximum number of concurrent search requests. "+
		"It shouldn't be high, since a single request can saturate all the CPU cores. See also -search.maxQueueDuration")
	maxQueueDuration  = flag.Duration("search.maxQueueDuration", 10*time.Second, "The maximum time the request waits for execution when -search.maxConcurrentRequests limit is reached")
	minScrapeInterval = flag.Duration("dedup.minScrapeInterval", 0, "Remove superflouos samples from time series if they are located closer to each other than this duration. "+
		"This may be useful for reducing overhead when multiple identically configured Prometheus instances write data to the same VictoriaMetrics. "+
		"Deduplication is disabled if the -dedup.minScrapeInterval is 0")
	resetCacheAuthKey = flag.String("search.resetCacheAuthKey", "", "Optional authKey for resetting rollup cache via /internal/resetRollupResultCache call. Will be replaced by internalAuthKey.")
	internalAuthKey   = flag.String("internalAuthKey", "", "Optional authKey for operation calls with prefix /internal. Use value of search.resetCacheAuthKey fif not set.")
	storageNodes      = flagutil.NewArray("storageNode", "Addresses of vmstorage nodes; usage: -storageNode=vmstorage-host1:8401 -storageNode=vmstorage-host2:8401")
)

func getDefaultMaxConcurrentRequests() int {
	n := runtime.GOMAXPROCS(-1)
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
	envflag.Parse()
	buildinfo.Init()
	logger.Init()

	logger.Infof("starting netstorage at storageNodes %s", *storageNodes)
	startTime := time.Now()
	storage.SetMinScrapeIntervalForDeduplication(*minScrapeInterval)
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

var internalHandlerPrefix = "/internal"

func requestHandler(w http.ResponseWriter, r *http.Request) bool {
	startTime := time.Now()
	// Limit the number of concurrent queries.
	select {
	case concurrencyCh <- struct{}{}:
		defer func() { <-concurrencyCh }()
	default:
		// Sleep for a while until giving up. This should resolve short bursts in requests.
		concurrencyLimitReached.Inc()
		t := timerpool.Get(*maxQueueDuration)
		select {
		case concurrencyCh <- struct{}{}:
			timerpool.Put(t)
			defer func() { <-concurrencyCh }()
		case <-t.C:
			timerpool.Put(t)
			concurrencyLimitTimeout.Inc()
			err := &httpserver.ErrorWithStatusCode{
				Err: fmt.Errorf("cannot handle more than %d concurrent search requests during %s; possible solutions: "+
					"increase `-search.maxQueueDuration`, increase `-search.maxConcurrentRequests`, increase server capacity",
					*maxConcurrentRequests, *maxQueueDuration),
				StatusCode: http.StatusServiceUnavailable,
			}
			httpserver.Errorf(w, "%s", err)
			return true
		}
	}

	path := strings.Replace(r.URL.Path, "//", "/", -1)
	if strings.HasPrefix(path, internalHandlerPrefix) {
		return internalHandler(w, r)
	}

	p, err := httpserver.ParsePath(path)
	if err != nil {
		httpserver.Errorf(w, "cannot parse path %q: %s", path, err)
		return true
	}
	at, err := auth.NewToken(p.AuthToken)
	if err != nil {
		httpserver.Errorf(w, "auth error: %s", err)
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

func selectHandler(startTime time.Time, w http.ResponseWriter, r *http.Request, p *httpserver.Path, at *auth.Token) bool {
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
		tsdbStatusRequests.Inc()
		if err := prometheus.TSDBStatusHandler(startTime, at, w, r); err != nil {
			tsdbStatusErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "prometheus/api/v1/export":
		exportRequests.Inc()
		if err := prometheus.ExportHandler(startTime, at, w, r); err != nil {
			exportErrors.Inc()
			httpserver.Errorf(w, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "prometheus/federate":
		federateRequests.Inc()
		if err := prometheus.FederateHandler(startTime, at, w, r); err != nil {
			federateErrors.Inc()
			httpserver.Errorf(w, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "prometheus/api/v1/rules":
		// Return dumb placeholder
		rulesRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%s", `{"status":"success","data":{"groups":[]}}`)
		return true
	case "prometheus/api/v1/alerts":
		// Return dumb placehloder
		alertsRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%s", `{"status":"success","data":{"alerts":[]}}`)
		return true
	case "prometheus/api/v1/metadata":
		// Return dumb placeholder
		metadataRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%s", `{"status":"success","data":{}}`)
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
			httpserver.Errorf(w, "error in %q: %s", r.URL.Path, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	default:
		return false
	}
}

func internalHandler(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	p := path[len(internalHandlerPrefix):]
	authKey := *internalAuthKey
	if authKey == "" {
		authKey = *resetCacheAuthKey
	}
	if authKey != "" && r.FormValue("authKey") != authKey {
		sendPrometheusError(w, r, fmt.Errorf("invalid authKey=%q for %q", r.FormValue("authKey"), path))
		return true
	}

	switch p {
	case "/resetRollupResultCache":
		promql.ResetRollupResultCache()
		return true
	case "/query/list":
		queryListRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		data, err := json.Marshal(promql.GetAllRunningQueries())
		if err != nil {
			queryListErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		fmt.Fprintf(w, `{"status":"success","data": %v}`, string(data))
		return true
	case "/query/kill":
		queryKillRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		pid := strings.TrimSpace(r.URL.Query().Get("pid"))
		if pid == "" {
			queryKillErrors.Inc()
			sendPrometheusError(w, r, fmt.Errorf("pid not set"))
			return true
		}
		err := promql.CancelRunningQuery(pid)
		if err != nil {
			queryKillErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		fmt.Fprintf(w, "%s", `{"status":"success","data":{}}`)
		return true
	default:
		return false
	}
}

func sendPrometheusError(w http.ResponseWriter, r *http.Request, err error) {
	logger.Warnf("error in %q: %s", r.RequestURI, err)

	w.Header().Set("Content-Type", "application/json")
	statusCode := http.StatusUnprocessableEntity
	if esc, ok := err.(*httpserver.ErrorWithStatusCode); ok {
		statusCode = esc.StatusCode
	}
	w.WriteHeader(statusCode)
	prometheus.WriteErrorResponse(w, statusCode, err)
}

var (
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

	tsdbStatusRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/status/tsdb"}`)
	tsdbStatusErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/prometheus/api/v1/status/tsdb"}`)

	deleteRequests = metrics.NewCounter(`vm_http_requests_total{path="/delete/{}/prometheus/api/v1/admin/tsdb/delete_series"}`)
	deleteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/delete/{}/prometheus/api/v1/admin/tsdb/delete_series"}`)

	exportRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/export"}`)
	exportErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/prometheus/api/v1/export"}`)

	federateRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/federate"}`)
	federateErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/select/{}/prometheus/federate"}`)

	rulesRequests    = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/rules"}`)
	alertsRequests   = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/alerts"}`)
	metadataRequests = metrics.NewCounter(`vm_http_requests_total{path="/select/{}/prometheus/api/v1/metadata"}`)

	queryListRequests = metrics.NewCounter(`vm_http_requests_total{path="/-/{}/query/list"}`)
	queryListErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/-/{}/query/list"}`)

	queryKillRequests = metrics.NewCounter(`vm_http_requests_total{path="/-/{}/query/kill"}`)
	queryKillErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/-/{}/query/kill"}`)
)
