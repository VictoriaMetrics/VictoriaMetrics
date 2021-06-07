package notifier

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
)

func TestAlert_ExecTemplate(t *testing.T) {
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
			tpl, err := tc.alert.ExecTemplate(qFn, tc.annotations)
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
