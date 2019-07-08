package vmselect

import (
	"flag"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/metrics"
)

var (
	deleteAuthKey         = flag.String("deleteAuthKey", "", "authKey for metrics' deletion via /api/v1/admin/tsdb/delete_series")
	maxConcurrentRequests = flag.Int("search.maxConcurrentRequests", runtime.GOMAXPROCS(-1)*2, "The maximum number of concurrent search requests. It shouldn't exceed 2*vCPUs for better performance. See also -search.maxQueueDuration")
	maxQueueDuration      = flag.Duration("search.maxQueueDuration", 10*time.Second, "The maximum time the request waits for execution when -search.maxConcurrentRequests limit is reached")
)

// Init initializes vmselect
func Init() {
	tmpDirPath := filepath.Join(*vmstorage.DataPath, "/tmp")
	fs.RemoveDirContents(tmpDirPath)
	netstorage.InitTmpBlocksDir(tmpDirPath)
	promql.InitRollupResultCache(filepath.Join(*vmstorage.DataPath, "/cache/rollupResult"))
	concurrencyCh = make(chan struct{}, *maxConcurrentRequests)
}

var concurrencyCh chan struct{}

// Stop stops vmselect
func Stop() {
	promql.StopRollupResultCache()
}

// RequestHandler handles remote read API requests for Prometheus
func RequestHandler(w http.ResponseWriter, r *http.Request) bool {
	// Limit the number of concurrent queries.
	// Sleep for a while until giving up. This should resolve short bursts in requests.
	t := timerpool.Get(*maxQueueDuration)
	select {
	case concurrencyCh <- struct{}{}:
		timerpool.Put(t)
		defer func() { <-concurrencyCh }()
	case <-t.C:
		timerpool.Put(t)
		httpserver.Errorf(w, "cannot handle more than %d concurrent requests", cap(concurrencyCh))
		return true
	}

	path := strings.Replace(r.URL.Path, "//", "/", -1)
	if strings.HasPrefix(path, "/api/v1/label/") {
		s := r.URL.Path[len("/api/v1/label/"):]
		if strings.HasSuffix(s, "/values") {
			labelValuesRequests.Inc()
			labelName := s[:len(s)-len("/values")]
			httpserver.EnableCORS(w, r)
			if err := prometheus.LabelValuesHandler(labelName, w, r); err != nil {
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
		if err := prometheus.QueryHandler(w, r); err != nil {
			queryErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/query_range":
		queryRangeRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.QueryRangeHandler(w, r); err != nil {
			queryRangeErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/series":
		seriesRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.SeriesHandler(w, r); err != nil {
			seriesErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/series/count":
		seriesCountRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.SeriesCountHandler(w, r); err != nil {
			seriesCountErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/labels":
		labelsRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.LabelsHandler(w, r); err != nil {
			labelsErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/labels/count":
		labelsCountRequests.Inc()
		httpserver.EnableCORS(w, r)
		if err := prometheus.LabelsCountHandler(w, r); err != nil {
			labelsCountErrors.Inc()
			sendPrometheusError(w, r, err)
			return true
		}
		return true
	case "/api/v1/export":
		exportRequests.Inc()
		if err := prometheus.ExportHandler(w, r); err != nil {
			exportErrors.Inc()
			httpserver.Errorf(w, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/federate":
		federateRequests.Inc()
		if err := prometheus.FederateHandler(w, r); err != nil {
			federateErrors.Inc()
			httpserver.Errorf(w, "error int %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/api/v1/admin/tsdb/delete_series":
		deleteRequests.Inc()
		authKey := r.FormValue("authKey")
		if authKey != *deleteAuthKey {
			httpserver.Errorf(w, "invalid authKey %q. It must match the value from -deleteAuthKey command line flag", authKey)
			return true
		}
		if err := prometheus.DeleteHandler(r); err != nil {
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
	logger.Errorf("error in %q: %s", r.URL.Path, err)

	w.Header().Set("Content-Type", "application/json")
	statusCode := 422
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

	deleteRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/admin/tsdb/delete_series"}`)
	deleteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/admin/tsdb/delete_series"}`)

	exportRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/export"}`)
	exportErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/export"}`)

	federateRequests = metrics.NewCounter(`vm_http_requests_total{path="/federate"}`)
	federateErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/federate"}`)
)
