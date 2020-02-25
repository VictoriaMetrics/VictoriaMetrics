package vminsert

import (
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/graphite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/prompush"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/promremotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/vmimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	graphiteserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/graphite"
	opentsdbserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/opentsdb"
	opentsdbhttpserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	graphiteListenAddr = flag.String("graphiteListenAddr", "", "TCP and UDP address to listen for Graphite plaintext data. Usually :2003 must be set. Doesn't work if empty")
	opentsdbListenAddr = flag.String("opentsdbListenAddr", "", "TCP and UDP address to listen for OpentTSDB metrics. "+
		"Telnet put messages and HTTP /api/put messages are simultaneously served on TCP port. "+
		"Usually :4242 must be set. Doesn't work if empty")
	opentsdbHTTPListenAddr = flag.String("opentsdbHTTPListenAddr", "", "TCP address to listen for OpentTSDB HTTP put requests. Usually :4242 must be set. Doesn't work if empty")
	maxLabelsPerTimeseries = flag.Int("maxLabelsPerTimeseries", 30, "The maximum number of labels accepted per time series. Superflouos labels are dropped")
)

var (
	graphiteServer     *graphiteserver.Server
	opentsdbServer     *opentsdbserver.Server
	opentsdbhttpServer *opentsdbhttpserver.Server
)

// Init initializes vminsert.
func Init() {
	storage.SetMaxLabelsPerTimeseries(*maxLabelsPerTimeseries)

	writeconcurrencylimiter.Init()
	if len(*graphiteListenAddr) > 0 {
		graphiteServer = graphiteserver.MustStart(*graphiteListenAddr, graphite.InsertHandler)
	}
	if len(*opentsdbListenAddr) > 0 {
		opentsdbServer = opentsdbserver.MustStart(*opentsdbListenAddr, opentsdb.InsertHandler, opentsdbhttp.InsertHandler)
	}
	if len(*opentsdbHTTPListenAddr) > 0 {
		opentsdbhttpServer = opentsdbhttpserver.MustStart(*opentsdbHTTPListenAddr, opentsdbhttp.InsertHandler)
	}
	promscrape.Init(prompush.Push)
}

// Stop stops vminsert.
func Stop() {
	promscrape.Stop()
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
		if err := promremotewrite.InsertHandler(r); err != nil {
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
	case "/targets":
		promscrapeTargetsRequests.Inc()
		w.Header().Set("Content-Type", "text/plain")
		promscrape.WriteHumanReadableTargetsStatus(w)
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

	promscrapeTargetsRequests = metrics.NewCounter(`vm_http_requests_total{path="/targets"}`)
)
