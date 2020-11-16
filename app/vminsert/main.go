package vminsert

import (
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/csvimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/graphite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/native"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/prometheusimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/prompush"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/promremotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/vmimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	graphiteserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/graphite"
	influxserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/influx"
	opentsdbserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/opentsdb"
	opentsdbhttpserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	graphiteListenAddr = flag.String("graphiteListenAddr", "", "TCP and UDP address to listen for Graphite plaintext data. Usually :2003 must be set. Doesn't work if empty")
	influxListenAddr   = flag.String("influxListenAddr", "", "TCP and UDP address to listen for Influx line protocol data. Usually :8189 must be set. Doesn't work if empty. "+
		"This flag isn't needed when ingesting data over HTTP - just send it to `http://<victoriametrics>:8428/write`")
	opentsdbListenAddr = flag.String("opentsdbListenAddr", "", "TCP and UDP address to listen for OpentTSDB metrics. "+
		"Telnet put messages and HTTP /api/put messages are simultaneously served on TCP port. "+
		"Usually :4242 must be set. Doesn't work if empty")
	opentsdbHTTPListenAddr = flag.String("opentsdbHTTPListenAddr", "", "TCP address to listen for OpentTSDB HTTP put requests. Usually :4242 must be set. Doesn't work if empty")
	maxLabelsPerTimeseries = flag.Int("maxLabelsPerTimeseries", 30, "The maximum number of labels accepted per time series. Superflouos labels are dropped")
)

var (
	influxServer       *influxserver.Server
	graphiteServer     *graphiteserver.Server
	opentsdbServer     *opentsdbserver.Server
	opentsdbhttpServer *opentsdbhttpserver.Server
)

// Init initializes vminsert.
func Init() {
	relabel.Init()
	storage.SetMaxLabelsPerTimeseries(*maxLabelsPerTimeseries)
	common.StartUnmarshalWorkers()
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
	promscrape.Init(prompush.Push)
}

// Stop stops vminsert.
func Stop() {
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
	common.StopUnmarshalWorkers()
}

// RequestHandler is a handler for Prometheus remote storage write API
func RequestHandler(w http.ResponseWriter, r *http.Request) bool {
	path := strings.Replace(r.URL.Path, "//", "/", -1)
	switch path {
	case "/api/v1/write":
		prometheusWriteRequests.Inc()
		if err := promremotewrite.InsertHandler(r); err != nil {
			prometheusWriteErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/api/v1/import":
		vmimportRequests.Inc()
		if err := vmimport.InsertHandler(r); err != nil {
			vmimportErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/api/v1/import/csv":
		csvimportRequests.Inc()
		if err := csvimport.InsertHandler(r); err != nil {
			csvimportErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/api/v1/import/prometheus":
		prometheusimportRequests.Inc()
		if err := prometheusimport.InsertHandler(r); err != nil {
			prometheusimportErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/api/v1/import/native":
		nativeimportRequests.Inc()
		if err := native.InsertHandler(r); err != nil {
			nativeimportErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/write", "/api/v2/write":
		influxWriteRequests.Inc()
		if err := influx.InsertHandlerForHTTP(r); err != nil {
			influxWriteErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
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
	case "/tags/tagSeries":
		graphiteTagsTagSeriesRequests.Inc()
		if err := graphite.TagsTagSeriesHandler(w, r); err != nil {
			graphiteTagsTagSeriesErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/tags/tagMultiSeries":
		graphiteTagsTagMultiSeriesRequests.Inc()
		if err := graphite.TagsTagMultiSeriesHandler(w, r); err != nil {
			graphiteTagsTagMultiSeriesErrors.Inc()
			httpserver.Errorf(w, r, "error in %q: %s", r.URL.Path, err)
			return true
		}
		return true
	case "/targets":
		promscrapeTargetsRequests.Inc()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		showOriginalLabels, _ := strconv.ParseBool(r.FormValue("show_original_labels"))
		promscrape.WriteHumanReadableTargetsStatus(w, showOriginalLabels)
		return true
	case "/api/v1/targets":
		promscrapeAPIV1TargetsRequests.Inc()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		state := r.FormValue("state")
		promscrape.WriteAPIV1Targets(w, state)
		return true
	case "/-/reload":
		promscrapeConfigReloadRequests.Inc()
		procutil.SelfSIGHUP()
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/ready":
		if rdy := atomic.LoadInt32(&promscrape.PendingScrapeConfigs); rdy > 0 {
			errMsg := fmt.Sprintf("waiting for scrape config to init targets, configs left: %d", rdy)
			http.Error(w, errMsg, http.StatusTooEarly)
		} else {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}
		return true
	default:
		// This is not our link
		return false
	}
}

var (
	prometheusWriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/write", protocol="promremotewrite"}`)
	prometheusWriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/write", protocol="promremotewrite"}`)

	vmimportRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/import", protocol="vmimport"}`)
	vmimportErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/import", protocol="vmimport"}`)

	csvimportRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/import/csv", protocol="csvimport"}`)
	csvimportErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/import/csv", protocol="csvimport"}`)

	prometheusimportRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/import/prometheus", protocol="prometheusimport"}`)
	prometheusimportErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/import/prometheus", protocol="prometheusimport"}`)

	nativeimportRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/import/native", protocol="nativeimport"}`)
	nativeimportErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/v1/import/native", protocol="nativeimport"}`)

	influxWriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/write", protocol="influx"}`)
	influxWriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/write", protocol="influx"}`)

	influxQueryRequests = metrics.NewCounter(`vm_http_requests_total{path="/query", protocol="influx"}`)

	graphiteTagsTagSeriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/tags/tagSeries", protocol="graphite"}`)
	graphiteTagsTagSeriesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/tags/tagSeries", protocol="graphite"}`)

	graphiteTagsTagMultiSeriesRequests = metrics.NewCounter(`vm_http_requests_total{path="/tags/tagMultiSeries", protocol="graphite"}`)
	graphiteTagsTagMultiSeriesErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/tags/tagMultiSeries", protocol="graphite"}`)

	promscrapeTargetsRequests      = metrics.NewCounter(`vm_http_requests_total{path="/targets"}`)
	promscrapeAPIV1TargetsRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/targets"}`)

	promscrapeConfigReloadRequests = metrics.NewCounter(`vm_http_requests_total{path="/-/reload"}`)

	_ = metrics.NewGauge(`vm_metrics_with_dropped_labels_total`, func() float64 {
		return float64(atomic.LoadUint64(&storage.MetricsWithDroppedLabels))
	})
	_ = metrics.NewGauge(`vm_too_long_label_names_total`, func() float64 {
		return float64(atomic.LoadUint64(&storage.TooLongLabelNames))
	})
	_ = metrics.NewGauge(`vm_too_long_label_values_total`, func() float64 {
		return float64(atomic.LoadUint64(&storage.TooLongLabelValues))
	})
)
