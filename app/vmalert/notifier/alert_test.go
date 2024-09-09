package notifier

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

func TestAlertExecTemplate(t *testing.T) {
	extLabels := make(map[string]string)
	const (
		extCluster = "prod"
		extDC      = "east"
		extURL     = "https://foo.bar"
	)
	extLabels["cluster"] = extCluster
	extLabels["dc"] = extDC
	_, err := Init(nil, extLabels, extURL)
	checkErr(t, err)

	f := func(alert *Alert, annotations map[string]string, tplExpected map[string]string) {
		t.Helper()

		if err := ValidateTemplates(annotations); err != nil {
			t.Fatalf("cannot validate annotations: %s", err)
		}

		qFn := func(_ string) ([]datasource.Metric, error) {
			return []datasource.Metric{
				{
					Labels: []datasource.Label{
						{Name: "foo", Value: "bar"},
						{Name: "baz", Value: "qux"},
					},
					Values:     []float64{1},
					Timestamps: []int64{1},
				},
				{
					Labels: []datasource.Label{
						{Name: "foo", Value: "garply"},
						{Name: "baz", Value: "fred"},
					},
					Values:     []float64{2},
					Timestamps: []int64{1},
				},
			}, nil
		}

		tpl, err := alert.ExecTemplate(qFn, alert.Labels, annotations)
		if err != nil {
			t.Fatalf("cannot execute template: %s", err)
		}
		if len(tpl) != len(tplExpected) {
			t.Fatalf("unexpected number of elements; got %d; want %d", len(tpl), len(tplExpected))
		}
		for k := range tplExpected {
			got, exp := tpl[k], tplExpected[k]
			if got != exp {
				t.Fatalf("unexpected template for key=%q; got %q; want %q", k, got, exp)
			}
		}
	}

	// empty-alert
	f(&Alert{}, map[string]string{}, map[string]string{})

	// no-template
	f(&Alert{
		Value: 1e4,
		Labels: map[string]string{
			"instance": "localhost",
		},
	}, map[string]string{}, map[string]string{})

	// label-template
	f(&Alert{
		Value: 1e4,
		Labels: map[string]string{
			"job":      "staging",
			"instance": "localhost",
		},
		For: 5 * time.Minute,
	}, map[string]string{
		"summary":     "Too high connection number for {{$labels.instance}} for job {{$labels.job}}",
		"description": "It is {{ $value }} connections for {{$labels.instance}} for more than {{ .For }}",
	}, map[string]string{
		"summary":     "Too high connection number for localhost for job staging",
		"description": "It is 10000 connections for localhost for more than 5m0s",
	})

	// expression-template
	f(&Alert{
		Expr: `vm_rows{"label"="bar"}<0`,
	}, map[string]string{
		"exprEscapedQuery":  "{{ $expr|queryEscape }}",
		"exprEscapedPath":   "{{ $expr|pathEscape }}",
		"exprEscapedJSON":   "{{ $expr|jsonEscape }}",
		"exprEscapedQuotes": "{{ $expr|quotesEscape }}",
		"exprEscapedHTML":   "{{ $expr|htmlEscape }}",
	}, map[string]string{
		"exprEscapedQuery":  "vm_rows%7B%22label%22%3D%22bar%22%7D%3C0",
		"exprEscapedPath":   "vm_rows%7B%22label%22=%22bar%22%7D%3C0",
		"exprEscapedJSON":   `"vm_rows{\"label\"=\"bar\"}\u003c0"`,
		"exprEscapedQuotes": `vm_rows{\"label\"=\"bar\"}\u003c0`,
		"exprEscapedHTML":   "vm_rows{&quot;label&quot;=&quot;bar&quot;}&lt;0",
	})

	// query
	f(&Alert{
		Expr: `vm_rows{"label"="bar"}>0`,
	}, map[string]string{
		"summary": `{{ query "foo" | first | value }}`,
		"desc":    `{{ range query "bar" }}{{ . | label "foo" }} {{ . | value }};{{ end }}`,
	}, map[string]string{
		"summary": "1",
		"desc":    "bar 1;garply 2;",
	})

	// external
	f(&Alert{
		Value: 1e4,
		Labels: map[string]string{
			"job":      "staging",
			"instance": "localhost",
		},
	}, map[string]string{
		"url":         "{{ $externalURL }}",
		"summary":     "Issues with {{$labels.instance}} (dc-{{$externalLabels.dc}}) for job {{$labels.job}}",
		"description": "It is {{ $value }} connections for {{$labels.instance}} (cluster-{{$externalLabels.cluster}})",
	}, map[string]string{
		"url":         extURL,
		"summary":     fmt.Sprintf("Issues with localhost (dc-%s) for job staging", extDC),
		"description": fmt.Sprintf("It is 10000 connections for localhost (cluster-%s)", extCluster),
	})

	// alert and group IDs
	f(&Alert{
		ID:      42,
		GroupID: 24,
	}, map[string]string{
		"url": "/api/v1/alert?alertID={{$alertID}}&groupID={{$groupID}}",
	}, map[string]string{
		"url": "/api/v1/alert?alertID=42&groupID=24",
	})

	// ActiveAt time
	f(&Alert{
		ActiveAt: time.Date(2022, 8, 19, 20, 34, 58, 651387237, time.UTC),
	}, map[string]string{
		"diagram": "![](http://example.com?render={{$activeAt.Unix}}",
	}, map[string]string{
		"diagram": "![](http://example.com?render=1660941298",
	})

	// ActiveAt time is nil
	f(&Alert{}, map[string]string{
		"default_time": "{{$activeAt}}",
	}, map[string]string{
		"default_time": "0001-01-01 00:00:00 +0000 UTC",
	})

	// ActiveAt custom format
	f(&Alert{
		ActiveAt: time.Date(2022, 8, 19, 20, 34, 58, 651387237, time.UTC),
	}, map[string]string{
		"fire_time": `{{$activeAt.Format "2006/01/02 15:04:05"}}`,
	}, map[string]string{
		"fire_time": "2022/08/19 20:34:58",
	})

	// ActiveAt query range
	f(&Alert{
		ActiveAt: time.Date(2022, 8, 19, 20, 34, 58, 651387237, time.UTC),
	}, map[string]string{
		"grafana_url": `vm-grafana.com?from={{($activeAt.Add (parseDurationTime "1h")).Unix}}&to={{($activeAt.Add (parseDurationTime "-1h")).Unix}}`,
	}, map[string]string{
		"grafana_url": "vm-grafana.com?from=1660944898&to=1660937698",
	})
}

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
