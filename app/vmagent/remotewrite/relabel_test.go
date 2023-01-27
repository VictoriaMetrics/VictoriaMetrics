package remotewrite

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestApplyRelabeling(t *testing.T) {
	f := func(extraLabels []prompbmarshal.Label, pcs *promrelabel.ParsedConfigs, sTss, sExpTss string) {
		rctx := &relabelCtx{}
		tss, expTss := parseSeries(sTss), parseSeries(sExpTss)
		gotTss := rctx.applyRelabeling(tss, extraLabels, pcs)
		if !reflect.DeepEqual(gotTss, expTss) {
			t.Fatalf("expected to have: \n%v;\ngot: \n%v", expTss, gotTss)
		}
	}

	f(nil, nil, "up", "up")
	f([]prompbmarshal.Label{{Name: "foo", Value: "bar"}}, nil, "up", `up{foo="bar"}`)
	f([]prompbmarshal.Label{{Name: "foo", Value: "bar"}}, nil, `up{foo="baz"}`, `up{foo="bar"}`)

	pcs, err := promrelabel.ParseRelabelConfigsData([]byte(`
- target_label: "foo"
  replacement: "aaa"
- action: labeldrop
  regex: "env.*"
`))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	f(nil, pcs, `up{foo="baz", env="prod"}`, `up{foo="aaa"}`)

	oldVal := *usePromCompatibleNaming
	*usePromCompatibleNaming = true
	f(nil, nil, `foo.bar`, `foo_bar`)
	*usePromCompatibleNaming = oldVal
}

func parseSeries(data string) []prompbmarshal.TimeSeries {
	var tss []prompbmarshal.TimeSeries
	tss = append(tss, prompbmarshal.TimeSeries{
		Labels: promutils.MustNewLabelsFromString(data).GetLabels(),
	})
	return tss
}
