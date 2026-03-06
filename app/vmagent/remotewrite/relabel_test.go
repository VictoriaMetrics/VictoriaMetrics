package remotewrite

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
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

func sha256hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func TestApplyObfuscation(t *testing.T) {
	f := func(labelNames []string, sTss, sExpTss string) {
		t.Helper()
		tss := parseSeries(sTss)
		expTss := parseSeries(sExpTss)
		applyObfuscation(tss, labelNames)
		if !reflect.DeepEqual(tss, expTss) {
			t.Fatalf("expected:\n%v\ngot:\n%v", expTss, tss)
		}
	}

	f(nil, `up{instance="localhost:8428"}`, `up{instance="localhost:8428"}`)
	f([]string{"instance"}, `up{instance="localhost:8428"}`,
		`up{instance="`+sha256hex("localhost:8428")+`"}`)
	f([]string{"instance", "ip"}, `up{instance="localhost",ip="10.0.0.1",env="prod"}`,
		`up{instance="`+sha256hex("localhost")+`",ip="`+sha256hex("10.0.0.1")+`",env="prod"}`)
	f([]string{"unknown"}, `up{instance="localhost:8428"}`, `up{instance="localhost:8428"}`)

	// multiple time series
	tss := []prompb.TimeSeries{
		{Labels: []prompb.Label{{Name: "__name__", Value: "up"}, {Name: "instance", Value: "host1"}}},
		{Labels: []prompb.Label{{Name: "__name__", Value: "up"}, {Name: "instance", Value: "host2"}}},
	}
	expTss := []prompb.TimeSeries{
		{Labels: []prompb.Label{{Name: "__name__", Value: "up"}, {Name: "instance", Value: sha256hex("host1")}}},
		{Labels: []prompb.Label{{Name: "__name__", Value: "up"}, {Name: "instance", Value: sha256hex("host2")}}},
	}
	applyObfuscation(tss, []string{"instance"})
	if !reflect.DeepEqual(tss, expTss) {
		t.Fatalf("expected:\n%v\ngot:\n%v", expTss, tss)
	}
}

func TestApplyObfuscationIsolation(t *testing.T) {
	original := []prompb.TimeSeries{
		{Labels: []prompb.Label{{Name: "__name__", Value: "up"}, {Name: "instance", Value: "host1"}}},
	}
	shallow := append([]prompb.TimeSeries{}, original...)
	applyObfuscation(shallow, []string{"instance"})

	if original[0].Labels[1].Value != "host1" {
		t.Fatalf("original was modified: got %q", original[0].Labels[1].Value)
	}
	if shallow[0].Labels[1].Value != sha256hex("host1") {
		t.Fatalf("shallow copy was not obfuscated: got %q", shallow[0].Labels[1].Value)
	}
}

func parseSeries(data string) []prompb.TimeSeries {
	var tss []prompb.TimeSeries
	tss = append(tss, prompb.TimeSeries{
		Labels: promutil.MustNewLabelsFromString(data).GetLabels(),
	})
	return tss
}
