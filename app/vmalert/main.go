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
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/templates"
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

	ruleTemplatesPath = flagutil.NewArray("rule.templates", `Path or glob pattern to location with go template definitions
	for rules annotations templating. Flag can be specified multiple times.
Examples:
 -rule.templates="/path/to/file". Path to a single file with go templates
 -rule.templates="dir/*.tpl" -rule.templates="/*.tpl". Relative path to all .tpl files in "dir" folder,
absolute path to all .tpl files in root.`)

	rulesCheckInterval = flag.Duration("rule.configCheckInterval", 0, "Interval for checking for changes in '-rule' files. "+
		"By default the checking is disabled. Send SIGHUP signal in order to force config check for changes. DEPRECATED - see '-configCheckInterval' instead")

	configCheckInterval = flag.Duration("configCheckInterval", 0, "Interval for checking for changes in '-rule' or '-notifier.config' files. "+
		"By default the checking is disabled. Send SIGHUP signal in order to force config check for changes.")

	httpListenAddr     = flag.String("httpListenAddr", ":8880", "Address to listen for http connections")
	evaluationInterval = flag.Duration("evaluationInterval", time.Minute, "How often to evaluate the rules")

	validateTemplates   = flag.Bool("rule.validateTemplates", true, "Whether to validate annotation and label templates")
	validateExpressions = flag.Bool("rule.validateExpressions", true, "Whether to validate rules expressions via MetricsQL engine")
	maxResolveDuration  = flag.Duration("rule.maxResolveDuration", 0, "Limits the maximum duration for automatic alert expiration, "+
		"which is by default equal to 3 evaluation intervals of the parent group.")
	resendDelay = flag.Duration("rule.resendDelay", 0, "Minimum amount of time to wait before resending an alert to notifier")

	externalURL         = flag.String("external.url", "", "External URL is used as alert's source for sent alerts to the notifier")
	externalAlertSource = flag.String("external.alert.source", "", `External Alert Source allows to override the Source link for alerts sent to AlertManager for cases where you want to build a custom link to Grafana, Prometheus or any other service.
eg. 'explore?orgId=1&left=[\"now-1h\",\"now\",\"VictoriaMetrics\",{\"expr\": \"{{$expr|quotesEscape|crlfEscape|queryEscape}}\"},{\"mode\":\"Metrics\"},{\"ui\":[true,true,true,\"none\"]}]'.If empty '/vmalert/api/v1/alert?group_id=&alert_id=' is used`)
	externalLabels = flagutil.NewArray("external.label", "Optional label in the form 'Name=value' to add to all generated recording rules and alerts. "+
		"Pass multiple -label flags in order to add multiple label sets.")

	remoteReadLookBack = flag.Duration("remoteRead.lookback", time.Hour, "Lookback defines how far to look into past for alerts timeseries."+
		" For example, if lookback=1h then range from now() to now()-1h will be scanned.")
	remoteReadIgnoreRestoreErrors = flag.Bool("remoteRead.ignoreRestoreErrors", true, "Whether to ignore errors from remote storage when restoring alerts state on startup.")

	disableAlertGroupLabel = flag.Bool("disableAlertgroupLabel", false, "Whether to disable adding group's Name as label to generated alerts and time series.")

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
	err := templates.Load(*ruleTemplatesPath, true)
	if err != nil {
		logger.Fatalf("failed to parse %q: %s", *ruleTemplatesPath, err)
	}

	if *dryRun {
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

	labels := make(map[string]string)
	for _, s := range *externalLabels {
		if len(s) == 0 {
			continue
		}
		n := strings.IndexByte(s, '=')
		if n < 0 {
			return nil, fmt.Errorf("missing '=' in `-label`. It must contain label in the form `Name=value`; got %q", s)
		}
		labels[s[:n]] = s[n+1:]
	}

	nts, err := notifier.Init(alertURLGeneratorFn, labels, *externalURL)
	if err != nil {
		return nil, fmt.Errorf("failed to init notifier: %w", err)
	}
	manager := &manager{
		groups:         make(map[uint64]*Group),
		querierBuilder: q,
		notifiers:      nts,
		labels:         labels,
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
		return func(a notifier.Alert) string {
			gID, aID := strconv.FormatUint(a.GroupID, 10), strconv.FormatUint(a.ID, 10)
			return fmt.Sprintf("%s/vmalert/api/v1/alert?%s=%s&%s=%s", externalURL, paramGroupID, gID, paramAlertID, aID)
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
		templated, err := alert.ExecTemplate(nil, nil, m)
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
	checkInterval := *configCheckInterval
	if checkInterval == 0 && *rulesCheckInterval > 0 {
		logger.Warnf("flag `rule.configCheckInterval` is deprecated - use `configCheckInterval` instead")
		checkInterval = *rulesCheckInterval
	}
	if checkInterval > 0 {
		ticker := time.NewTicker(checkInterval)
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
			tmplMsg := ""
			if len(*ruleTemplatesPath) > 0 {
				tmplMsg = fmt.Sprintf("and templates %q ", *ruleTemplatesPath)
			}
			logger.Infof("SIGHUP received. Going to reload rules %q %s...", *rulePath, tmplMsg)
			configReloads.Inc()
		case <-configCheckCh:
		}
		if err := notifier.Reload(); err != nil {
			configReloadErrors.Inc()
			configSuccess.Set(0)
			logger.Errorf("failed to reload notifier config: %s", err)
			continue
		}
		err := templates.Load(*ruleTemplatesPath, false)
		if err != nil {
			configReloadErrors.Inc()
			configSuccess.Set(0)
			logger.Errorf("failed to load new templates: %s", err)
			continue
		}
		newGroupsCfg, err := config.Parse(*rulePath, *validateTemplates, *validateExpressions)
		if err != nil {
			configReloadErrors.Inc()
			configSuccess.Set(0)
			logger.Errorf("cannot parse configuration file: %s", err)
			continue
		}
		if configsEqual(newGroupsCfg, groupsCfg) {
			templates.Reload()
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
		templates.Reload()
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
