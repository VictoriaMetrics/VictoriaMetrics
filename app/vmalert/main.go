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

	datasourceURL     = flag.String("datasource.url", "", "Victoria Metrics or VMSelect url. Required parameter. e.g. http://127.0.0.1:8428")
	basicAuthUsername = flag.String("datasource.basicAuth.username", "", "Optional basic auth username for -datasource.url")
	basicAuthPassword = flag.String("datasource.basicAuth.password", "", "Optional basic auth password for -datasource.url")

	remoteWriteURL = flag.String("remotewrite.url", "", "Optional URL to remote-write compatible storage where to write timeseries"+
		"based on active alerts. E.g. http://127.0.0.1:8428")
	remoteWriteUsername = flag.String("remotewrite.basicAuth.username", "", "Optional basic auth username for -remotewrite.url")
	remoteWritePassword = flag.String("remotewrite.basicAuth.password", "", "Optional basic auth password for -remotewrite.url")

	remoteReadURL = flag.String("remoteread.url", "", "Optional URL to datasource that will be used to restore alerts state."+
		"This configuration makes sense only if vmalert was configured with `remotewrite.url` before and has been successfully stored its state.")
	remoteReadUsername = flag.String("remoteread.basicAuth.username", "", "Optional basic auth username for -remotewrite.url")
	remoteReadPassword = flag.String("remoteread.basicAuth.password", "", "Optional basic auth password for -remotewrite.url")
	remoteReadLookBack = flag.Duration("remoteread.lookback", time.Hour, "Lookback defines how far to look into past for alerts metrics."+
		"For example, if lookback=1h then range from now() to now()-1h will be scanned.")

	evaluationInterval = flag.Duration("evaluationInterval", time.Minute, "How often to evaluate the rules. Default 1m")
	notifierURL        = flag.String("notifier.url", "", "Prometheus alertmanager URL. Required parameter. e.g. http://127.0.0.1:9093")
	externalURL        = flag.String("external.url", "", "External URL is used as alert's source for sent alerts to the notifier")
)

// TODO: hot configuration reload
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
	groups, err := Parse(*rulePath, *validateTemplates)
	if err != nil {
		logger.Fatalf("cannot parse configuration file: %s", err)
	}

	w := &watchdog{
		storage: datasource.NewVMStorage(*datasourceURL, *basicAuthUsername, *basicAuthPassword, &http.Client{}),
		alertProvider: notifier.NewAlertManager(*notifierURL, func(group, name string) string {
			return fmt.Sprintf("%s/api/v1/%s/%s/status", eu, group, name)
		}, &http.Client{}),
	}

	if *remoteWriteURL != "" {
		c, err := remotewrite.NewClient(ctx, remotewrite.Config{
			Addr:          *remoteWriteURL,
			FlushInterval: *evaluationInterval,
			BasicAuthUser: *remoteWriteUsername,
			BasicAuthPass: *remoteWritePassword,
		})
		if err != nil {
			logger.Fatalf("failed to init remotewrite client: %s", err)
		}
		w.rw = c
	}

	var restoreDS *datasource.VMStorage
	if *remoteReadURL != "" {
		restoreDS = datasource.NewVMStorage(*remoteReadURL, *remoteReadUsername, *remoteReadPassword, &http.Client{})
	}

	wg := sync.WaitGroup{}
	for _, g := range groups {
		if restoreDS != nil {
			err := g.Restore(ctx, restoreDS, *remoteReadLookBack)
			if err != nil {
				logger.Errorf("error while restoring state for group %q: %s", g.Name, err)
			}
		}
		wg.Add(1)
		go func(group Group) {
			w.run(ctx, group, *evaluationInterval)
			wg.Done()
		}(g)
	}

	go httpserver.Serve(*httpListenAddr, (&requestHandler{groups: groups}).handler)

	sig := procutil.WaitForSigterm()
	logger.Infof("service received signal %s", sig)
	if err := httpserver.Stop(*httpListenAddr); err != nil {
		logger.Fatalf("cannot stop the webservice: %s", err)
	}
	cancel()
	if w.rw != nil {
		err := w.rw.Close()
		if err != nil {
			logger.Fatalf("cannot stop the remotewrite: %s", err)
		}
	}
	wg.Wait()
}

type watchdog struct {
	storage       *datasource.VMStorage
	alertProvider notifier.Notifier
	rw            *remotewrite.Client
}

var (
	iterationTotal    = metrics.NewCounter(`vmalert_iteration_total`)
	iterationDuration = metrics.NewSummary(`vmalert_iteration_duration_seconds`)

	execTotal    = metrics.NewCounter(`vmalert_execution_total`)
	execErrors   = metrics.NewCounter(`vmalert_execution_errors_total`)
	execDuration = metrics.NewSummary(`vmalert_execution_duration_seconds`)

	alertsFired      = metrics.NewCounter(`vmalert_alerts_fired_total`)
	alertsSent       = metrics.NewCounter(`vmalert_alerts_sent_total`)
	alertsSendErrors = metrics.NewCounter(`vmalert_alerts_send_errors_total`)

	remoteWriteSent   = metrics.NewCounter(`vmalert_remotewrite_sent_total`)
	remoteWriteErrors = metrics.NewCounter(`vmalert_remotewrite_errors_total`)
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

				var alertsToSend []notifier.Alert
				for _, a := range rule.alerts {
					if a.State != notifier.StatePending {
						alertsToSend = append(alertsToSend, *a)
					}
					if a.State == notifier.StateInactive || w.rw == nil {
						continue
					}
					tss := rule.AlertToTimeSeries(a, execStart)
					for _, ts := range tss {
						remoteWriteSent.Inc()
						if err := w.rw.Push(ts); err != nil {
							remoteWriteErrors.Inc()
							logger.Errorf("failed to push timeseries to remotewrite: %s", err)
						}
					}
				}
				alertsSent.Add(len(alertsToSend))
				if err := w.alertProvider.Send(alertsToSend); err != nil {
					alertsSendErrors.Inc()
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
