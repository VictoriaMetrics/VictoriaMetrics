package notifier

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

func TestAlert_ExecTemplate(t *testing.T) {
	extLabels := make(map[string]string, 0)
	const (
		extCluster = "prod"
		extDC      = "east"
		extURL     = "https://foo.bar"
	)
	extLabels["cluster"] = extCluster
	extLabels["dc"] = extDC
	_, err := Init(nil, extLabels, extURL)
	checkErr(t, err)

	testCases := []struct {
		name        string
		alert       *Alert
		annotations map[string]string
		expTpl      map[string]string
	}{
		{
			name:        "empty-alert",
			alert:       &Alert{},
			annotations: map[string]string{},
			expTpl:      map[string]string{},
		},
		{
			name: "no-template",
			alert: &Alert{
				Value: 1e4,
				Labels: map[string]string{
					"instance": "localhost",
				},
			},
			annotations: map[string]string{},
			expTpl:      map[string]string{},
		},
		{
			name: "label-template",
			alert: &Alert{
				Value: 1e4,
				Labels: map[string]string{
					"job":      "staging",
					"instance": "localhost",
				},
			},
			annotations: map[string]string{
				"summary":     "Too high connection number for {{$labels.instance}} for job {{$labels.job}}",
				"description": "It is {{ $value }} connections for {{$labels.instance}}",
			},
			expTpl: map[string]string{
				"summary":     "Too high connection number for localhost for job staging",
				"description": "It is 10000 connections for localhost",
			},
		},
		{
			name: "expression-template",
			alert: &Alert{
				Expr: `vm_rows{"label"="bar"}>0`,
			},
			annotations: map[string]string{
				"exprEscapedQuery": "{{ $expr|quotesEscape|queryEscape }}",
				"exprEscapedPath":  "{{ $expr|quotesEscape|pathEscape }}",
			},
			expTpl: map[string]string{
				"exprEscapedQuery": "vm_rows%7B%5C%22label%5C%22%3D%5C%22bar%5C%22%7D%3E0",
				"exprEscapedPath":  "vm_rows%7B%5C%22label%5C%22=%5C%22bar%5C%22%7D%3E0",
			},
		},
		{
			name:  "query",
			alert: &Alert{Expr: `vm_rows{"label"="bar"}>0`},
			annotations: map[string]string{
				"summary": `{{ query "foo" | first | value }}`,
				"desc":    `{{ range query "bar" }}{{ . | label "foo" }} {{ . | value }};{{ end }}`,
			},
			expTpl: map[string]string{
				"summary": "1",
				"desc":    "bar 1;garply 2;",
			},
		},
		{
			name: "external",
			alert: &Alert{
				Value: 1e4,
				Labels: map[string]string{
					"job":      "staging",
					"instance": "localhost",
				},
			},
			annotations: map[string]string{
				"url":         "{{ $externalURL }}",
				"summary":     "Issues with {{$labels.instance}} (dc-{{$externalLabels.dc}}) for job {{$labels.job}}",
				"description": "It is {{ $value }} connections for {{$labels.instance}} (cluster-{{$externalLabels.cluster}})",
			},
			expTpl: map[string]string{
				"url":         extURL,
				"summary":     fmt.Sprintf("Issues with localhost (dc-%s) for job staging", extDC),
				"description": fmt.Sprintf("It is 10000 connections for localhost (cluster-%s)", extCluster),
			},
		},
	}

	qFn := func(q string) ([]datasource.Metric, error) {
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
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tpl, err := tc.alert.ExecTemplate(qFn, tc.alert.Labels, tc.annotations)
			if err != nil {
				t.Fatal(err)
			}
			if len(tpl) != len(tc.expTpl) {
				t.Fatalf("expected %d elements; got %d", len(tc.expTpl), len(tpl))
			}
			for k := range tc.expTpl {
				got, exp := tpl[k], tc.expTpl[k]
				if got != exp {
					t.Fatalf("expected %q=%q; got %q=%q", k, exp, k, got)
				}
			}
		})
	}
}

func TestAlert_toPromLabels(t *testing.T) {
	fn := func(labels map[string]string, exp []prompbmarshal.Label, relabel *promrelabel.ParsedConfigs) {
		t.Helper()
		a := Alert{Labels: labels}
		got := a.toPromLabels(relabel)
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

	pcs, err := promrelabel.ParseRelabelConfigsData([]byte(`
- target_label: "foo"
  replacement: "aaa"
- action: labeldrop
  regex: "env.*"
`), false)
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
