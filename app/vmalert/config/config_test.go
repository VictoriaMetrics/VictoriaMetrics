package config

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
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
			"function \"value\" not defined",
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
			[]string{"testdata/*.yaml"},
			"no groups found",
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
		t.Errorf("exptected empty name error")
	}
	if err := (&Rule{Alert: "alert"}).Validate(); err == nil {
		t.Errorf("exptected empty expr error")
	}
	if err := (&Rule{Alert: "alert", Expr: "test>0"}).Validate(); err != nil {
		t.Errorf("exptected valid rule; got %s", err)
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
							"summary": "{{ value|query }}",
						},
					},
				},
			},
			expErr:              "error parsing annotation",
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
			Rule{Alert: "alert", Expr: "up == 1", For: time.Minute},
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
