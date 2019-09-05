package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/concurrencylimiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/graphite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
)

var (
	graphiteListenAddr     = flag.String("graphiteListenAddr", "", "TCP and UDP address to listen for Graphite plaintext data. Usually :2003 must be set. Doesn't work if empty")
	opentsdbListenAddr     = flag.String("opentsdbListenAddr", "", "TCP and UDP address to listen for OpentTSDB put messages. Usually :4242 must be set. Doesn't work if empty")
	opentsdbHTTPListenAddr = flag.String("opentsdbHTTPListenAddr", "", "TCP address to listen for OpentTSDB HTTP put requests. Usually :4242 must be set. Doesn't work if empty")
	httpListenAddr         = flag.String("httpListenAddr", ":8480", "Address to listen for http connections")
	maxInsertRequestSize   = flag.Int("maxInsertRequestSize", 32*1024*1024, "The maximum size of a single insert request in bytes")
	maxLabelsPerTimeseries = flag.Int("maxLabelsPerTimeseries", 30, "The maximum number of labels accepted per time series. Superflouos labels are dropped")
	storageNodes           = flagutil.NewArray("storageNode", "Address of vmstorage nodes; usage: -storageNode=vmstorage-host1:8400 -storageNode=vmstorage-host2:8400")
)

func main() {
	flag.Parse()
	buildinfo.Init()
	logger.Init()

	logger.Infof("initializing netstorage for storageNodes %s...", *storageNodes)
	startTime := time.Now()
	if len(*storageNodes) == 0 {
		logger.Fatalf("missing -storageNode arg")
	}
	netstorage.InitStorageNodes(*storageNodes)
	logger.Infof("successfully initialized netstorage in %s", time.Since(startTime))

	storage.SetMaxLabelsPerTimeseries(*maxLabelsPerTimeseries)

	concurrencylimiter.Init()
	if len(*graphiteListenAddr) > 0 {
		go graphite.Serve(*graphiteListenAddr)
	}
	if len(*opentsdbListenAddr) > 0 {
		go opentsdb.Serve(*opentsdbListenAddr)
	}
	if len(*opentsdbHTTPListenAddr) > 0 {
		go opentsdbhttp.Serve(*opentsdbHTTPListenAddr, int64(*maxInsertRequestSize))
	}

	go func() {
		httpserver.Serve(*httpListenAddr, requestHandler)
	}()

	sig := procutil.WaitForSigterm()
	logger.Infof("service received signal %s", sig)

	logger.Infof("gracefully shutting down the service at %q", *httpListenAddr)
	startTime = time.Now()
	if err := httpserver.Stop(*httpListenAddr); err != nil {
		logger.Fatalf("cannot stop the service: %s", err)
	}
	logger.Infof("successfully shut down the service in %s", time.Since(startTime))

	if len(*graphiteListenAddr) > 0 {
		graphite.Stop()
	}
	if len(*opentsdbListenAddr) > 0 {
		opentsdb.Stop()
	}
	if len(*opentsdbHTTPListenAddr) > 0 {
		opentsdbhttp.Stop()
	}

	logger.Infof("shutting down neststorage...")
	startTime = time.Now()
	netstorage.Stop()
	logger.Infof("successfully stopped netstorage in %s", time.Since(startTime))

	fs.MustStopDirRemover()

	logger.Infof("the vminsert has been stopped")
}

func requestHandler(w http.ResponseWriter, r *http.Request) bool {
	p, err := httpserver.ParsePath(r.URL.Path)
	if err != nil {
		httpserver.Errorf(w, "cannot parse path %q: %s", r.URL.Path, err)
		return true
	}
	if p.Prefix != "insert" {
		// This is not our link.
		return false
	}
	at, err := auth.NewToken(p.AuthToken)
	if err != nil {
		httpserver.Errorf(w, "auth error: %s", err)
		return true
	}

	switch p.Suffix {
	case "prometheus/", "prometheus", "prometheus/api/v1/write":
		prometheusWriteRequests.Inc()
		if err := prometheus.InsertHandler(at, r, int64(*maxInsertRequestSize)); err != nil {
			prometheusWriteErrors.Inc()
			httpserver.Errorf(w, "error in %q: %s", r.URL.Path, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "influx/write", "influx/api/v2/write":
		influxWriteRequests.Inc()
		if err := influx.InsertHandler(at, r); err != nil {
			influxWriteErrors.Inc()
			httpserver.Errorf(w, "error in %q: %s", r.URL.Path, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "influx/query":
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
	prometheusWriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/prometheus/", protocol="prometheus"}`)
	prometheusWriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/insert/{}/prometheus/", protocol="prometheus"}`)

	influxWriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/influx/", protocol="influx"}`)
	influxWriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/insert/{}/influx/", protocol="influx"}`)

	influxQueryRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/influx/query", protocol="influx"}`)
)
