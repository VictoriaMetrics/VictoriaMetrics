package unittest

import (
	"slices"
	"strings"
	"text/template/parse"

	"github.com/VictoriaMetrics/metricsql"

	vmalertconfig "github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
)

const (
	// Series vmalert writes for every pending or firing alert.
	alertMetricName         = "ALERTS"
	alertForStateMetricName = "ALERTS_FOR_STATE"

	// alertNameLabel is the label name indicating the name of an alert.
	alertNameLabel = "alertname"
	// metricNameLabel is the label name holding the metric name of a series.
	metricNameLabel = "__name__"
)

// ruleGraph describes which rule groups every rule group may read series from.
//
// A rule reads test input series, metrics recorded by rule groups and the ALERTS and
// ALERTS_FOR_STATE series of alerts defined by rule groups. Input series are always
// present, so only the last two create dependencies between groups.
type ruleGraph struct {
	// groups holds the rule groups indexed by the graph, in their original order.
	groups []vmalertconfig.Group
	// producedBy maps a recorded metric name to the rule groups recording it.
	producedBy map[string][]int
	// alertsIn maps an alert name to the rule groups defining it.
	alertsIn map[string][]int
	// deps holds, for every rule group, the groups it may read series from.
	deps [][]int
	// hasUnresolvedReads marks rule groups which may read series that cannot be
	// determined statically, for example through a templated query whose
	// expression is not a string literal. Once such a group is needed, every
	// group is.
	hasUnresolvedReads []bool
}

// newRuleGraph builds the dependency graph between the given rule groups.
func newRuleGraph(groups []vmalertconfig.Group) *ruleGraph {
	rg := &ruleGraph{
		groups:             groups,
		producedBy:         make(map[string][]int),
		alertsIn:           make(map[string][]int),
		deps:               make([][]int, len(groups)),
		hasUnresolvedReads: make([]bool, len(groups)),
	}
	for i, g := range groups {
		for _, r := range g.Rules {
			if r.Record != "" {
				rg.producedBy[r.Record] = append(rg.producedBy[r.Record], i)
			}
			if r.Alert != "" {
				rg.alertsIn[r.Alert] = append(rg.alertsIn[r.Alert], i)
			}
		}
	}
	// Selectors are resolved against the names collected above, so every group has to be indexed first.
	for i, g := range groups {
		depSet := make(map[int]struct{})
		for _, r := range g.Rules {
			exprs := []string{r.Expr}
			// Alert labels and annotations may run queries via the `query`
			// template function, which reads the same storage as the rule
			// expression does.
			for _, m := range []map[string]string{r.Labels, r.Annotations} {
				for _, text := range m {
					queries, ok := templateQueries(text)
					if !ok {
						rg.hasUnresolvedReads[i] = true
					}
					exprs = append(exprs, queries...)
				}
			}
			for _, expr := range exprs {
				idxs, ok := rg.resolveExpr(expr)
				if !ok {
					rg.hasUnresolvedReads[i] = true
					continue
				}
				for _, idx := range idxs {
					depSet[idx] = struct{}{}
				}
			}
		}
		rg.deps[i] = sortedKeys(depSet)
	}
	return rg
}

// resolveExpr returns the rule groups whose output the given expression may read.
//
// The second return value is false when the expression cannot be parsed.
func (rg *ruleGraph) resolveExpr(expr string) ([]int, bool) {
	e, err := metricsql.Parse(expr)
	if err != nil {
		return nil, false
	}
	depSet := make(map[int]struct{})
	metricsql.VisitAll(e, func(x metricsql.Expr) {
		me, ok := x.(*metricsql.MetricExpr)
		if !ok {
			return
		}
		if len(me.LabelFilterss) == 0 {
			// A selector without label filters matches every series.
			rg.resolveSelector(nil, depSet)
			return
		}
		for _, lfs := range me.LabelFilterss {
			rg.resolveSelector(lfs, depSet)
		}
	})
	return sortedKeys(depSet), true
}

// resolveSelector adds to dst the rule groups whose output may match a series
// selector made of the given label filters.
//
// Only the filters on __name__ and alertname are taken into account. Other
// label filters are ignored, which over-approximates the result but never
// misses a dependency.
func (rg *ruleGraph) resolveSelector(lfs []metricsql.LabelFilter, dst map[int]struct{}) {
	if name, ok := exactLabelValue(lfs, metricNameLabel); ok {
		rg.resolveName(name, lfs, dst)
		return
	}
	for name := range rg.producedBy {
		if labelMatches(lfs, metricNameLabel, name) {
			rg.resolveName(name, lfs, dst)
		}
	}
	for _, name := range []string{alertMetricName, alertForStateMetricName} {
		if labelMatches(lfs, metricNameLabel, name) {
			rg.resolveName(name, lfs, dst)
		}
	}
}

// resolveName adds to dst the rule groups producing series with the given metric
// name. For the alert series the alertname filters narrow down the alert groups.
func (rg *ruleGraph) resolveName(name string, lfs []metricsql.LabelFilter, dst map[int]struct{}) {
	for _, i := range rg.producedBy[name] {
		dst[i] = struct{}{}
	}
	if name != alertMetricName && name != alertForStateMetricName {
		return
	}
	if alertname, ok := exactLabelValue(lfs, alertNameLabel); ok {
		for _, i := range rg.alertsIn[alertname] {
			dst[i] = struct{}{}
		}
		return
	}
	for alertname, idxs := range rg.alertsIn {
		if !labelMatches(lfs, alertNameLabel, alertname) {
			continue
		}
		for _, i := range idxs {
			dst[i] = struct{}{}
		}
	}
}

// exactLabelValue returns the value the given label is pinned to by a
// `label="value"` filter. It reports false when the label is not filtered or is
// filtered by regexp or negative matchers only.
func exactLabelValue(lfs []metricsql.LabelFilter, label string) (string, bool) {
	for _, lf := range lfs {
		if lf.Label == label && !lf.IsRegexp && !lf.IsNegative {
			return lf.Value, true
		}
	}
	return "", false
}

// labelMatches reports whether a series with the given label value passes all
// the filters on that label. A filter with an invalid regexp is assumed to match.
func labelMatches(lfs []metricsql.LabelFilter, label, value string) bool {
	for _, lf := range lfs {
		if lf.Label != label {
			continue
		}
		var matches bool
		if lf.IsRegexp {
			re, err := metricsql.CompileRegexpAnchored(lf.Value)
			if err != nil {
				continue
			}
			matches = re.MatchString(value)
		} else {
			matches = lf.Value == value
		}
		if matches == lf.IsNegative {
			return false
		}
	}
	return true
}

// templateQueries returns the expressions passed as string literals to the
// `query` function in the given label or annotation template.
//
// The second return value is false when the template cannot be parsed or calls
// `query` with an expression that is not a string literal, since the series it
// reads cannot be determined then.
func templateQueries(text string) ([]string, bool) {
	if !strings.Contains(text, "{{") {
		return nil, true
	}
	t := parse.New("")
	t.Mode = parse.SkipFuncCheck
	trees := make(map[string]*parse.Tree)
	if _, err := t.Parse(notifier.TemplateHeader()+text, "", "", trees); err != nil {
		return nil, false
	}
	var queries []string
	resolved := true
	var walk func(n parse.Node)
	walk = func(n parse.Node) {
		switch n := n.(type) {
		case *parse.ListNode:
			if n == nil {
				return
			}
			for _, c := range n.Nodes {
				walk(c)
			}
		case *parse.ActionNode:
			walk(n.Pipe)
		case *parse.PipeNode:
			if n == nil {
				return
			}
			for _, c := range n.Cmds {
				walk(c)
			}
		case *parse.CommandNode:
			if len(n.Args) > 0 && isIdentifier(n.Args[0], "query") {
				s, ok := n.Args[len(n.Args)-1].(*parse.StringNode)
				if !ok || len(n.Args) != 2 {
					resolved = false
					return
				}
				queries = append(queries, s.Text)
				return
			}
			for _, c := range n.Args {
				walk(c)
			}
		case *parse.IfNode:
			walk(n.Pipe)
			walk(n.List)
			walk(n.ElseList)
		case *parse.RangeNode:
			walk(n.Pipe)
			walk(n.List)
			walk(n.ElseList)
		case *parse.WithNode:
			walk(n.Pipe)
			walk(n.List)
			walk(n.ElseList)
		case *parse.TemplateNode:
			walk(n.Pipe)
		case *parse.ChainNode:
			walk(n.Node)
		case *parse.IdentifierNode:
			// `query` in any position other than the head of a command, for
			// example `{{ "up" | query }}`, is not analysed.
			if n.Ident == "query" {
				resolved = false
			}
		}
	}
	for _, tree := range trees {
		walk(tree.Root)
	}
	return queries, resolved
}

func isIdentifier(n parse.Node, ident string) bool {
	id, ok := n.(*parse.IdentifierNode)
	return ok && id.Ident == ident
}

func sortedKeys(set map[int]struct{}) []int {
	if len(set) == 0 {
		return nil
	}
	res := make([]int, 0, len(set))
	for i := range set {
		res = append(res, i)
	}
	slices.Sort(res)
	return res
}

// reachableGroups returns the rule groups that must be evaluated for the given
// test group, in the order the groups were given.
//
// The result starts from the groups the test asserts on directly and is then
// expanded over dependencies: if a needed group may read a metric recorded by
// another group, or an alert defined by another group through the ALERTS series,
// that group is needed too. A needed group whose reads cannot be resolved
// statically makes every group needed.
func (rg *ruleGraph) reachableGroups(tg *testGroup) []vmalertconfig.Group {
	total := len(rg.deps)
	needed := make([]bool, total)
	var pending []int

	add := func(i int) {
		if !needed[i] {
			needed[i] = true
			pending = append(pending, i)
		}
	}

	for _, at := range tg.AlertRuleTests {
		for _, i := range rg.alertsIn[at.Alertname] {
			add(i)
		}
	}
	for _, mt := range tg.MetricsqlExprTests {
		idxs, ok := rg.resolveExpr(mt.Expr)
		if !ok {
			// The assertion may read anything, so nothing can be skipped.
			return rg.groups
		}
		for _, i := range idxs {
			add(i)
		}
	}

	for len(pending) > 0 {
		i := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if rg.hasUnresolvedReads[i] {
			return rg.groups
		}
		for _, dep := range rg.deps[i] {
			add(dep)
		}
	}

	res := make([]vmalertconfig.Group, 0, total)
	for i, ok := range needed {
		if ok {
			res = append(res, rg.groups[i])
		}
	}
	return res
}
