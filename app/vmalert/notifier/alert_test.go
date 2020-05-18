package notifier

import (
	"net/url"
	"testing"
)

func TestAlert_ExecTemplate(t *testing.T) {
	u, _ := url.Parse("https://victoriametrics.com/path")
	InitTemplateFunc(u)
	testCases := []struct {
		name string
		alert       *Alert
		annotations map[string]string
		expTpl      map[string]string
	}{
		{
			name: "empty-alert",
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
				Expr:  "vm_rows>0",
			},
			annotations: map[string]string{
				"exprEscapedQuery": "{{ $expr|queryEscape }}",
				"exprEscapedPath":  "{{ $expr|pathEscape }}",
			},
			expTpl: map[string]string{
				"exprEscapedQuery": "vm_rows%3E0",
				"exprEscapedPath":  "vm_rows%3E0",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tpl, err := tc.alert.ExecTemplate(tc.annotations)
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
