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

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
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
 -rule="/path/to/file". Path to a single file with alerting rules
 -rule="dir/*.yaml" -rule="/*.yaml". Relative path to all .yaml files in "dir" folder,
absolute path to all .yaml files in root.
Rule files may contain %{ENV_VAR} placeholders, which are substituted by the corresponding env vars.`)

	rulesCheckInterval = flag.Duration("rule.configCheckInterval", 0, "Interval for checking for changes in '-rule' files. "+
		"By default the checking is disabled. Send SIGHUP signal in order to force config check for changes")

	httpListenAddr     = flag.String("httpListenAddr", ":8880", "Address to listen for http connections")
	evaluationInterval = flag.Duration("evaluationInterval", time.Minute, "How often to evaluate the rules")

	validateTemplates   = flag.Bool("rule.validateTemplates", true, "Whether to validate annotation and label templates")
	validateExpressions = flag.Bool("rule.validateExpressions", true, "Whether to validate rules expressions via MetricsQL engine")
	maxResolveDuration  = flag.Duration("rule.maxResolveDuration", 0, "Limits the maximum duration for automatic alert expiration, "+
		"which is by default equal to 3 evaluation intervals of the parent group.")
	externalURL         = flag.String("external.url", "", "External URL is used as alert's source for sent alerts to the notifier")
	externalAlertSource = flag.String("external.alert.source", "", `External Alert Source allows to override the Source link for alerts sent to AlertManager for cases where you want to build a custom link to Grafana, Prometheus or any other service.
eg. 'explore?orgId=1&left=[\"now-1h\",\"now\",\"VictoriaMetrics\",{\"expr\": \"{{$expr|quotesEscape|crlfEscape|queryEscape}}\"},{\"mode\":\"Metrics\"},{\"ui\":[true,true,true,\"none\"]}]'.If empty '/api/v1/:groupID/alertID/status' is used`)
	externalLabels = flagutil.NewArray("external.label", "Optional label in the form 'name=value' to add to all generated recording rules and alerts. "+
		"Pass multiple -label flags in order to add multiple label sets.")

	remoteReadLookBack = flag.Duration("remoteRead.lookback", time.Hour, "Lookback defines how far to look into past for alerts timeseries."+
		" For example, if lookback=1h then range from now() to now()-1h will be scanned.")
	remoteReadIgnoreRestoreErrors = flag.Bool("remoteRead.ignoreRestoreErrors", true, "Whether to ignore errors from remote storage when restoring alerts state on startup.")

	disableAlertGroupLabel = flag.Bool("disableAlertgroupLabel", false, "Whether to disable adding group's name as label to generated alerts and time series.")

	dryRun = flag.Bool("dryRun", false, "Whether to check only config files without running vmalert. The rules file are validated. The `-rule` flag must be specified.")
)

var alertURLGeneratorFn notifier.AlertURLGenerator

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	buildinfo.Init()
	logger.Init()

	if *dryRun {
		u, _ := url.Parse("https://victoriametrics.com/")
		notifier.InitTemplateFunc(u)
		groups, err := config.Parse(*rulePath, true, true)
		if err != nil {
			logger.Fatalf("failed to parse %q: %s", *rulePath, err)
		}
		if len(groups) == 0 {
			logger.Fatalf("No rules for validation. Please specify path to file(s) with alerting and/or recording rules using `-rule` flag")
		}
		return
	}

	eu, err := getExternalURL(*externalURL, *httpListenAddr, httpserver.IsTLS())
	if err != nil {
		logger.Fatalf("failed to init `external.url`: %s", err)
	}
	notifier.InitTemplateFunc(eu)
	alertURLGeneratorFn, err = getAlertURLGenerator(eu, *externalAlertSource, *validateTemplates)
	if err != nil {
		logger.Fatalf("failed to init `external.alert.source`: %s", err)
	}

	if *replayFrom != "" || *replayTo != "" {
		rw, err := remotewrite.Init(context.Background())
		if err != nil {
			logger.Fatalf("failed to init remoteWrite: %s", err)
		}
		if rw == nil {
			logger.Fatalf("remoteWrite.url can't be empty in replay mode")
		}
		notifier.InitTemplateFunc(eu)
		groupsCfg, err := config.Parse(*rulePath, *validateTemplates, *validateExpressions)
		if err != nil {
			logger.Fatalf("cannot parse configuration file: %s", err)
		}
		// prevent queries from caching and boundaries aligning
		// when querying VictoriaMetrics datasource.
		q, err := datasource.Init(url.Values{"nocache": {"1"}})
		if err != nil {
			logger.Fatalf("failed to init datasource: %s", err)
		}
		if err := replay(groupsCfg, q, rw); err != nil {
			logger.Fatalf("replay failed: %s", err)
		}
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	manager, err := newManager(ctx)
	if err != nil {
		logger.Fatalf("failed to init: %s", err)
	}

	logger.Infof("reading rules configuration file from %q", strings.Join(*rulePath, ";"))
	groupsCfg, err := config.Parse(*rulePath, *validateTemplates, *validateExpressions)
	if err != nil {
		logger.Fatalf("cannot parse configuration file: %s", err)
	}

	// Register SIGHUP handler for config re-read just before manager.start call.
	// This guarantees that the config will be re-read if the signal arrives during manager.start call.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1240
	sighupCh := procutil.NewSighupChan()

	if err := manager.start(ctx, groupsCfg); err != nil {
		logger.Fatalf("failed to start: %s", err)
	}

	go configReload(ctx, manager, groupsCfg, sighupCh)

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
	q, err := datasource.Init(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to init datasource: %w", err)
	}
	nts, err := notifier.Init(alertURLGeneratorFn)
	if err != nil {
		return nil, fmt.Errorf("failed to init notifier: %w", err)
	}
	manager := &manager{
		groups:         make(map[uint64]*Group),
		querierBuilder: q,
		notifiers:      nts,
		labels:         map[string]string{},
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

	for _, s := range *externalLabels {
		if len(s) == 0 {
			continue
		}
		n := strings.IndexByte(s, '=')
		if n < 0 {
			return nil, fmt.Errorf("missing '=' in `-label`. It must contain label in the form `name=value`; got %q", s)
		}
		manager.labels[s[:n]] = s[n+1:]
	}
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
		templated, err := alert.ExecTemplate(nil, m)
		if err != nil {
			logger.Errorf("can not exec source template %s", err)
		}
		return fmt.Sprintf("%s/%s", externalURL, templated["tpl"])
	}, nil
}

func usage() {
	const s = `
vmalert processes alerts and recording rules.

See the docs at https://docs.victoriametrics.com/vmalert.html .
`
	flagutil.Usage(s)
}

func configReload(ctx context.Context, m *manager, groupsCfg []config.Group, sighupCh <-chan os.Signal) {
	var configCheckCh <-chan time.Time
	if *rulesCheckInterval > 0 {
		ticker := time.NewTicker(*rulesCheckInterval)
		configCheckCh = ticker.C
		defer ticker.Stop()
	}

	// init reload metrics with positive values to improve alerting conditions
	configSuccess.Set(1)
	configTimestamp.Set(fasttime.UnixTimestamp())
	for {
		select {
		case <-ctx.Done():
			return
		case <-sighupCh:
			logger.Infof("SIGHUP received. Going to reload rules %q ...", *rulePath)
			configReloads.Inc()
		case <-configCheckCh:
		}
		newGroupsCfg, err := config.Parse(*rulePath, *validateTemplates, *validateExpressions)
		if err != nil {
			configReloadErrors.Inc()
			configSuccess.Set(0)
			logger.Errorf("cannot parse configuration file: %s", err)
			continue
		}
		if configsEqual(newGroupsCfg, groupsCfg) {
			// set success to 1 since previous reload
			// could have been unsuccessful
			configSuccess.Set(1)
			// config didn't change - skip it
			continue
		}
		if err := m.update(ctx, newGroupsCfg, false); err != nil {
			configReloadErrors.Inc()
			configSuccess.Set(0)
			logger.Errorf("error while reloading rules: %s", err)
			continue
		}
		groupsCfg = newGroupsCfg
		configSuccess.Set(1)
		configTimestamp.Set(fasttime.UnixTimestamp())
		logger.Infof("Rules reloaded successfully from %q", *rulePath)
	}
}

func configsEqual(a, b []config.Group) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Checksum != b[i].Checksum {
			return false
		}
	}
	return true
}
