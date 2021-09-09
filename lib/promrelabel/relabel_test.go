package promrelabel

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestLabelsToString(t *testing.T) {
	f := func(labels []prompbmarshal.Label, sExpected string) {
		t.Helper()
		s := labelsToString(labels)
		if s != sExpected {
			t.Fatalf("unexpected result;\ngot\n%s\nwant\n%s", s, sExpected)
		}
	}
	f(nil, "{}")
	f([]prompbmarshal.Label{
		{
			Name:  "__name__",
			Value: "foo",
		},
	}, "foo")
	f([]prompbmarshal.Label{
		{
			Name:  "foo",
			Value: "bar",
		},
	}, `{foo="bar"}`)
	f([]prompbmarshal.Label{
		{
			Name:  "foo",
			Value: "bar",
		},
		{
			Name:  "a",
			Value: "bc",
		},
	}, `{a="bc",foo="bar"}`)
	f([]prompbmarshal.Label{
		{
			Name:  "foo",
			Value: "bar",
		},
		{
			Name:  "__name__",
			Value: "xxx",
		},
		{
			Name:  "a",
			Value: "bc",
		},
	}, `xxx{a="bc",foo="bar"}`)
}

func TestApplyRelabelConfigs(t *testing.T) {
	f := func(config string, labels []prompbmarshal.Label, isFinalize bool, resultExpected []prompbmarshal.Label) {
		t.Helper()
		pcs, err := ParseRelabelConfigsData([]byte(config), false)
		if err != nil {
			t.Fatalf("cannot parse %q: %s", config, err)
		}
		result := pcs.Apply(labels, 0, isFinalize)
		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected result; got\n%v\nwant\n%v", result, resultExpected)
		}
	}
	t.Run("empty_relabel_configs", func(t *testing.T) {
		f("", nil, false, nil)
		f("", nil, true, nil)
		f("", []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "bar",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "bar",
			},
		})
		f("", []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "bar",
			},
			{
				Name:  "__name__",
				Value: "xxx",
			},
			{
				Name:  "__aaa",
				Value: "yyy",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "xxx",
			},
			{
				Name:  "foo",
				Value: "bar",
			},
		})
	})
	t.Run("replace-miss", func(t *testing.T) {
		f(`
- action: replace
  target_label: bar
`, nil, false, []prompbmarshal.Label{})
		f(`
- action: replace
  source_labels: ["foo"]
  target_label: bar
`, nil, false, []prompbmarshal.Label{})
		f(`
- action: replace
  source_labels: ["foo"]
  target_label: "bar"
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		})
		f(`
- action: replace
  source_labels: ["foo"]
  target_label: "bar"
  regex: ".+"
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		})
	})
	t.Run("replace-hit", func(t *testing.T) {
		f(`
- action: replace
  source_labels: ["xxx", "foo"]
  target_label: "bar"
  replacement: "a-$1-b"
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "bar",
				Value: "a-yyy;-b",
			},
			{
				Name:  "xxx",
				Value: "yyy",
			},
		})
	})
	t.Run("replace-remove-label-value-hit", func(t *testing.T) {
		f(`
- action: replace
  source_labels: ["foo"]
  target_label: "foo"
  regex: "xxx"
  replacement: ""
`, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "xxx",
			},
			{
				Name:  "bar",
				Value: "baz",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "bar",
				Value: "baz",
			},
		})
	})
	t.Run("replace-remove-label-value-miss", func(t *testing.T) {
		f(`
- action: replace
  source_labels: ["foo"]
  target_label: "foo"
  regex: "xxx"
  replacement: ""
`, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "yyy",
			},
			{
				Name:  "bar",
				Value: "baz",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "bar",
				Value: "baz",
			},
			{
				Name:  "foo",
				Value: "yyy",
			},
		})
	})
	t.Run("replace-hit-remove-label", func(t *testing.T) {
		f(`
- action: replace
  source_labels: ["xxx", "foo"]
  regex: "yyy;.+"
  target_label: "foo"
  replacement: ""
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
			{
				Name:  "foo",
				Value: "bar",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		})
	})
	t.Run("replace-miss-remove-label", func(t *testing.T) {
		f(`
- action: replace
  source_labels: ["xxx", "foo"]
  regex: "yyy;.+"
  target_label: "foo"
  replacement: ""
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyyz",
			},
			{
				Name:  "foo",
				Value: "bar",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "bar",
			},
			{
				Name:  "xxx",
				Value: "yyyz",
			},
		})
	})
	t.Run("replace-hit-target-label-with-capture-group", func(t *testing.T) {
		f(`
- action: replace
  source_labels: ["xxx", "foo"]
  target_label: "bar-$1"
  replacement: "a-$1-b"
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "bar-yyy;",
				Value: "a-yyy;-b",
			},
			{
				Name:  "xxx",
				Value: "yyy",
			},
		})
	})
	t.Run("replace_all-miss", func(t *testing.T) {
		f(`
- action: replace_all
  source_labels: [foo]
  target_label: "bar"
`, nil, false, []prompbmarshal.Label{})
		f(`
- action: replace_all
  source_labels: ["foo"]
  target_label: "bar"
`, nil, false, []prompbmarshal.Label{})
		f(`
- action: replace_all
  source_labels: ["foo"]
  target_label: "bar"
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		})
		f(`
- action: replace_all
  source_labels: ["foo"]
  target_label: "bar"
  regex: ".+"
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		})
	})
	t.Run("replace_all-hit", func(t *testing.T) {
		f(`
- action: replace_all
  source_labels: ["xxx"]
  target_label: "xxx"
  regex: "-"
  replacement: "."
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "a-b-c",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "a.b.c",
			},
		})
	})
	t.Run("replace_all-regex-hit", func(t *testing.T) {
		f(`
- action: replace_all
  source_labels: ["xxx", "foo"]
  target_label: "xxx"
  regex: "(;)"
  replacement: "-$1-"
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "y;y",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "y-;-y-;-",
			},
		})
	})
	t.Run("replace-add-multi-labels", func(t *testing.T) {
		f(`
- action: replace
  source_labels: ["xxx"]
  target_label: "bar"
  replacement: "a-$1"
- action: replace
  source_labels: ["bar"]
  target_label: "zar"
  replacement: "b-$1"
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
			{
				Name:  "instance",
				Value: "a.bc",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "bar",
				Value: "a-yyy",
			},
			{
				Name:  "instance",
				Value: "a.bc",
			},
			{
				Name:  "xxx",
				Value: "yyy",
			},
			{
				Name:  "zar",
				Value: "b-a-yyy",
			},
		})
	})
	t.Run("replace-self", func(t *testing.T) {
		f(`
- action: replace
  source_labels: ["foo"]
  target_label: "foo"
  replacement: "a-$1"
`, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "aaxx",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "a-aaxx",
			},
		})
	})
	t.Run("replace-missing-source", func(t *testing.T) {
		f(`
- action: replace
  target_label: foo
  replacement: "foobar"
`, []prompbmarshal.Label{}, true, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "foobar",
			},
		})
	})
	t.Run("keep_if_equal-miss", func(t *testing.T) {
		f(`
- action: keep_if_equal
  source_labels: ["foo", "bar"]
`, nil, true, nil)
		f(`
- action: keep_if_equal
  source_labels: ["xxx", "bar"]
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		}, true, []prompbmarshal.Label{})
	})
	t.Run("keep_if_equal-hit", func(t *testing.T) {
		f(`
- action: keep_if_equal
  source_labels: ["xxx", "bar"]
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
			{
				Name:  "bar",
				Value: "yyy",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "bar",
				Value: "yyy",
			},
			{
				Name:  "xxx",
				Value: "yyy",
			},
		})
	})
	t.Run("drop_if_equal-miss", func(t *testing.T) {
		f(`
- action: drop_if_equal
  source_labels: ["foo", "bar"]
`, nil, true, nil)
		f(`
- action: drop_if_equal
  source_labels: ["xxx", "bar"]
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		})
	})
	t.Run("drop_if_equal-hit", func(t *testing.T) {
		f(`
- action: drop_if_equal
  source_labels: [xxx, bar]
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
			{
				Name:  "bar",
				Value: "yyy",
			},
		}, true, []prompbmarshal.Label{})
	})
	t.Run("keep-miss", func(t *testing.T) {
		f(`
- action: keep
  source_labels: [foo]
  regex: ".+"
`, nil, true, nil)
		f(`
- action: keep
  source_labels: [foo]
  regex: ".+"
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		}, true, []prompbmarshal.Label{})
	})
	t.Run("keep-hit", func(t *testing.T) {
		f(`
- action: keep
  source_labels: [foo]
  regex: "yyy"
`, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "yyy",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "yyy",
			},
		})
	})
	t.Run("keep-hit-regexp", func(t *testing.T) {
		f(`
- action: keep
  source_labels: ["foo"]
  regex: ".+"
`, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "yyy",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "yyy",
			},
		})
	})
	t.Run("keep_metrics-miss", func(t *testing.T) {
		f(`
- action: keep_metrics
  regex:
  - foo
  - bar
`, []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "xxx",
			},
		}, true, []prompbmarshal.Label{})
	})
	t.Run("keep_metrics-hit", func(t *testing.T) {
		f(`
- action: keep_metrics
  regex:
  - foo
  - bar
`, []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "foo",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "foo",
			},
		})
	})
	t.Run("drop-miss", func(t *testing.T) {
		f(`
- action: drop
  source_labels: [foo]
  regex: ".+"
`, nil, false, nil)
		f(`
- action: drop
  source_labels: [foo]
  regex: ".+"
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		})
	})
	t.Run("drop-hit", func(t *testing.T) {
		f(`
- action: drop
  source_labels: [foo]
  regex: yyy
`, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "yyy",
			},
		}, true, []prompbmarshal.Label{})
	})
	t.Run("drop-hit-regexp", func(t *testing.T) {
		f(`
- action: drop
  source_labels: [foo]
  regex: ".+"
`, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "yyy",
			},
		}, true, []prompbmarshal.Label{})
	})
	t.Run("drop_metrics-miss", func(t *testing.T) {
		f(`
- action: drop_metrics
  regex:
  - foo
  - bar
`, []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "xxx",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "xxx",
			},
		})
	})
	t.Run("drop_metrics-hit", func(t *testing.T) {
		f(`
- action: drop_metrics
  regex:
  - foo
  - bar
`, []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "foo",
			},
		}, true, []prompbmarshal.Label{})
	})
	t.Run("hashmod-miss", func(t *testing.T) {
		f(`
- action: hashmod
  source_labels: [foo]
  target_label: aaa
  modulus: 123
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "aaa",
				Value: "81",
			},
			{
				Name:  "xxx",
				Value: "yyy",
			},
		})
	})
	t.Run("hashmod-hit", func(t *testing.T) {
		f(`
- action: hashmod
  source_labels: [foo]
  target_label: aaa
  modulus: 123
`, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "yyy",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "aaa",
				Value: "73",
			},
			{
				Name:  "foo",
				Value: "yyy",
			},
		})
	})
	t.Run("labelmap-copy-label", func(t *testing.T) {
		f(`
- action: labelmap
  regex: "foo"
  replacement: "bar"
`, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "yyy",
			},
			{
				Name:  "foobar",
				Value: "aaa",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "bar",
				Value: "yyy",
			},
			{
				Name:  "foo",
				Value: "yyy",
			},
			{
				Name:  "foobar",
				Value: "aaa",
			},
		})
	})
	t.Run("labelmap-remove-prefix-dot-star", func(t *testing.T) {
		f(`
- action: labelmap
  regex: "foo(.*)"
`, []prompbmarshal.Label{
			{
				Name:  "xoo",
				Value: "yyy",
			},
			{
				Name:  "foobar",
				Value: "aaa",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "bar",
				Value: "aaa",
			},
			{
				Name:  "foobar",
				Value: "aaa",
			},
			{
				Name:  "xoo",
				Value: "yyy",
			},
		})
	})
	t.Run("labelmap-remove-prefix-dot-plus", func(t *testing.T) {
		f(`
- action: labelmap
  regex: "foo(.+)"
`, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "yyy",
			},
			{
				Name:  "foobar",
				Value: "aaa",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "bar",
				Value: "aaa",
			},
			{
				Name:  "foo",
				Value: "yyy",
			},
			{
				Name:  "foobar",
				Value: "aaa",
			},
		})
	})
	t.Run("labelmap-regex", func(t *testing.T) {
		f(`
- action: labelmap
  regex: "foo(.+)"
  replacement: "$1-x"
`, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "yyy",
			},
			{
				Name:  "foobar",
				Value: "aaa",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "bar-x",
				Value: "aaa",
			},
			{
				Name:  "foo",
				Value: "yyy",
			},
			{
				Name:  "foobar",
				Value: "aaa",
			},
		})
	})
	t.Run("labelmap_all", func(t *testing.T) {
		f(`
- action: labelmap_all
  regex: "\\."
  replacement: "-"
`, []prompbmarshal.Label{
			{
				Name:  "foo.bar.baz",
				Value: "yyy",
			},
			{
				Name:  "foobar",
				Value: "aaa",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "foo-bar-baz",
				Value: "yyy",
			},
			{
				Name:  "foobar",
				Value: "aaa",
			},
		})
	})
	t.Run("labelmap_all-regexp", func(t *testing.T) {
		f(`
- action: labelmap_all
  regex: "ba(.)"
  replacement: "${1}ss"
`, []prompbmarshal.Label{
			{
				Name:  "foo.bar.baz",
				Value: "yyy",
			},
			{
				Name:  "foozar",
				Value: "aaa",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "foo.rss.zss",
				Value: "yyy",
			},
			{
				Name:  "foozar",
				Value: "aaa",
			},
		})
	})
	t.Run("labeldrop", func(t *testing.T) {
		f(`
- action: labeldrop
  regex: dropme
`, []prompbmarshal.Label{
			{
				Name:  "aaa",
				Value: "bbb",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "aaa",
				Value: "bbb",
			},
		})
		f(`
- action: labeldrop
  regex: dropme
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
			{
				Name:  "dropme",
				Value: "aaa",
			},
			{
				Name:  "foo",
				Value: "bar",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "bar",
			},
			{
				Name:  "xxx",
				Value: "yyy",
			},
		})
		// regex in single quotes
		f(`
- action: labeldrop
  regex: 'dropme'
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
			{
				Name:  "dropme",
				Value: "aaa",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		})
		// regex in double quotes
		f(`
- action: labeldrop
  regex: "dropme"
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
			{
				Name:  "dropme",
				Value: "aaa",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		})
	})
	t.Run("labeldrop-prefix", func(t *testing.T) {
		f(`
- action: labeldrop
  regex: "dropme.*"
`, []prompbmarshal.Label{
			{
				Name:  "aaa",
				Value: "bbb",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "aaa",
				Value: "bbb",
			},
		})
		f(`
- action: labeldrop
  regex: "dropme(.+)"
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
			{
				Name:  "dropme-please",
				Value: "aaa",
			},
			{
				Name:  "foo",
				Value: "bar",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "bar",
			},
			{
				Name:  "xxx",
				Value: "yyy",
			},
		})
	})
	t.Run("labeldrop-regexp", func(t *testing.T) {
		f(`
- action: labeldrop
  regex: ".*dropme.*"
`, []prompbmarshal.Label{
			{
				Name:  "aaa",
				Value: "bbb",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "aaa",
				Value: "bbb",
			},
		})
		f(`
- action: labeldrop
  regex: ".*dropme.*"
`, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
			{
				Name:  "dropme-please",
				Value: "aaa",
			},
			{
				Name:  "foo",
				Value: "bar",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "bar",
			},
			{
				Name:  "xxx",
				Value: "yyy",
			},
		})
	})
	t.Run("labelkeep", func(t *testing.T) {
		f(`
- action: labelkeep
  regex: "keepme"
`, []prompbmarshal.Label{
			{
				Name:  "keepme",
				Value: "aaa",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "keepme",
				Value: "aaa",
			},
		})
		f(`
- action: labelkeep
  regex: keepme
`, []prompbmarshal.Label{
			{
				Name:  "keepme",
				Value: "aaa",
			},
			{
				Name:  "aaaa",
				Value: "awef",
			},
			{
				Name:  "keepme-aaa",
				Value: "234",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "keepme",
				Value: "aaa",
			},
		})
	})
	t.Run("labelkeep-regexp", func(t *testing.T) {
		f(`
- action: labelkeep
  regex: "keepme.*"
`, []prompbmarshal.Label{
			{
				Name:  "keepme",
				Value: "aaa",
			},
		}, true, []prompbmarshal.Label{
			{
				Name:  "keepme",
				Value: "aaa",
			},
		})
		f(`
- action: labelkeep
  regex: "keepme.*"
`, []prompbmarshal.Label{
			{
				Name:  "keepme",
				Value: "aaa",
			},
			{
				Name:  "aaaa",
				Value: "awef",
			},
			{
				Name:  "keepme-aaa",
				Value: "234",
			},
		}, false, []prompbmarshal.Label{
			{
				Name:  "keepme",
				Value: "aaa",
			},
			{
				Name:  "keepme-aaa",
				Value: "234",
			},
		})
	})
}

func TestFinalizeLabels(t *testing.T) {
	f := func(labels, resultExpected []prompbmarshal.Label) {
		t.Helper()
		result := FinalizeLabels(nil, labels)
		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected result; got\n%v\nwant\n%v", result, resultExpected)
		}
	}
	f(nil, nil)
	f([]prompbmarshal.Label{
		{
			Name:  "foo",
			Value: "bar",
		},
		{
			Name:  "__aaa",
			Value: "ass",
		},
		{
			Name:  "instance",
			Value: "foo.com",
		},
	}, []prompbmarshal.Label{
		{
			Name:  "foo",
			Value: "bar",
		},
		{
			Name:  "instance",
			Value: "foo.com",
		},
	})
	f([]prompbmarshal.Label{
		{
			Name:  "foo",
			Value: "bar",
		},
		{
			Name:  "instance",
			Value: "ass",
		},
		{
			Name:  "__address__",
			Value: "foo.com",
		},
	}, []prompbmarshal.Label{
		{
			Name:  "foo",
			Value: "bar",
		},
		{
			Name:  "instance",
			Value: "ass",
		},
	})
}

func TestRemoveMetaLabels(t *testing.T) {
	f := func(labels, resultExpected []prompbmarshal.Label) {
		t.Helper()
		result := RemoveMetaLabels(nil, labels)
		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected result of RemoveMetaLabels;\ngot\n%v\nwant\n%v", result, resultExpected)
		}
	}
	f(nil, nil)
	f([]prompbmarshal.Label{
		{
			Name:  "foo",
			Value: "bar",
		},
	}, []prompbmarshal.Label{
		{
			Name:  "foo",
			Value: "bar",
		},
	})
	f([]prompbmarshal.Label{
		{
			Name:  "__meta_foo",
			Value: "bar",
		},
	}, nil)
	f([]prompbmarshal.Label{
		{
			Name:  "__meta_foo",
			Value: "bdffr",
		},
		{
			Name:  "foo",
			Value: "bar",
		},
		{
			Name:  "__meta_xxx",
			Value: "basd",
		},
	}, []prompbmarshal.Label{
		{
			Name:  "foo",
			Value: "bar",
		},
	})
}
