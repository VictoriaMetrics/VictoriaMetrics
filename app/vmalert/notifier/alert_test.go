package notifier

import (
	"fmt"
	"testing"
)

func TestAlert_ExecTemplate(t *testing.T) {
	testCases := []struct {
		alert       *Alert
		annotations map[string]string
		expTpl      map[string]string
	}{
		{
			alert:       &Alert{},
			annotations: map[string]string{},
			expTpl:      map[string]string{},
		},
		{
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
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
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
