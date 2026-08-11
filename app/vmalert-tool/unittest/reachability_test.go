package unittest

import (
	"reflect"
	"slices"
	"testing"

	vmalertconfig "github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func TestMetricNamesInExpr_Analysable(t *testing.T) {
	f := func(expr string, namesExpected []string) {
		t.Helper()

		names, ok := metricNamesInExpr(expr)
		if !ok {
			t.Fatalf("expecting analysable expression %q", expr)
		}
		slices.Sort(names)
		slices.Sort(namesExpected)
		if !reflect.DeepEqual(names, namesExpected) {
			t.Fatalf("unexpected metric names for %q; got %q; want %q", expr, names, namesExpected)
		}
	}

	f(`up`, []string{"up"})
	f(`up == 0`, []string{"up"})
	f(`{__name__="up"}`, []string{"up"})
	f(`sum(rate(foo_total[5m])) by (job)`, []string{"foo_total"})
	f(`count_over_time(up[5m:])`, []string{"up"})
	f(`absent(test)`, []string{"test"})
	f(`histogram_quantile(0.99, sum(rate(lat_bucket[5m])) by (le))`, []string{"lat_bucket"})
	f(`label_replace(foo, "a", "b", "c", "d")`, []string{"foo"})
	f(`foo / on(a) bar`, []string{"bar", "foo"})
	f(`foo > 0 and on(instance) bar{tag="x"}`, []string{"bar", "foo"})

	// Expressions without any selector are analysable and read nothing.
	f(`1`, nil)
	f(`time()`, nil)
}

func TestMetricNamesInExpr_Unanalysable(t *testing.T) {
	f := func(expr string) {
		t.Helper()

		names, ok := metricNamesInExpr(expr)
		if ok {
			t.Fatalf("expecting unanalysable expression %q; got names %q", expr, names)
		}
	}

	// A regexp or a negated matcher on __name__ may select metrics which are not
	// mentioned in the expression.
	f(`{__name__=~"blobby.*"}`)
	f(`{__name__!="up"}`)

	// Label filters without a metric name match everything.
	f(`{job="x"}`)
	f(`sum({job="x"}) by (instance)`)

	// A single unanalysable selector poisons the whole expression.
	f(`up + {job="x"}`)

	f(`not a valid expr(((`)
}

func TestAlertNamesInExpr(t *testing.T) {
	f := func(expr string, namesExpected []string, pinnedExpected bool) {
		t.Helper()

		names, pinned := alertNamesInExpr(expr)
		if pinned != pinnedExpected {
			t.Fatalf("unexpected pinned for %q; got %v; want %v", expr, pinned, pinnedExpected)
		}
		slices.Sort(names)
		slices.Sort(namesExpected)
		if !reflect.DeepEqual(names, namesExpected) {
			t.Fatalf("unexpected alert names for %q; got %q; want %q", expr, names, namesExpected)
		}
	}

	f(`ALERTS{alertname="Foo"}`, []string{"Foo"}, true)
	f(`ALERTS_FOR_STATE{alertname="Foo"}`, []string{"Foo"}, true)
	f(`ALERTS_FOR_STATE`, nil, false)
	f(`ALERTS{alertname="Foo", alertstate="firing"}`, []string{"Foo"}, true)
	f(`count(ALERTS{alertname="Foo"}) by (instance)`, []string{"Foo"}, true)

	// ALERTS read without pinning an exact alert name may observe any alert.
	f(`ALERTS`, nil, false)
	f(`ALERTS{alertstate="firing"}`, nil, false)
	f(`ALERTS{alertname=~"Foo.*"}`, nil, false)
	f(`ALERTS{alertname!="Foo"}`, nil, false)

	// Expressions not reading ALERTS at all pin nothing and read no alert.
	f(`up == 0`, nil, true)
}

// testRuleGroups builds rule groups from a compact description, so that a test
// can state only the record, alert and expr fields the analysis looks at.
func testRuleGroups(groups ...vmalertconfig.Group) []vmalertconfig.Group {
	return groups
}

func group(name string, rules ...vmalertconfig.Rule) vmalertconfig.Group {
	return vmalertconfig.Group{Name: name, Rules: rules}
}

func record(name, expr string) vmalertconfig.Rule {
	return vmalertconfig.Rule{Record: name, Expr: expr}
}

func alert(name, expr string) vmalertconfig.Rule {
	return vmalertconfig.Rule{Alert: name, Expr: expr}
}

func TestReachableGroups(t *testing.T) {
	f := func(groups []vmalertconfig.Group, tg *testGroup, namesExpected []string) {
		t.Helper()

		rg := newRuleGraph(groups)
		idxs := rg.reachableGroups(tg, len(groups))
		var names []string
		for _, i := range idxs {
			names = append(names, groups[i].Name)
		}
		if !reflect.DeepEqual(names, namesExpected) {
			t.Fatalf("unexpected reachable groups; got %q; want %q", names, namesExpected)
		}
	}

	alertOn := func(names ...string) *testGroup {
		tg := &testGroup{}
		for _, n := range names {
			tg.AlertRuleTests = append(tg.AlertRuleTests, alertTestCase{
				Alertname: n,
				EvalTime:  promutil.NewDuration(0),
			})
		}
		return tg
	}
	exprOn := func(exprs ...string) *testGroup {
		tg := &testGroup{}
		for _, e := range exprs {
			tg.MetricsqlExprTests = append(tg.MetricsqlExprTests, metricsqlTestCase{
				Expr:     e,
				EvalTime: promutil.NewDuration(0),
			})
		}
		return tg
	}

	// Only the group defining the asserted alert is needed.
	independent := testRuleGroups(
		group("g1", alert("A1", `up == 0`)),
		group("g2", alert("A2", `down == 1`)),
	)
	f(independent, alertOn("A1"), []string{"g1"})
	f(independent, alertOn("A2"), []string{"g2"})
	f(independent, alertOn("A1", "A2"), []string{"g1", "g2"})

	// A test asserting nothing needs no group.
	f(independent, &testGroup{}, nil)

	// An alert reading a recorded metric pulls in the recording group, and the
	// closure keeps walking through a chain of recording rules.
	chain := testRuleGroups(
		group("base", record("r1", `raw`)),
		group("mid", record("r2", `r1 * 2`)),
		group("top", alert("A", `r2 > 1`)),
		group("unrelated", record("r3", `other`)),
	)
	f(chain, alertOn("A"), []string{"base", "mid", "top"})

	// Asserting a recorded metric directly pulls in its producer and its inputs.
	f(chain, exprOn(`r2`), []string{"base", "mid"})
	f(chain, exprOn(`r3`), []string{"unrelated"})

	// The same alert name defined in several groups needs all of them, since the
	// assertion cannot tell which group produced it.
	dup := testRuleGroups(
		group("g1", alert("Same", `up == 0`)),
		group("g2", alert("Same", `down == 1`)),
		group("g3", alert("Other", `x > 0`)),
	)
	f(dup, alertOn("Same"), []string{"g1", "g2"})

	// Groups that cannot be analysed statically are always evaluated.
	dynamic := testRuleGroups(
		group("g1", alert("A", `up == 0`)),
		group("catchall", record("r", `{job="x"}`)),
	)
	f(dynamic, alertOn("A"), []string{"g1", "catchall"})

	// An unanalysable assertion cannot be narrowed down at all.
	f(independent, exprOn(`{job="x"}`), []string{"g1", "g2"})

	// A group reading ALERTS for a named alert pulls in the group defining it.
	viaAlerts := testRuleGroups(
		group("src", alert("Src", `up == 0`)),
		group("agg", alert("Agg", `count(ALERTS{alertname="Src"}) > 0`)),
		group("unrelated", alert("Other", `x > 0`)),
	)
	f(viaAlerts, alertOn("Agg"), []string{"src", "agg"})

	// A group reading ALERTS without a name may observe any alert, so every group
	// defining one is needed.
	viaAnyAlert := testRuleGroups(
		group("src", alert("Src", `up == 0`)),
		group("agg", alert("Agg", `count(ALERTS) > 0`)),
		group("other", alert("Other", `x > 0`)),
		group("rec", record("r", `raw`)),
	)
	f(viaAnyAlert, alertOn("Agg"), []string{"src", "agg", "other"})

	// Reading ALERTS in a group the test does not need must not pull anything in.
	f(viaAnyAlert, exprOn(`r`), []string{"rec"})

	// A group reading ALERTS without a name is often reached only by expanding the
	// dependencies of another group, so it must be expanded as well.
	indirectAnyAlert := testRuleGroups(
		group("src", alert("Src", `up == 0`)),
		group("mid", record("r_mid", `count(ALERTS)`)),
		group("top", record("r_top", `r_mid * 2`)),
	)
	f(indirectAnyAlert, exprOn(`r_top`), []string{"src", "mid", "top"})

	// ALERTS_FOR_STATE is generated for alerts too, so reading it depends on the
	// group defining the alert.
	viaAlertsForState := testRuleGroups(
		group("src", alert("Src", `up == 0`)),
		group("reader", record("r_state", `ALERTS_FOR_STATE{alertname="Src"}`)),
		group("unrelated", alert("Other", `x > 0`)),
	)
	f(viaAlertsForState, exprOn(`r_state`), []string{"src", "reader"})

	// ALERTS_FOR_STATE without an alert name may observe any alert.
	viaAnyAlertsForState := testRuleGroups(
		group("src", alert("Src", `up == 0`)),
		group("reader", record("r_state", `ALERTS_FOR_STATE`)),
		group("other", alert("Other", `x > 0`)),
	)
	f(viaAnyAlertsForState, exprOn(`r_state`), []string{"src", "reader", "other"})
}

// TestReachableGroups_Cycle makes sure a recording rule cycle terminates instead
// of expanding forever.
func TestReachableGroups_Cycle(t *testing.T) {
	groups := testRuleGroups(
		group("g1", record("a", `b`)),
		group("g2", record("b", `a`)),
		group("g3", alert("A", `a > 0`)),
	)
	rg := newRuleGraph(groups)
	tg := &testGroup{
		AlertRuleTests: []alertTestCase{{Alertname: "A", EvalTime: promutil.NewDuration(0)}},
	}
	idxs := rg.reachableGroups(tg, len(groups))
	if len(idxs) != 3 {
		t.Fatalf("expecting all groups of a cycle; got %v", idxs)
	}
}
