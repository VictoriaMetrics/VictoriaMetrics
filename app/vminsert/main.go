package vminsert

import (
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/concurrencylimiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/graphite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/vmimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
)

var (
	graphiteListenAddr = flag.String("graphiteListenAddr", "", "TCP and UDP address to listen for Graphite plaintext data. Usually :2003 must be set. Doesn't work if empty")
	opentsdbListenAddr = flag.String("opentsdbListenAddr", "", "TCP and UDP address to listen for OpentTSDB metrics. "+
		"Telnet put messages and HTTP /api/put messages are simultaneously served on TCP port. "+
		"Usually :4242 must be set. Doesn't work if empty")
	opentsdbHTTPListenAddr = flag.String("opentsdbHTTPListenAddr", "", "TCP address to listen for OpentTSDB HTTP put requests. Usually :4242 must be set. Doesn't work if empty")
	maxInsertRequestSize   = flag.Int("maxInsertRequestSize", 32*1024*1024, "The maximum size of a single insert request in bytes")
	maxLabelsPerTimeseries = flag.Int("maxLabelsPerTimeseries", 30, "The maximum number of labels accepted per time series. Superflouos labels are dropped")
)

var (
	graphiteServer     *graphite.Server
	opentsdbServer     *opentsdb.Server
	opentsdbhttpServer *opentsdbhttp.Server
)

// Init initializes vminsert.
func Init() {
	storage.SetMaxLabelsPerTimeseries(*maxLabelsPerTimeseries)

	concurrencylimiter.Init()
	if len(*graphiteListenAddr) > 0 {
		graphiteServer = graphite.MustStart(*graphiteListenAddr)
	}
	if len(*opentsdbListenAddr) > 0 {
		opentsdbServer = opentsdb.MustStart(*opentsdbListenAddr, int64(*maxInsertRequestSize))
	}
	if len(*opentsdbHTTPListenAddr) > 0 {
		opentsdbhttpServer = opentsdbhttp.MustStart(*opentsdbHTTPListenAddr, int64(*maxInsertRequestSize))
	}
}

// Stop stops vminsert.
func Stop() {
	if len(*graphiteListenAddr) > 0 {
		graphiteServer.MustStop()
	}
	if len(*opentsdbListenAddr) > 0 {
		opentsdbServer.MustStop()
	}
	if len(*opentsdbHTTPListenAddr) > 0 {
		opentsdbhttpServer.MustStop()
	}
}

// RequestHandler is a handler for Prometheus remote storage write API
func RequestHandler(w http.ResponseWriter, r *http.Request) bool {
	path := strings.Replace(r.URL.Path, "//", "/", -1)
	switch path {
	case "/api/v1/write":
		prometheusWriteRequests.Inc()
		if err := prometheus.InsertHandler(r, int64(*maxInsertRequestSize)); err != nil {
			prometheusWriteErrors.Inc()
			httpserver.Errorf(w, "error in %q: %s", r.URL.Path, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/api/v1/import":
		vmimportRequests.Inc()
		if err := vmimport.InsertHandler(r); err != nil {
			vmimportErrors.Inc()
			httpserver.Errorf(w, "error in %q: %s", r.URL.Path, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/write", "/api/v2/write":
		influxWriteRequests.Inc()
		if err := influx.InsertHandler(r); err != nil {
			influxWriteErrors.Inc()
			httpserver.Errorf(w, "error in %q: %s", r.URL.Path, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/query":
		// Emulate fake response for influx query.
		// This is required for TSBS benchmark.
		influxQueryRequests.Inc()
		fmt.Fprintf(w, `{"results":[{"series":[{"values":[]}]}]}`)
		return true
	default:
		// This is not our link
		return false
	}
}

var (
	prometheusWriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/write", protocol="prometheus"}`)
	prometheusWriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/write", protocol="prometheus"}`)

	vmimportRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/import", protocol="vm"}`)
	vmimportErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/import", protocol="vm"}`)

	influxWriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/write", protocol="influx"}`)
	influxWriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/write", protocol="influx"}`)

	influxQueryRequests = metrics.NewCounter(`vm_http_requests_total{path="/query", protocol="influx"}`)
)
