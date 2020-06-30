package vmselect

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/metrics"
)

var (
	deleteAuthKey         = flag.String("deleteAuthKey", "", "authKey for metrics' deletion via /api/v1/admin/tsdb/delete_series")
	maxConcurrentRequests = flag.Int("search.maxConcurrentRequests", getDefaultMaxConcurrentRequests(), "The maximum number of concurrent search requests. "+
		"It shouldn't be high, since a single request can saturate all the CPU cores. See also -search.maxQueueDuration")
	maxQueueDuration  = flag.Duration("search.maxQueueDuration", 10*time.Second, "The maximum time the request waits for execution when -search.maxConcurrentRequests limit is reached")
	resetCacheAuthKey = flag.String("search.resetCacheAuthKey", "", "Optional authKey for resetting rollup cache via /internal/resetRollupResultCache call")
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

// Init initializes vmselect
func Init() {
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
	case "/api/v1/status/tsdb":
		tsdbStatusRequests.Inc()
		if err := prometheus.TSDBStatusHandler(startTime, w, r); err != nil {
			tsdbStatusErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/export":
		exportRequests.Inc()
		if err := prometheus.ExportHandler(startTime, w, r); err != nil {
			exportErrors.Inc()
			httpserver.Errorf(w, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/federate":
		federateRequests.Inc()
		if err := prometheus.FederateHandler(startTime, w, r); err != nil {
			federateErrors.Inc()
			httpserver.Errorf(w, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/api/v1/rules":
		// Return dumb placeholder
		rulesRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%s", `{"status":"success","data":{"groups":[]}}`)
		return true
	case "/api/v1/alerts":
		// Return dumb placehloder
		alertsRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%s", `{"status":"success","data":{"alerts":[]}}`)
		return true
	case "/api/v1/metadata":
		// Return dumb placeholder
		metadataRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%s", `{"status":"success","data":{}}`)
		return true
	case "/api/v1/admin/tsdb/delete_series":
		deleteRequests.Inc()
		authKey := r.FormValue("authKey")
		if authKey != *deleteAuthKey {
			httpserver.Errorf(w, "invalid authKey %q. It must match the value from -deleteAuthKey command line flag", authKey)
			return true
		}
		if err := prometheus.DeleteHandler(startTime, r); err != nil {
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

func sendPrometheusError(w http.ResponseWriter, r *http.Request, err error) {
	logger.Warnf("error in %q: %s", r.RequestURI, err)

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

	tsdbStatusRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/status/tsdb"}`)
	tsdbStatusErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/status/tsdb"}`)

	deleteRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/admin/tsdb/delete_series"}`)
	deleteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/admin/tsdb/delete_series"}`)

	exportRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/export"}`)
	exportErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/export"}`)

	federateRequests = metrics.NewCounter(`vm_http_requests_total{path="/federate"}`)
	federateErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/federate"}`)

	rulesRequests    = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/rules"}`)
	alertsRequests   = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/alerts"}`)
	metadataRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/metadata"}`)
)
