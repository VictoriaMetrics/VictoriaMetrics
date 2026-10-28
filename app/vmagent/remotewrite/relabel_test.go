package remotewrite

import (
	"reflect"
	"sort"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func TestApplyRelabeling(t *testing.T) {
	f := func(pcs *promrelabel.ParsedConfigs, sTss, sExpTss string) {
		rctx := &relabelCtx{}
		tss, expTss := parseSeries(sTss), parseSeries(sExpTss)
		gotTss := rctx.applyRelabeling(tss, pcs)
		if !reflect.DeepEqual(gotTss, expTss) {
			t.Fatalf("expected to have: \n%v;\ngot: \n%v", expTss, gotTss)
		}
	}

	f(nil, "up", "up")

	pcs, err := promrelabel.ParseRelabelConfigsData([]byte(`
- target_label: "foo"
  replacement: "aaa"
- action: labeldrop
  regex: "env.*"
`))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	f(pcs, `up{foo="baz", env="prod"}`, `up{foo="aaa"}`)

	oldVal := *usePromCompatibleNaming
	*usePromCompatibleNaming = true
	f(nil, `foo.bar`, `foo_bar`)
	*usePromCompatibleNaming = oldVal
}

func TestAppendExtraLabels(t *testing.T) {
	f := func(extraLabels []prompb.Label, sTss, sExpTss string) {
		t.Helper()
		rctx := &relabelCtx{}
		tss, expTss := parseSeries(sTss), parseSeries(sExpTss)
		rctx.appendExtraLabels(tss, extraLabels)
		if !reflect.DeepEqual(tss, expTss) {
			t.Fatalf("expected to have: \n%v;\ngot: \n%v", expTss, tss)
		}
	}

	f(nil, "up", "up")
	f([]prompb.Label{{Name: "foo", Value: "bar"}}, "up", `up{foo="bar"}`)
	f([]prompb.Label{{Name: "foo", Value: "bar"}}, `up{foo="baz"}`, `up{foo="bar"}`)
	f([]prompb.Label{{Name: "baz", Value: "qux"}}, `up{foo="baz"}`, `up{foo="baz",baz="qux"}`)

	oldVal := *usePromCompatibleNaming
	*usePromCompatibleNaming = true
	f([]prompb.Label{{Name: "foo.bar", Value: "baz"}}, "up", `up{foo.bar="baz"}`)
	*usePromCompatibleNaming = oldVal
}

func TestAppendExtraLabelsSorted(t *testing.T) {
	oldSortLabels := *sortLabels
	*sortLabels = true
	t.Cleanup(func() {
		*sortLabels = oldSortLabels
	})

	rctx := &relabelCtx{}
	tss := parseSeries(`up{z="last",m="old"}`)
	tss = append(tss, parseSeries(`down{y="old",b="old"}`)...)
	sortLabelsIfNeeded(tss)

	// Mimic per-URL relabeling, which runs after sortLabelsIfNeeded() and can
	// append labels out of order.
	pcs, err := promrelabel.ParseRelabelConfigsData([]byte(`
- target_label: a
  replacement: relabeled
`))
	if err != nil {
		t.Fatalf("unexpected error when parsing relabel config: %s", err)
	}
	perURLRelabelCtx := &relabelCtx{}
	tss = perURLRelabelCtx.applyRelabeling(tss, pcs)
	for _, ts := range tss {
		if sort.SliceIsSorted(ts.Labels, func(i, j int) bool {
			return ts.Labels[i].Name < ts.Labels[j].Name
		}) {
			t.Fatalf("unexpected sorted labels after per-URL relabeling: %v", ts.Labels)
		}
	}

	rctx.appendExtraLabels(tss, []prompb.Label{
		{Name: "z", Value: "new-last"},
		{Name: "a", Value: "first"},
		{Name: "n", Value: "middle"},
		{Name: "m", Value: "new-middle"},
		{Name: "m", Value: "final-middle"},
	})

	expTss := parseSeries(`up{a="first",m="final-middle",n="middle",z="new-last"}`)
	expTss = append(expTss, parseSeries(`down{a="first",b="old",m="final-middle",n="middle",y="old",z="new-last"}`)...)
	if !reflect.DeepEqual(tss, expTss) {
		t.Fatalf("expected to have: \n%v;\ngot: \n%v", expTss, tss)
	}
}

func parseSeries(data string) []prompb.TimeSeries {
	var tss []prompb.TimeSeries
	tss = append(tss, prompb.TimeSeries{
		Labels: promutil.MustNewLabelsFromString(data).GetLabels(),
	})
	return tss
}
