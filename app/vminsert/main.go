package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/clusternative"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/csvimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/datadogsketches"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/datadogv1"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/datadogv2"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/graphite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/native"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/newrelic"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/opentelemetry"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/prometheusimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/promremotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/vmimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/influxutil"
	clusternativeserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/clusternative"
	graphiteserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/graphite"
	influxserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/influx"
	opentsdbserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/opentsdb"
	opentsdbhttpserver "github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/firehose"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pushmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeserieslimits"
)

var (
	clusternativeListenAddr = flag.String("clusternativeListenAddr", "", "TCP address to listen for data from other vminsert nodes in multi-level cluster setup. "+
		"See https://docs.victoriametrics.com/cluster-victoriametrics/#multi-level-cluster-setup . Usually :8400 should be set to match default vmstorage port for vminsert. Disabled work if empty")
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
	httpListenAddrs  = flagutil.NewArrayString("httpListenAddr", "Address to listen for incoming http requests. See also -httpListenAddr.useProxyProtocol")
	useProxyProtocol = flagutil.NewArrayBool("httpListenAddr.useProxyProtocol", "Whether to use proxy protocol for connections accepted at the given -httpListenAddr . "+
		"See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . "+
		"With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing")
	storageNodes = flagutil.NewArrayString("storageNode", "Comma-separated addresses of vmstorage nodes; usage: -storageNode=vmstorage-host1,...,vmstorage-hostN . "+
		"Enterprise version of VictoriaMetrics supports automatic discovery of vmstorage addresses via DNS SRV records. For example, -storageNode=srv+vmstorage.addrs . "+
		"See https://docs.victoriametrics.com/cluster-victoriametrics/#automatic-vmstorage-discovery")
	maxLabelsPerTimeseries = flag.Int("maxLabelsPerTimeseries", 40, "The maximum number of labels per time series to be accepted. Series with superfluous labels are ignored. In this case the vm_rows_ignored_total{reason=\"too_many_labels\"} metric at /metrics page is incremented")
	maxLabelNameLen        = flag.Int("maxLabelNameLen", 256, "The maximum length of label name in the accepted time series. Series with longer label name are ignored. In this case the vm_rows_ignored_total{reason=\"too_long_label_name\"} metric at /metrics page is incremented")
	maxLabelValueLen       = flag.Int("maxLabelValueLen", 4*1024, "The maximum length of label values in the accepted time series. Series with longer label value are ignored. In this case the vm_rows_ignored_total{reason=\"too_long_label_value\"} metric at /metrics page is incremented")
)

var (
	clusternativeServer *clusternativeserver.Server
	graphiteServer      *graphiteserver.Server
	influxServer        *influxserver.Server
	opentsdbServer      *opentsdbserver.Server
	opentsdbhttpServer  *opentsdbhttpserver.Server
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	buildinfo.Init()
	logger.Init()

	logger.Infof("initializing netstorage for storageNodes %s...", *storageNodes)
	startTime := time.Now()
	if len(*storageNodes) == 0 {
		logger.Fatalf("missing -storageNode arg")
	}
	if hasEmptyValues(*storageNodes) {
		logger.Fatalf("found empty address of storage node in the -storageNodes flag, please make sure that all -storageNode args are non-empty")
	}
	if duplicatedAddr := checkDuplicates(*storageNodes); duplicatedAddr != "" {
		logger.Fatalf("found equal addresses of storage nodes in the -storageNodes flag: %q", duplicatedAddr)
	}
	hashSeed := uint64(0)
	if *clusternativeListenAddr != "" {
		// Use different hash seed for the second level of vminsert nodes in multi-level cluster setup.
		// This should fix uneven distribution of time series among storage nodes.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1672
		hashSeed = 0xabcdef0123456789
	}
	netstorage.Init(*storageNodes, hashSeed)
	logger.Infof("successfully initialized netstorage in %.3f seconds", time.Since(startTime).Seconds())

	relabel.Init()
	timeserieslimits.Init(*maxLabelsPerTimeseries, *maxLabelNameLen, *maxLabelValueLen)
	protoparserutil.StartUnmarshalWorkers()
	if len(*clusternativeListenAddr) > 0 {
		clusternativeServer = clusternativeserver.MustStart(*clusternativeListenAddr, func(c net.Conn) error {
			return clusternative.InsertHandler(c)
		})
	}
	if len(*graphiteListenAddr) > 0 {
		graphiteServer = graphiteserver.MustStart(*graphiteListenAddr, *graphiteUseProxyProtocol, func(r io.Reader) error {
			return graphite.InsertHandler(nil, r)
		})
	}
	if len(*influxListenAddr) > 0 {
		influxServer = influxserver.MustStart(*influxListenAddr, *influxUseProxyProtocol, func(r io.Reader) error {
			return influx.InsertHandlerForReader(nil, r)
		})
	}
	if len(*opentsdbListenAddr) > 0 {
		opentsdbServer = opentsdbserver.MustStart(*opentsdbListenAddr, *opentsdbUseProxyProtocol, func(r io.Reader) error {
			return opentsdb.InsertHandler(nil, r)
		}, opentsdbhttp.InsertHandler)
	}
	if len(*opentsdbHTTPListenAddr) > 0 {
		opentsdbhttpServer = opentsdbhttpserver.MustStart(*opentsdbHTTPListenAddr, *opentsdbHTTPUseProxyProtocol, opentsdbhttp.InsertHandler)
	}

	listenAddrs := *httpListenAddrs
	if len(listenAddrs) == 0 {
		listenAddrs = []string{":8480"}
	}
	go httpserver.Serve(listenAddrs, useProxyProtocol, requestHandler)

	pushmetrics.Init()
	sig := procutil.WaitForSigterm()
	logger.Infof("service received signal %s", sig)
	pushmetrics.Stop()

	logger.Infof("gracefully shutting down http service at %q", listenAddrs)
	startTime = time.Now()
	if err := httpserver.Stop(listenAddrs); err != nil {
		logger.Fatalf("cannot stop http service: %s", err)
	}
	logger.Infof("successfully shut down http service in %.3f seconds", time.Since(startTime).Seconds())

	if len(*clusternativeListenAddr) > 0 {
		clusternativeServer.MustStop()
	}
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
	protoparserutil.StopUnmarshalWorkers()

	logger.Infof("shutting down neststorage...")
	startTime = time.Now()
	netstorage.MustStop()
	logger.Infof("successfully stopped netstorage in %.3f seconds", time.Since(startTime).Seconds())

	relabel.Stop()

	fs.MustStopDirRemover()

	logger.Infof("the vminsert has been stopped")
}

func requestHandler(w http.ResponseWriter, r *http.Request) bool {
	startTime := time.Now()
	defer requestDuration.UpdateDuration(startTime)

	if r.URL.Path == "/" {
		if r.Method != http.MethodGet {
			return false
		}
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `vminsert - a component of VictoriaMetrics cluster<br/>
			<a href="https://docs.victoriametrics.com/cluster-victoriametrics/">docs</a><br>
`)
		return true
	}
	p, err := httpserver.ParsePath(r.URL.Path)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse path %q: %s", r.URL.Path, err)
		return true
	}
	if p.Prefix != "insert" {
		// This is not our link.
		return false
	}
	at, err := auth.NewTokenPossibleMultitenant(p.AuthToken)
	if err != nil {
		httpserver.Errorf(w, r, "auth error: %s", err)
		return true
	}

	if strings.HasPrefix(p.Suffix, "prometheus/api/v1/import/prometheus") {
		prometheusimportRequests.Inc()
		if err := prometheusimport.InsertHandler(at, r); err != nil {
			prometheusimportErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		statusCode := http.StatusNoContent
		if strings.HasPrefix(p.Suffix, "prometheus/api/v1/import/prometheus/metrics/job/") {
			// Return 200 status code for pushgateway requests.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3636
			statusCode = http.StatusOK
		}
		w.WriteHeader(statusCode)
		return true
	}
	if strings.HasPrefix(p.Suffix, "datadog/") {
		// Trim suffix from paths starting from /datadog/ in order to support legacy DataDog agent.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2670
		p.Suffix = strings.TrimSuffix(p.Suffix, "/")
	}
	switch p.Suffix {
	case "prometheus/", "prometheus", "prometheus/api/v1/write", "prometheus/api/v1/push":
		if protoparserutil.HandleVMProtoServerHandshake(w, r) {
			return true
		}
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
		addInfluxResponseHeaders(w)
		if err := influx.InsertHandlerForHTTP(at, r); err != nil {
			influxWriteErrors.Inc()
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case "influx/query":
		influxQueryRequests.Inc()
		addInfluxResponseHeaders(w)
		influxutil.WriteDatabaseNames(w)
		return true
	case "influx/health":
		influxHealthRequests.Inc()
		influxutil.WriteHealthCheckResponse(w)
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
		w.Header().Set("Content-Type", "application/json")
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
		w.Header().Set("Content-Type", "application/json")
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
	case "/-/reload":
		procutil.SelfSIGHUP()
		w.WriteHeader(http.StatusNoContent)
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

	prometheusWriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/prometheus/", protocol="promremotewrite"}`)
	prometheusWriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/insert/{}/prometheus/", protocol="promremotewrite"}`)

	vmimportRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/prometheus/api/v1/import", protocol="vmimport"}`)
	vmimportErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/insert/{}/prometheus/api/v1/import", protocol="vmimport"}`)

	csvimportRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/prometheus/api/v1/import/csv", protocol="csvimport"}`)
	csvimportErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/insert/{}/prometheus/api/v1/import/csv", protocol="csvimport"}`)

	prometheusimportRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/prometheus/api/v1/import/prometheus", protocol="prometheusimport"}`)
	prometheusimportErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/insert/{}/prometheus/api/v1/import/prometheus", protocol="prometheusimport"}`)

	nativeimportRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/prometheus/api/v1/import/native", protocol="nativeimport"}`)
	nativeimportErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/insert/{}/prometheus/api/v1/import/native", protocol="nativeimport"}`)

	influxWriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/influx/write", protocol="influx"}`)
	influxWriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/insert/{}/influx/write", protocol="influx"}`)

	influxQueryRequests  = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/influx/query", protocol="influx"}`)
	influxHealthRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/influx/health", protocol="influx"}`)

	opentelemetryPushRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/opentelemetry/v1/metrics", protocol="opentelemetry"}`)
	opentelemetryPushErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/insert/{}/opentelemetry/v1/metrics", protocol="opentelemetry"}`)

	newrelicWriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/newrelic/infra/v2/metrics/events/bulk", protocol="newrelic"}`)
	newrelicWriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/insert/{}/newrelic/infra/v2/metrics/events/bulk", protocol="newrelic"}`)

	newrelicInventoryRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/newrelic/inventory/deltas", protocol="newrelic"}`)
	newrelicCheckRequest      = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/newrelic", protocol="newrelic"}`)

	datadogv1WriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/datadog/api/v1/series", protocol="datadog"}`)
	datadogv1WriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/insert/{}/datadog/api/v1/series", protocol="datadog"}`)

	datadogv2WriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/datadog/api/v2/series", protocol="datadog"}`)
	datadogv2WriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/insert/{}/datadog/api/v2/series", protocol="datadog"}`)

	datadogsketchesWriteRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/datadog/api/beta/sketches", protocol="datadog"}`)
	datadogsketchesWriteErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/insert/{}/datadog/api/beta/sketches", protocol="datadog"}`)

	datadogValidateRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/datadog/api/v1/validate", protocol="datadog"}`)
	datadogCheckRunRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/datadog/api/v1/check_run", protocol="datadog"}`)
	datadogIntakeRequests   = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/datadog/intake", protocol="datadog"}`)
	datadogMetadataRequests = metrics.NewCounter(`vm_http_requests_total{path="/insert/{}/datadog/api/v1/metadata", protocol="datadog"}`)
)

func usage() {
	const s = `
vminsert accepts data via popular data ingestion protocols and routes it to vmstorage nodes configured via -storageNode.

See the docs at https://docs.victoriametrics.com/cluster-victoriametrics/ .
`
	flagutil.Usage(s)
}

func checkDuplicates(arr []string) string {
	visited := make(map[string]struct{})
	for _, s := range arr {
		if _, ok := visited[s]; ok {
			return s
		}
		visited[s] = struct{}{}
	}
	return ""
}

func hasEmptyValues(arr []string) bool {
	for _, s := range arr {
		if s == "" {
			return true
		}
	}
	return false
}
