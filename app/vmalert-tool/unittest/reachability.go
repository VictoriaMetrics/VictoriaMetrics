package unittest

import (
	"github.com/VictoriaMetrics/metricsql"

	vmalertconfig "github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
)

// alertMetricName is the series vmalert writes for every pending or firing rule.
const alertMetricName = "ALERTS"

// ruleGraph describes which metrics every rule group produces and reads.
// It is used to narrow the rule groups evaluated for a single test group.
type ruleGraph struct {
	// producedBy maps a recorded metric name to the rule groups recording it.
	producedBy map[string][]int
	// alertsIn maps an alert name to the rule groups defining it.
	alertsIn map[string][]int
	// reads holds the metric names read by every rule group.
	reads [][]string
	// readsAlerts holds the alert names every rule group reads through ALERTS.
	readsAlerts [][]string
	// readsAnyAlert marks groups reading ALERTS without pinning an alert name.
	readsAnyAlert []bool
	// dynamic marks rule groups that cannot be analysed statically, because they
	// select metrics by a regexp or by label filters without a metric name.
	// Such groups are always evaluated.
	dynamic []bool
}

// newRuleGraph builds the dependency graph between the given rule groups.
//
// A group whose expression cannot be parsed or which selects series without naming
// a metric is marked dynamic and is never skipped, so an incomplete analysis can
// only cost extra evaluations, never a missed one.
func newRuleGraph(groups []vmalertconfig.Group) *ruleGraph {
	rg := &ruleGraph{
		producedBy:    make(map[string][]int),
		alertsIn:      make(map[string][]int),
		reads:         make([][]string, len(groups)),
		readsAlerts:   make([][]string, len(groups)),
		readsAnyAlert: make([]bool, len(groups)),
		dynamic:       make([]bool, len(groups)),
	}
	for i, g := range groups {
		readSet := make(map[string]struct{})
		alertSet := make(map[string]struct{})
		for _, r := range g.Rules {
			if r.Record != "" {
				rg.producedBy[r.Record] = append(rg.producedBy[r.Record], i)
			}
			if r.Alert != "" {
				rg.alertsIn[r.Alert] = append(rg.alertsIn[r.Alert], i)
			}
			alertNames, pinned := alertNamesInExpr(r.Expr)
			if !pinned {
				rg.readsAnyAlert[i] = true
			}
			for _, n := range alertNames {
				alertSet[n] = struct{}{}
			}
			names, ok := metricNamesInExpr(r.Expr)
			if !ok {
				rg.dynamic[i] = true
				continue
			}
			for _, n := range names {
				readSet[n] = struct{}{}
			}
		}
		reads := make([]string, 0, len(readSet))
		for n := range readSet {
			reads = append(reads, n)
		}
		rg.reads[i] = reads
		alerts := make([]string, 0, len(alertSet))
		for n := range alertSet {
			alerts = append(alerts, n)
		}
		rg.readsAlerts[i] = alerts
	}
	return rg
}

// metricNamesInExpr returns the metric names selected by the given expression.
//
// The second return value is false when the expression selects series without
// naming a metric, for example by a regexp on __name__ or by label filters only.
// The set of names is meaningless in that case, since the selector may match
// metrics that are not mentioned anywhere in the expression.
func metricNamesInExpr(expr string) ([]string, bool) {
	e, err := metricsql.Parse(expr)
	if err != nil {
		return nil, false
	}
	var names []string
	analysable := true
	metricsql.VisitAll(e, func(x metricsql.Expr) {
		me, ok := x.(*metricsql.MetricExpr)
		if !ok {
			return
		}
		if len(me.LabelFilterss) == 0 {
			// A selector without label filters matches everything.
			analysable = false
			return
		}
		for _, lfs := range me.LabelFilterss {
			name, ok := metricNameFromFilters(lfs)
			if !ok {
				analysable = false
				return
			}
			names = append(names, name)
		}
	})
	if !analysable {
		return nil, false
	}
	return names, true
}

// alertNamesInExpr returns the alert names the expression reads through ALERTS.
// No rule records ALERTS, so this dependency is invisible to metricNamesInExpr.
//
// The second return value is false when ALERTS is read without pinning
// alertname to an exact value.
func alertNamesInExpr(expr string) ([]string, bool) {
	e, err := metricsql.Parse(expr)
	if err != nil {
		// Unparsable expressions already mark the group dynamic.
		return nil, true
	}
	var names []string
	pinned := true
	metricsql.VisitAll(e, func(x metricsql.Expr) {
		me, ok := x.(*metricsql.MetricExpr)
		if !ok {
			return
		}
		for _, lfs := range me.LabelFilterss {
			name, ok := metricNameFromFilters(lfs)
			if !ok || name != alertMetricName {
				continue
			}
			alertname, ok := exactLabelValue(lfs, "alertname")
			if !ok {
				pinned = false
				continue
			}
			names = append(names, alertname)
		}
	})
	return names, pinned
}

// exactLabelValue returns the value the given label is pinned to.
func exactLabelValue(lfs []metricsql.LabelFilter, label string) (string, bool) {
	for _, lf := range lfs {
		if lf.Label != label {
			continue
		}
		if lf.IsRegexp || lf.IsNegative {
			return "", false
		}
		return lf.Value, true
	}
	return "", false
}

// metricNameFromFilters returns the metric name selected by a single group of
// label filters. It reports false when the group does not pin __name__ to an
// exact value.
func metricNameFromFilters(lfs []metricsql.LabelFilter) (string, bool) {
	for _, lf := range lfs {
		if lf.Label != "__name__" {
			continue
		}
		if lf.IsRegexp || lf.IsNegative {
			return "", false
		}
		return lf.Value, true
	}
	return "", false
}

// reachableGroups returns the indexes of rule groups that must be evaluated for
// the given test group, in the order the groups were given.
//
// The result starts from the groups the test asserts on directly, and is then
// expanded over recording rule dependencies: if a needed group reads a metric
// recorded by another group, that group is needed too. Groups that cannot be
// analysed statically are always included.
func (rg *ruleGraph) reachableGroups(tg *testGroup, total int) []int {
	needed := make([]bool, total)
	var queue []int

	add := func(i int) {
		if !needed[i] {
			needed[i] = true
			queue = append(queue, i)
		}
	}

	for i, isDynamic := range rg.dynamic {
		if isDynamic {
			add(i)
		}
	}
	for _, at := range tg.AlertRuleTests {
		for _, i := range rg.alertsIn[at.Alertname] {
			add(i)
		}
	}
	for _, mt := range tg.MetricsqlExprTests {
		names, ok := metricNamesInExpr(mt.Expr)
		if !ok {
			// The assertion may read anything, so nothing can be skipped.
			return allIndexes(total)
		}
		for _, n := range names {
			for _, i := range rg.producedBy[n] {
				add(i)
			}
		}
	}

	// A group reading ALERTS without pinning an alert name may observe any alert,
	// so every group that defines one has to be evaluated.
	for i, anyAlert := range rg.readsAnyAlert {
		if anyAlert && needed[i] {
			for _, idxs := range rg.alertsIn {
				for _, dep := range idxs {
					add(dep)
				}
			}
			break
		}
	}

	// Expand over recording and alert dependencies.
	for len(queue) > 0 {
		i := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		for _, name := range rg.reads[i] {
			for _, dep := range rg.producedBy[name] {
				add(dep)
			}
		}
		for _, name := range rg.readsAlerts[i] {
			for _, dep := range rg.alertsIn[name] {
				add(dep)
			}
		}
	}

	res := make([]int, 0, total)
	for i, ok := range needed {
		if ok {
			res = append(res, i)
		}
	}
	return res
}

func allIndexes(n int) []int {
	res := make([]int, n)
	for i := range res {
		res[i] = i
	}
	return res
}
