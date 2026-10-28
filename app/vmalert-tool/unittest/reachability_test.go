package unittest

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/metricsql"

	vmalertconfig "github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func TestTemplateQueries(t *testing.T) {
	f := func(text string, queriesExpected []string, resolvedExpected bool) {
		t.Helper()

		queries, resolved := templateQueries(text)
		if resolved != resolvedExpected {
			t.Fatalf("unexpected resolved for %q; got %v; want %v", text, resolved, resolvedExpected)
		}
		if !reflect.DeepEqual(queries, queriesExpected) {
			t.Fatalf("unexpected queries for %q; got %q; want %q", text, queries, queriesExpected)
		}
	}

	// Plain text and templates without the query function read nothing.
	f(`High latency`, nil, true)
	f(`{{ $labels.instance }} is down for {{ $value }}s`, nil, true)
	f(`{{ if gt $value 10.0 }}critical{{ else }}warning{{ end }}`, nil, true)

	// String literals passed to query are returned, wherever the call sits.
	f(`{{ query "up" | first | value }}`, []string{"up"}, true)
	f(`{{ with query "sum(rate(foo[5m]))" }}{{ . | first | value }}{{ end }}`, []string{"sum(rate(foo[5m]))"}, true)
	f(`{{ range query "up" }}{{ .Labels.instance }}{{ end }}`, []string{"up"}, true)
	f(`{{ if query "up" }}x{{ else if query "down" }}y{{ end }}`, []string{"up", "down"}, true)
	f(`{{ (query "up") | first }}`, []string{"up"}, true)
	f(`{{ define "x" }}{{ query "up" }}{{ end }}{{ template "x" . }}`, []string{"up"}, true)

	// A query built at run time cannot be analysed.
	f(`{{ query (printf "up{instance=%q}" $labels.instance) }}`, nil, false)
	f(`{{ "up" | query }}`, nil, false)
	f(`{{ $q := "up" }}{{ query $q }}`, nil, false)
	f(`{{ query }}`, nil, false)

	// Neither can a template which does not parse.
	f(`{{ query "up" `, nil, false)
}

func TestLabelMatches(t *testing.T) {
	f := func(selector, label, value string, expected bool) {
		t.Helper()

		e, err := metricsql.Parse(selector)
		if err != nil {
			t.Fatalf("cannot parse %q: %s", selector, err)
		}
		me, ok := e.(*metricsql.MetricExpr)
		if !ok || len(me.LabelFilterss) == 0 {
			t.Fatalf("expecting a metric selector for %q; got %T", selector, e)
		}
		lfs := me.LabelFilterss[0]
		if got := labelMatches(lfs, label, value); got != expected {
			t.Fatalf("unexpected result for %q on %s=%q; got %v; want %v", selector, label, value, got, expected)
		}
	}

	// A label which is not filtered matches any value.
	f(`{job="x"}`, "__name__", "up", true)

	f(`{__name__="up"}`, "__name__", "up", true)
	f(`{__name__="up"}`, "__name__", "down", false)
	f(`{__name__!="up"}`, "__name__", "up", false)
	f(`{__name__!="up"}`, "__name__", "down", true)

	// Regexps are anchored, as in the query engine.
	f(`{__name__=~"up.*"}`, "__name__", "uptime", true)
	f(`{__name__=~"up"}`, "__name__", "uptime", false)
	f(`{__name__=~"up|down"}`, "__name__", "down", true)
	f(`{__name__!~"up.*"}`, "__name__", "uptime", false)
	f(`{__name__!~"up.*"}`, "__name__", "down", true)

	// Every filter on the label has to pass.
	f(`{__name__=~"up.*", __name__!="uptime"}`, "__name__", "uptime", false)
	f(`{__name__=~"up.*", __name__!="uptime"}`, "__name__", "up", true)
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
		reachable := rg.reachableGroups(tg)
		var names []string
		for _, g := range reachable {
			names = append(names, g.Name)
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
	independent := []vmalertconfig.Group{
		group("g1", alert("A1", `up == 0`)),
		group("g2", alert("A2", `down == 1`)),
	}
	f(independent, alertOn("A1"), []string{"g1"})
	f(independent, alertOn("A2"), []string{"g2"})
	f(independent, alertOn("A1", "A2"), []string{"g1", "g2"})

	// A test asserting nothing needs no group.
	f(independent, &testGroup{}, nil)

	// An alert reading a recorded metric pulls in the recording group, and the
	// closure keeps walking through a chain of recording rules.
	chain := []vmalertconfig.Group{
		group("base", record("r1", `raw`)),
		group("mid", record("r2", `r1 * 2`)),
		group("top", alert("A", `r2 > 1`)),
		group("unrelated", record("r3", `other`)),
	}
	f(chain, alertOn("A"), []string{"base", "mid", "top"})

	// Asserting a recorded metric directly pulls in its producer and its inputs.
	f(chain, exprOn(`r2`), []string{"base", "mid"})
	f(chain, exprOn(`r3`), []string{"unrelated"})

	// The same alert name defined in several groups needs all of them, since the
	// assertion cannot tell which group produced it.
	dup := []vmalertconfig.Group{
		group("g1", alert("Same", `up == 0`)),
		group("g2", alert("Same", `down == 1`)),
		group("g3", alert("Other", `x > 0`)),
	}
	f(dup, alertOn("Same"), []string{"g1", "g2"})

	// A selector without a metric name may read any recorded metric and any
	// alert series, but a group with such a selector is needed only when the
	// test reaches it.
	catchall := []vmalertconfig.Group{
		group("g1", alert("A", `up == 0`)),
		group("catchall", record("r", `{job="x"}`)),
		group("rec", record("r2", `raw`)),
	}
	f(catchall, alertOn("A"), []string{"g1"})
	f(catchall, exprOn(`r`), []string{"g1", "catchall", "rec"})

	// A regexp on __name__ is matched against the recorded metric names.
	byRegexp := []vmalertconfig.Group{
		group("foo", record("foo:rate", `raw`)),
		group("bar", record("bar:rate", `raw`)),
		group("agg", record("all:rate", `sum({__name__=~"foo:.*"})`)),
		group("neg", record("neg:rate", `sum({__name__!="foo:rate"})`)),
	}
	f(byRegexp, exprOn(`all:rate`), []string{"foo", "agg"})
	f(byRegexp, exprOn(`neg:rate`), []string{"foo", "bar", "agg", "neg"})
	f(byRegexp, exprOn(`{__name__=~"bar:.*"}`), []string{"bar"})

	// A regexp on __name__ matching the alert series pulls in the alert groups
	// whose names pass the alertname filters.
	alertsByRegexp := []vmalertconfig.Group{
		group("src", alert("Src", `up == 0`)),
		group("other", alert("Other", `x > 0`)),
		group("agg", record("r", `count({__name__=~"ALERTS.*", alertname=~"S.*"})`)),
	}
	f(alertsByRegexp, exprOn(`r`), []string{"src", "agg"})

	// Queries run from alert templates read the storage like the rule
	// expression does.
	templated := []vmalertconfig.Group{
		group("rec", record("dep:x", `raw`)),
		group("tpl", vmalertconfig.Rule{
			Alert:       "Tpl",
			Expr:        `raw > 0`,
			Annotations: map[string]string{"x": `{{ query "dep:x" | first | value }}`},
		}),
		group("unrelated", record("r", `raw`)),
	}
	f(templated, alertOn("Tpl"), []string{"rec", "tpl"})

	// A template query that cannot be analysed may read anything.
	opaque := []vmalertconfig.Group{
		group("rec", record("dep:x", `raw`)),
		group("tpl", vmalertconfig.Rule{
			Alert:  "Tpl",
			Expr:   `raw > 0`,
			Labels: map[string]string{"x": `{{ query (printf "dep:%s" $labels.job) | first | value }}`},
		}),
		group("unrelated", record("r", `raw`)),
	}
	f(opaque, alertOn("Tpl"), []string{"rec", "tpl", "unrelated"})
	f(opaque, exprOn(`r`), []string{"unrelated"})

	// A test assertion without a metric name may read anything recorded.
	f(independent, exprOn(`{job="x"}`), []string{"g1", "g2"})

	// A test assertion which does not parse cannot be narrowed down at all.
	f(independent, exprOn(`not a valid expr(((`), []string{"g1", "g2"})

	// A test reading the alert series directly needs the alert groups.
	f(independent, exprOn(`ALERTS{alertname="A2"}`), []string{"g2"})
	f(independent, exprOn(`count(ALERTS)`), []string{"g1", "g2"})

	// A recording rule may record a metric named ALERTS, so its group is needed
	// together with the alert groups.
	alertsRecorded := []vmalertconfig.Group{
		group("rec", record("ALERTS", `vector(1)`)),
		group("src", alert("Src", `up == 0`)),
	}
	f(alertsRecorded, exprOn(`ALERTS`), []string{"rec", "src"})
	f(alertsRecorded, exprOn(`ALERTS{alertname="Src"}`), []string{"rec", "src"})

	// A group reading ALERTS for a named alert pulls in the group defining it.
	viaAlerts := []vmalertconfig.Group{
		group("src", alert("Src", `up == 0`)),
		group("agg", alert("Agg", `count(ALERTS{alertname="Src"}) > 0`)),
		group("unrelated", alert("Other", `x > 0`)),
	}
	f(viaAlerts, alertOn("Agg"), []string{"src", "agg"})

	// A group reading ALERTS without a name may observe any alert, so every group
	// defining one is needed.
	viaAnyAlert := []vmalertconfig.Group{
		group("src", alert("Src", `up == 0`)),
		group("agg", alert("Agg", `count(ALERTS) > 0`)),
		group("other", alert("Other", `x > 0`)),
		group("rec", record("r", `raw`)),
	}
	f(viaAnyAlert, alertOn("Agg"), []string{"src", "agg", "other"})

	// Reading ALERTS in a group the test does not need must not pull anything in.
	f(viaAnyAlert, exprOn(`r`), []string{"rec"})

	// A group reading ALERTS without a name is often reached only by expanding the
	// dependencies of another group, so it must be expanded as well.
	indirectAnyAlert := []vmalertconfig.Group{
		group("src", alert("Src", `up == 0`)),
		group("mid", record("r_mid", `count(ALERTS)`)),
		group("top", record("r_top", `r_mid * 2`)),
	}
	f(indirectAnyAlert, exprOn(`r_top`), []string{"src", "mid", "top"})

	// ALERTS_FOR_STATE is generated for alerts too, so reading it depends on the
	// group defining the alert.
	viaAlertsForState := []vmalertconfig.Group{
		group("src", alert("Src", `up == 0`)),
		group("reader", record("r_state", `ALERTS_FOR_STATE{alertname="Src"}`)),
		group("unrelated", alert("Other", `x > 0`)),
	}
	f(viaAlertsForState, exprOn(`r_state`), []string{"src", "reader"})

	// ALERTS_FOR_STATE without an alert name may observe any alert.
	viaAnyAlertsForState := []vmalertconfig.Group{
		group("src", alert("Src", `up == 0`)),
		group("reader", record("r_state", `ALERTS_FOR_STATE`)),
		group("other", alert("Other", `x > 0`)),
	}
	f(viaAnyAlertsForState, exprOn(`r_state`), []string{"src", "reader", "other"})

	// A recording rule cycle terminates.
	cycle := []vmalertconfig.Group{
		group("g1", record("a", `b`)),
		group("g2", record("b", `a`)),
		group("g3", alert("A", `a > 0`)),
	}
	f(cycle, alertOn("A"), []string{"g1", "g2", "g3"})
}
