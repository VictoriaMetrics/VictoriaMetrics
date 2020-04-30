package main

import (
	"flag"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/csvimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/graphite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/promremotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/vmimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	graphiteserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/graphite"
	influxserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/influx"
	opentsdbserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/opentsdb"
	opentsdbhttpserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	httpListenAddr = flag.String("httpListenAddr", ":8429", "TCP address to listen for http connections. "+
		"Set this flag to empty value in order to disable listening on any port. This mode may be useful for running multiple vmagent instances on the same server. "+
		"Note that /targets and /metrics pages aren't available if -httpListenAddr=''")
	influxListenAddr   = flag.String("influxListenAddr", "", "TCP and UDP address to listen for Influx line protocol data. Usually :8189 must be set. Doesn't work if empty")
	graphiteListenAddr = flag.String("graphiteListenAddr", "", "TCP and UDP address to listen for Graphite plaintext data. Usually :2003 must be set. Doesn't work if empty")
	opentsdbListenAddr = flag.String("opentsdbListenAddr", "", "TCP and UDP address to listen for OpentTSDB metrics. "+
		"Telnet put messages and HTTP /api/put messages are simultaneously served on TCP port. "+
		"Usually :4242 must be set. Doesn't work if empty")
	opentsdbHTTPListenAddr = flag.String("opentsdbHTTPListenAddr", "", "TCP address to listen for OpentTSDB HTTP put requests. Usually :4242 must be set. Doesn't work if empty")
)

var (
	influxServer       *influxserver.Server
	graphiteServer     *graphiteserver.Server
	opentsdbServer     *opentsdbserver.Server
	opentsdbhttpServer *opentsdbhttpserver.Server
)

func main() {
	envflag.Parse()
	buildinfo.Init()
	logger.Init()
	logger.Infof("starting vmagent at %q...", *httpListenAddr)
	startTime := time.Now()
	remotewrite.Init()
	writeconcurrencylimiter.Init()
	if len(*influxListenAddr) > 0 {
		influxServer = influxserver.MustStart(*influxListenAddr, influx.InsertHandlerForReader)
	}
	if len(*graphiteListenAddr) > 0 {
		graphiteServer = graphiteserver.MustStart(*graphiteListenAddr, graphite.InsertHandler)
	}
	if len(*opentsdbListenAddr) > 0 {
		opentsdbServer = opentsdbserver.MustStart(*opentsdbListenAddr, opentsdb.InsertHandler, opentsdbhttp.InsertHandler)
	}
	if len(*opentsdbHTTPListenAddr) > 0 {
		opentsdbhttpServer = opentsdbhttpserver.MustStart(*opentsdbHTTPListenAddr, opentsdbhttp.InsertHandler)
	}

	promscrape.Init(remotewrite.Push)

	if len(*httpListenAddr) > 0 {
		go httpserver.Serve(*httpListenAddr, requestHandler)
	}
	logger.Infof("started vmagent in %.3f seconds", time.Since(startTime).Seconds())

	sig := procutil.WaitForSigterm()
	logger.Infof("received signal %s", sig)

	startTime = time.Now()
	if len(*httpListenAddr) > 0 {
		logger.Infof("gracefully shutting down webservice at %q", *httpListenAddr)
		if err := httpserver.Stop(*httpListenAddr); err != nil {
			logger.Fatalf("cannot stop the webservice: %s", err)
		}
		logger.Infof("successfully shut down the webservice in %.3f seconds", time.Since(startTime).Seconds())
	}

	promscrape.Stop()

	if len(*influxListenAddr) > 0 {
		influxServer.MustStop()
	}
	if len(*graphiteListenAddr) > 0 {
		graphiteServer.MustStop()
	}
	if len(*opentsdbListenAddr) > 0 {
		opentsdbServer.MustStop()
	}
	if len(*opentsdbHTTPListenAddr) > 0 {
		opentsdbhttpServer.MustStop()
	}
	remotewrite.Stop()

	logger.Infof("successfully stopped vmagent in %.3f seconds", time.Since(startTime).Seconds())
}

func requestHandler(w http.ResponseWriter, r *http.Request) bool {
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
	case "/api/v1/import/csv":
		csvimportRequests.Inc()
		if err := csvimport.InsertHandler(r); err != nil {
			csvimportErrors.Inc()
			httpserver.Errorf(w, "error in %q: %s", r.URL.Path, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/write", "/api/v2/write":
		influxWriteRequests.Inc()
		if err := influx.InsertHandlerForHTTP(r); err != nil {
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
	case "/-/reload":
		promscrapeConfigReloadRequests.Inc()
		procutil.SelfSIGHUP()
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

var (
	prometheusWriteRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/api/v1/write", protocol="promremotewrite"}`)
	prometheusWriteErrors   = metrics.NewCounter(`vmagent_http_request_errors_total{path="/api/v1/write", protocol="promremotewrite"}`)

	vmimportRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/api/v1/import", protocol="vmimport"}`)
	vmimportErrors   = metrics.NewCounter(`vmagent_http_request_errors_total{path="/api/v1/import", protocol="vmimport"}`)

	csvimportRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/api/v1/import/csv", protocol="csvimport"}`)
	csvimportErrors   = metrics.NewCounter(`vmagent_http_request_errors_total{path="/api/v1/import/csv", protocol="csvimport"}`)

	influxWriteRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/write", protocol="influx"}`)
	influxWriteErrors   = metrics.NewCounter(`vmagent_http_request_errors_total{path="/write", protocol="influx"}`)

	influxQueryRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/query", protocol="influx"}`)

	promscrapeTargetsRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/targets"}`)

	promscrapeConfigReloadRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/-/reload"}`)
)
