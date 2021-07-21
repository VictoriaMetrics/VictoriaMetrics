package config

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
	"gopkg.in/yaml.v2"
)

func TestMain(m *testing.M) {
	u, _ := url.Parse("https://victoriametrics.com/path")
	notifier.InitTemplateFunc(u)
	os.Exit(m.Run())
}

func TestParseGood(t *testing.T) {
	if _, err := Parse([]string{"testdata/*good.rules", "testdata/dir/*good.*"}, true, true); err != nil {
		t.Errorf("error parsing files %s", err)
	}
}

func TestParseBad(t *testing.T) {
	testCases := []struct {
		path   []string
		expErr string
	}{
		{
			[]string{"testdata/rules0-bad.rules"},
			"unexpected token",
		},
		{
			[]string{"testdata/dir/rules0-bad.rules"},
			"error parsing annotation",
		},
		{
			[]string{"testdata/dir/rules1-bad.rules"},
			"duplicate in file",
		},
		{
			[]string{"testdata/dir/rules2-bad.rules"},
			"function \"unknown\" not defined",
		},
		{
			[]string{"testdata/dir/rules3-bad.rules"},
			"either `record` or `alert` must be set",
		},
		{
			[]string{"testdata/dir/rules4-bad.rules"},
			"either `record` or `alert` must be set",
		},
		{
			[]string{"testdata/rules1-bad.rules"},
			"bad graphite expr",
		},
	}
	for _, tc := range testCases {
		_, err := Parse(tc.path, true, true)
		if err == nil {
			t.Errorf("expected to get error")
			return
		}
		if !strings.Contains(err.Error(), tc.expErr) {
			t.Errorf("expected err to contain %q; got %q instead", tc.expErr, err)
		}
	}
}

func TestRule_Validate(t *testing.T) {
	if err := (&Rule{}).Validate(); err == nil {
		t.Errorf("expected empty name error")
	}
	if err := (&Rule{Alert: "alert"}).Validate(); err == nil {
		t.Errorf("expected empty expr error")
	}
	if err := (&Rule{Alert: "alert", Expr: "test>0"}).Validate(); err != nil {
		t.Errorf("expected valid rule; got %s", err)
	}
}

func TestGroup_Validate(t *testing.T) {
	testCases := []struct {
		group               *Group
		rules               []Rule
		validateAnnotations bool
		validateExpressions bool
		expErr              string
	}{
		{
			group:  &Group{},
			expErr: "group name must be set",
		},
		{
			group:  &Group{Name: "test"},
			expErr: "contain no rules",
		},
		{
			group: &Group{Name: "test",
				Rules: []Rule{
					{
						Record: "record",
						Expr:   "up | 0",
					},
				},
			},
			expErr: "",
		},
		{
			group: &Group{Name: "test",
				Rules: []Rule{
					{
						Record: "record",
						Expr:   "up | 0",
					},
				},
			},
			expErr:              "invalid expression",
			validateExpressions: true,
		},
		{
			group: &Group{Name: "test",
				Rules: []Rule{
					{
						Alert: "alert",
						Expr:  "up == 1",
						Labels: map[string]string{
							"summary": "{{ value|query }}",
						},
					},
				},
			},
			expErr: "",
		},
		{
			group: &Group{Name: "test",
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
			},
			validateAnnotations: true,
		},
		{
			group: &Group{Name: "test",
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
			},
			expErr: "duplicate",
		},
		{
			group: &Group{Name: "test",
				Rules: []Rule{
					{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
						"summary": "{{ value|query }}",
					}},
					{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
						"summary": "{{ value|query }}",
					}},
				},
			},
			expErr: "duplicate",
		},
		{
			group: &Group{Name: "test",
				Rules: []Rule{
					{Record: "record", Expr: "up == 1", Labels: map[string]string{
						"summary": "{{ value|query }}",
					}},
					{Record: "record", Expr: "up == 1", Labels: map[string]string{
						"summary": "{{ value|query }}",
					}},
				},
			},
			expErr: "duplicate",
		},
		{
			group: &Group{Name: "test",
				Rules: []Rule{
					{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
						"summary": "{{ value|query }}",
					}},
					{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
						"description": "{{ value|query }}",
					}},
				},
			},
			expErr: "",
		},
		{
			group: &Group{Name: "test",
				Rules: []Rule{
					{Record: "alert", Expr: "up == 1", Labels: map[string]string{
						"summary": "{{ value|query }}",
					}},
					{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
						"summary": "{{ value|query }}",
					}},
				},
			},
			expErr: "",
		},
		{
			group: &Group{Name: "test thanos",
				Type: datasource.NewRawType("thanos"),
				Rules: []Rule{
					{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
						"description": "{{ value|query }}",
					}},
				},
			},
			validateExpressions: true,
			expErr:              "unknown datasource type",
		},
		{
			group: &Group{Name: "test graphite",
				Type: datasource.NewGraphiteType(),
				Rules: []Rule{
					{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
						"description": "some-description",
					}},
				},
			},
			validateExpressions: true,
			expErr:              "",
		},
		{
			group: &Group{Name: "test prometheus",
				Type: datasource.NewPrometheusType(),
				Rules: []Rule{
					{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
						"description": "{{ value|query }}",
					}},
				},
			},
			validateExpressions: true,
			expErr:              "",
		},
		{
			group: &Group{
				Name: "test graphite inherit",
				Type: datasource.NewGraphiteType(),
				Rules: []Rule{
					{
						Expr: "sumSeries(time('foo.bar',10))",
						For:  utils.NewPromDuration(10 * time.Millisecond),
					},
					{
						Expr: "sum(up == 0 ) by (host)",
						Type: datasource.NewPrometheusType(),
					},
				},
			},
		},
		{
			group: &Group{
				Name: "test graphite prometheus bad expr",
				Type: datasource.NewGraphiteType(),
				Rules: []Rule{
					{
						Expr: "sum(up == 0 ) by (host)",
						For:  utils.NewPromDuration(10 * time.Millisecond),
					},
					{
						Expr: "sumSeries(time('foo.bar',10))",
						Type: datasource.NewPrometheusType(),
					},
				},
			},
			expErr: "invalid rule",
		},
	}
	for _, tc := range testCases {
		err := tc.group.Validate(tc.validateAnnotations, tc.validateExpressions)
		if err == nil {
			if tc.expErr != "" {
				t.Errorf("expected to get err %q; got nil insted", tc.expErr)
			}
			continue
		}
		if !strings.Contains(err.Error(), tc.expErr) {
			t.Errorf("expected err to contain %q; got %q instead", tc.expErr, err)
		}
	}
}

func TestHashRule(t *testing.T) {
	testCases := []struct {
		a, b  Rule
		equal bool
	}{
		{
			Rule{Record: "record", Expr: "up == 1"},
			Rule{Record: "record", Expr: "up == 1"},
			true,
		},
		{
			Rule{Alert: "alert", Expr: "up == 1"},
			Rule{Alert: "alert", Expr: "up == 1"},
			true,
		},
		{
			Rule{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
				"foo": "bar",
				"baz": "foo",
			}},
			Rule{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
				"foo": "bar",
				"baz": "foo",
			}},
			true,
		},
		{
			Rule{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
				"foo": "bar",
				"baz": "foo",
			}},
			Rule{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
				"baz": "foo",
				"foo": "bar",
			}},
			true,
		},
		{
			Rule{Alert: "record", Expr: "up == 1"},
			Rule{Alert: "record", Expr: "up == 1"},
			true,
		},
		{
			Rule{Alert: "alert", Expr: "up == 1", For: utils.NewPromDuration(time.Minute)},
			Rule{Alert: "alert", Expr: "up == 1"},
			true,
		},
		{
			Rule{Alert: "record", Expr: "up == 1"},
			Rule{Record: "record", Expr: "up == 1"},
			false,
		},
		{
			Rule{Record: "record", Expr: "up == 1"},
			Rule{Record: "record", Expr: "up == 2"},
			false,
		},
		{
			Rule{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
				"foo": "bar",
				"baz": "foo",
			}},
			Rule{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
				"baz": "foo",
				"foo": "baz",
			}},
			false,
		},
		{
			Rule{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
				"foo": "bar",
				"baz": "foo",
			}},
			Rule{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
				"baz": "foo",
			}},
			false,
		},
		{
			Rule{Alert: "alert", Expr: "up == 1", Labels: map[string]string{
				"foo": "bar",
				"baz": "foo",
			}},
			Rule{Alert: "alert", Expr: "up == 1"},
			false,
		},
	}
	for i, tc := range testCases {
		aID, bID := HashRule(tc.a), HashRule(tc.b)
		if tc.equal != (aID == bID) {
			t.Fatalf("missmatch for rule %d", i)
		}
	}
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

	t.Run("Ok, `for` must change cs", func(t *testing.T) {
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

}
