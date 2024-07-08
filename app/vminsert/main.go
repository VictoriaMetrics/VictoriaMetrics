package vminsert

import (
	"embed"
	"flag"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/VictoriaMetrics/metrics"

	vminsertCommon "github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/csvimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/datadogsketches"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/datadogv1"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/datadogv2"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/graphite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/native"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/newrelic"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/opentelemetry"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/prometheusimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/prompush"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/promremotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/vmimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/firehose"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

var (
	graphiteListenAddr = flag.String("graphiteListenAddr", "", "TCP and UDP address to listen for Graphite plaintext data. Usually :2003 must be set. Doesn't work if empty. "+
		"See also -graphiteListenAddr.useProxyProtocol")
	graphiteUseProxyProtocol = flag.Bool("graphiteListenAddr.useProxyProtocol", false, "Whether to use proxy protocol for connections accepted at -graphiteListenAddr . "+
		"See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt")
	influxListenAddr = flag.String("influxListenAddr", "", "TCP and UDP address to listen for InfluxDB line protocol data. Usually :8089 must be set. Doesn't work if empty. "+
		"This flag isn't needed when ingesting data over HTTP - just send it to http://<victoriametrics>:8428/write . "+
		"See also -influxListenAddr.useProxyProtocol")
	influxUseProxyProtocol = flag.Bool("influxListenAddr.useProxyProtocol", false, "Whether to use proxy protocol for connections accepted at -influxListenAddr . "+
		"See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt")
	opentsdbListenAddr = flag.String("opentsdbListenAddr", "", "TCP and UDP address to listen for OpenTSDB metrics. "+
		"Telnet put messages and HTTP /api/put messages are simultaneously served on TCP port. "+
		"Usually :4242 must be set. Doesn't work if empty. "+
		"See also -opentsdbListenAddr.useProxyProtocol")
	opentsdbUseProxyProtocol = flag.Bool("opentsdbListenAddr.useProxyProtocol", false, "Whether to use proxy protocol for connections accepted at -opentsdbListenAddr . "+
		"See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt")
	opentsdbHTTPListenAddr = flag.String("opentsdbHTTPListenAddr", "", "TCP address to listen for OpenTSDB HTTP put requests. Usually :4242 must be set. Doesn't work if empty. "+
		"See also -opentsdbHTTPListenAddr.useProxyProtocol")
	opentsdbHTTPUseProxyProtocol = flag.Bool("opentsdbHTTPListenAddr.useProxyProtocol", false, "Whether to use proxy protocol for connections accepted "+
		"at -opentsdbHTTPListenAddr . See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt")
	configAuthKey          = flagutil.NewPassword("configAuthKey", "Authorization key for accessing /config page. It must be passed via authKey query arg. It overrides httpAuth.* settings.")
	reloadAuthKey          = flagutil.NewPassword("reloadAuthKey", "Auth key for /-/reload http endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings.")
	maxLabelsPerTimeseries = flag.Int("maxLabelsPerTimeseries", 30, "The maximum number of labels accepted per time series. Superfluous labels are dropped. In this case the vm_metrics_with_dropped_labels_total metric at /metrics page is incremented")
	maxLabelValueLen       = flag.Int("maxLabelValueLen", 1024, "The maximum length of label values in the accepted time series. Longer label values are truncated. In this case the vm_too_long_label_values_total metric at /metrics page is incremented")
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
	vminsertCommon.InitStreamAggr()
	storage.SetMaxLabelsPerTimeseries(*maxLabelsPerTimeseries)
	storage.SetMaxLabelValueLen(*maxLabelValueLen)
	common.StartUnmarshalWorkers()
	if len(*graphiteListenAddr) > 0 {
		graphiteServer = graphiteserver.MustStart(*graphiteListenAddr, *graphiteUseProxyProtocol, graphite.InsertHandler)
	}
	if len(*influxListenAddr) > 0 {
		influxServer = influxserver.MustStart(*influxListenAddr, *influxUseProxyProtocol, influx.InsertHandlerForReader)
	}
	if len(*opentsdbListenAddr) > 0 {
		opentsdbServer = opentsdbserver.MustStart(*opentsdbListenAddr, *opentsdbUseProxyProtocol, opentsdb.InsertHandler, opentsdbhttp.InsertHandler)
	}
	if len(*opentsdbHTTPListenAddr) > 0 {
		opentsdbhttpServer = opentsdbhttpserver.MustStart(*opentsdbHTTPListenAddr, *opentsdbHTTPUseProxyProtocol, opentsdbhttp.InsertHandler)
	}
	promscrape.Init(func(_ *auth.Token, wr *prompbmarshal.WriteRequest) {
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
	vminsertCommon.MustStopStreamAggr()
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
		statusCode := http.StatusNoContent
		if strings.HasPrefix(path, "/prometheus/api/v1/import/prometheus/metrics/job/") ||
			strings.HasPrefix(path, "/api/v1/import/prometheus/metrics/job/") {
			// Return 200 status code for pushgateway requests.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3636
			statusCode = http.StatusOK
		}
		w.WriteHeader(statusCode)
		return true
	}
	if strings.HasPrefix(path, "/datadog/") {
		// Trim suffix from paths starting from /datadog/ in order to support legacy DataDog agent.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2670
		path = strings.TrimSuffix(path, "/")
	}
	switch path {
	case "/prometheus/api/v1/write", "/api/v1/write", "/api/v1/push", "/prometheus/api/v1/push":
		if common.HandleVMProtoServerHandshake(w, r) {
			return true
		}
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
	case "/opentelemetry/api/v1/push", "/opentelemetry/v1/metrics":
		opentelemetryPushRequests.Inc()
		if err := opentelemetry.InsertHandler(r); err != nil {
			opentelemetryPushErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		firehose.WriteSuccessResponse(w, r)
		return true
	case "/newrelic":
		newrelicCheckRequest.Inc()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		fmt.Fprintf(w, `{"status":"ok"}`)
		return true
	case "/newrelic/inventory/deltas":
		newrelicInventoryRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		fmt.Fprintf(w, `{"payload":{"version": 1, "state": {}, "reset": "false"}}`)
		return true
	case "/newrelic/infra/v2/metrics/events/bulk":
		newrelicWriteRequests.Inc()
		if err := newrelic.InsertHandlerForHTTP(r); err != nil {
			newrelicWriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		fmt.Fprintf(w, `{"status":"ok"}`)
		return true
	case "/datadog/api/v1/series":
		datadogv1WriteRequests.Inc()
		if err := datadogv1.InsertHandlerForHTTP(r); err != nil {
			datadogv1WriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		fmt.Fprintf(w, `{"status":"ok"}`)
		return true
	case "/datadog/api/v2/series":
		datadogv2WriteRequests.Inc()
		if err := datadogv2.InsertHandlerForHTTP(r); err != nil {
			datadogv2WriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		fmt.Fprintf(w, `{"status":"ok"}`)
		return true
	case "/datadog/api/beta/sketches":
		datadogsketchesWriteRequests.Inc()
		if err := datadogsketches.InsertHandlerForHTTP(r); err != nil {
			datadogsketchesWriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(202)
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
		if !httpserver.CheckAuthFlag(w, r, configAuthKey.Get(), "configAuthKey") {
			return true
		}
		promscrapeConfigRequests.Inc()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		promscrape.WriteConfigData(w)
		return true
	case "/prometheus/api/v1/status/config", "/api/v1/status/config":
		// See https://prometheus.io/docs/prometheus/latest/querying/api/#config
		if !httpserver.CheckAuthFlag(w, r, configAuthKey.Get(), "configAuthKey") {
			return true
		}
		promscrapeStatusConfigRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		var bb bytesutil.ByteBuffer
		promscrape.WriteConfigData(&bb)
		fmt.Fprintf(w, `{"status":"success","data":{"yaml":%q}}`, bb.B)
		return true
	case "/prometheus/-/reload", "/-/reload":
		if !httpserver.CheckAuthFlag(w, r, reloadAuthKey.Get(), "reloadAuthKey") {
			return true
		}
		promscrapeConfigReloadRequests.Inc()
		procutil.SelfSIGHUP()
		w.WriteHeader(http.StatusOK)
		return true
	case "/ready":
		if rdy := promscrape.PendingScrapeConfigs.Load(); rdy > 0 {
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

	datadogv1WriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/datadog/api/v1/series", protocol="datadog"}`)
	datadogv1WriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/datadog/api/v1/series", protocol="datadog"}`)

	datadogv2WriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/datadog/api/v2/series", protocol="datadog"}`)
	datadogv2WriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/datadog/api/v2/series", protocol="datadog"}`)

	datadogsketchesWriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/datadog/api/beta/sketches", protocol="datadog"}`)
	datadogsketchesWriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/datadog/api/beta/sketches", protocol="datadog"}`)

	datadogValidateRequests = metrics.NewCounter(`vm_http_requests_total{path="/datadog/api/v1/validate", protocol="datadog"}`)
	datadogCheckRunRequests = metrics.NewCounter(`vm_http_requests_total{path="/datadog/api/v1/check_run", protocol="datadog"}`)
	datadogIntakeRequests   = metrics.NewCounter(`vm_http_requests_total{path="/datadog/intake", protocol="datadog"}`)
	datadogMetadataRequests = metrics.NewCounter(`vm_http_requests_total{path="/datadog/api/v1/metadata", protocol="datadog"}`)

	opentelemetryPushRequests = metrics.NewCounter(`vm_http_requests_total{path="/opentelemetry/v1/metrics", protocol="opentelemetry"}`)
	opentelemetryPushErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/opentelemetry/v1/metrics", protocol="opentelemetry"}`)

	newrelicWriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/newrelic/infra/v2/metrics/events/bulk", protocol="newrelic"}`)
	newrelicWriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/newrelic/infra/v2/metrics/events/bulk", protocol="newrelic"}`)

	newrelicInventoryRequests = metrics.NewCounter(`vm_http_requests_total{path="/newrelic/inventory/deltas", protocol="newrelic"}`)
	newrelicCheckRequest      = metrics.NewCounter(`vm_http_requests_total{path="/newrelic", protocol="newrelic"}`)

	promscrapeTargetsRequests          = metrics.NewCounter(`vm_http_requests_total{path="/targets"}`)
	promscrapeServiceDiscoveryRequests = metrics.NewCounter(`vm_http_requests_total{path="/service-discovery"}`)

	promscrapeAPIV1TargetsRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/targets"}`)

	promscrapeTargetResponseRequests = metrics.NewCounter(`vm_http_requests_total{path="/target_response"}`)
	promscrapeTargetResponseErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/target_response"}`)

	promscrapeConfigRequests       = metrics.NewCounter(`vm_http_requests_total{path="/config"}`)
	promscrapeStatusConfigRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/v1/status/config"}`)

	promscrapeConfigReloadRequests = metrics.NewCounter(`vm_http_requests_total{path="/-/reload"}`)

	_ = metrics.NewGauge(`vm_metrics_with_dropped_labels_total`, func() float64 {
		return float64(storage.MetricsWithDroppedLabels.Load())
	})
	_ = metrics.NewGauge(`vm_too_long_label_names_total`, func() float64 {
		return float64(storage.TooLongLabelNames.Load())
	})
	_ = metrics.NewGauge(`vm_too_long_label_values_total`, func() float64 {
		return float64(storage.TooLongLabelValues.Load())
	})
)
