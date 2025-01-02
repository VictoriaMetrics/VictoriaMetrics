package notifier

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

func TestAlert_toPromLabels(t *testing.T) {
	fn := func(labels map[string]string, exp []prompbmarshal.Label, relabel *promrelabel.ParsedConfigs) {
		t.Helper()
		a := Alert{Labels: labels}
		got := a.applyRelabelingIfNeeded(relabel)
		if !reflect.DeepEqual(got, exp) {
			t.Fatalf("expected to have: \n%v;\ngot:\n%v",
				exp, got)
		}
	}

	fn(nil, nil, nil)
	fn(
		map[string]string{"foo": "bar", "a": "baz"}, // unsorted
		[]prompbmarshal.Label{{Name: "a", Value: "baz"}, {Name: "foo", Value: "bar"}},
		nil,
	)
	fn(
		map[string]string{"foo.bar": "baz", "service!name": "qux"},
		[]prompbmarshal.Label{{Name: "foo_bar", Value: "baz"}, {Name: "service_name", Value: "qux"}},
		nil,
	)

	pcs, err := promrelabel.ParseRelabelConfigsData([]byte(`
- target_label: "foo"
  replacement: "aaa"
- action: labeldrop
  regex: "env.*"
`))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	fn(
		map[string]string{"a": "baz"},
		[]prompbmarshal.Label{{Name: "a", Value: "baz"}, {Name: "foo", Value: "aaa"}},
		pcs,
	)
	fn(
		map[string]string{"foo": "bar", "a": "baz"},
		[]prompbmarshal.Label{{Name: "a", Value: "baz"}, {Name: "foo", Value: "aaa"}},
		pcs,
	)
	fn(
		map[string]string{"qux": "bar", "env": "prod", "environment": "production"},
		[]prompbmarshal.Label{{Name: "foo", Value: "aaa"}, {Name: "qux", Value: "bar"}},
		pcs,
	)
}
