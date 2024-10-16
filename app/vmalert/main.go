package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remoteread"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/rule"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/templates"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pushmetrics"
)

var (
	rulePath = flagutil.NewArrayString("rule", `Path to the files or http url with alerting and/or recording rules in YAML format.
Supports hierarchical patterns and regexpes.
Examples:
 -rule="/path/to/file". Path to a single file with alerting rules.
 -rule="http://<some-server-addr>/path/to/rules". HTTP URL to a page with alerting rules.
 -rule="dir/*.yaml" -rule="/*.yaml" -rule="gcs://vmalert-rules/tenant_%{TENANT_ID}/prod". 
 -rule="dir/**/*.yaml". Includes all the .yaml files in "dir" subfolders recursively.
Rule files support YAML multi-document. Files may contain %{ENV_VAR} placeholders, which are substituted by the corresponding env vars.

Enterprise version of vmalert supports S3 and GCS paths to rules.
For example: gs://bucket/path/to/rules, s3://bucket/path/to/rules
S3 and GCS paths support only matching by prefix, e.g. s3://bucket/dir/rule_ matches
all files with prefix rule_ in folder dir.
See https://docs.victoriametrics.com/vmalert/#reading-rules-from-object-storage
`)

	ruleTemplatesPath = flagutil.NewArrayString("rule.templates", `Path or glob pattern to location with go template definitions `+
		`for rules annotations templating. Flag can be specified multiple times.
Examples:
 -rule.templates="/path/to/file". Path to a single file with go templates
 -rule.templates="dir/*.tpl" -rule.templates="/*.tpl". Relative path to all .tpl files in "dir" folder,
absolute path to all .tpl files in root.
 -rule.templates="dir/**/*.tpl". Includes all the .tpl files in "dir" subfolders recursively.
`)

	configCheckInterval = flag.Duration("configCheckInterval", 0, "Interval for checking for changes in '-rule' or '-notifier.config' files. "+
		"By default, the checking is disabled. Send SIGHUP signal in order to force config check for changes.")

	httpListenAddrs  = flagutil.NewArrayString("httpListenAddr", "Address to listen for incoming http requests. See also -tls and -httpListenAddr.useProxyProtocol")
	useProxyProtocol = flagutil.NewArrayBool("httpListenAddr.useProxyProtocol", "Whether to use proxy protocol for connections accepted at the corresponding -httpListenAddr . "+
		"See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . "+
		"With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing")
	evaluationInterval = flag.Duration("evaluationInterval", time.Minute, "How often to evaluate the rules")

	validateTemplates   = flag.Bool("rule.validateTemplates", true, "Whether to validate annotation and label templates")
	validateExpressions = flag.Bool("rule.validateExpressions", true, "Whether to validate rules expressions via MetricsQL engine")

	externalURL         = flag.String("external.url", "", "External URL is used as alert's source for sent alerts to the notifier. By default, hostname is used as address.")
	externalAlertSource = flag.String("external.alert.source", "", `External Alert Source allows to override the Source link for alerts sent to AlertManager `+
		`for cases where you want to build a custom link to Grafana, Prometheus or any other service. `+
		`Supports templating - see https://docs.victoriametrics.com/vmalert/#templating . `+
		`For example, link to Grafana: -external.alert.source='explore?orgId=1&left={"datasource":"VictoriaMetrics","queries":[{"expr":{{.Expr|jsonEscape|queryEscape}},"refId":"A"}],"range":{"from":"now-1h","to":"now"}}'. `+
		`Link to VMUI: -external.alert.source='vmui/#/?g0.expr={{.Expr|queryEscape}}'. `+
		`If empty 'vmalert/alert?group_id={{.GroupID}}&alert_id={{.AlertID}}' is used.`)
	externalLabels = flagutil.NewArrayString("external.label", "Optional label in the form 'Name=value' to add to all generated recording rules and alerts. "+
		"In case of conflicts, original labels are kept with prefix `exported_`.")

	remoteReadIgnoreRestoreErrors = flag.Bool("remoteRead.ignoreRestoreErrors", true, "Whether to ignore errors from remote storage when restoring alerts state on startup. DEPRECATED - this flag has no effect and will be removed in the next releases.")

	dryRun = flag.Bool("dryRun", false, "Whether to check only config files without running vmalert. The rules file are validated. The -rule flag must be specified.")
)

var alertURLGeneratorFn notifier.AlertURLGenerator

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	remoteread.InitSecretFlags()
	remotewrite.InitSecretFlags()
	datasource.InitSecretFlags()
	notifier.InitSecretFlags()
	buildinfo.Init()
	logger.Init()

	if !*remoteReadIgnoreRestoreErrors {
		logger.Warnf("flag `remoteRead.ignoreRestoreErrors` is deprecated and will be removed in next releases.")
	}

	err := templates.Load(*ruleTemplatesPath, true)
	if err != nil {
		logger.Fatalf("failed to parse %q: %s", *ruleTemplatesPath, err)
	}

	if *dryRun {
		groups, err := config.Parse(*rulePath, notifier.ValidateTemplates, true)
		if err != nil {
			logger.Fatalf("failed to parse %q: %s", *rulePath, err)
		}
		if len(groups) == 0 {
			logger.Fatalf("No rules for validation. Please specify path to file(s) with alerting and/or recording rules using `-rule` flag")
		}
		return
	}

	eu, err := getExternalURL(*externalURL)
	if err != nil {
		logger.Fatalf("failed to init `-external.url`: %s", err)
	}

	alertURLGeneratorFn, err = getAlertURLGenerator(eu, *externalAlertSource, *validateTemplates)
	if err != nil {
		logger.Fatalf("failed to init `external.alert.source`: %s", err)
	}

	var validateTplFn config.ValidateTplFn
	if *validateTemplates {
		validateTplFn = notifier.ValidateTemplates
	}

	if *replayFrom != "" {
		rw, err := remotewrite.Init(context.Background())
		if err != nil {
			logger.Fatalf("failed to init remoteWrite: %s", err)
		}
		if rw == nil {
			logger.Fatalf("remoteWrite.url can't be empty in replay mode")
		}
		groupsCfg, err := config.Parse(*rulePath, validateTplFn, *validateExpressions)
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
		logger.Infof("replay succeed!")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	manager, err := newManager(ctx)
	if err != nil {
		logger.Fatalf("failed to init: %s", err)
	}
	logger.Infof("reading rules configuration file from %q", strings.Join(*rulePath, ";"))
	groupsCfg, err := config.Parse(*rulePath, validateTplFn, *validateExpressions)
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

	listenAddrs := *httpListenAddrs
	if len(listenAddrs) == 0 {
		listenAddrs = []string{":8880"}
	}
	rh := &requestHandler{m: manager}
	go httpserver.Serve(listenAddrs, useProxyProtocol, rh.handler)

	pushmetrics.Init()
	sig := procutil.WaitForSigterm()
	logger.Infof("service received signal %s", sig)
	pushmetrics.Stop()

	if err := httpserver.Stop(listenAddrs); err != nil {
		logger.Fatalf("cannot stop the webservice: %s", err)
	}
	cancel()
	manager.close()
}

var (
	configReloads      = metrics.NewCounter(`vmalert_config_last_reload_total`)
	configReloadErrors = metrics.NewCounter(`vmalert_config_last_reload_errors_total`)
	configSuccess      = metrics.NewGauge(`vmalert_config_last_reload_successful`, nil)
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
		groups:         make(map[uint64]*rule.Group),
		querierBuilder: q,
		notifiers:      nts,
		labels:         labels,
	}
	rw, err := remotewrite.Init(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to init remoteWrite: %w", err)
	}
	if rw != nil {
		manager.rw = rw
	}

	rr, err := remoteread.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to init remoteRead: %w", err)
	}
	manager.rr = rr

	return manager, nil
}

func getExternalURL(customURL string) (*url.URL, error) {
	if customURL == "" {
		// use local hostname as external URL
		listenAddr := ":8880"
		if len(*httpListenAddrs) > 0 {
			listenAddr = (*httpListenAddrs)[0]
		}
		isTLS := httpserver.IsTLS(0)

		return getHostnameAsExternalURL(listenAddr, isTLS)
	}
	u, err := url.Parse(customURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid scheme %q in url %q, only 'http' and 'https' are supported", u.Scheme, u.String())
	}
	return u, nil
}

func getHostnameAsExternalURL(addr string, isSecure bool) (*url.URL, error) {
	hname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}
	port := ""
	if ipport := strings.Split(addr, ":"); len(ipport) > 1 {
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
			return fmt.Sprintf("%s/vmalert/alert?%s=%s&%s=%s", externalURL, paramGroupID, gID, paramAlertID, aID)
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
		qFn := func(_ string) ([]datasource.Metric, error) {
			return nil, fmt.Errorf("`query` template isn't supported for alert source template")
		}
		templated, err := alert.ExecTemplate(qFn, alert.Labels, m)
		if err != nil {
			logger.Errorf("can not exec source template %s", err)
		}
		return fmt.Sprintf("%s/%s", externalURL, templated["tpl"])
	}, nil
}

func usage() {
	const s = `
vmalert processes alerts and recording rules.

See the docs at https://docs.victoriametrics.com/vmalert/ .
`
	flagutil.Usage(s)
}

func configReload(ctx context.Context, m *manager, groupsCfg []config.Group, sighupCh <-chan os.Signal) {
	var configCheckCh <-chan time.Time
	checkInterval := *configCheckInterval
	if checkInterval > 0 {
		ticker := time.NewTicker(checkInterval)
		configCheckCh = ticker.C
		defer ticker.Stop()
	}

	var validateTplFn config.ValidateTplFn
	if *validateTemplates {
		validateTplFn = notifier.ValidateTemplates
	}

	// init metrics for config state with positive values to improve alerting conditions
	setConfigSuccessAt(fasttime.UnixTimestamp())

	parseFn := config.Parse
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
			// allow logs emitting during manual config reload
			parseFn = config.Parse
		case <-configCheckCh:
			// disable logs emitting during per-interval config reload
			parseFn = config.ParseSilent
		}
		if err := notifier.Reload(); err != nil {
			setConfigError(err)
			logger.Errorf("failed to reload notifier config: %s", err)
			continue
		}
		err := templates.Load(*ruleTemplatesPath, false)
		if err != nil {
			setConfigError(err)
			logger.Errorf("failed to load new templates: %s", err)
			continue
		}
		newGroupsCfg, err := parseFn(*rulePath, validateTplFn, *validateExpressions)
		if err != nil {
			setConfigError(err)
			logger.Errorf("cannot parse configuration file: %s", err)
			continue
		}
		if configsEqual(newGroupsCfg, groupsCfg) {
			templates.Reload()
			// set success to 1 since previous reload could have been unsuccessful
			// do not update configTimestamp as config version remains old.
			configSuccess.Set(1)
			// reset the last config error since the config change was rolled back
			setLastConfigErr(nil)
			// config didn't change - skip iteration
			continue
		}
		if err := m.update(ctx, newGroupsCfg, false); err != nil {
			setConfigError(err)
			logger.Errorf("error while reloading rules: %s", err)
			continue
		}
		templates.Reload()
		groupsCfg = newGroupsCfg
		setConfigSuccessAt(fasttime.UnixTimestamp())
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

// setConfigSuccessAt updates config related metrics as successful.
func setConfigSuccessAt(at uint64) {
	configSuccess.Set(1)
	configTimestamp.Set(at)
	// reset the lastConfigErr
	setLastConfigErr(nil)
}

// setConfigError updates config related metrics according to the error.
func setConfigError(err error) {
	configReloadErrors.Inc()
	configSuccess.Set(0)
	setLastConfigErr(err)
}

var (
	lastConfigErrMu sync.RWMutex
	// lastConfigErr represent the error message from the last config reload.
	// The message is used in web UI as notification
	lastConfigErr error
)

func setLastConfigErr(err error) {
	lastConfigErrMu.Lock()
	lastConfigErr = err
	lastConfigErrMu.Unlock()
}

func getLastConfigError() error {
	lastConfigErrMu.RLock()
	defer lastConfigErrMu.RUnlock()
	return lastConfigErr
}
