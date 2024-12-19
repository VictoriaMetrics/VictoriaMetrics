package unittest

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	vmalertconfig "github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/rule"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/templates"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/promremotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/metrics"
)

var (
	storagePath    string
	httpListenAddr = ":8880"
	// insert series from 1970-01-01T00:00:00
	testStartTime = time.Unix(0, 0).UTC()

	testPromWriteHTTPPath = "http://127.0.0.1" + httpListenAddr + "/api/v1/write"
	testDataSourcePath    = "http://127.0.0.1" + httpListenAddr + "/prometheus"
	testRemoteWritePath   = "http://127.0.0.1" + httpListenAddr
	testHealthHTTPPath    = "http://127.0.0.1" + httpListenAddr + "/health"

	testLogLevel           = "ERROR"
	disableAlertgroupLabel bool
)

const (
	testStoragePath = "vmalert-unittest"
)

// UnitTest runs unittest for files
func UnitTest(files []string, disableGroupLabel bool, externalLabels []string, externalURL, logLevel string) bool {
	if logLevel != "" {
		testLogLevel = logLevel
	}
	eu, err := url.Parse(externalURL)
	if err != nil {
		logger.Fatalf("failed to parse external URL: %w", err)
	}
	if err := templates.Load([]string{}, *eu); err != nil {
		logger.Fatalf("failed to load template: %v", err)
	}
	storagePath = filepath.Join(os.TempDir(), testStoragePath)
	processFlags()
	vminsert.Init()
	vmselect.Init()
	// storagePath will be created again when closing vmselect, so remove it again.
	defer fs.MustRemoveAll(storagePath)
	defer vminsert.Stop()
	defer vmselect.Stop()
	disableAlertgroupLabel = disableGroupLabel

	testfiles, err := config.ReadFromFS(files)
	if err != nil {
		logger.Fatalf("failed to load test files %q: %v", files, err)
	}
	if len(testfiles) == 0 {
		logger.Fatalf("no test file found")
	}

	labels := make(map[string]string)
	for _, s := range externalLabels {
		if len(s) == 0 {
			continue
		}
		n := strings.IndexByte(s, '=')
		if n < 0 {
			logger.Fatalf("missing '=' in `-label`. It must contain label in the form `name=value`; got %q", s)
		}
		labels[s[:n]] = s[n+1:]
	}
	_, err = notifier.Init(nil, labels, externalURL)
	if err != nil {
		logger.Fatalf("failed to init notifier: %v", err)
	}

	var failed bool
	for fileName, file := range testfiles {
		if err := ruleUnitTest(fileName, file, labels); err != nil {
			fmt.Println("FAILED")
			fmt.Printf("failed to run unit test for file %q: \n%v", fileName, err)
			failed = true
		} else {
			fmt.Println("  SUCCESS")
		}
	}

	return failed
}

func ruleUnitTest(filename string, content []byte, externalLabels map[string]string) []error {
	fmt.Println("\n\nUnit Testing: ", filename)
	var unitTestInp unitTestFile
	if err := yaml.UnmarshalStrict(content, &unitTestInp); err != nil {
		return []error{fmt.Errorf("failed to unmarshal file: %w", err)}
	}

	// add file directory for rule files if needed
	for i, rf := range unitTestInp.RuleFiles {
		if rf != "" && !filepath.IsAbs(rf) && !strings.HasPrefix(rf, "http") {
			unitTestInp.RuleFiles[i] = filepath.Join(filepath.Dir(filename), rf)
		}
	}

	if unitTestInp.EvaluationInterval.Duration() == 0 {
		fmt.Println("evaluation_interval set to 1m by default")
		unitTestInp.EvaluationInterval = &promutils.Duration{D: 1 * time.Minute}
	}

	groupOrderMap := make(map[string]int)
	for i, gn := range unitTestInp.GroupEvalOrder {
		if _, ok := groupOrderMap[gn]; ok {
			return []error{fmt.Errorf("group name repeated in `group_eval_order`: %s", gn)}
		}
		groupOrderMap[gn] = i
	}

	testGroups, err := vmalertconfig.Parse(unitTestInp.RuleFiles, nil, true)
	if err != nil {
		return []error{fmt.Errorf("failed to parse `rule_files`: %w", err)}
	}
	if len(testGroups) == 0 {
		return []error{fmt.Errorf("found no rule group in %v", unitTestInp.RuleFiles)}
	}

	var errs []error
	for _, t := range unitTestInp.Tests {
		if err := verifyTestGroup(t); err != nil {
			errs = append(errs, err)
			continue
		}
		testErrs := t.test(unitTestInp.EvaluationInterval.Duration(), groupOrderMap, testGroups, externalLabels)
		errs = append(errs, testErrs...)
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func verifyTestGroup(group testGroup) error {
	var testGroupName string
	if group.TestGroupName != "" {
		testGroupName = fmt.Sprintf("testGroupName: %s\n", group.TestGroupName)
	}
	for _, at := range group.AlertRuleTests {
		if at.Alertname == "" {
			return fmt.Errorf("\n%s    missing required field \"alertname\"", testGroupName)
		}
		if !disableAlertgroupLabel && at.GroupName == "" {
			return fmt.Errorf("\n%s    missing required field \"groupname\" when flag \"disableAlertgroupLabel\" is false", testGroupName)
		}
		if disableAlertgroupLabel && at.GroupName != "" {
			return fmt.Errorf("\n%s    shouldn't set field \"groupname\" when flag \"disableAlertgroupLabel\" is true", testGroupName)
		}
		if at.EvalTime == nil {
			return fmt.Errorf("\n%s    missing required field \"eval_time\"", testGroupName)
		}
	}
	for _, et := range group.MetricsqlExprTests {
		if et.Expr == "" {
			return fmt.Errorf("\n%s    missing required field \"expr\"", testGroupName)
		}
		if et.EvalTime == nil {
			return fmt.Errorf("\n%s    missing required field \"eval_time\"", testGroupName)
		}
	}
	if group.ExternalLabels != nil {
		fmt.Printf("\n%s    warning: filed `external_labels` will be deprecated soon, please use `-external.label` cmd-line flag instead. "+
			"Check https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6735 for details.\n", testGroupName)
	}
	return nil
}

func processFlags() {
	flag.Parse()
	for _, fv := range []struct {
		flag  string
		value string
	}{
		{flag: "storageDataPath", value: storagePath},
		{flag: "loggerLevel", value: testLogLevel},
		{flag: "search.disableCache", value: "true"},
		// set storage retention time to 100 years, allow to store series from 1970-01-01T00:00:00.
		{flag: "retentionPeriod", value: "100y"},
		{flag: "datasource.url", value: testDataSourcePath},
		{flag: "remoteWrite.url", value: testRemoteWritePath},
		{flag: "notifier.blackhole", value: "true"},
	} {
		// panics if flag doesn't exist
		if err := flag.Lookup(fv.flag).Value.Set(fv.value); err != nil {
			logger.Fatalf("unable to set %q with value %q, err: %v", fv.flag, fv.value, err)
		}
	}
}

func setUp() {
	vmstorage.Init(promql.ResetRollupResultCacheIfNeeded)
	var ab flagutil.ArrayBool
	go httpserver.Serve([]string{httpListenAddr}, &ab, func(w http.ResponseWriter, r *http.Request) bool {
		switch r.URL.Path {
		case "/prometheus/api/v1/query":
			if err := prometheus.QueryHandler(nil, time.Now(), w, r); err != nil {
				httpserver.Errorf(w, r, "%s", err)
			}
			return true
		case "/prometheus/api/v1/write", "/api/v1/write":
			if err := promremotewrite.InsertHandler(r); err != nil {
				httpserver.Errorf(w, r, "%s", err)
			}
			return true
		default:
		}
		return false
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	readyCheckFunc := func() bool {
		resp, err := http.Get(testHealthHTTPPath)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == 200
	}
checkCheck:
	for {
		select {
		case <-ctx.Done():
			logger.Fatalf("http server can't be ready in 30s")
		default:
			if readyCheckFunc() {
				break checkCheck
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func tearDown() {
	if err := httpserver.Stop([]string{httpListenAddr}); err != nil {
		logger.Errorf("cannot stop the webservice: %s", err)
	}
	vmstorage.Stop()
	metrics.UnregisterAllMetrics()
	fs.MustRemoveAll(storagePath)
}

func (tg *testGroup) test(evalInterval time.Duration, groupOrderMap map[string]int, testGroups []vmalertconfig.Group, externalLabels map[string]string) (checkErrs []error) {
	// set up vmstorage and http server for ingest and read queries
	setUp()
	// tear down vmstorage and clean the data dir
	defer tearDown()

	if tg.Interval == nil {
		tg.Interval = promutils.NewDuration(evalInterval)
	}
	err := writeInputSeries(tg.InputSeries, tg.Interval, testStartTime, testPromWriteHTTPPath)
	if err != nil {
		return []error{err}
	}

	q, err := datasource.Init(nil)
	if err != nil {
		return []error{fmt.Errorf("failed to init datasource: %v", err)}
	}
	rw, err := remotewrite.NewDebugClient()
	if err != nil {
		return []error{fmt.Errorf("failed to init wr: %v", err)}
	}

	alertEvalTimesMap := map[time.Duration]struct{}{}
	alertExpResultMap := map[time.Duration]map[string]map[string][]expAlert{}
	for _, at := range tg.AlertRuleTests {
		et := at.EvalTime.Duration()
		alertEvalTimesMap[et] = struct{}{}
		if _, ok := alertExpResultMap[et]; !ok {
			alertExpResultMap[et] = make(map[string]map[string][]expAlert)
		}
		if _, ok := alertExpResultMap[et][at.GroupName]; !ok {
			alertExpResultMap[et][at.GroupName] = make(map[string][]expAlert)
		}
		alertExpResultMap[et][at.GroupName][at.Alertname] = at.ExpAlerts
	}
	alertEvalTimes := make([]time.Duration, 0, len(alertEvalTimesMap))
	for k := range alertEvalTimesMap {
		alertEvalTimes = append(alertEvalTimes, k)
	}
	sort.Slice(alertEvalTimes, func(i, j int) bool {
		return alertEvalTimes[i] < alertEvalTimes[j]
	})

	// sort group eval order according to the given "group_eval_order".
	sort.Slice(testGroups, func(i, j int) bool {
		return groupOrderMap[testGroups[i].Name] < groupOrderMap[testGroups[j].Name]
	})

	// create groups with given rule
	var groups []*rule.Group
	for _, group := range testGroups {
		mergedExternalLabels := make(map[string]string)
		for k, v := range tg.ExternalLabels {
			mergedExternalLabels[k] = v
		}
		for k, v := range externalLabels {
			mergedExternalLabels[k] = v
		}
		ng := rule.NewGroup(group, q, time.Minute, mergedExternalLabels)
		groups = append(groups, ng)
	}

	evalIndex := 0
	maxEvalTime := testStartTime.Add(tg.maxEvalTime())
	for ts := testStartTime; ts.Before(maxEvalTime) || ts.Equal(maxEvalTime); ts = ts.Add(evalInterval) {
		for _, g := range groups {
			if len(g.Rules) == 0 {
				continue
			}
			errs := g.ExecOnce(context.Background(), func() []notifier.Notifier { return nil }, rw, ts)
			for err := range errs {
				if err != nil {
					checkErrs = append(checkErrs, fmt.Errorf("\nfailed to exec group: %q, time: %s, err: %w", g.Name,
						ts, err))
					return
				}
			}
			// flush series after each group evaluation
			vmstorage.Storage.DebugFlush()
		}

		// check alert_rule_test case at every eval time
		for evalIndex < len(alertEvalTimes) {
			if ts.Sub(testStartTime) > alertEvalTimes[evalIndex] ||
				alertEvalTimes[evalIndex] >= ts.Add(evalInterval).Sub(testStartTime) {
				break
			}
			gotAlertsMap := map[string]map[string]labelsAndAnnotations{}
			for _, g := range groups {
				if disableAlertgroupLabel {
					g.Name = ""
				}
				if _, ok := alertExpResultMap[alertEvalTimes[evalIndex]][g.Name]; !ok {
					continue
				}
				if _, ok := gotAlertsMap[g.Name]; !ok {
					gotAlertsMap[g.Name] = make(map[string]labelsAndAnnotations)
				}
				for _, r := range g.Rules {
					ar, isAlertRule := r.(*rule.AlertingRule)
					if !isAlertRule {
						continue
					}
					if _, ok := alertExpResultMap[alertEvalTimes[evalIndex]][g.Name][ar.Name]; ok {
						for _, got := range ar.GetAlerts() {
							if got.State != notifier.StateFiring {
								continue
							}
							if disableAlertgroupLabel {
								delete(got.Labels, "alertgroup")
							}
							laa := labelAndAnnotation{
								Labels:      datasource.ConvertToLabels(got.Labels),
								Annotations: datasource.ConvertToLabels(got.Annotations),
							}
							gotAlertsMap[g.Name][ar.Name] = append(gotAlertsMap[g.Name][ar.Name], laa)
						}
					}

				}
			}
			for groupname, gres := range alertExpResultMap[alertEvalTimes[evalIndex]] {
				for alertname, res := range gres {
					var expAlerts labelsAndAnnotations
					for _, expAlert := range res {
						if expAlert.ExpLabels == nil {
							expAlert.ExpLabels = make(map[string]string)
						}
						// alertGroupNameLabel is added as additional labels when `disableAlertgroupLabel` is false
						if !disableAlertgroupLabel {
							expAlert.ExpLabels["alertgroup"] = groupname
						}
						// alertNameLabel is added as additional labels in vmalert.
						expAlert.ExpLabels["alertname"] = alertname
						expAlerts = append(expAlerts, labelAndAnnotation{
							Labels:      datasource.ConvertToLabels(expAlert.ExpLabels),
							Annotations: datasource.ConvertToLabels(expAlert.ExpAnnotations),
						})
					}
					sort.Sort(expAlerts)

					gotAlerts := gotAlertsMap[groupname][alertname]
					sort.Sort(gotAlerts)
					if !reflect.DeepEqual(expAlerts, gotAlerts) {
						var testGroupName string
						if tg.TestGroupName != "" {
							testGroupName = fmt.Sprintf("testGroupName: %s,\n", tg.TestGroupName)
						}
						expString := indentLines(expAlerts.String(), "            ")
						gotString := indentLines(gotAlerts.String(), "            ")
						checkErrs = append(checkErrs, fmt.Errorf("\n%s    groupname: %s, alertname: %s, time: %s, \n        exp:%v, \n        got:%v ",
							testGroupName, groupname, alertname, alertEvalTimes[evalIndex].String(), expString, gotString))
					}
				}
			}
			evalIndex++
		}

	}

	checkErrs = append(checkErrs, checkMetricsqlCase(tg.MetricsqlExprTests, q)...)
	return checkErrs
}

// unitTestFile holds the contents of a single unit test file
type unitTestFile struct {
	RuleFiles          []string            `yaml:"rule_files"`
	EvaluationInterval *promutils.Duration `yaml:"evaluation_interval"`
	GroupEvalOrder     []string            `yaml:"group_eval_order"`
	Tests              []testGroup         `yaml:"tests"`
}

// testGroup is a group of input series and test cases associated with it
type testGroup struct {
	Interval           *promutils.Duration `yaml:"interval"`
	InputSeries        []series            `yaml:"input_series"`
	AlertRuleTests     []alertTestCase     `yaml:"alert_rule_test"`
	MetricsqlExprTests []metricsqlTestCase `yaml:"metricsql_expr_test"`
	ExternalLabels     map[string]string   `yaml:"external_labels"`
	TestGroupName      string              `yaml:"name"`
}

// maxEvalTime returns the max eval time among all alert_rule_test and metricsql_expr_test
func (tg *testGroup) maxEvalTime() time.Duration {
	var maxd time.Duration
	for _, alert := range tg.AlertRuleTests {
		if alert.EvalTime.Duration() > maxd {
			maxd = alert.EvalTime.Duration()
		}
	}
	for _, met := range tg.MetricsqlExprTests {
		if met.EvalTime.Duration() > maxd {
			maxd = met.EvalTime.Duration()
		}
	}
	return maxd
}
