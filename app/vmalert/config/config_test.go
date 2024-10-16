package config

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/templates"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"gopkg.in/yaml.v2"
)

func TestMain(m *testing.M) {
	if err := templates.Load([]string{"testdata/templates/*good.tmpl"}, true); err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestParseFromURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/bad", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("foo bar"))
	})
	mux.HandleFunc("/good-alert", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`
groups:
  - name: TestGroup
    rules:
      - alert: Conns
        expr: vm_tcplistener_conns > 0`))
	})
	mux.HandleFunc("/good-rr", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`
groups:
  - name: TestGroup
    rules:
      - record: conns
        expr: max(vm_tcplistener_conns)`))
	})
	mux.HandleFunc("/good-multi-doc", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`
groups:
  - name: foo
    rules:
      - record: conns
        expr: max(vm_tcplistener_conns)
---
groups:
  - name: bar
    rules:
      - record: conns
        expr: max(vm_tcplistener_conns)`))
	})
	mux.HandleFunc("/bad-multi-doc", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`
bad_field:
  - name: foo
    rules:
      - record: conns
        expr: max(vm_tcplistener_conns)
---
groups:
  - name: bar
    rules:
      - record: conns
        expr: max(vm_tcplistener_conns)`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := func(urls []string, expErr bool) {
		for i, u := range urls {
			urls[i] = srv.URL + u
		}
		_, err := Parse(urls, notifier.ValidateTemplates, true)
		if err != nil && !expErr {
			t.Fatalf("error parsing URLs %s", err)
		}
		if err == nil && expErr {
			t.Fatalf("expecting error parsing URLs but got none")
		}
	}

	f([]string{"/good-alert", "/good-rr", "/good-multi-doc"}, false)
	f([]string{"/bad"}, true)
	f([]string{"/bad-multi-doc"}, true)
	f([]string{"/good-alert", "/bad"}, true)
}

func TestParse_Success(t *testing.T) {
	_, err := Parse([]string{"testdata/rules/*good.rules", "testdata/dir/*good.*"}, notifier.ValidateTemplates, true)
	if err != nil {
		t.Fatalf("error parsing files %s", err)
	}
}

func TestParse_Failure(t *testing.T) {
	f := func(paths []string, errStrExpected string) {
		t.Helper()

		_, err := Parse(paths, notifier.ValidateTemplates, true)
		if err == nil {
			t.Fatalf("expected to get error")
		}
		if !strings.Contains(err.Error(), errStrExpected) {
			t.Fatalf("expected err to contain %q; got %q instead", errStrExpected, err)
		}
	}

	f([]string{"testdata/rules/rules_interval_bad.rules"}, "eval_offset should be smaller than interval")
	f([]string{"testdata/rules/rules0-bad.rules"}, "unexpected token")
	f([]string{"testdata/dir/rules0-bad.rules"}, "error parsing annotation")
	f([]string{"testdata/dir/rules1-bad.rules"}, "duplicate in file")
	f([]string{"testdata/dir/rules2-bad.rules"}, "function \"unknown\" not defined")
	f([]string{"testdata/dir/rules3-bad.rules"}, "either `record` or `alert` must be set")
	f([]string{"testdata/dir/rules4-bad.rules"}, "either `record` or `alert` must be set")
	f([]string{"testdata/rules/rules1-bad.rules"}, "bad graphite expr")
	f([]string{"testdata/dir/rules6-bad.rules"}, "missing ':' in header")
	f([]string{"testdata/rules/rules-multi-doc-bad.rules"}, "unknown fields")
	f([]string{"testdata/rules/rules-multi-doc-duplicates-bad.rules"}, "duplicate")
	f([]string{"http://unreachable-url"}, "failed to")
}

func TestRuleValidate(t *testing.T) {
	if err := (&Rule{}).Validate(); err == nil {
		t.Fatalf("expected empty name error")
	}
	if err := (&Rule{Alert: "alert"}).Validate(); err == nil {
		t.Fatalf("expected empty expr error")
	}
	if err := (&Rule{Alert: "alert", Expr: "test>0"}).Validate(); err != nil {
		t.Fatalf("expected valid rule; got %s", err)
	}
}

func TestGroupValidate_Failure(t *testing.T) {
	f := func(group *Group, validateExpressions bool, errStrExpected string) {
		t.Helper()

		err := group.Validate(nil, validateExpressions)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		errStr := err.Error()
		if !strings.Contains(errStr, errStrExpected) {
			t.Fatalf("missing %q in the returned error %q", errStrExpected, errStr)
		}
	}

	f(&Group{}, false, "group name must be set")

	f(&Group{
		Name:     "negative interval",
		Interval: promutils.NewDuration(-1),
	}, false, "interval shouldn't be lower than 0")

	f(&Group{
		Name:       "wrong eval_offset",
		Interval:   promutils.NewDuration(time.Minute),
		EvalOffset: promutils.NewDuration(2 * time.Minute),
	}, false, "eval_offset should be smaller than interval")

	f(&Group{
		Name:  "wrong limit",
		Limit: -1,
	}, false, "invalid limit")

	f(&Group{
		Name:        "wrong concurrency",
		Concurrency: -1,
	}, false, "invalid concurrency")

	f(&Group{
		Name: "test",
		Rules: []Rule{
			{
				Alert: "alert",
				Expr:  "up == 1",
			},
			{
				Alert: "alert",
				Expr:  "up == 1",
			},
		},
	}, false, "duplicate")

	f(&Group{
		Name: "test",
		Rules: []Rule{
			{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
				"summary": "{{ value|query }}",
			}},
			{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
				"summary": "{{ value|query }}",
			}},
		},
	}, false, "duplicate")

	f(&Group{
		Name: "test",
		Rules: []Rule{
			{Record: "record", Expr: "up == 1", Labels: map[string]string{
				"summary": "{{ value|query }}",
			}},
			{Record: "record", Expr: "up == 1", Labels: map[string]string{
				"summary": "{{ value|query }}",
			}},
		},
	}, false, "duplicate")

	f(&Group{
		Name: "test",
		Rules: []Rule{
			{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
				"summary": "{{ value|query }}",
			}},
			{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
				"description": "{{ value|query }}",
			}},
		},
	}, false, "duplicate")

	f(&Group{
		Name: "test",
		Rules: []Rule{
			{Record: "alert", Expr: "up == 1", Labels: map[string]string{
				"summary": "{{ value|query }}",
			}},
			{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
				"summary": "{{ value|query }}",
			}},
		},
	}, false, "duplicate")

	f(&Group{
		Name: "test graphite prometheus bad expr",
		Type: NewGraphiteType(),
		Rules: []Rule{
			{
				Expr: "sum(up == 0 ) by (host)",
				For:  promutils.NewDuration(10 * time.Millisecond),
			},
			{
				Expr: "sumSeries(time('foo.bar',10))",
			},
		},
	}, false, "invalid rule")

	f(&Group{
		Name: "test graphite inherit",
		Type: NewGraphiteType(),
		Rules: []Rule{
			{
				Expr: "sumSeries(time('foo.bar',10))",
				For:  promutils.NewDuration(10 * time.Millisecond),
			},
			{
				Expr: "sum(up == 0 ) by (host)",
			},
		},
	}, false, "either `record` or `alert` must be set")

	// validate expressions
	f(&Group{
		Name: "test",
		Rules: []Rule{
			{
				Record: "record",
				Expr:   "up | 0",
			},
		},
	}, true, "invalid expression")

	f(&Group{
		Name: "test thanos",
		Type: NewRawType("thanos"),
		Rules: []Rule{
			{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
				"description": "{{ value|query }}",
			}},
		},
	}, true, "unknown datasource type")

	f(&Group{
		Name: "test graphite",
		Type: NewGraphiteType(),
		Rules: []Rule{
			{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
				"description": "some-description",
			}},
		},
	}, true, "bad graphite expr")
}

func TestGroupValidate_Success(t *testing.T) {
	f := func(group *Group, validateAnnotations, validateExpressions bool) {
		t.Helper()

		var validateTplFn ValidateTplFn
		if validateAnnotations {
			validateTplFn = notifier.ValidateTemplates
		}
		err := group.Validate(validateTplFn, validateExpressions)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}

	f(&Group{
		Name: "test",
		Rules: []Rule{
			{
				Record: "record",
				Expr:   "up | 0",
			},
		},
	}, false, false)

	f(&Group{
		Name: "test",
		Rules: []Rule{
			{
				Alert: "alert",
				Expr:  "up == 1",
				Labels: map[string]string{
					"summary": "{{ value|query }}",
				},
			},
		},
	}, false, false)

	// validate annotiations
	f(&Group{
		Name: "test",
		Rules: []Rule{
			{
				Alert: "alert",
				Expr:  "up == 1",
				Labels: map[string]string{
					"summary": `
{{ with printf "node_memory_MemTotal{job='node',instance='%s'}" "localhost" | query }}
  {{ . | first | value | humanize1024 }}B
{{ end }}`,
				},
			},
		},
	}, true, false)

	// validate expressions
	f(&Group{
		Name: "test prometheus",
		Type: NewPrometheusType(),
		Rules: []Rule{
			{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
				"description": "{{ value|query }}",
			}},
		},
	}, false, true)
}

func TestHashRule_NotEqual(t *testing.T) {
	f := func(a, b Rule) {
		t.Helper()

		aID, bID := HashRule(a), HashRule(b)
		if aID == bID {
			t.Fatalf("rule hashes mustn't be equal; got %d", aID)
		}
	}

	f(Rule{Alert: "record", Expr: "up == 1"}, Rule{Record: "record", Expr: "up == 1"})

	f(Rule{Record: "record", Expr: "up == 1"}, Rule{Record: "record", Expr: "up == 2"})

	f(Rule{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
		"foo": "bar",
		"baz": "foo",
	}}, Rule{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
		"baz": "foo",
		"foo": "baz",
	}})

	f(Rule{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
		"foo": "bar",
		"baz": "foo",
	}}, Rule{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
		"baz": "foo",
	}})

	f(Rule{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
		"foo": "bar",
		"baz": "foo",
	}}, Rule{Alert: "alert", Expr: "up == 1"})
}

func TestHashRule_Equal(t *testing.T) {
	f := func(a, b Rule) {
		t.Helper()

		aID, bID := HashRule(a), HashRule(b)
		if aID != bID {
			t.Fatalf("rule hashes must be equal; got %d and %d", aID, bID)
		}
	}

	f(Rule{Record: "record", Expr: "up == 1"}, Rule{Record: "record", Expr: "up == 1"})

	f(Rule{Alert: "alert", Expr: "up == 1"}, Rule{Alert: "alert", Expr: "up == 1"})

	f(Rule{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
		"foo": "bar",
		"baz": "foo",
	}}, Rule{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
		"foo": "bar",
		"baz": "foo",
	}})

	f(Rule{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
		"foo": "bar",
		"baz": "foo",
	}}, Rule{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
		"baz": "foo",
		"foo": "bar",
	}})

	f(Rule{Alert: "record", Expr: "up == 1"}, Rule{Alert: "record", Expr: "up == 1"})

	f(Rule{
		Alert: "alert", Expr: "up == 1", For: promutils.NewDuration(time.Minute), KeepFiringFor: promutils.NewDuration(time.Minute),
	}, Rule{Alert: "alert", Expr: "up == 1"})
}

func TestGroupChecksum(t *testing.T) {
	f := func(t *testing.T, data, newData string) {
		t.Helper()
		var g Group
		if err := yaml.Unmarshal([]byte(data), &g); err != nil {
			t.Fatalf("failed to unmarshal: %s", err)
		}
		if g.Checksum == "" {
			t.Fatalf("expected to get non-empty checksum")
		}

		var ng Group
		if err := yaml.Unmarshal([]byte(newData), &ng); err != nil {
			t.Fatalf("failed to unmarshal: %s", err)
		}
		if g.Checksum == ng.Checksum {
			t.Fatalf("expected to get different checksums")
		}
	}
	t.Run("Ok", func(t *testing.T) {
		f(t, `
name: TestGroup
rules:
  - alert: ExampleAlertAlwaysFiring
    expr: sum by(job) (up == 1)
  - record: handler:requests:rate5m
    expr: sum(rate(prometheus_http_requests_total[5m])) by (handler)
`, `
name: TestGroup
rules:
  - record: handler:requests:rate5m
    expr: sum(rate(prometheus_http_requests_total[5m])) by (handler)
  - alert: ExampleAlertAlwaysFiring
    expr: sum by(job) (up == 1)
`)
	})

	t.Run("`for` change", func(t *testing.T) {
		f(t, `
name: TestGroup
rules:
  - alert: ExampleAlertWithFor
    expr: sum by(job) (up == 1)
    for: 5m
`, `
name: TestGroup
rules:
  - alert: ExampleAlertWithFor
    expr: sum by(job) (up == 1)
`)
	})
	t.Run("`interval` change", func(t *testing.T) {
		f(t, `
name: TestGroup
interval: 2s
rules:
  - alert: ExampleAlertWithFor
    expr: sum by(job) (up == 1)
`, `
name: TestGroup
interval: 4s
rules:
  - alert: ExampleAlertWithFor
    expr: sum by(job) (up == 1)
`)
	})
	t.Run("`concurrency` change", func(t *testing.T) {
		f(t, `
name: TestGroup
concurrency: 2
rules:
  - alert: ExampleAlertWithFor
    expr: sum by(job) (up == 1)
`, `
name: TestGroup
concurrency: 16
rules:
  - alert: ExampleAlertWithFor
    expr: sum by(job) (up == 1)
`)
	})

	t.Run("`params` change", func(t *testing.T) {
		f(t, `
name: TestGroup
params:
    nocache: ["1"]
rules:
  - alert: foo
    expr: sum by(job) (up == 1)
`, `
name: TestGroup
params:
    nocache: ["0"]
rules:
  - alert: foo
    expr: sum by(job) (up == 1)
`)
	})

	t.Run("`limit` change", func(t *testing.T) {
		f(t, `
name: TestGroup
limit: 5
rules:
  - alert: foo
    expr: sum by(job) (up == 1)
`, `
name: TestGroup
limit: 10
rules:
  - alert: foo
    expr: sum by(job) (up == 1)
`)
	})

	t.Run("`headers` change", func(t *testing.T) {
		f(t, `
name: TestGroup
headers:
  - "TenantID: foo"
rules:
  - alert: foo
    expr: sum by(job) (up == 1)
`, `
name: TestGroup
headers:
  - "TenantID: bar"
rules:
  - alert: foo
    expr: sum by(job) (up == 1)
`)
	})

	t.Run("`notifier_headers` change", func(t *testing.T) {
		f(t, `
name: TestGroup
notifier_headers:
  - "TenantID: foo"
rules:
  - alert: foo
    expr: sum by(job) (up == 1)
`, `
name: TestGroup
notifier_headers:
  - "TenantID: bar"
rules:
  - alert: foo
    expr: sum by(job) (up == 1)
`)
	})

	t.Run("`debug` change", func(t *testing.T) {
		f(t, `
name: TestGroup
rules:
  - alert: foo
    expr: sum by(job) (up == 1)
`, `
name: TestGroup
rules:
  - alert: foo
    expr: sum by(job) (up == 1)
    debug: true
`)
	})
	t.Run("`update_entries_limit` change", func(t *testing.T) {
		f(t, `
name: TestGroup
rules:
  - alert: foo
    expr: sum by(job) (up == 1)
`, `
name: TestGroup
rules:
  - alert: foo
    expr: sum by(job) (up == 1)
    update_entries_limit: 33
`)
	})
}

func TestGroupParams(t *testing.T) {
	f := func(t *testing.T, data string, expParams url.Values) {
		t.Helper()
		var g Group
		if err := yaml.Unmarshal([]byte(data), &g); err != nil {
			t.Fatalf("failed to unmarshal: %s", err)
		}
		got, exp := g.Params.Encode(), expParams.Encode()
		if got != exp {
			t.Fatalf("expected to have %q; got %q", exp, got)
		}
	}

	t.Run("no params", func(t *testing.T) {
		f(t, `
name: TestGroup
rules:
  - alert: ExampleAlertAlwaysFiring
    expr: sum by(job) (up == 1)
`, url.Values{})
	})

	t.Run("params", func(t *testing.T) {
		f(t, `
name: TestGroup
params:
  nocache: ["1"]
  denyPartialResponse: ["true"]
rules:
  - alert: ExampleAlertAlwaysFiring
    expr: sum by(job) (up == 1)
`, url.Values{"nocache": {"1"}, "denyPartialResponse": {"true"}})
	})
}
