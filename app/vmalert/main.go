package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
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
	validateTemplates = flag.Bool("rule.validateTemplates", true, "Indicates to validate annotation and label templates")
	httpListenAddr    = flag.String("httpListenAddr", ":8880", "Address to listen for http connections")

	datasourceURL = flag.String("datasource.url", "", "Victoria Metrics or VMSelect url. Required parameter."+
		" E.g. http://127.0.0.1:8428")
	basicAuthUsername = flag.String("datasource.basicAuth.username", "", "Optional basic auth username for -datasource.url")
	basicAuthPassword = flag.String("datasource.basicAuth.password", "", "Optional basic auth password for -datasource.url")

	remoteWriteURL = flag.String("remoteWrite.url", "", "Optional URL to Victoria Metrics or VMInsert where to persist alerts state"+
		" in form of timeseries. E.g. http://127.0.0.1:8428")
	remoteWriteUsername     = flag.String("remoteWrite.basicAuth.username", "", "Optional basic auth username for -remotewrite.url")
	remoteWritePassword     = flag.String("remoteWrite.basicAuth.password", "", "Optional basic auth password for -remotewrite.url")
	remoteWriteMaxQueueSize = flag.Int("remoteWrite.maxQueueSize", 100, "Optional remote write maxQueueSize, default 100, you need to make it bigger if push print err: `failed to push timeseries to remotewrite: failed to push timeseries - queue is full`")

	remoteReadURL = flag.String("remoteRead.url", "", "Optional URL to Victoria Metrics or VMSelect that will be used to restore alerts"+
		" state. This configuration makes sense only if `vmalert` was configured with `remotewrite.url` before and has been successfully persisted its state."+
		" E.g. http://127.0.0.1:8428")
	remoteReadUsername = flag.String("remoteRead.basicAuth.username", "", "Optional basic auth username for -remoteread.url")
	remoteReadPassword = flag.String("remoteRead.basicAuth.password", "", "Optional basic auth password for -remoteread.url")
	remoteReadLookBack = flag.Duration("remoteRead.lookback", time.Hour, "Lookback defines how far to look into past for alerts timeseries."+
		" For example, if lookback=1h then range from now() to now()-1h will be scanned.")

	evaluationInterval = flag.Duration("evaluationInterval", time.Minute, "How often to evaluate the rules. Default 1m")
	notifierURL        = flag.String("notifier.url", "", "Prometheus alertmanager URL. Required parameter. e.g. http://127.0.0.1:9093")
	externalURL        = flag.String("external.url", "", "External URL is used as alert's source for sent alerts to the notifier")
)

func main() {
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

	manager := &manager{
		groups:  make(map[uint64]*Group),
		storage: datasource.NewVMStorage(*datasourceURL, *basicAuthUsername, *basicAuthPassword, &http.Client{}),
		notifier: notifier.NewAlertManager(*notifierURL, func(group, alert string) string {
			return fmt.Sprintf("%s/api/v1/%s/%s/status", eu, group, alert)
		}, &http.Client{}),
	}
	if *remoteWriteURL != "" {
		c, err := remotewrite.NewClient(ctx, remotewrite.Config{
			Addr:          *remoteWriteURL,
			MaxQueueSize:  *remoteWriteMaxQueueSize,
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

	if err := manager.start(ctx, *rulePath, *validateTemplates); err != nil {
		logger.Fatalf("failed to start: %s", err)
	}

	go func() {
		// init reload metrics with positive values to improve alerting conditions
		configSuccess.Set(1)
		configTimestamp.Set(uint64(time.Now().UnixNano()) / 1e9)
		sigHup := procutil.NewSighupChan()
		for {
			<-sigHup
			configReloads.Inc()
			logger.Infof("SIGHUP received. Going to reload rules %q ...", *rulePath)
			if err := manager.update(ctx, *rulePath, *validateTemplates, false); err != nil {
				configReloadErrors.Inc()
				configSuccess.Set(0)
				logger.Errorf("error while reloading rules: %s", err)
				continue
			}
			configSuccess.Set(1)
			configTimestamp.Set(uint64(time.Now().UnixNano()) / 1e9)
			logger.Infof("Rules reloaded successfully from %q", *rulePath)
		}
	}()

	rh := &requestHandler{m: manager}
	go httpserver.Serve(*httpListenAddr, (rh).handler)

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
