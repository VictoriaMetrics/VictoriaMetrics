package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rulePath = flagutil.NewArray("rule", `Path to file with alert rules, accepts patterns. 
Flag can be specified multiple time. 
Examples:
 -rule /path/to/file. Path to single file with alerting rules
 -rule dir/*.yaml -rule /*.yaml. Paths to all yaml files in relative dir folder and absolute yaml file in a root.`)
	validateAlertAnnotations = flag.Bool("rule.validateAnnotations", true, "Indicates to validate annotation templates")
	httpListenAddr           = flag.String("httpListenAddr", ":8880", "Address to listen for http connections")
	datasourceURL            = flag.String("datasource.url", "", "Victoria Metrics or VMSelect url. Required parameter. e.g. http://127.0.0.1:8428")
	basicAuthUsername        = flag.String("datasource.basicAuth.username", "", "Optional basic auth username to use for -datasource.url")
	basicAuthPassword        = flag.String("datasource.basicAuth.password", "", "Optional basic auth password to use for -datasource.url")
	evaluationInterval       = flag.Duration("evaluationInterval", 1*time.Minute, "How often to evaluate the rules. Default 1m")
	notifierURL              = flag.String("notifier.url", "", "Prometheus alertmanager URL. Required parameter. e.g. http://127.0.0.1:9093")
	externalURL              = flag.String("external.url", "", "URL is used to generate sharable alert URL")
)

// TODO: hot configuration reload
// TODO: alerts state persistence
func main() {
	envflag.Parse()
	buildinfo.Init()
	logger.Init()
	checkFlags()
	ctx, cancel := context.WithCancel(context.Background())
	eu, err := getExternalURL(*externalURL, *httpListenAddr, httpserver.IsTLS())
	if err != nil {
		logger.Fatalf("can not get external url:%s ", err)
	}
	notifier.InitTemplateFunc(eu)

	logger.Infof("reading alert rules configuration file from %s", strings.Join(*rulePath, ";"))
	groups, err := Parse(*rulePath, *validateAlertAnnotations)
	if err != nil {
		logger.Fatalf("Cannot parse configuration file: %s", err)
	}

	w := &watchdog{
		storage: datasource.NewVMStorage(*datasourceURL, *basicAuthUsername, *basicAuthPassword, &http.Client{}),
		alertProvider: notifier.NewAlertManager(*notifierURL, func(group, name string) string {
			return fmt.Sprintf("%s/api/v1/%s/%s/status", eu, group, name)
		}, &http.Client{}),
	}
	wg := sync.WaitGroup{}
	for i := range groups {
		wg.Add(1)
		go func(group Group) {
			w.run(ctx, group, *evaluationInterval)
			wg.Done()
		}(groups[i])
	}

	go httpserver.Serve(*httpListenAddr, (&requestHandler{groups: groups}).handler)

	sig := procutil.WaitForSigterm()
	logger.Infof("service received signal %s", sig)
	if err := httpserver.Stop(*httpListenAddr); err != nil {
		logger.Fatalf("cannot stop the webservice: %s", err)
	}
	cancel()
	wg.Wait()
}

type watchdog struct {
	storage       *datasource.VMStorage
	alertProvider notifier.Notifier
}

var (
	iterationTotal    = metrics.NewCounter(`vmalert_iteration_total`)
	iterationDuration = metrics.NewSummary(`vmalert_iteration_duration_seconds`)

	execTotal    = metrics.NewCounter(`vmalert_execution_total`)
	execErrors   = metrics.NewCounter(`vmalert_execution_errors_total`)
	execDuration = metrics.NewSummary(`vmalert_execution_duration_seconds`)
)

func (w *watchdog) run(ctx context.Context, group Group, evaluationInterval time.Duration) {
	logger.Infof("watchdog for %s has been started", group.Name)
	t := time.NewTicker(evaluationInterval)
	defer t.Stop()
	for {

		select {
		case <-t.C:
			iterationTotal.Inc()
			iterationStart := time.Now()
			for _, rule := range group.Rules {
				execTotal.Inc()

				execStart := time.Now()
				err := rule.Exec(ctx, w.storage)
				execDuration.UpdateDuration(execStart)

				if err != nil {
					execErrors.Inc()
					logger.Errorf("failed to execute rule %q.%q: %s", group.Name, rule.Name, err)
					continue
				}

				if err := rule.Send(ctx, w.alertProvider); err != nil {
					logger.Errorf("failed to send alert for rule %q.%q: %s", group.Name, rule.Name, err)
				}
			}
			iterationDuration.UpdateDuration(iterationStart)
		case <-ctx.Done():
			logger.Infof("%s received stop signal", group.Name)
			return
		}
	}
}

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
