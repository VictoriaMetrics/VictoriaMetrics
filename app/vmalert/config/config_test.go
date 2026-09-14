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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"gopkg.in/yaml.v2"
)

func TestMain(m *testing.M) {
	if err := templates.Load([]string{"testdata/templates/*good.tmpl"}, url.URL{}); err != nil {
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
	f([]string{"testdata/dir/rules0-bad.rules"}, "invalid annotations")
	f([]string{"testdata/dir/rules1-bad.rules"}, "duplicate in file")
	f([]string{"testdata/dir/rules2-bad.rules"}, "function \"unknown\" not defined")
	f([]string{"testdata/dir/rules3-bad.rules"}, "either `record` or `alert` must be set")
	f([]string{"testdata/dir/rules4-bad.rules"}, "either `record` or `alert` must be set")
	f([]string{"testdata/rules/rules1-bad.rules"}, "bad GraphiteQL expr")
	f([]string{"testdata/rules/vlog-rules0-bad.rules"}, "bad LogsQL expr")
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
	if err := (&Rule{Record: "record", Expr: "sum(test)", Labels: map[string]string{"__name__": "test"}}).Validate(); err == nil {
		t.Fatalf("invalid rule label; got %s", err)
	}
	if err := (&Rule{Alert: "alert", Expr: "test>0"}).Validate(); err != nil {
		t.Fatalf("expected valid rule; got %s", err)
	}
}

func TestGroupValidate_Failure(t *testing.T) {
	f := func(data []byte, validateExpressions bool, errStrExpected string) {
		t.Helper()

		_, err := parse(map[string][]byte{"test.yaml": data}, nil, validateExpressions)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		errStr := err.Error()
		if !strings.Contains(errStr, errStrExpected) {
			t.Fatalf("missing %q in the returned error %q", errStrExpected, errStr)
		}
	}

	f([]byte(`
groups:
- name: ""
`), false, "group name must be set")

	f([]byte(`
groups:
- name: both record and alert are not set
  rules:
  - expr: "sum(up == 0 ) by (host)"
    for: 10ms
  - expr: "sumSeries(time('foo.bar',10))"
`), false, "invalid rule")

	f([]byte(`
groups:
- name: negative interval
  interval: -1ms
`), false, "interval shouldn't be lower than 0")

	f([]byte(`
groups:
- name: too big eval_offset
  interval: 1m
  eval_offset: 2m
`), false, "eval_offset should be smaller than interval")

	f([]byte(`
groups:
- name: too big negative eval_offset
  interval: 1m
  eval_offset: -2m
`), false, "eval_offset should be smaller than interval")

	f([]byte(`
groups:
- name: wrong limit
  limit: -1
`), false, "invalid limit")

	f([]byte(`
groups:
- name: wrong concurrency
  concurrency: -1
`), false, "invalid concurrency")

	f([]byte(`
groups:
- name: test
  rules:
  - alert: alert
    expr: up == 1
  - alert: alert
    expr: up == 1
`), false, "duplicate")

	f([]byte(`
groups:
- name: test
  rules:
  - alert: alert
    expr: up == 1
    labels:
      summary: "{{ value|query }}"
  - alert: alert
    expr: up == 1
    labels:
      summary: "{{ value|query }}"
`), false, "duplicate")

	f([]byte(`
groups:
- name: test
  rules:
  - record: record
    expr: up == 1
    labels:
      summary: "{{ value|query }}"
  - record: record
    expr: up == 1
    labels:
      summary: "{{ value|query }}"
`), false, "duplicate")

	f([]byte(`
groups:
- name: test thanos
  type: thanos
  rules:
  - alert: alert
    expr: up == 1
    labels:
      description: "{{ value|query }}"
`), true, "unknown datasource type")

	// validate expressions
	f([]byte(`
groups:
- name: test prometheus expr
  type: prometheus
  rules:
  - record: record
    expr: "up | 0"
`), true, "bad MetricsQL expr")

	f([]byte(`
groups:
- name: test graphite expr
  type: graphite
  rules:
  - alert: alert
    expr: up == 1
    labels:
      description: some-description
`), true, "bad GraphiteQL expr")

	f([]byte(`
groups:
- name: test vlogs expr
  type: vlogs
  rules:
  - alert: alert
    expr: "stats count(*) as requests"
`), true, "bad LogsQL expr")

	f([]byte(`
groups:
- name: test vlogs expr multipart
  type: vlogs
  rules:
  - alert: alert
    expr: "_time: 1m | stats by (path, _time: 1m) count(*) as requests"
`), true, "bad LogsQL expr")

	f([]byte(`
groups:
- name: test graphite with prometheus expr
  type: graphite
  rules:
  - record: r1
    expr: "sumSeries(time('foo.bar',10))"
    for: 10ms
  - record: r2
    expr: "sum(up == 0 ) by (host)"
`), true, "bad GraphiteQL expr")

	f([]byte(`
groups:
- name: test vlogs with prometheus expr
  type: vlogs
  rules:
  - record: r1
    expr: "sum(up == 0 ) by (host)"
    for: 10ms
`), true, "bad LogsQL expr")

	f([]byte(`
groups:
- name: test prometheus with vlogs expr
  type: prometheus
  rules:
  - record: r1
    expr: "* | stats by (path) count()"
    for: 10ms
`), true, "bad MetricsQL expr")
}

func TestGroupValidate_Success(t *testing.T) {
	f := func(data []byte, validateAnnotations, validateExpressions bool) {
		t.Helper()

		var validateTplFn ValidateTplFn
		if validateAnnotations {
			validateTplFn = notifier.ValidateTemplates
		}
		_, err := parse(map[string][]byte{"test.yaml": data}, validateTplFn, validateExpressions)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}

	f([]byte(`
groups:
- name: test
  rules:
  - record: record
    expr: "up | 0"
`), false, false)

	f([]byte(`
groups:
- name: test
  rules:
  - alert: alert
    expr: up == 1
    labels:
      summary: "{{ value|query }}"
`), false, false)

	// validate annotations
	f([]byte(`
groups:
- name: test
  rules:
  - alert: alert
    expr: up == 1
    labels:
      summary: "\n{{ with printf \"node_memory_MemTotal{job='node',instance='%s'}\" \"localhost\" | query }}\n  {{ . | first | value | humanize1024 }}B\n{{ end }}"
`), true, false)

	// validate expressions
	f([]byte(`
groups:
- name: test prometheus
  type: prometheus
  rules:
  - alert: alert
    expr: up == 1
    labels:
      description: "{{ value|query }}"
`), false, true)

	f([]byte(`
groups:
- name: test victorialogs
  type: vlogs
  rules:
  - alert: alert
    expr: " _time: 1m | stats count(*) as requests"
    labels:
      description: "{{ value|query }}"
`), false, true)
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
		Alert: "alert", Expr: "up == 1", For: promutil.NewDuration(time.Minute), KeepFiringFor: promutil.NewDuration(time.Minute),
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
