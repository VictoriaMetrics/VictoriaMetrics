package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"

	"gopkg.in/yaml.v2"

	vmalertconfig "github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/unittest"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/promremotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/metrics"
)

var (
	storagePath string
	// insert series from 1970-01-01T00:00:00
	testStartTime = time.Unix(0, 0).UTC()

	testPromWriteHTTPPath = "http://127.0.0.1" + *httpListenAddr + "/api/v1/write"
	testDataSourcePath    = "http://127.0.0.1" + *httpListenAddr + "/prometheus"
	testRemoteWritePath   = "http://127.0.0.1" + *httpListenAddr
	testHealthHTTPPath    = "http://127.0.0.1" + *httpListenAddr + "/health"
)

const (
	testStoragePath = "vmalert-unittest"
	testLogLevel    = "ERROR"
)

func unitRule(files ...string) bool {
	storagePath = filepath.Join(os.TempDir(), testStoragePath)
	processFlags()
	vminsert.Init()
	vmselect.Init()
	// storagePath will be created again when closing vmselect, so remove it again.
	defer fs.MustRemoveAll(storagePath)
	defer vminsert.Stop()
	defer vmselect.Stop()
	return rulesUnitTest(files...)
}

func rulesUnitTest(files ...string) bool {
	var failed bool
	for _, f := range files {
		if err := ruleUnitTest(f); err != nil {
			fmt.Println("  FAILED")
			fmt.Printf("\nfailed to run unit test for file %q: \n%s", f, err)
			failed = true
		} else {
			fmt.Println("  SUCCESS")
		}
	}
	return failed
}

func ruleUnitTest(filename string) []error {
	fmt.Println("\nUnit Testing: ", filename)
	b, err := os.ReadFile(filename)
	if err != nil {
		return []error{fmt.Errorf("failed to read file: %w", err)}
	}

	var unitTestInp unitTestFile
	if err := yaml.UnmarshalStrict(b, &unitTestInp); err != nil {
		return []error{fmt.Errorf("failed to unmarshal file: %w", err)}
	}
	if err := resolveAndGlobFilepaths(filepath.Dir(filename), &unitTestInp); err != nil {
		return []error{fmt.Errorf("failed to resolve path for `rule_files`: %w", err)}
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

	testGroups, err := vmalertconfig.Parse(unitTestInp.RuleFiles, nil, true, *evaluationInterval)
	if err != nil {
		return []error{fmt.Errorf("failed to parse `rule_files`: %w", err)}
	}

	var errs []error
	for _, t := range unitTestInp.Tests {
		if err := verifyTestGroup(t); err != nil {
			errs = append(errs, err)
			continue
		}
		testErrs := t.test(unitTestInp.EvaluationInterval.Duration(), groupOrderMap, testGroups)
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
			return fmt.Errorf("\n%s    missing required filed \"alertname\"", testGroupName)
		}
		if !*disableAlertGroupLabel && at.GroupName == "" {
			return fmt.Errorf("\n%s    missing required filed \"groupname\" when flag \"disableAlertGroupLabel\" is false", testGroupName)
		}
		if at.EvalTime == nil {
			return fmt.Errorf("\n%s    missing required filed \"eval_time\"", testGroupName)
		}
	}
	for _, et := range group.MetricsqlExprTests {
		if et.Expr == "" {
			return fmt.Errorf("\n%s    missing required filed \"expr\"", testGroupName)
		}
		if et.EvalTime == nil {
			return fmt.Errorf("\n%s    missing required filed \"eval_time\"", testGroupName)
		}
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
	} {
		// panics if flag doesn't exist
		if err := flag.Lookup(fv.flag).Value.Set(fv.value); err != nil {
			logger.Fatalf("unable to set %q with value %q, err: %v", fv.flag, fv.value, err)
		}
	}
}

func setUp() {
	vmstorage.Init(promql.ResetRollupResultCacheIfNeeded)
	go httpserver.Serve(*httpListenAddr, false, func(w http.ResponseWriter, r *http.Request) bool {
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
			time.Sleep(3 * time.Second)
		}
	}
}

func tearDown() {
	if err := httpserver.Stop(*httpListenAddr); err != nil {
		logger.Errorf("cannot stop the webservice: %s", err)
	}
	vmstorage.Stop()
	metrics.UnregisterAllMetrics()
	fs.MustRemoveAll(storagePath)
}

// resolveAndGlobFilepaths joins all relative paths in a configuration
// with a given base directory and replaces all globs with matching files.
func resolveAndGlobFilepaths(baseDir string, utf *unitTestFile) error {
	for i, rf := range utf.RuleFiles {
		if rf != "" && !filepath.IsAbs(rf) {
			utf.RuleFiles[i] = filepath.Join(baseDir, rf)
		}
	}

	var globbedFiles []string
	for _, rf := range utf.RuleFiles {
		m, err := filepath.Glob(rf)
		if err != nil {
			return err
		}
		if len(m) == 0 {
			fmt.Fprintln(os.Stderr, "  WARNING: no file match pattern", rf)
		}
		globbedFiles = append(globbedFiles, m...)
	}
	utf.RuleFiles = globbedFiles
	return nil
}

func (tg *testGroup) test(evalInterval time.Duration, groupOrderMap map[string]int, testGroups []vmalertconfig.Group) (checkErrs []error) {
	// set up vmstorage and http server for ingest and read queries
	setUp()
	// tear down vmstorage and clean the data dir
	defer tearDown()

	err := unittest.WriteInputSeries(tg.InputSeries, tg.Interval, testStartTime, testPromWriteHTTPPath)
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
	alertExpResultMap := map[time.Duration]map[string]map[string][]unittest.ExpAlert{}
	for _, at := range tg.AlertRuleTests {
		et := at.EvalTime.Duration()
		alertEvalTimesMap[et] = struct{}{}
		if _, ok := alertExpResultMap[et]; !ok {
			alertExpResultMap[et] = make(map[string]map[string][]unittest.ExpAlert)
		}
		if _, ok := alertExpResultMap[et][at.GroupName]; !ok {
			alertExpResultMap[et][at.GroupName] = make(map[string][]unittest.ExpAlert)
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
	var groups []*Group
	for _, group := range testGroups {
		ng := newGroup(group, q, tg.ExternalLabels)
		groups = append(groups, ng)
	}

	e := &executor{
		rw:                       rw,
		notifiers:                func() []notifier.Notifier { return nil },
		previouslySentSeriesToRW: make(map[uint64]map[string][]prompbmarshal.Label),
	}

	evalIndex := 0
	maxEvalTime := testStartTime.Add(tg.maxEvalTime())
	for ts := testStartTime; ts.Before(maxEvalTime) || ts.Equal(maxEvalTime); ts = ts.Add(evalInterval) {
		for _, g := range groups {
			resolveDuration := getResolveDuration(g.Interval, *resendDelay, *maxResolveDuration)
			errs := e.execConcurrently(context.Background(), g.Rules, ts, g.Concurrency, resolveDuration, g.Limit)
			for err := range errs {
				if err != nil {
					checkErrs = append(checkErrs, fmt.Errorf("\nfailed to exec group: %q, time: %s, err: %w", g.Name,
						ts, err))
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
			gotAlertsMap := map[string]map[string]unittest.LabelsAndAnnotations{}
			for _, g := range groups {
				if *disableAlertGroupLabel {
					g.Name = ""
				}
				if _, ok := alertExpResultMap[time.Duration(ts.UnixNano())][g.Name]; !ok {
					continue
				}
				if _, ok := gotAlertsMap[g.Name]; !ok {
					gotAlertsMap[g.Name] = make(map[string]unittest.LabelsAndAnnotations)
				}
				for _, rule := range g.Rules {
					ar, isAlertRule := rule.(*AlertingRule)
					if !isAlertRule {
						continue
					}
					if _, ok := alertExpResultMap[time.Duration(ts.UnixNano())][g.Name][ar.Name]; ok {
						for _, got := range ar.alerts {
							if got.State != notifier.StateFiring {
								continue
							}
							laa := unittest.LabelAndAnnotation{
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
					var expAlerts unittest.LabelsAndAnnotations
					for _, expAlert := range res {
						if expAlert.ExpLabels == nil {
							expAlert.ExpLabels = make(map[string]string)
						}
						// alertGroupNameLabel is added as additional labels when `disableAlertGroupLabel` is false
						if !*disableAlertGroupLabel {
							expAlert.ExpLabels[alertGroupNameLabel] = groupname
						}
						// alertNameLabel is added as additional labels in vmalert.
						expAlert.ExpLabels[alertNameLabel] = alertname
						expAlerts = append(expAlerts, unittest.LabelAndAnnotation{
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
						expString := unittest.IndentLines(expAlerts.String(), "            ")
						gotString := unittest.IndentLines(gotAlerts.String(), "            ")
						checkErrs = append(checkErrs, fmt.Errorf("\n%s    groupname: %s, alertname: %s, time: %s, \n        exp:%v, \n        got:%v ",
							testGroupName, groupname, alertname, alertEvalTimes[evalIndex].String(), expString, gotString))
					}
				}
			}
			evalIndex++
		}

	}

	checkErrs = append(checkErrs, unittest.CheckMetricsqlCase(tg.MetricsqlExprTests, q)...)
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
	Interval           *promutils.Duration          `yaml:"interval"`
	InputSeries        []unittest.Series            `yaml:"input_series"`
	AlertRuleTests     []unittest.AlertTestCase     `yaml:"alert_rule_test"`
	MetricsqlExprTests []unittest.MetricsqlTestCase `yaml:"metricsql_expr_test"`
	ExternalLabels     map[string]string            `yaml:"external_labels"`
	TestGroupName      string                       `yaml:"name"`
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
