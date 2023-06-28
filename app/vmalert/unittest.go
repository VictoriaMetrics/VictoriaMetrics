package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v2"

	testutil "github.com/VictoriaMetrics/VictoriaMetrics/app/victoria-metrics/test"
	vmalertconfig "github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/metrics"
	"github.com/VictoriaMetrics/metricsql"
)

var (
	storagePath string
	// insert series from 1970-01-01T00:00:00
	testStartTime = time.Unix(0, 0).UTC()

	testPromWriteHTTPPath = "http://127.0.0.1" + *httpListenAddr + "/api/v1/write"
	testHealthHTTPPath    = "http://127.0.0.1" + *httpListenAddr + "/health"
	testDataSourcePath    = "http://127.0.0.1" + *httpListenAddr + "/prometheus"
	testRemoteWritePath   = "http://127.0.0.1" + *httpListenAddr
)

const (
	testFixturesDir   = "ruletest"
	testStorageSuffix = "vm-test-storage"
	testLogLevel      = "ERROR"

	testStorageInitTimeout = 10 * time.Second
)

func unitRule(files ...string) bool {
	storagePath = filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d", testStorageSuffix, time.Now().Unix()))
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
			fmt.Println(err)
			failed = true
		} else {
			fmt.Println("  SUCCESS")
		}
	}
	return failed
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
		{flag: "disableAlertgroupLabel", value: "true"},
	} {
		// panics if flag doesn't exist
		if err := flag.Lookup(fv.flag).Value.Set(fv.value); err != nil {
			logger.Fatalf("unable to set %q with value %q, err: %v", fv.flag, fv.value, err)
		}
	}
}

func testrequestHandler(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/" {
		if r.Method != http.MethodGet {
			return false
		}
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<h2>Single-node VictoriaMetrics</h2></br>")
		fmt.Fprintf(w, "See docs at <a href='https://docs.victoriametrics.com/'>https://docs.victoriametrics.com/</a></br>")
		fmt.Fprintf(w, "Useful endpoints:</br>")
		httpserver.WriteAPIHelp(w, [][2]string{
			{"vmui", "Web UI"},
			{"targets", "status for discovered active targets"},
			{"service-discovery", "labels before and after relabeling for discovered targets"},
			{"metric-relabel-debug", "debug metric relabeling"},
			{"expand-with-exprs", "WITH expressions' tutorial"},
			{"api/v1/targets", "advanced information about discovered targets in JSON format"},
			{"config", "-promscrape.config contents"},
			{"metrics", "available service metrics"},
			{"flags", "command-line flags"},
			{"api/v1/status/tsdb", "tsdb status page"},
			{"api/v1/status/top_queries", "top queries"},
			{"api/v1/status/active_queries", "active queries"},
		})
		return true
	}
	if vminsert.RequestHandler(w, r) {
		return true
	}
	if vmselect.RequestHandler(w, r) {
		return true
	}
	if vmstorage.RequestHandler(w, r) {
		return true
	}
	return false
}

func waitFor(timeout time.Duration, f func() bool) error {
	fraction := timeout / 10
	for i := fraction; i < timeout; i += fraction {
		if f() {
			return nil
		}
		time.Sleep(fraction)
	}
	return fmt.Errorf("timeout")
}

func setUp() {
	vmstorage.Init(promql.ResetRollupResultCacheIfNeeded)
	go httpserver.Serve(*httpListenAddr, false, testrequestHandler)
	readyStorageCheckFunc := func() bool {
		resp, err := http.Get(testHealthHTTPPath)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == 200
	}
	if err := waitFor(testStorageInitTimeout, readyStorageCheckFunc); err != nil {
		logger.Fatalf("http server can't start for %s seconds, err %s", testStorageInitTimeout, err)
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

func ruleUnitTest(filename string) []error {
	fmt.Println("\nUnit Testing: ", filename)
	b, err := os.ReadFile(filename)
	if err != nil {
		return []error{fmt.Errorf("failed to read unit test file %s: %v", filename, err)}
	}

	var unitTestInp unitTestFile
	if err := yaml.UnmarshalStrict(b, &unitTestInp); err != nil {
		return []error{fmt.Errorf("failed to unmarshal unit test file %s: %v", filename, err)}
	}
	if err := resolveAndGlobFilepaths(filepath.Dir(filename), &unitTestInp); err != nil {
		return []error{fmt.Errorf("failed to unmarshal unit test file %s: %v", filename, err)}
	}

	if unitTestInp.EvaluationInterval.Duration() == 0 {
		unitTestInp.EvaluationInterval = &promutils.Duration{D: 1 * time.Minute}
	}

	groupOrderMap := make(map[string]int)
	for i, gn := range unitTestInp.GroupEvalOrder {
		if _, ok := groupOrderMap[gn]; ok {
			return []error{fmt.Errorf("group name repeated in evaluation order: %s", gn)}
		}
		groupOrderMap[gn] = i
	}

	var errs []error
	for _, t := range unitTestInp.Tests {
		errs = append(errs, t.test(unitTestInp.EvaluationInterval.Duration(), groupOrderMap, unitTestInp.RuleFiles...)...)
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func httpWrite(address string, r io.Reader) {
	resp, err := http.Post(address, "", r)
	if err != nil || resp.StatusCode != 204 {
		logger.Errorf("failed to send to storage: %v", err)
	}
	resp.Body.Close()
}

func (tg *testGroup) test(evalInterval time.Duration, groupOrderMap map[string]int, ruleFiles ...string) []error {
	// set up vmstorage and http server
	setUp()
	// tear down vmstorage and clean the data dir
	defer tearDown()
	r := testutil.WriteRequest{}
	for _, data := range tg.InputSeries {
		expr, err := metricsql.Parse(data.Series)
		if err != nil {
			return []error{fmt.Errorf("failed to parse series %s: %v", data.Series, err)}
		}
		promvals, err := parseInputValue(data.Values, true)
		if err != nil {
			return []error{fmt.Errorf("failed to parse input series value %s: %v", data.Values, err)}
		}
		metricExpr, ok := expr.(*metricsql.MetricExpr)
		if !ok {
			return []error{fmt.Errorf("failed to parse series %s to metric expr: %v", data.Series, err)}
		}
		samples := make([]testutil.Sample, 0, len(promvals))
		// add samples from 1970-01-01T00:00:00
		ts := testStartTime
		for _, v := range promvals {
			if !v.omitted {
				samples = append(samples, testutil.Sample{
					Timestamp: ts.UnixMilli(),
					Value:     v.value,
				})
			}
			ts = ts.Add(tg.Interval.Duration())
		}
		var ls []testutil.Label
		for _, filter := range metricExpr.LabelFilters {
			ls = append(ls, testutil.Label{Name: filter.Label, Value: filter.Value})
		}
		r.Timeseries = append(r.Timeseries, testutil.TimeSeries{Labels: ls, Samples: samples})
	}
	testGroups, err := vmalertconfig.Parse(ruleFiles, nil, true)
	if err != nil {
		return []error{fmt.Errorf("failed to parse test group: %v", err)}
	}
	// sort group eval order according to given "group_eval_order".
	sort.Slice(testGroups, func(i, j int) bool {
		return groupOrderMap[testGroups[i].Name] < groupOrderMap[testGroups[j].Name]
	})

	// write input series to vm
	data, err := testutil.Compress(r)
	if err != nil {
		return []error{fmt.Errorf("failed to compress data: %v", err)}
	}
	httpWrite(testPromWriteHTTPPath, bytes.NewBuffer(data))
	vmstorage.Storage.DebugFlush()

	alertEvalTimesMap := map[time.Duration]struct{}{}
	alertExpResultMap := map[time.Duration]map[string][]alert{}
	for _, at := range tg.AlertRuleTests {
		alertEvalTimesMap[at.EvalTime.Duration()] = struct{}{}
		if _, ok := alertExpResultMap[at.EvalTime.Duration()]; !ok {
			alertExpResultMap[at.EvalTime.Duration()] = make(map[string][]alert)
		}
		alertExpResultMap[at.EvalTime.Duration()][at.Alertname] = at.ExpAlerts
	}
	alertEvalTimes := make([]time.Duration, 0, len(alertEvalTimesMap))
	for k := range alertEvalTimesMap {
		alertEvalTimes = append(alertEvalTimes, k)
	}
	sort.Slice(alertEvalTimes, func(i, j int) bool {
		return alertEvalTimes[i] < alertEvalTimes[j]
	})

	maxEvalTime := testStartTime.Add(tg.maxEvalTime())

	q, err := datasource.Init(url.Values{"nocache": {"1"}})
	if err != nil {
		return []error{fmt.Errorf("failed to init datasource: %v", err)}
	}
	rw, err := remotewrite.Init(context.Background())
	if err != nil {
		return []error{fmt.Errorf("failed to init wr: %v", err)}
	}

	var groups []*Group
	for _, group := range testGroups {
		ng := newGroup(group, q, *evaluationInterval, tg.ExternalLabels)
		groups = append(groups, ng)
	}
	e := &executor{
		rw:                       rw,
		notifiers:                func() []notifier.Notifier { return nil },
		previouslySentSeriesToRW: make(map[uint64]map[string][]prompbmarshal.Label),
	}

	var checkErrs []error
	evalIndex := 0
	for ts := testStartTime; ts.Before(maxEvalTime) || ts.Equal(maxEvalTime); ts = ts.Add(evalInterval) {
		for _, g := range groups {
			resolveDuration := getResolveDuration(g.Interval, *resendDelay, *maxResolveDuration)
			errs := e.execConcurrently(context.Background(), g.Rules, ts, g.Concurrency, resolveDuration, g.Limit)
			for err := range errs {
				if err != nil {
					checkErrs = append(checkErrs, fmt.Errorf("failed to exec group: %q, time: %s, err: %w", g.Name,
						ts, err))
				}
			}
			// need to flush immediately cause evaluation may rely on other group's result.
			// we don't rely on remoteWrite.flushInterval to do this, cause it can't be controlled.
			rw.DebugFlush()
			vmstorage.Storage.DebugFlush()
		}

		// check alert at every eval time
		for evalIndex < len(alertEvalTimes) {
			if ts.Sub(testStartTime) > alertEvalTimes[evalIndex] ||
				alertEvalTimes[evalIndex] >= ts.Add(evalInterval).Sub(testStartTime) {
				break
			}
			gotAlertsMap := map[string]labelsAndAnnotations{}
			for _, g := range groups {
				for _, rule := range g.Rules {
					ar, isAlertRule := rule.(*AlertingRule)
					if !isAlertRule {
						continue
					}
					if _, ok := alertExpResultMap[time.Duration(ts.UnixNano())][ar.Name]; ok {
						for _, got := range ar.alerts {
							if got.State != notifier.StateFiring {
								continue
							}
							gotAlertsMap[ar.Name] = append(gotAlertsMap[ar.Name], labelAndAnnotation{
								Labels:      convertToLabels(got.Labels),
								Annotations: convertToLabels(got.Annotations),
							})
						}
					}

				}
			}

			for alertName, res := range alertExpResultMap[alertEvalTimes[evalIndex]] {
				var expAlerts labelsAndAnnotations
				for _, expAlert := range res {
					if expAlert.ExpLabels == nil {
						expAlert.ExpLabels = make(map[string]string)
					}
					// alertNameLabel is added as additional labels in vmalert.
					expAlert.ExpLabels[alertNameLabel] = alertName
					expAlerts = append(expAlerts, labelAndAnnotation{
						Labels:      convertToLabels(expAlert.ExpLabels),
						Annotations: convertToLabels(expAlert.ExpAnnotations),
					})
				}
				sort.Sort(expAlerts)

				gotAlerts := gotAlertsMap[alertName]
				sort.Sort(gotAlerts)
				if !reflect.DeepEqual(expAlerts, gotAlerts) {
					var testName string
					if tg.TestGroupName != "" {
						testName = fmt.Sprintf("groupname: %s,\n", tg.TestGroupName)
					}
					expString := indentLines(expAlerts.String(), "            ")
					gotString := indentLines(gotAlerts.String(), "            ")
					checkErrs = append(checkErrs, fmt.Errorf("\n%s    alertname: %s, time: %s, \n        exp:%v, \n        got:%v ",
						testName, alertName, alertEvalTimes[evalIndex].String(), expString, gotString))
				}
			}
			evalIndex++
		}

	}

	// check metricsql expr test
	queries := q.BuildWithParams(datasource.QuerierParams{QueryParams: url.Values{"nocache": {"1"}, "latency_offset": {"1ms"}, "search.latencyOffset": {"1ms"}}, DataSourceType: "prometheus", Debug: true})
Outer:
	for _, mt := range tg.MetricsqlExprTests {
		result, _, err := queries.Query(context.Background(), mt.Expr, mt.EvalTime.ParseTime())
		if err != nil {
			checkErrs = append(checkErrs, fmt.Errorf("    expr: %q, time: %s, err: %w", mt.Expr,
				mt.EvalTime.Duration().String(), err))
			continue
		}
		var gotSamples []parsedSample
		for _, s := range result.Data {
			sort.Slice(s.Labels, func(i, j int) bool {
				return s.Labels[i].Name < s.Labels[j].Name
			})
			gotSamples = append(gotSamples, parsedSample{
				Labels: s.Labels,
				Value:  s.Values[0],
			})
		}
		var expSamples []parsedSample
		for _, s := range mt.ExpSamples {
			expLb := labels{}
			if s.Labels != "" {
				metricsqlExpr, err := metricsql.Parse(s.Labels)
				if err != nil {
					checkErrs = append(checkErrs, fmt.Errorf("\n    expr: %q, time: %s, err: %v", mt.Expr,
						mt.EvalTime.Duration().String(), fmt.Errorf("failed to parse labels %q: %w", s.Labels, err)))
					continue Outer
				}
				metricsqlMetricExpr, ok := metricsqlExpr.(*metricsql.MetricExpr)
				if !ok {
					checkErrs = append(checkErrs, fmt.Errorf("\n    expr: %q, time: %s, err: %v", mt.Expr,
						mt.EvalTime.Duration().String(), fmt.Errorf("got unsupported metricsql type")))
					continue Outer
				}
				for _, l := range metricsqlMetricExpr.LabelFilters {
					expLb = append(expLb, datasource.Label{
						Name:  l.Label,
						Value: l.Value,
					})
				}
			}
			sort.Slice(expLb, func(i, j int) bool {
				return expLb[i].Name < expLb[j].Name
			})
			expSamples = append(expSamples, parsedSample{
				Labels: expLb,
				Value:  s.Value,
			})
		}
		sort.Slice(expSamples, func(i, j int) bool {
			return labelCompare(expSamples[i].Labels, expSamples[j].Labels) <= 0
		})
		sort.Slice(gotSamples, func(i, j int) bool {
			return labelCompare(gotSamples[i].Labels, gotSamples[j].Labels) <= 0
		})
		if !reflect.DeepEqual(expSamples, gotSamples) {
			checkErrs = append(checkErrs, fmt.Errorf("\n    expr: %q, time: %s,\n        exp: %v\n        got: %v", mt.Expr,
				mt.EvalTime.Duration().String(), parsedSamplesString(expSamples), parsedSamplesString(gotSamples)))
		}

	}
	return checkErrs
}

// unitTestFile holds the contents of a single unit test file.
type unitTestFile struct {
	RuleFiles          []string            `yaml:"rule_files"`
	EvaluationInterval *promutils.Duration `yaml:"evaluation_interval"`
	GroupEvalOrder     []string            `yaml:"group_eval_order"`
	Tests              []testGroup         `yaml:"tests"`
}

// testGroup is a group of input series and tests associated with it.
type testGroup struct {
	Interval           *promutils.Duration `yaml:"interval"`
	InputSeries        []series            `yaml:"input_series"`
	AlertRuleTests     []alertTestCase     `yaml:"alert_rule_test"`
	MetricsqlExprTests []metricsqlTestCase `yaml:"metricsql_expr_test"`
	ExternalLabels     map[string]string   `yaml:"external_labels"`
	TestGroupName      string              `yaml:"name"`
}

// maxEvalTime returns the max eval time among all alert and promql unit tests.
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

type series struct {
	Series string `yaml:"series"`
	Values string `yaml:"values"`
}

type alertTestCase struct {
	EvalTime  *promutils.Duration `yaml:"eval_time"`
	Alertname string              `yaml:"alertname"`
	ExpAlerts []alert             `yaml:"exp_alerts"`
}

type alert struct {
	ExpLabels      map[string]string `yaml:"exp_labels"`
	ExpAnnotations map[string]string `yaml:"exp_annotations"`
}

type metricsqlTestCase struct {
	Expr       string              `yaml:"expr"`
	EvalTime   *promutils.Duration `yaml:"eval_time"`
	ExpSamples []sample            `yaml:"exp_samples"`
}

type sample struct {
	Labels string  `yaml:"labels"`
	Value  float64 `yaml:"value"`
}

// parsedSample is a sample with parsed Labels.
type parsedSample struct {
	Labels labels
	Value  float64
}

func (ps *parsedSample) String() string {
	return ps.Labels.String() + " " + strconv.FormatFloat(ps.Value, 'E', -1, 64)
}

func parsedSamplesString(pss []parsedSample) string {
	if len(pss) == 0 {
		return "nil"
	}
	s := pss[0].String()
	for _, ps := range pss[1:] {
		s += ", " + ps.String()
	}
	return s
}

// indentLines prefixes each line in the supplied string with the given "indent" string.
func indentLines(lines, indent string) string {
	sb := strings.Builder{}
	n := strings.Split(lines, "\n")
	for i, l := range n {
		if i > 0 {
			sb.WriteString(indent)
		}
		sb.WriteString(l)
		if i != len(n)-1 {
			sb.WriteRune('\n')
		}
	}
	return sb.String()
}

type labels []datasource.Label

func (ls labels) Len() int           { return len(ls) }
func (ls labels) Swap(i, j int)      { ls[i], ls[j] = ls[j], ls[i] }
func (ls labels) Less(i, j int) bool { return ls[i].Name < ls[j].Name }

func (ls labels) String() string {
	var b bytes.Buffer

	b.WriteByte('{')
	for i, l := range ls {
		if i > 0 {
			b.WriteByte(',')
			b.WriteByte(' ')
		}
		b.WriteString(l.Name)
		b.WriteByte('=')
		b.WriteString(strconv.Quote(l.Value))
	}
	b.WriteByte('}')
	return b.String()
}

func convertToLabels(m map[string]string) (labelset labels) {
	for k, v := range m {
		labelset = append(labelset, datasource.Label{
			Name:  k,
			Value: v,
		})
	}
	// sort label
	slices.SortFunc(labelset, func(a, b datasource.Label) bool { return a.Name < b.Name })
	return
}

type labelAndAnnotation struct {
	Labels      labels
	Annotations labels
}

func (la *labelAndAnnotation) String() string {
	return "Labels:" + la.Labels.String() + "\nAnnotations:" + la.Annotations.String()
}

type labelsAndAnnotations []labelAndAnnotation

func (la labelsAndAnnotations) Len() int { return len(la) }

func (la labelsAndAnnotations) Swap(i, j int) { la[i], la[j] = la[j], la[i] }
func (la labelsAndAnnotations) Less(i, j int) bool {
	diff := labelCompare(la[i].Labels, la[j].Labels)
	if diff != 0 {
		return diff < 0
	}
	return labelCompare(la[i].Annotations, la[j].Annotations) < 0
}

func (la labelsAndAnnotations) String() string {
	if len(la) == 0 {
		return "[]"
	}
	s := "[\n0:" + indentLines("\n"+la[0].String(), "  ")
	for i, l := range la[1:] {
		s += ",\n" + fmt.Sprintf("%d", i+1) + ":" + indentLines("\n"+l.String(), "  ")
	}
	s += "\n]"

	return s
}

func labelCompare(a, b labels) int {
	l := len(a)
	if len(b) < l {
		l = len(b)
	}

	for i := 0; i < l; i++ {
		if a[i].Name != b[i].Name {
			if a[i].Name < b[i].Name {
				return -1
			}
			return 1
		}
		if a[i].Value != b[i].Value {
			if a[i].Value < b[i].Value {
				return -1
			}
			return 1
		}
	}
	// if all labels so far were in common, the set with fewer labels comes first.
	return len(a) - len(b)
}

// parseInputValue support input like "1", "1+1x1 _ -4 3+20x1", see more examples in test.
func parseInputValue(input string, origin bool) ([]sequenceValue, error) {
	var res []sequenceValue
	items := strings.Split(input, " ")
	reg2 := regexp.MustCompile(`\D?\d*\D?`)
	for _, item := range items {
		vals := reg2.FindAllString(item, -1)
		switch len(vals) {
		case 1:
			if vals[0] == "_" {
				res = append(res, sequenceValue{omitted: true})
				continue
			}
			v, err := strconv.ParseFloat(vals[0], 64)
			if err != nil {
				return nil, err
			}
			res = append(res, sequenceValue{value: v})
			continue
		case 2:
			p1 := vals[0][:len(vals[0])-1]
			v2, err := strconv.ParseInt(vals[1], 10, 64)
			if err != nil {
				return nil, err
			}
			option := vals[0][len(vals[0])-1]
			switch option {
			case '+':
				v1, err := strconv.ParseFloat(p1, 64)
				if err != nil {
					return nil, err
				}
				res = append(res, sequenceValue{value: v1 + float64(v2)})
			case 'x':
				for i := 0; i <= int(v2); i++ {
					if p1 == "_" {
						if i == 0 {
							i = 1
						}
						res = append(res, sequenceValue{omitted: true})
						continue
					}
					v1, err := strconv.ParseFloat(p1, 64)
					if err != nil {
						return nil, err
					}
					if !origin || v1 == 0 {
						res = append(res, sequenceValue{value: v1 * float64(i)})
						continue
					}
					newVal := fmt.Sprintf("%s+0x%s", p1, vals[1])
					newRes, err := parseInputValue(newVal, false)
					if err != nil {
						return nil, err
					}
					res = append(res, newRes...)
					break
				}

			default:
				return nil, fmt.Errorf("got invalid operation %b", option)
			}
		case 3:
			r1, err := parseInputValue(fmt.Sprintf("%s%s", vals[1], vals[2]), false)
			if err != nil {
				return nil, err
			}
			p1 := vals[0][:len(vals[0])-1]
			v1, err := strconv.ParseFloat(p1, 64)
			if err != nil {
				return nil, err
			}
			option := vals[0][len(vals[0])-1]
			var isAdd bool
			if option == '+' {
				isAdd = true
			}
			for _, r := range r1 {
				if isAdd {
					res = append(res, sequenceValue{
						value: r.value + v1,
					})
				} else {
					res = append(res, sequenceValue{
						value: v1 - r.value,
					})
				}
			}
		default:
			return nil, fmt.Errorf("unsupported input %s", input)
		}
	}
	return res, nil
}

// sequenceValue is an omittable value in a sequence of time series values.
type sequenceValue struct {
	value   float64
	omitted bool
}
