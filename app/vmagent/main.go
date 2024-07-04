package main

import (
	"embed"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/csvimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/datadogsketches"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/datadogv1"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/datadogv2"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/graphite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/native"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/newrelic"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/opentelemetry"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/prometheusimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/promremotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/vmimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/influxutils"
	graphiteserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/graphite"
	influxserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/influx"
	opentsdbserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/opentsdb"
	opentsdbhttpserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/firehose"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pushmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	httpListenAddrs = flagutil.NewArrayString("httpListenAddr", "TCP address to listen for incoming http requests. "+
		"Set this flag to empty value in order to disable listening on any port. This mode may be useful for running multiple vmagent instances on the same server. "+
		"Note that /targets and /metrics pages aren't available if -httpListenAddr=''. See also -tls and -httpListenAddr.useProxyProtocol")
	useProxyProtocol = flagutil.NewArrayBool("httpListenAddr.useProxyProtocol", "Whether to use proxy protocol for connections accepted at the corresponding -httpListenAddr . "+
		"See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . "+
		"With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing")
	influxListenAddr = flag.String("influxListenAddr", "", "TCP and UDP address to listen for InfluxDB line protocol data. Usually :8089 must be set. Doesn't work if empty. "+
		"This flag isn't needed when ingesting data over HTTP - just send it to http://<vmagent>:8429/write . "+
		"See also -influxListenAddr.useProxyProtocol")
	influxUseProxyProtocol = flag.Bool("influxListenAddr.useProxyProtocol", false, "Whether to use proxy protocol for connections accepted at -influxListenAddr . "+
		"See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt")
	graphiteListenAddr = flag.String("graphiteListenAddr", "", "TCP and UDP address to listen for Graphite plaintext data. Usually :2003 must be set. Doesn't work if empty. "+
		"See also -graphiteListenAddr.useProxyProtocol")
	graphiteUseProxyProtocol = flag.Bool("graphiteListenAddr.useProxyProtocol", false, "Whether to use proxy protocol for connections accepted at -graphiteListenAddr . "+
		"See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt")
	opentsdbListenAddr = flag.String("opentsdbListenAddr", "", "TCP and UDP address to listen for OpenTSDB metrics. "+
		"Telnet put messages and HTTP /api/put messages are simultaneously served on TCP port. "+
		"Usually :4242 must be set. Doesn't work if empty. See also -opentsdbListenAddr.useProxyProtocol")
	opentsdbUseProxyProtocol = flag.Bool("opentsdbListenAddr.useProxyProtocol", false, "Whether to use proxy protocol for connections accepted at -opentsdbListenAddr . "+
		"See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt")
	opentsdbHTTPListenAddr = flag.String("opentsdbHTTPListenAddr", "", "TCP address to listen for OpenTSDB HTTP put requests. Usually :4242 must be set. Doesn't work if empty. "+
		"See also -opentsdbHTTPListenAddr.useProxyProtocol")
	opentsdbHTTPUseProxyProtocol = flag.Bool("opentsdbHTTPListenAddr.useProxyProtocol", false, "Whether to use proxy protocol for connections accepted "+
		"at -opentsdbHTTPListenAddr . See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt")
	configAuthKey = flagutil.NewPassword("configAuthKey", "Authorization key for accessing /config page. It must be passed via authKey query arg. It overrides httpAuth.* settings.")
	reloadAuthKey = flagutil.NewPassword("reloadAuthKey", "Auth key for /-/reload http endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings.")
	dryRun        = flag.Bool("dryRun", false, "Whether to check config files without running vmagent. The following files are checked: "+
		"-promscrape.config, -remoteWrite.relabelConfig, -remoteWrite.urlRelabelConfig, -remoteWrite.streamAggr.config . "+
		"Unknown config entries aren't allowed in -promscrape.config by default. This can be changed by passing -promscrape.config.strictParse=false command-line flag")
)

var (
	influxServer       *influxserver.Server
	graphiteServer     *graphiteserver.Server
	opentsdbServer     *opentsdbserver.Server
	opentsdbhttpServer *opentsdbhttpserver.Server
)

var (
	//go:embed static
	staticFiles  embed.FS
	staticServer = http.FileServer(http.FS(staticFiles))
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	remotewrite.InitSecretFlags()
	buildinfo.Init()
	logger.Init()

	if promscrape.IsDryRun() {
		if err := promscrape.CheckConfig(); err != nil {
			logger.Fatalf("error when checking -promscrape.config: %s", err)
		}
		logger.Infof("-promscrape.config is ok; exiting with 0 status code")
		return
	}
	if *dryRun {
		if err := promscrape.CheckConfig(); err != nil {
			logger.Fatalf("error when checking -promscrape.config: %s", err)
		}
		if err := remotewrite.CheckRelabelConfigs(); err != nil {
			logger.Fatalf("error when checking relabel configs: %s", err)
		}
		if err := remotewrite.CheckStreamAggrConfigs(); err != nil {
			logger.Fatalf("error when checking -remoteWrite.streamAggr.config: %s", err)
		}
		logger.Infof("all the configs are ok; exiting with 0 status code")
		return
	}

	listenAddrs := *httpListenAddrs
	if len(listenAddrs) == 0 {
		listenAddrs = []string{":8429"}
	}
	logger.Infof("starting vmagent at %q...", listenAddrs)
	startTime := time.Now()
	remotewrite.StartIngestionRateLimiter()
	remotewrite.Init()
	common.StartUnmarshalWorkers()
	if len(*influxListenAddr) > 0 {
		influxServer = influxserver.MustStart(*influxListenAddr, *influxUseProxyProtocol, func(r io.Reader) error {
			return influx.InsertHandlerForReader(nil, r, false)
		})
	}
	if len(*graphiteListenAddr) > 0 {
		graphiteServer = graphiteserver.MustStart(*graphiteListenAddr, *graphiteUseProxyProtocol, graphite.InsertHandler)
	}
	if len(*opentsdbListenAddr) > 0 {
		httpInsertHandler := getOpenTSDBHTTPInsertHandler()
		opentsdbServer = opentsdbserver.MustStart(*opentsdbListenAddr, *opentsdbUseProxyProtocol, opentsdb.InsertHandler, httpInsertHandler)
	}
	if len(*opentsdbHTTPListenAddr) > 0 {
		httpInsertHandler := getOpenTSDBHTTPInsertHandler()
		opentsdbhttpServer = opentsdbhttpserver.MustStart(*opentsdbHTTPListenAddr, *opentsdbHTTPUseProxyProtocol, httpInsertHandler)
	}

	promscrape.Init(remotewrite.PushDropSamplesOnFailure)

	go httpserver.Serve(listenAddrs, useProxyProtocol, requestHandler)
	logger.Infof("started vmagent in %.3f seconds", time.Since(startTime).Seconds())

	pushmetrics.Init()
	sig := procutil.WaitForSigterm()
	logger.Infof("received signal %s", sig)
	remotewrite.StopIngestionRateLimiter()
	pushmetrics.Stop()

	startTime = time.Now()
	logger.Infof("gracefully shutting down webservice at %q", listenAddrs)
	if err := httpserver.Stop(listenAddrs); err != nil {
		logger.Fatalf("cannot stop the webservice: %s", err)
	}
	logger.Infof("successfully shut down the webservice in %.3f seconds", time.Since(startTime).Seconds())

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
	remotewrite.Stop()

	logger.Infof("successfully stopped vmagent in %.3f seconds", time.Since(startTime).Seconds())
}

func getOpenTSDBHTTPInsertHandler() func(req *http.Request) error {
	if !remotewrite.MultitenancyEnabled() {
		return func(req *http.Request) error {
			path := strings.Replace(req.URL.Path, "//", "/", -1)
			if path != "/api/put" {
				return fmt.Errorf("unsupported path requested: %q; expecting '/api/put'", path)
			}
			return opentsdbhttp.InsertHandler(nil, req)
		}
	}
	return func(req *http.Request) error {
		path := strings.Replace(req.URL.Path, "//", "/", -1)
		at, err := getAuthTokenFromPath(path)
		if err != nil {
			return fmt.Errorf("cannot obtain auth token from path %q: %w", path, err)
		}
		return opentsdbhttp.InsertHandler(at, req)
	}
}

func getAuthTokenFromPath(path string) (*auth.Token, error) {
	p, err := httpserver.ParsePath(path)
	if err != nil {
		return nil, fmt.Errorf("cannot parse multitenant path: %w", err)
	}
	if p.Prefix != "insert" {
		return nil, fmt.Errorf(`unsupported multitenant prefix: %q; expected "insert"`, p.Prefix)
	}
	if p.Suffix != "opentsdb/api/put" {
		return nil, fmt.Errorf("unsupported path requested: %q; expecting 'opentsdb/api/put'", p.Suffix)
	}
	return auth.NewTokenPossibleMultitenant(p.AuthToken)
}

func requestHandler(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/" {
		if r.Method != http.MethodGet {
			return false
		}
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<h2>vmagent</h2>")
		fmt.Fprintf(w, "See docs at <a href='https://docs.victoriametrics.com/vmagent/'>https://docs.victoriametrics.com/vmagent/</a></br>")
		fmt.Fprintf(w, "Useful endpoints:</br>")
		httpserver.WriteAPIHelp(w, [][2]string{
			{"targets", "status for discovered active targets"},
			{"service-discovery", "labels before and after relabeling for discovered targets"},
			{"metric-relabel-debug", "debug metric relabeling"},
			{"api/v1/targets", "advanced information about discovered targets in JSON format"},
			{"config", "-promscrape.config contents"},
			{"metrics", "available service metrics"},
			{"flags", "command-line flags"},
			{"-/reload", "reload configuration"},
		})
		return true
	}

	path := strings.Replace(r.URL.Path, "//", "/", -1)
	if strings.HasPrefix(path, "/prometheus/api/v1/import/prometheus") || strings.HasPrefix(path, "/api/v1/import/prometheus") {
		prometheusimportRequests.Inc()
		if err := prometheusimport.InsertHandler(nil, r); err != nil {
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
		if err := promremotewrite.InsertHandler(nil, r); err != nil {
			prometheusWriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/prometheus/api/v1/import", "/api/v1/import":
		vmimportRequests.Inc()
		if err := vmimport.InsertHandler(nil, r); err != nil {
			vmimportErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/prometheus/api/v1/import/csv", "/api/v1/import/csv":
		csvimportRequests.Inc()
		if err := csvimport.InsertHandler(nil, r); err != nil {
			csvimportErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/prometheus/api/v1/import/native", "/api/v1/import/native":
		nativeimportRequests.Inc()
		if err := native.InsertHandler(nil, r); err != nil {
			nativeimportErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/influx/write", "/influx/api/v2/write", "/write", "/api/v2/write":
		influxWriteRequests.Inc()
		if err := influx.InsertHandlerForHTTP(nil, r); err != nil {
			influxWriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/influx/query", "/query":
		influxQueryRequests.Inc()
		influxutils.WriteDatabaseNames(w)
		return true
	case "/opentelemetry/api/v1/push", "/opentelemetry/v1/metrics":
		opentelemetryPushRequests.Inc()
		if err := opentelemetry.InsertHandler(nil, r); err != nil {
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
		if err := newrelic.InsertHandlerForHTTP(nil, r); err != nil {
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
		if err := datadogv1.InsertHandlerForHTTP(nil, r); err != nil {
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
		if err := datadogv2.InsertHandlerForHTTP(nil, r); err != nil {
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
		if err := datadogsketches.InsertHandlerForHTTP(nil, r); err != nil {
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
	case "/prometheus/metric-relabel-debug", "/metric-relabel-debug":
		promscrapeMetricRelabelDebugRequests.Inc()
		promscrape.WriteMetricRelabelDebug(w, r)
		return true
	case "/prometheus/target-relabel-debug", "/target-relabel-debug":
		promscrapeTargetRelabelDebugRequests.Inc()
		promscrape.WriteTargetRelabelDebug(w, r)
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
			errMsg := fmt.Sprintf("waiting for scrapes to init, left: %d", rdy)
			http.Error(w, errMsg, http.StatusTooEarly)
		} else {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}
		return true
	default:
		if strings.HasPrefix(r.URL.Path, "/static") {
			staticServer.ServeHTTP(w, r)
			return true
		}
		if remotewrite.MultitenancyEnabled() {
			return processMultitenantRequest(w, r, path)
		}
		return false
	}
}

func processMultitenantRequest(w http.ResponseWriter, r *http.Request, path string) bool {
	p, err := httpserver.ParsePath(path)
	if err != nil {
		// Cannot parse multitenant path. Skip it - probably it will be parsed later.
		return false
	}
	if p.Prefix != "insert" {
		httpserver.Errorf(w, r, `unsupported multitenant prefix: %q; expected "insert"`, p.Prefix)
		return true
	}
	at, err := auth.NewToken(p.AuthToken)
	if err != nil {
		httpserver.Errorf(w, r, "cannot obtain auth token: %s", err)
		return true
	}
	if strings.HasPrefix(p.Suffix, "prometheus/api/v1/import/prometheus") {
		prometheusimportRequests.Inc()
		if err := prometheusimport.InsertHandler(at, r); err != nil {
			prometheusimportErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	if strings.HasPrefix(p.Suffix, "datadog/") {
		// Trim suffix from paths starting from /datadog/ in order to support legacy DataDog agent.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2670
		p.Suffix = strings.TrimSuffix(p.Suffix, "/")
	}
	switch p.Suffix {
	case "prometheus/", "prometheus", "prometheus/api/v1/write", "prometheus/api/v1/push":
		prometheusWriteRequests.Inc()
		if err := promremotewrite.InsertHandler(at, r); err != nil {
			prometheusWriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "prometheus/api/v1/import":
		vmimportRequests.Inc()
		if err := vmimport.InsertHandler(at, r); err != nil {
			vmimportErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "prometheus/api/v1/import/csv":
		csvimportRequests.Inc()
		if err := csvimport.InsertHandler(at, r); err != nil {
			csvimportErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "prometheus/api/v1/import/native":
		nativeimportRequests.Inc()
		if err := native.InsertHandler(at, r); err != nil {
			nativeimportErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "influx/write", "influx/api/v2/write":
		influxWriteRequests.Inc()
		if err := influx.InsertHandlerForHTTP(at, r); err != nil {
			influxWriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "influx/query":
		influxQueryRequests.Inc()
		influxutils.WriteDatabaseNames(w)
		return true
	case "opentelemetry/api/v1/push", "opentelemetry/v1/metrics":
		opentelemetryPushRequests.Inc()
		if err := opentelemetry.InsertHandler(at, r); err != nil {
			opentelemetryPushErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		firehose.WriteSuccessResponse(w, r)
		return true
	case "newrelic":
		newrelicCheckRequest.Inc()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		fmt.Fprintf(w, `{"status":"ok"}`)
		return true
	case "newrelic/inventory/deltas":
		newrelicInventoryRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		fmt.Fprintf(w, `{"payload":{"version": 1, "state": {}, "reset": "false"}}`)
		return true
	case "newrelic/infra/v2/metrics/events/bulk":
		newrelicWriteRequests.Inc()
		if err := newrelic.InsertHandlerForHTTP(at, r); err != nil {
			newrelicWriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		fmt.Fprintf(w, `{"status":"ok"}`)
		return true
	case "datadog/api/v1/series":
		datadogv1WriteRequests.Inc()
		if err := datadogv1.InsertHandlerForHTTP(at, r); err != nil {
			datadogv1WriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(202)
		fmt.Fprintf(w, `{"status":"ok"}`)
		return true
	case "datadog/api/v2/series":
		datadogv2WriteRequests.Inc()
		if err := datadogv2.InsertHandlerForHTTP(at, r); err != nil {
			datadogv2WriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
		w.WriteHeader(202)
		fmt.Fprintf(w, `{"status":"ok"}`)
		return true
	case "datadog/api/beta/sketches":
		datadogsketchesWriteRequests.Inc()
		if err := datadogsketches.InsertHandlerForHTTP(at, r); err != nil {
			datadogsketchesWriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(202)
		return true
	case "datadog/api/v1/validate":
		datadogValidateRequests.Inc()
		// See https://docs.datadoghq.com/api/latest/authentication/#validate-api-key
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"valid":true}`)
		return true
	case "datadog/api/v1/check_run":
		datadogCheckRunRequests.Inc()
		// See https://docs.datadoghq.com/api/latest/service-checks/#submit-a-service-check
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		fmt.Fprintf(w, `{"status":"ok"}`)
		return true
	case "datadog/intake":
		datadogIntakeRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{}`)
		return true
	case "datadog/api/v1/metadata":
		datadogMetadataRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{}`)
		return true
	default:
		httpserver.Errorf(w, r, "unsupported multitenant path suffix: %q", p.Suffix)
		return true
	}
}

var (
	prometheusWriteRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/api/v1/write", protocol="promremotewrite"}`)
	prometheusWriteErrors   = metrics.NewCounter(`vmagent_http_request_errors_total{path="/api/v1/write", protocol="promremotewrite"}`)

	vmimportRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/api/v1/import", protocol="vmimport"}`)
	vmimportErrors   = metrics.NewCounter(`vmagent_http_request_errors_total{path="/api/v1/import", protocol="vmimport"}`)

	csvimportRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/api/v1/import/csv", protocol="csvimport"}`)
	csvimportErrors   = metrics.NewCounter(`vmagent_http_request_errors_total{path="/api/v1/import/csv", protocol="csvimport"}`)

	prometheusimportRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/api/v1/import/prometheus", protocol="prometheusimport"}`)
	prometheusimportErrors   = metrics.NewCounter(`vmagent_http_request_errors_total{path="/api/v1/import/prometheus", protocol="prometheusimport"}`)

	nativeimportRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/api/v1/import/native", protocol="nativeimport"}`)
	nativeimportErrors   = metrics.NewCounter(`vmagent_http_request_errors_total{path="/api/v1/import/native", protocol="nativeimport"}`)

	influxWriteRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/influx/write", protocol="influx"}`)
	influxWriteErrors   = metrics.NewCounter(`vmagent_http_request_errors_total{path="/influx/write", protocol="influx"}`)

	influxQueryRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/influx/query", protocol="influx"}`)

	datadogv1WriteRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/datadog/api/v1/series", protocol="datadog"}`)
	datadogv1WriteErrors   = metrics.NewCounter(`vmagent_http_request_errors_total{path="/datadog/api/v1/series", protocol="datadog"}`)

	datadogv2WriteRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/datadog/api/v2/series", protocol="datadog"}`)
	datadogv2WriteErrors   = metrics.NewCounter(`vmagent_http_request_errors_total{path="/datadog/api/v2/series", protocol="datadog"}`)

	datadogsketchesWriteRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/datadog/api/beta/sketches", protocol="datadog"}`)
	datadogsketchesWriteErrors   = metrics.NewCounter(`vmagent_http_request_errors_total{path="/datadog/api/beta/sketches", protocol="datadog"}`)

	datadogValidateRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/datadog/api/v1/validate", protocol="datadog"}`)
	datadogCheckRunRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/datadog/api/v1/check_run", protocol="datadog"}`)
	datadogIntakeRequests   = metrics.NewCounter(`vmagent_http_requests_total{path="/datadog/intake", protocol="datadog"}`)
	datadogMetadataRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/datadog/api/v1/metadata", protocol="datadog"}`)

	opentelemetryPushRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/opentelemetry/v1/metrics", protocol="opentelemetry"}`)
	opentelemetryPushErrors   = metrics.NewCounter(`vmagent_http_request_errors_total{path="/opentelemetry/v1/metrics", protocol="opentelemetry"}`)

	newrelicWriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/newrelic/infra/v2/metrics/events/bulk", protocol="newrelic"}`)
	newrelicWriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/newrelic/infra/v2/metrics/events/bulk", protocol="newrelic"}`)

	newrelicInventoryRequests = metrics.NewCounter(`vm_http_requests_total{path="/newrelic/inventory/deltas", protocol="newrelic"}`)
	newrelicCheckRequest      = metrics.NewCounter(`vm_http_requests_total{path="/newrelic", protocol="newrelic"}`)

	promscrapeTargetsRequests          = metrics.NewCounter(`vmagent_http_requests_total{path="/targets"}`)
	promscrapeServiceDiscoveryRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/service-discovery"}`)

	promscrapeMetricRelabelDebugRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/metric-relabel-debug"}`)
	promscrapeTargetRelabelDebugRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/target-relabel-debug"}`)

	promscrapeAPIV1TargetsRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/api/v1/targets"}`)

	promscrapeTargetResponseRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/target_response"}`)
	promscrapeTargetResponseErrors   = metrics.NewCounter(`vmagent_http_request_errors_total{path="/target_response"}`)

	promscrapeConfigRequests       = metrics.NewCounter(`vmagent_http_requests_total{path="/config"}`)
	promscrapeStatusConfigRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/api/v1/status/config"}`)

	promscrapeConfigReloadRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/-/reload"}`)
)

func usage() {
	const s = `
vmagent collects metrics data via popular data ingestion protocols and routes it to VictoriaMetrics.

See the docs at https://docs.victoriametrics.com/vmagent/ .
`
	flagutil.Usage(s)
}
