package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remoteread"
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

	httpListenAddr     = flag.String("httpListenAddr", ":8880", "Address to listen for http connections")
	evaluationInterval = flag.Duration("evaluationInterval", time.Minute, "How often to evaluate the rules")

	validateTemplates   = flag.Bool("rule.validateTemplates", true, "Whether to validate annotation and label templates")
	validateExpressions = flag.Bool("rule.validateExpressions", true, "Whether to validate rules expressions via MetricsQL engine")
	externalURL         = flag.String("external.url", "", "External URL is used as alert's source for sent alerts to the notifier")
	externalAlertSource = flag.String("external.alert.source", "", `External Alert Source allows to override the Source link for alerts sent to AlertManager for cases where you want to build a custom link to Grafana, Prometheus or any other service.
eg. 'explore?orgId=1&left=[\"now-1h\",\"now\",\"VictoriaMetrics\",{\"expr\": \"{{$expr|quotesEscape|pathEscape}}\"},{\"mode\":\"Metrics\"},{\"ui\":[true,true,true,\"none\"]}]'.If empty '/api/v1/:groupID/alertID/status' is used`)

	remoteReadLookBack = flag.Duration("remoteRead.lookback", time.Hour, "Lookback defines how far to look into past for alerts timeseries."+
		" For example, if lookback=1h then range from now() to now()-1h will be scanned.")
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	buildinfo.Init()
	logger.Init()

	ctx, cancel := context.WithCancel(context.Background())
	manager, err := newManager(ctx)
	if err != nil {
		logger.Fatalf("failed to init: %s", err)
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

func newManager(ctx context.Context) (*manager, error) {
	q, err := datasource.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to init datasource: %w", err)
	}
	eu, err := getExternalURL(*externalURL, *httpListenAddr, httpserver.IsTLS())
	if err != nil {
		return nil, fmt.Errorf("failed to init `external.url`: %w", err)
	}
	notifier.InitTemplateFunc(eu)
	aug, err := getAlertURLGenerator(eu, *externalAlertSource, *validateTemplates)
	if err != nil {
		return nil, fmt.Errorf("failed to init `external.alert.source`: %w", err)
	}
	nts, err := notifier.Init(aug)
	if err != nil {
		return nil, fmt.Errorf("failed to init notifier: %w", err)
	}

	manager := &manager{
		groups:    make(map[uint64]*Group),
		querier:   q,
		notifiers: nts,
	}
	rw, err := remotewrite.Init(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to init remoteWrite: %w", err)
	}
	manager.rw = rw

	rr, err := remoteread.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to init remoteRead: %w", err)
	}
	manager.rr = rr
	return manager, nil
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

func getAlertURLGenerator(externalURL *url.URL, externalAlertSource string, validateTemplate bool) (notifier.AlertURLGenerator, error) {
	if externalAlertSource == "" {
		return func(alert notifier.Alert) string {
			return fmt.Sprintf("%s/api/v1/%s/%s/status", externalURL, strconv.FormatUint(alert.GroupID, 10), strconv.FormatUint(alert.ID, 10))
		}, nil
	}
	if validateTemplate {
		if err := notifier.ValidateTemplates(map[string]string{
			"tpl": externalAlertSource,
		}); err != nil {
			return nil, fmt.Errorf("error validating source template %s: %w", externalAlertSource, err)
		}
	}
	m := map[string]string{
		"tpl": externalAlertSource,
	}
	return func(alert notifier.Alert) string {
		templated, err := alert.ExecTemplate(m)
		if err != nil {
			logger.Errorf("can not exec source template %s", err)
		}
		return fmt.Sprintf("%s/%s", externalURL, templated["tpl"])
	}, nil
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
