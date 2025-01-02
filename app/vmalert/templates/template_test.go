package templates

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestValidateTemplates(t *testing.T) {
	f := func(annotations map[string]string, isValid bool) {
		t.Helper()

		err := ValidateTemplates(annotations)
		if (err == nil) != isValid {
			t.Fatalf("failed to validate template, got %t; want %t", (err == nil), isValid)
		}
	}

	// empty
	f(map[string]string{}, true)

	// wrong text
	f(map[string]string{
		"summary": "{{",
	}, false)

	// valid
	f(map[string]string{
		"value":   "{{$value}}",
		"summary": "it's a test summary",
	}, true)

	// invalid variable
	f(map[string]string{
		"value":   "{{$invalidValue}}",
		"summary": "it's a test summary",
	}, false)
}

func TestExecuteWithoutTemplate(t *testing.T) {
	extLabels := make(map[string]string)
	const (
		extCluster = "prod"
		extDC      = "east"
		extURL     = "https://foo.bar"
	)
	url, _ := url.Parse(extURL)
	extLabels["cluster"] = extCluster
	extLabels["dc"] = extDC

	err := Init(nil, extLabels, *url)
	if err != nil {
		t.Fatalf("cannot init templates: %s", err)
	}

	f := func(data AlertTplData, annotations, expResults map[string]string) {
		t.Helper()

		qFn := func(_ string) ([]datasource.Metric, error) {
			return []datasource.Metric{
				{
					Labels: []prompbmarshal.Label{
						{Name: "foo", Value: "bar"},
						{Name: "baz", Value: "qux"},
					},
					Values:     []float64{1},
					Timestamps: []int64{1},
				},
				{
					Labels: []prompbmarshal.Label{
						{Name: "foo", Value: "garply"},
						{Name: "baz", Value: "fred"},
					},
					Values:     []float64{2},
					Timestamps: []int64{1},
				},
			}, nil
		}

		for k := range annotations {
			v, err := ExecuteWithoutTemplate(qFn, annotations[k], data)
			if err != nil {
				t.Fatalf("cannot execute template: %s", err)
			}
			if v != expResults[k] {
				t.Fatalf("unexpected result; got %s; want %s", v, expResults[k])
			}
		}

	}

	// empty-alert
	f(AlertTplData{}, map[string]string{}, map[string]string{})

	// no-template
	f(AlertTplData{
		Value: 1e4,
		Labels: map[string]string{
			"instance": "localhost",
		},
	}, map[string]string{
		"summary":     "it's a test summary",
		"description": "it's a test description",
	}, map[string]string{
		"summary":     "it's a test summary",
		"description": "it's a test description",
	})

	// label-template
	f(AlertTplData{
		Value: 1e4,
		Labels: map[string]string{
			"job":      "staging",
			"instance": "localhost",
		},
		For: 5 * time.Minute,
	}, map[string]string{
		"summary":            "Too high connection number for {{$labels.instance}} for job {{$labels.job}}",
		"description":        "It is {{ $value }} connections for {{$labels.instance}} for more than {{ .For }}",
		"non-existing-label": "{{$labels.nonexisting}}",
	}, map[string]string{
		"summary":            "Too high connection number for localhost for job staging",
		"description":        "It is 10000 connections for localhost for more than 5m0s",
		"non-existing-label": "",
	})

	// label template override
	f(AlertTplData{
		Value: 1e4,
	}, map[string]string{
		"summary":     `{{- define "default.template" -}} {{ printf "summary" }} {{- end -}} {{ template "default.template" . }}`,
		"description": `{{- define "default.template" -}} {{ printf "description" }} {{- end -}} {{ template "default.template" . }}`,
		"value":       `{{$value }}`,
	}, map[string]string{
		"summary":     "summary",
		"description": "description",
		"value":       "10000",
	})

	// expression-template
	f(AlertTplData{
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

	// query function
	f(AlertTplData{
		Expr: `vm_rows{"label"="bar"}>0`,
	}, map[string]string{
		"summary": `{{ query "foo" | first | value }}`,
		"desc":    `{{ range query "bar" }}{{ . | label "foo" }} {{ . | value }};{{ end }}`,
	}, map[string]string{
		"summary": "1",
		"desc":    "bar 1;garply 2;",
	})

	// external
	f(AlertTplData{
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

	// alert, group IDs & ActiveAt time
	f(AlertTplData{
		AlertID:  42,
		GroupID:  24,
		ActiveAt: time.Date(2022, 8, 19, 20, 34, 58, 651387237, time.UTC),
	}, map[string]string{
		"url":     "/api/v1/alert?alertID={{$alertID}}&groupID={{$groupID}}",
		"diagram": "![](http://example.com?render={{$activeAt.Unix}}",
	}, map[string]string{
		"url":     "/api/v1/alert?alertID=42&groupID=24",
		"diagram": "![](http://example.com?render=1660941298",
	})

	// ActiveAt time is nil
	f(AlertTplData{}, map[string]string{
		"default_time": "{{$activeAt}}",
	}, map[string]string{
		"default_time": "0001-01-01 00:00:00 +0000 UTC",
	})

	// ActiveAt custom format
	f(AlertTplData{
		ActiveAt: time.Date(2022, 8, 19, 20, 34, 58, 651387237, time.UTC),
	}, map[string]string{
		"fire_time": `{{$activeAt.Format "2006/01/02 15:04:05"}}`,
	}, map[string]string{
		"fire_time": "2022/08/19 20:34:58",
	})

	// ActiveAt query range
	f(AlertTplData{
		ActiveAt: time.Date(2022, 8, 19, 20, 34, 58, 651387237, time.UTC),
	}, map[string]string{
		"grafana_url": `vm-grafana.com?from={{($activeAt.Add (parseDurationTime "1h")).Unix}}&to={{($activeAt.Add (parseDurationTime "-1h")).Unix}}`,
	}, map[string]string{
		"grafana_url": "vm-grafana.com?from=1660944898&to=1660937698",
	})
}
