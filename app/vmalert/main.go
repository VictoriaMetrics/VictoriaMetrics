package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rulePath = flagutil.NewArray("rule", `Path to the file with alert rules. 
Supports patterns. Flag can be specified multiple times. 
Examples:
 -rule /path/to/file. Path to a single file with alerting rules
 -rule dir/*.yaml -rule /*.yaml. Relative path to all .yaml files in "dir" folder, 
absolute path to all .yaml files in root.`)

	validateTemplates   = flag.Bool("rule.validateTemplates", true, "Whether to validate annotation and label templates")
	validateExpressions = flag.Bool("rule.validateExpressions", true, "Whether to validate rules expressions via MetricsQL engine")

	httpListenAddr = flag.String("httpListenAddr", ":8880", "Address to listen for http connections")

	datasourceURL = flag.String("datasource.url", "", "Victoria Metrics or VMSelect url. Required parameter."+
		" E.g. http://127.0.0.1:8428")
	basicAuthUsername = flag.String("datasource.basicAuth.username", "", "Optional basic auth username for -datasource.url")
	basicAuthPassword = flag.String("datasource.basicAuth.password", "", "Optional basic auth password for -datasource.url")

	remoteWriteURL = flag.String("remoteWrite.url", "", "Optional URL to Victoria Metrics or VMInsert where to persist alerts state"+
		" and recording rules results in form of timeseries. E.g. http://127.0.0.1:8428")
	remoteWriteUsername     = flag.String("remoteWrite.basicAuth.username", "", "Optional basic auth username for -remoteWrite.url")
	remoteWritePassword     = flag.String("remoteWrite.basicAuth.password", "", "Optional basic auth password for -remoteWrite.url")
	remoteWriteMaxQueueSize = flag.Int("remoteWrite.maxQueueSize", 1e5, "Defines the max number of pending datapoints to remote write endpoint")
	remoteWriteMaxBatchSize = flag.Int("remoteWrite.maxBatchSize", 1e3, "Defines defines max number of timeseries to be flushed at once")
	remoteWriteConcurrency  = flag.Int("remoteWrite.concurrency", 1, "Defines number of writers for concurrent writing into remote storage")

	remoteReadURL = flag.String("remoteRead.url", "", "Optional URL to Victoria Metrics or VMSelect that will be used to restore alerts"+
		" state. This configuration makes sense only if `vmalert` was configured with `remoteWrite.url` before and has been successfully persisted its state."+
		" E.g. http://127.0.0.1:8428")
	remoteReadUsername = flag.String("remoteRead.basicAuth.username", "", "Optional basic auth username for -remoteRead.url")
	remoteReadPassword = flag.String("remoteRead.basicAuth.password", "", "Optional basic auth password for -remoteRead.url")
	remoteReadLookBack = flag.Duration("remoteRead.lookback", time.Hour, "Lookback defines how far to look into past for alerts timeseries."+
		" For example, if lookback=1h then range from now() to now()-1h will be scanned.")

	evaluationInterval  = flag.Duration("evaluationInterval", time.Minute, "How often to evaluate the rules")
	notifierURL         = flag.String("notifier.url", "", "Prometheus alertmanager URL. Required parameter. e.g. http://127.0.0.1:9093")
	externalURL         = flag.String("external.url", "", "External URL is used as alert's source for sent alerts to the notifier")
	externalAlertSource = flag.String("external.alert.source", "", `Alert's source URL path can be rewritten by using External Alert Source. 
Source supports all alert variables and functions, can be useful for redefining source url path for Grafana query, eg. 'explore?orgId=1&left=[\"now-1h\",\"now\",\"VictoriaMetrics\",{\"expr\": \"{{$expr|quotesEscape|pathEscape}}\"},{\"mode\":\"Metrics\"},{\"ui\":[true,true,true,\"none\"]}]'' . 
If empty '/api/v1/:groupID/alertID/status' is used`)
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	buildinfo.Init()
	logger.Init()
	checkFlags()
	ctx, cancel := context.WithCancel(context.Background())
	eu, err := getExternalURL(*externalURL, *httpListenAddr, httpserver.IsTLS())
	if err != nil {
		logger.Fatalf("can not get external url: %s ", err)
	}
	notifier.InitTemplateFunc(eu)
	gu, err := getAlertURLGenerator(eu)
	if err != nil {
		logger.Fatalf("can not get source url: %s ", err)
	}

	manager := &manager{
		groups:   make(map[uint64]*Group),
		storage:  datasource.NewVMStorage(*datasourceURL, *basicAuthUsername, *basicAuthPassword, &http.Client{}),
		notifier: notifier.NewAlertManager(*notifierURL, gu, &http.Client{}),
	}
	if *remoteWriteURL != "" {
		c, err := remotewrite.NewClient(ctx, remotewrite.Config{
			Addr:          *remoteWriteURL,
			Concurrency:   *remoteWriteConcurrency,
			MaxQueueSize:  *remoteWriteMaxQueueSize,
			MaxBatchSize:  *remoteWriteMaxBatchSize,
			FlushInterval: *evaluationInterval,
			BasicAuthUser: *remoteWriteUsername,
			BasicAuthPass: *remoteWritePassword,
		})
		if err != nil {
			logger.Fatalf("failed to init remotewrite client: %s", err)
		}
		manager.rw = c
	}
	if *remoteReadURL != "" {
		manager.rr = datasource.NewVMStorage(*remoteReadURL, *remoteReadUsername, *remoteReadPassword, &http.Client{})
	}

	if err := manager.start(ctx, *rulePath, *validateTemplates, *validateExpressions); err != nil {
		logger.Fatalf("failed to start: %s", err)
	}

	go func() {
		// init reload metrics with positive values to improve alerting conditions
		configSuccess.Set(1)
		configTimestamp.Set(fasttime.UnixTimestamp())
		sigHup := procutil.NewSighupChan()
		for {
			<-sigHup
			configReloads.Inc()
			logger.Infof("SIGHUP received. Going to reload rules %q ...", *rulePath)
			if err := manager.update(ctx, *rulePath, *validateTemplates, *validateExpressions, false); err != nil {
				configReloadErrors.Inc()
				configSuccess.Set(0)
				logger.Errorf("error while reloading rules: %s", err)
				continue
			}
			configSuccess.Set(1)
			configTimestamp.Set(fasttime.UnixTimestamp())
			logger.Infof("Rules reloaded successfully from %q", *rulePath)
		}
	}()

	rh := &requestHandler{m: manager}
	go httpserver.Serve(*httpListenAddr, rh.handler)

	sig := procutil.WaitForSigterm()
	logger.Infof("service received signal %s", sig)
	if err := httpserver.Stop(*httpListenAddr); err != nil {
		logger.Fatalf("cannot stop the webservice: %s", err)
	}
	cancel()
	manager.close()
}

var (
	configReloads      = metrics.NewCounter(`vmalert_config_last_reload_total`)
	configReloadErrors = metrics.NewCounter(`vmalert_config_last_reload_errors_total`)
	configSuccess      = metrics.NewCounter(`vmalert_config_last_reload_successful`)
	configTimestamp    = metrics.NewCounter(`vmalert_config_last_reload_success_timestamp_seconds`)
)

func getExternalURL(externalURL, httpListenAddr string, isSecure bool) (*url.URL, error) {
	if externalURL != "" {
		return url.Parse(externalURL)
	}
	hname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	port := ""
	if ipport := strings.Split(httpListenAddr, ":"); len(ipport) > 1 {
		port = ":" + ipport[1]
	}
	schema := "http://"
	if isSecure {
		schema = "https://"
	}
	return url.Parse(fmt.Sprintf("%s%s%s", schema, hname, port))
}

func getAlertURLGenerator(externalURL *url.URL) (notifier.AlertURLGenerator, error) {
	sourceURL, err := notifier.AlertSourceFunc(externalURL, *externalAlertSource, *validateTemplates)
	if err != nil {
		return nil, err
	}
	if sourceURL != nil {
		return sourceURL, nil
	}
	return func(alert notifier.Alert) string {
		return fmt.Sprintf("%s/api/v1/%s/%s/status", externalURL, strconv.FormatUint(alert.GroupID, 10), strconv.FormatUint(alert.ID, 10))
	}, nil
}

func checkFlags() {
	if *notifierURL == "" {
		flag.PrintDefaults()
		logger.Fatalf("notifier.url is empty")
	}
	if *datasourceURL == "" {
		flag.PrintDefaults()
		logger.Fatalf("datasource.url is empty")
	}
}

func usage() {
	const s = `
vmalert processes alerts and recording rules.

See the docs at https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vmalert/README.md .
`

	f := flag.CommandLine.Output()
	fmt.Fprintf(f, "%s\n", s)
	flag.PrintDefaults()
}
