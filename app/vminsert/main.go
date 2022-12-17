package vminsert

import (
	"embed"
	"flag"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/csvimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/datadog"
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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/influxutils"
	graphiteserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/graphite"
	influxserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/influx"
	opentsdbserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/opentsdb"
	opentsdbhttpserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	graphiteListenAddr = flag.String("graphiteListenAddr", "", "TCP and UDP address to listen for Graphite plaintext data. Usually :2003 must be set. Doesn't work if empty")
	influxListenAddr   = flag.String("influxListenAddr", "", "TCP and UDP address to listen for InfluxDB line protocol data. Usually :8089 must be set. Doesn't work if empty. "+
		"This flag isn't needed when ingesting data over HTTP - just send it to http://<victoriametrics>:8428/write")
	opentsdbListenAddr = flag.String("opentsdbListenAddr", "", "TCP and UDP address to listen for OpentTSDB metrics. "+
		"Telnet put messages and HTTP /api/put messages are simultaneously served on TCP port. "+
		"Usually :4242 must be set. Doesn't work if empty")
	opentsdbHTTPListenAddr = flag.String("opentsdbHTTPListenAddr", "", "TCP address to listen for OpentTSDB HTTP put requests. Usually :4242 must be set. Doesn't work if empty")
	configAuthKey          = flag.String("configAuthKey", "", "Authorization key for accessing /config page. It must be passed via authKey query arg")
	maxLabelsPerTimeseries = flag.Int("maxLabelsPerTimeseries", 30, "The maximum number of labels accepted per time series. Superfluous labels are dropped. In this case the vm_metrics_with_dropped_labels_total metric at /metrics page is incremented")
	maxLabelValueLen       = flag.Int("maxLabelValueLen", 16*1024, "The maximum length of label values in the accepted time series. Longer label values are truncated. In this case the vm_too_long_label_values_total metric at /metrics page is incremented")
)

var (
	graphiteServer     *graphiteserver.Server
	influxServer       *influxserver.Server
	opentsdbServer     *opentsdbserver.Server
	opentsdbhttpServer *opentsdbhttpserver.Server
)

//go:embed static
var staticFiles embed.FS

var staticServer = http.FileServer(http.FS(staticFiles))

// Init initializes vminsert.
func Init() {
	relabel.Init()
	storage.SetMaxLabelsPerTimeseries(*maxLabelsPerTimeseries)
	storage.SetMaxLabelValueLen(*maxLabelValueLen)
	common.StartUnmarshalWorkers()
	writeconcurrencylimiter.Init()
	if len(*graphiteListenAddr) > 0 {
		graphiteServer = graphiteserver.MustStart(*graphiteListenAddr, graphite.InsertHandler)
	}
	if len(*influxListenAddr) > 0 {
		influxServer = influxserver.MustStart(*influxListenAddr, influx.InsertHandlerForReader)
	}
	if len(*opentsdbListenAddr) > 0 {
		opentsdbServer = opentsdbserver.MustStart(*opentsdbListenAddr, opentsdb.InsertHandler, opentsdbhttp.InsertHandler)
	}
	if len(*opentsdbHTTPListenAddr) > 0 {
		opentsdbhttpServer = opentsdbhttpserver.MustStart(*opentsdbHTTPListenAddr, opentsdbhttp.InsertHandler)
	}
	promscrape.Init(func(at *auth.Token, wr *prompbmarshal.WriteRequest) {
		prompush.Push(wr)
	})
}

// Stop stops vminsert.
func Stop() {
	promscrape.Stop()
	if len(*graphiteListenAddr) > 0 {
		graphiteServer.MustStop()
	}
	if len(*influxListenAddr) > 0 {
		influxServer.MustStop()
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
	startTime := time.Now()
	defer requestDuration.UpdateDuration(startTime)

	path := strings.Replace(r.URL.Path, "//", "/", -1)
	if strings.HasPrefix(path, "/static") {
		staticServer.ServeHTTP(w, r)
		return true
	}
	if strings.HasPrefix(path, "/prometheus/static") {
		r.URL.Path = strings.TrimPrefix(path, "/prometheus")
		staticServer.ServeHTTP(w, r)
		return true
	}
	if strings.HasPrefix(path, "/prometheus/api/v1/import/prometheus") || strings.HasPrefix(path, "/api/v1/import/prometheus") {
		prometheusimportRequests.Inc()
		if err := prometheusimport.InsertHandler(r); err != nil {
			prometheusimportErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	if strings.HasPrefix(path, "/datadog/") {
		// Trim suffix from paths starting from /datadog/ in order to support legacy DataDog agent.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2670
		path = strings.TrimSuffix(path, "/")
	}
	switch path {
	case "/prometheus/api/v1/write", "/api/v1/write":
		prometheusWriteRequests.Inc()
		if err := promremotewrite.InsertHandler(r); err != nil {
			prometheusWriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/prometheus/api/v1/import", "/api/v1/import":
		vmimportRequests.Inc()
		if err := vmimport.InsertHandler(r); err != nil {
			vmimportErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/prometheus/api/v1/import/csv", "/api/v1/import/csv":
		csvimportRequests.Inc()
		if err := csvimport.InsertHandler(r); err != nil {
			csvimportErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/prometheus/api/v1/import/native", "/api/v1/import/native":
		nativeimportRequests.Inc()
		if err := native.InsertHandler(r); err != nil {
			nativeimportErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/influx/write", "/influx/api/v2/write", "/write", "/api/v2/write":
		influxWriteRequests.Inc()
		addInfluxResponseHeaders(w)
		if err := influx.InsertHandlerForHTTP(r); err != nil {
			influxWriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/influx/query", "/query":
		influxQueryRequests.Inc()
		addInfluxResponseHeaders(w)
		influxutils.WriteDatabaseNames(w)
		return true
	case "/datadog/api/v1/series":
		datadogWriteRequests.Inc()
		if err := datadog.InsertHandlerForHTTP(r); err != nil {
			datadogWriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		fmt.Fprintf(w, `{"status":"ok"}`)
		return true
	case "/datadog/api/v1/validate":
		datadogValidateRequests.Inc()
		// See https://docs.datadoghq.com/api/latest/authentication/#validate-api-key
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"valid":true}`)
		return true
	case "/datadog/api/v1/check_run":
		datadogCheckRunRequests.Inc()
		// See https://docs.datadoghq.com/api/latest/service-checks/#submit-a-service-check
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		fmt.Fprintf(w, `{"status":"ok"}`)
		return true
	case "/datadog/intake":
		datadogIntakeRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{}`)
		return true
	case "/datadog/api/v1/metadata":
		datadogMetadataRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{}`)
		return true
	case "/prometheus/targets", "/targets":
		promscrapeTargetsRequests.Inc()
		promscrape.WriteHumanReadableTargetsStatus(w, r)
		return true
	case "/prometheus/service-discovery", "/service-discovery":
		promscrapeServiceDiscoveryRequests.Inc()
		promscrape.WriteServiceDiscovery(w, r)
		return true
	case "/prometheus/api/v1/targets", "/api/v1/targets":
		promscrapeAPIV1TargetsRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		state := r.FormValue("state")
		promscrape.WriteAPIV1Targets(w, state)
		return true
	case "/prometheus/target_response", "/target_response":
		promscrapeTargetResponseRequests.Inc()
		if err := promscrape.WriteTargetResponse(w, r); err != nil {
			promscrapeTargetResponseErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		return true
	case "/prometheus/config", "/config":
		if *configAuthKey != "" && r.FormValue("authKey") != *configAuthKey {
			err := &httpserver.ErrorWithStatusCode{
				Err:        fmt.Errorf("The provided authKey doesn't match -configAuthKey"),
				StatusCode: http.StatusUnauthorized,
			}
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		promscrapeConfigRequests.Inc()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		promscrape.WriteConfigData(w)
		return true
	case "/prometheus/api/v1/status/config", "/api/v1/status/config":
		// See https://prometheus.io/docs/prometheus/latest/querying/api/#config
		if *configAuthKey != "" && r.FormValue("authKey") != *configAuthKey {
			err := &httpserver.ErrorWithStatusCode{
				Err:        fmt.Errorf("The provided authKey doesn't match -configAuthKey"),
				StatusCode: http.StatusUnauthorized,
			}
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		promscrapeStatusConfigRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		var bb bytesutil.ByteBuffer
		promscrape.WriteConfigData(&bb)
		fmt.Fprintf(w, `{"status":"success","data":{"yaml":%q}}`, bb.B)
		return true
	case "/prometheus/-/reload", "/-/reload":
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

func addInfluxResponseHeaders(w http.ResponseWriter) {
	// This is needed for some clients, which expect InfluxDB version header.
	// See, for example, https://github.com/ntop/ntopng/issues/5449#issuecomment-1005347597
	w.Header().Set("X-Influxdb-Version", "1.8.0")
}

var (
	requestDuration = metrics.NewHistogram(`vminsert_request_duration_seconds`)

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

	influxWriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/influx/write", protocol="influx"}`)
	influxWriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/influx/write", protocol="influx"}`)

	influxQueryRequests = metrics.NewCounter(`vm_http_requests_total{path="/influx/query", protocol="influx"}`)

	datadogWriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/datadog/api/v1/series", protocol="datadog"}`)
	datadogWriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/datadog/api/v1/series", protocol="datadog"}`)

	datadogValidateRequests = metrics.NewCounter(`vm_http_requests_total{path="/datadog/api/v1/validate", protocol="datadog"}`)
	datadogCheckRunRequests = metrics.NewCounter(`vm_http_requests_total{path="/datadog/api/v1/check_run", protocol="datadog"}`)
	datadogIntakeRequests   = metrics.NewCounter(`vm_http_requests_total{path="/datadog/intake", protocol="datadog"}`)
	datadogMetadataRequests = metrics.NewCounter(`vm_http_requests_total{path="/datadog/api/v1/metadata", protocol="datadog"}`)

	promscrapeTargetsRequests          = metrics.NewCounter(`vm_http_requests_total{path="/targets"}`)
	promscrapeServiceDiscoveryRequests = metrics.NewCounter(`vm_http_requests_total{path="/service-discovery"}`)

	promscrapeAPIV1TargetsRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/targets"}`)

	promscrapeTargetResponseRequests = metrics.NewCounter(`vm_http_requests_total{path="/target_response"}`)
	promscrapeTargetResponseErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/target_response"}`)

	promscrapeConfigRequests       = metrics.NewCounter(`vm_http_requests_total{path="/config"}`)
	promscrapeStatusConfigRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/status/config"}`)

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
