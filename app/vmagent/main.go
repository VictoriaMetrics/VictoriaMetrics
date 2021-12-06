package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/csvimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/datadog"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/graphite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/native"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/prometheusimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/promremotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/vmimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	httpListenAddr = flag.String("httpListenAddr", ":8429", "TCP address to listen for http connections. "+
		"Set this flag to empty value in order to disable listening on any port. This mode may be useful for running multiple vmagent instances on the same server. "+
		"Note that /targets and /metrics pages aren't available if -httpListenAddr=''")
	influxListenAddr = flag.String("influxListenAddr", "", "TCP and UDP address to listen for InfluxDB line protocol data. Usually :8189 must be set. Doesn't work if empty. "+
		"This flag isn't needed when ingesting data over HTTP - just send it to http://<vmagent>:8429/write")
	graphiteListenAddr = flag.String("graphiteListenAddr", "", "TCP and UDP address to listen for Graphite plaintext data. Usually :2003 must be set. Doesn't work if empty")
	opentsdbListenAddr = flag.String("opentsdbListenAddr", "", "TCP and UDP address to listen for OpentTSDB metrics. "+
		"Telnet put messages and HTTP /api/put messages are simultaneously served on TCP port. "+
		"Usually :4242 must be set. Doesn't work if empty")
	opentsdbHTTPListenAddr = flag.String("opentsdbHTTPListenAddr", "", "TCP address to listen for OpentTSDB HTTP put requests. Usually :4242 must be set. Doesn't work if empty")
	configAuthKey          = flag.String("configAuthKey", "", "Authorization key for accessing /config page. It must be passed via authKey query arg")
	dryRun                 = flag.Bool("dryRun", false, "Whether to check only config files without running vmagent. The following files are checked: "+
		"-promscrape.config, -remoteWrite.relabelConfig, -remoteWrite.urlRelabelConfig . "+
		"Unknown config entries are allowed in -promscrape.config by default. This can be changed with -promscrape.config.strictParse")
)

var (
	influxServer       *influxserver.Server
	graphiteServer     *graphiteserver.Server
	opentsdbServer     *opentsdbserver.Server
	opentsdbhttpServer *opentsdbhttpserver.Server
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
		logger.Infof("-promscrape.config is ok; exitting with 0 status code")
		return
	}
	if *dryRun {
		if err := remotewrite.CheckRelabelConfigs(); err != nil {
			logger.Fatalf("error when checking relabel configs: %s", err)
		}
		if err := promscrape.CheckConfig(); err != nil {
			logger.Fatalf("error when checking -promscrape.config: %s", err)
		}
		logger.Infof("all the configs are ok; exitting with 0 status code")
		return
	}

	logger.Infof("starting vmagent at %q...", *httpListenAddr)
	startTime := time.Now()
	remotewrite.Init()
	common.StartUnmarshalWorkers()
	writeconcurrencylimiter.Init()
	if len(*influxListenAddr) > 0 {
		influxServer = influxserver.MustStart(*influxListenAddr, func(r io.Reader) error {
			return influx.InsertHandlerForReader(r, false)
		})
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
	common.StopUnmarshalWorkers()
	remotewrite.Stop()

	logger.Infof("successfully stopped vmagent in %.3f seconds", time.Since(startTime).Seconds())
}

func requestHandler(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/" {
		if r.Method != "GET" {
			return false
		}
		fmt.Fprintf(w, "<h2>vmagent</h2>")
		fmt.Fprintf(w, "See docs at <a href='https://docs.victoriametrics.com/vmagent.html'>https://docs.victoriametrics.com/vmagent.html</a></br>")
		fmt.Fprintf(w, "Useful endpoints:</br>")
		httpserver.WriteAPIHelp(w, [][2]string{
			{"targets", "discovered targets list"},
			{"api/v1/targets", "advanced information about discovered targets in JSON format"},
			{"config", "-promscrape.config contents"},
			{"metrics", "available service metrics"},
			{"flags", "command-line flags"},
			{"-/reload", "reload configuration"},
		})
		return true
	}

	path := strings.Replace(r.URL.Path, "//", "/", -1)
	switch path {
	case "/api/v1/write":
		prometheusWriteRequests.Inc()
		if err := promremotewrite.InsertHandler(nil, r); err != nil {
			prometheusWriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/api/v1/import":
		vmimportRequests.Inc()
		if err := vmimport.InsertHandler(nil, r); err != nil {
			vmimportErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/api/v1/import/csv":
		csvimportRequests.Inc()
		if err := csvimport.InsertHandler(nil, r); err != nil {
			csvimportErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/api/v1/import/prometheus":
		prometheusimportRequests.Inc()
		if err := prometheusimport.InsertHandler(nil, r); err != nil {
			prometheusimportErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/api/v1/import/native":
		nativeimportRequests.Inc()
		if err := native.InsertHandler(nil, r); err != nil {
			nativeimportErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/write", "/api/v2/write":
		influxWriteRequests.Inc()
		if err := influx.InsertHandlerForHTTP(nil, r); err != nil {
			influxWriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "/query":
		influxQueryRequests.Inc()
		influxutils.WriteDatabaseNames(w)
		return true
	case "/datadog/api/v1/series":
		datadogWriteRequests.Inc()
		if err := datadog.InsertHandlerForHTTP(nil, r); err != nil {
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
	case "/datadog/intake/":
		datadogIntakeRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{}`)
		return true
	case "/targets":
		promscrapeTargetsRequests.Inc()
		promscrape.WriteHumanReadableTargetsStatus(w, r)
		return true
	case "/config":
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
	case "/api/v1/targets":
		promscrapeAPIV1TargetsRequests.Inc()
		w.Header().Set("Content-Type", "application/json")
		state := r.FormValue("state")
		promscrape.WriteAPIV1Targets(w, state)
		return true
	case "/-/reload":
		promscrapeConfigReloadRequests.Inc()
		procutil.SelfSIGHUP()
		w.WriteHeader(http.StatusOK)
		return true
	case "/ready":
		if rdy := atomic.LoadInt32(&promscrape.PendingScrapeConfigs); rdy > 0 {
			errMsg := fmt.Sprintf("waiting for scrapes to init, left: %d", rdy)
			http.Error(w, errMsg, http.StatusTooEarly)
		} else {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}
		return true
	}
	if remotewrite.MultitenancyEnabled() {
		return processMultitenantRequest(w, r, path)
	}
	return false
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
	switch p.Suffix {
	case "prometheus/", "prometheus", "prometheus/api/v1/write":
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
	case "prometheus/api/v1/import/prometheus":
		prometheusimportRequests.Inc()
		if err := prometheusimport.InsertHandler(at, r); err != nil {
			prometheusimportErrors.Inc()
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
	case "datadog/api/v1/series":
		datadogWriteRequests.Inc()
		if err := datadog.InsertHandlerForHTTP(at, r); err != nil {
			datadogWriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
		w.WriteHeader(202)
		fmt.Fprintf(w, `{"status":"ok"}`)
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
	case "datadog/intake/":
		datadogIntakeRequests.Inc()
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

	datadogWriteRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/datadog/api/v1/series", protocol="datadog"}`)
	datadogWriteErrors   = metrics.NewCounter(`vmagent_http_request_errors_total{path="/datadog/api/v1/series", protocol="datadog"}`)

	datadogValidateRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/datadog/api/v1/validate", protocol="datadog"}`)
	datadogCheckRunRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/datadog/api/v1/check_run", protocol="datadog"}`)
	datadogIntakeRequests   = metrics.NewCounter(`vmagent_http_requests_total{path="/datadog/intake/", protocol="datadog"}`)

	promscrapeTargetsRequests      = metrics.NewCounter(`vmagent_http_requests_total{path="/targets"}`)
	promscrapeAPIV1TargetsRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/api/v1/targets"}`)

	promscrapeConfigRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/config"}`)

	promscrapeConfigReloadRequests = metrics.NewCounter(`vmagent_http_requests_total{path="/-/reload"}`)
)

func usage() {
	const s = `
vmagent collects metrics data via popular data ingestion protocols and routes it to VictoriaMetrics.

See the docs at https://docs.victoriametrics.com/vmagent.html .
`
	flagutil.Usage(s)
}
