package promrelabel

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestSanitizeMetricName(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()
		for i := 0; i < 5; i++ {
			result := SanitizeMetricName(s)
			if result != resultExpected {
				t.Fatalf("unexpected result for SanitizeMetricName(%q) at iteration %d; got %q; want %q", s, i, result, resultExpected)
			}
		}
	}
	f("", "")
	f("a", "a")
	f("foo.bar/baz:a", "foo_bar_baz:a")
	f("foo...bar", "foo___bar")
}

func TestSanitizeLabelName(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()
		for i := 0; i < 5; i++ {
			result := SanitizeLabelName(s)
			if result != resultExpected {
				t.Fatalf("unexpected result for SanitizeLabelName(%q) at iteration %d; got %q; want %q", s, i, result, resultExpected)
			}
		}
	}
	f("", "")
	f("a", "a")
	f("foo.bar/baz:a", "foo_bar_baz_a")
	f("foo...bar", "foo___bar")
}

func TestLabelsToString(t *testing.T) {
	f := func(labels []prompbmarshal.Label, sExpected string) {
		t.Helper()
		s := LabelsToString(labels)
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

func TestParsedRelabelConfigsApplyDebug(t *testing.T) {
	f := func(config, metric string, dssExpected []DebugStep) {
		t.Helper()
		pcs, err := ParseRelabelConfigsData([]byte(config))
		if err != nil {
			t.Fatalf("cannot parse %q: %s", config, err)
		}
		labels := promutils.MustNewLabelsFromString(metric)
		_, dss := pcs.ApplyDebug(labels.GetLabels())
		if !reflect.DeepEqual(dss, dssExpected) {
			t.Fatalf("unexpected result; got\n%s\nwant\n%s", dss, dssExpected)
		}
	}

	// empty relabel config
	f(``, `foo`, nil)
	// add label
	f(`
- target_label: abc
  replacement: xyz
`, `foo{bar="baz"}`, []DebugStep{
		{
			Rule: "target_label: abc\nreplacement: xyz\n",
			In:   `foo{bar="baz"}`,
			Out:  `foo{abc="xyz",bar="baz"}`,
		},
	})
	// drop label
	f(`
- target_label: bar
  replacement: ''
`, `foo{bar="baz"}`, []DebugStep{
		{
			Rule: "target_label: bar\nreplacement: \"\"\n",
			In:   `foo{bar="baz"}`,
			Out:  `foo{bar=""}`,
		},
		{
			Rule: "remove empty labels",
			In:   `foo{bar=""}`,
			Out:  `foo`,
		},
	})
	// drop metric
	f(`
- action: drop
  source_labels: [bar]
  regex: baz
`, `foo{bar="baz",abc="def"}`, []DebugStep{
		{
			Rule: "action: drop\nsource_labels: [bar]\nregex: baz\n",
			In:   `foo{abc="def",bar="baz"}`,
			Out:  `{}`,
		},
	})
	// Multiple steps
	f(`
- action: labeldrop
  regex: "foo.*"
- target_label: foobar
  replacement: "abc"
`, `m{foo="x",foobc="123",a="b"}`, []DebugStep{
		{
			Rule: "action: labeldrop\nregex: foo.*\n",
			In:   `m{a="b",foo="x",foobc="123"}`,
			Out:  `m{a="b"}`,
		},
		{
			Rule: "target_label: foobar\nreplacement: abc\n",
			In:   `m{a="b"}`,
			Out:  `m{a="b",foobar="abc"}`,
		},
	})
}

func TestParsedRelabelConfigsApply(t *testing.T) {
	f := func(config, metric string, isFinalize bool, resultExpected string) {
		t.Helper()
		pcs, err := ParseRelabelConfigsData([]byte(config))
		if err != nil {
			t.Fatalf("cannot parse %q: %s", config, err)
		}
		labels := promutils.MustNewLabelsFromString(metric)
		resultLabels := pcs.Apply(labels.GetLabels(), 0)
		if isFinalize {
			resultLabels = FinalizeLabels(resultLabels[:0], resultLabels)
		}
		SortLabels(resultLabels)
		result := LabelsToString(resultLabels)
		if result != resultExpected {
			t.Fatalf("unexpected result; got\n%s\nwant\n%s", result, resultExpected)
		}
	}

	// empty_relabel_configs
	f("", `{}`, false, `{}`)
	f("", `{}`, true, `{}`)
	f("", `{foo="bar"}`, false, `{foo="bar"}`)
	f("", `xxx{foo="bar",__aaa="yyy"}`, false, `xxx{__aaa="yyy",foo="bar"}`)
	f("", `xxx{foo="bar",__aaa="yyy"}`, true, `xxx{foo="bar"}`)

	// replace-miss"
	f(`
- action: replace
  target_label: bar
`, `{}`, false, `{}`)
	f(`
- action: replace
  source_labels: ["foo"]
  target_label: bar
`, `{}`, false, `{}`)
	f(`
- action: replace
  source_labels: ["foo"]
  target_label: "bar"
`, `{xxx="yyy"}`, false, `{xxx="yyy"}`)
	f(`
- action: replace
  source_labels: ["foo"]
  target_label: "bar"
  regex: ".+"
`, `{xxx="yyy"}`, false, `{xxx="yyy"}`)
	f(`
- action: replace
  source_labels: ["foo"]
  target_label: "xxx"
  regex: ".+"
`, `{xxx="yyy"}`, false, `{xxx="yyy"}`)

	// replace-if-miss
	f(`
- action: replace
  if: '{foo="bar"}'
  source_labels: ["xxx", "foo"]
  target_label: "bar"
  replacement: "a-$1-b"
`, `{xxx="yyy"}`, false, `{xxx="yyy"}`)

	// replace-hit
	f(`
- action: replace
  source_labels: ["xxx", "foo"]
  target_label: "bar"
  replacement: "a-$1-b"
`, `{xxx="yyy"}`, false, `{bar="a-yyy;-b",xxx="yyy"}`)
	f(`
- action: replace
  source_labels: ["xxx", "foo"]
  target_label: "xxx"
`, `{xxx="yyy"}`, false, `{xxx="yyy;"}`)
	f(`
- action: replace
  source_labels: ["foo"]
  target_label: "xxx"
`, `{xxx="yyy"}`, false, `{}`)

	// replace-if-hit
	f(`
- action: replace
  if: '{xxx=~".y."}'
  source_labels: ["xxx", "foo"]
  target_label: "bar"
  replacement: "a-$1-b"
`, `{xxx="yyy"}`, false, `{bar="a-yyy;-b",xxx="yyy"}`)

	// replace-remove-label-value-hit
	f(`
- action: replace
  source_labels: ["foo"]
  target_label: "foo"
  regex: "xxx"
  replacement: ""
`, `{foo="xxx",bar="baz"}`, false, `{bar="baz"}`)

	// replace-remove-label-value-miss
	f(`
- action: replace
  source_labels: ["foo"]
  target_label: "foo"
  regex: "xxx"
  replacement: ""
`, `{foo="yyy",bar="baz"}`, false, `{bar="baz",foo="yyy"}`)

	// replace-hit-remove-label
	f(`
- action: replace
  source_labels: ["xxx", "foo"]
  regex: "yyy;.+"
  target_label: "foo"
  replacement: ""
`, `{xxx="yyy",foo="bar"}`, false, `{xxx="yyy"}`)

	// replace-miss-remove-label
	f(`
- action: replace
  source_labels: ["xxx", "foo"]
  regex: "yyy;.+"
  target_label: "foo"
  replacement: ""
`, `{xxx="yyyz",foo="bar"}`, false, `{foo="bar",xxx="yyyz"}`)

	// replace-hit-target-label-with-capture-group
	f(`
- action: replace
  source_labels: ["xxx", "foo"]
  target_label: "bar-$1"
  replacement: "a-$1-b"
`, `{xxx="yyy"}`, false, `{bar-yyy;="a-yyy;-b",xxx="yyy"}`)

	// replace_all-miss
	f(`
- action: replace_all
  source_labels: [foo]
  target_label: "bar"
`, `{}`, false, `{}`)
	f(`
- action: replace_all
  source_labels: ["foo"]
  target_label: "bar"
`, `{}`, false, `{}`)
	f(`
- action: replace_all
  source_labels: ["foo"]
  target_label: "bar"
`, `{xxx="yyy"}`, false, `{xxx="yyy"}`)
	f(`
- action: replace_all
  source_labels: ["foo"]
  target_label: "bar"
  regex: ".+"
`, `{xxx="yyy"}`, false, `{xxx="yyy"}`)

	// replace_all-if-miss
	f(`
- action: replace_all
  if: 'foo'
  source_labels: ["xxx"]
  target_label: "xxx"
  regex: "-"
  replacement: "."
`, `{xxx="a-b-c"}`, false, `{xxx="a-b-c"}`)

	// replace_all-hit
	f(`
- action: replace_all
  source_labels: ["xxx"]
  target_label: "xxx"
  regex: "-"
  replacement: "."
`, `{xxx="a-b-c"}`, false, `{xxx="a.b.c"}`)

	// replace_all-if-hit
	f(`
- action: replace_all
  if: '{non_existing_label=~".*"}'
  source_labels: ["xxx"]
  target_label: "xxx"
  regex: "-"
  replacement: "."
`, `{xxx="a-b-c"}`, false, `{xxx="a.b.c"}`)

	// replace_all-regex-hit
	f(`
- action: replace_all
  source_labels: ["xxx", "foo"]
  target_label: "xxx"
  regex: "(;)"
  replacement: "-$1-"
`, `{xxx="y;y"}`, false, `{xxx="y-;-y-;-"}`)

	// replace-add-multi-labels
	f(`
- action: replace
  source_labels: ["xxx"]
  target_label: "bar"
  replacement: "a-$1"
- action: replace
  source_labels: ["bar"]
  target_label: "zar"
  replacement: "b-$1"
`, `{xxx="yyy",instance="a.bc"}`, true, `{bar="a-yyy",instance="a.bc",xxx="yyy",zar="b-a-yyy"}`)

	// replace-self
	f(`
- action: replace
  source_labels: ["foo"]
  target_label: "foo"
  replacement: "a-$1"
`, `{foo="aaxx"}`, true, `{foo="a-aaxx"}`)

	// replace-missing-source
	f(`
- action: replace
  target_label: foo
  replacement: "foobar"
`, `{}`, true, `{foo="foobar"}`)

	// keep_if_contains-non-existing-target-and-source
	f(`
- action: keep_if_contains
  target_label: foo
  source_labels: [bar]
`, `{x="y"}`, true, `{x="y"}`)

	// keep_if_contains-non-existing-target
	f(`
- action: keep_if_contains
  target_label: foo
  source_labels: [bar]
`, `{bar="aaa"}`, true, `{}`)

	// keep_if_contains-non-existing-source
	f(`
- action: keep_if_contains
  target_label: foo
  source_labels: [bar]
`, `{foo="aaa"}`, true, `{foo="aaa"}`)

	// keep_if_contains-matching-source-target
	f(`
- action: keep_if_contains
  target_label: foo
  source_labels: [bar]
`, `{bar="aaa",foo="aaa"}`, true, `{bar="aaa",foo="aaa"}`)

	// keep_if_contains-matching-sources-target
	f(`
- action: keep_if_contains
  target_label: foo
  source_labels: [bar, baz]
`, `{bar="aaa",foo="aaa",baz="aaa"}`, true, `{bar="aaa",baz="aaa",foo="aaa"}`)

	// keep_if_contains-mismatching-source-target
	f(`
- action: keep_if_contains
  target_label: foo
  source_labels: [bar]
`, `{bar="aaa",foo="bbb"}`, true, `{}`)

	// keep_if_contains-mismatching-sources-target
	f(`
- action: keep_if_contains
  target_label: foo
  source_labels: [bar, baz]
`, `{bar="aaa",foo="aaa",baz="bbb"}`, true, `{}`)

	// drop_if_contains-non-existing-target-and-source
	f(`
- action: drop_if_contains
  target_label: foo
  source_labels: [bar]
`, `{x="y"}`, true, `{}`)

	// drop_if_contains-non-existing-target
	f(`
- action: drop_if_contains
  target_label: foo
  source_labels: [bar]
`, `{bar="aaa"}`, true, `{bar="aaa"}`)

	// drop_if_contains-non-existing-source
	f(`
- action: drop_if_contains
  target_label: foo
  source_labels: [bar]
`, `{foo="aaa"}`, true, `{}`)

	// drop_if_contains-matching-source-target
	f(`
- action: drop_if_contains
  target_label: foo
  source_labels: [bar]
`, `{bar="aaa",foo="aaa"}`, true, `{}`)

	// drop_if_contains-matching-sources-target
	f(`
- action: drop_if_contains
  target_label: foo
  source_labels: [bar, baz]
`, `{bar="aaa",foo="aaa",baz="aaa"}`, true, `{}`)

	// drop_if_contains-mismatching-source-target
	f(`
- action: drop_if_contains
  target_label: foo
  source_labels: [bar]
`, `{bar="aaa",foo="bbb"}`, true, `{bar="aaa",foo="bbb"}`)

	// drop_if_contains-mismatching-sources-target
	f(`
- action: drop_if_contains
  target_label: foo
  source_labels: [bar, baz]
`, `{bar="aaa",foo="aaa",baz="bbb"}`, true, `{bar="aaa",baz="bbb",foo="aaa"}`)

	// keep_if_equal-miss
	f(`
- action: keep_if_equal
  source_labels: ["foo", "bar"]
`, `{}`, true, `{}`)
	f(`
- action: keep_if_equal
  source_labels: ["xxx", "bar"]
`, `{xxx="yyy"}`, true, `{}`)

	// keep_if_equal-hit
	f(`
- action: keep_if_equal
  source_labels: ["xxx", "bar"]
`, `{xxx="yyy",bar="yyy"}`, true, `{bar="yyy",xxx="yyy"}`)

	// drop_if_equal-miss
	f(`
- action: drop_if_equal
  source_labels: ["foo", "bar"]
`, `{}`, true, `{}`)
	f(`
- action: drop_if_equal
  source_labels: ["xxx", "bar"]
`, `{xxx="yyy"}`, true, `{xxx="yyy"}`)

	// drop_if_equal-hit
	f(`
- action: drop_if_equal
  source_labels: [xxx, bar]
`, `{xxx="yyy",bar="yyy"}`, true, `{}`)

	// keepequal-hit
	f(`
- action: keepequal
  source_labels: [foo]
  target_label: bar
`, `{foo="a",bar="a"}`, true, `{bar="a",foo="a"}`)

	// keepequal-miss
	f(`
- action: keepequal
  source_labels: [foo]
  target_label: bar
`, `{foo="a",bar="x"}`, true, `{}`)

	// dropequal-hit
	f(`
- action: dropequal
  source_labels: [foo]
  target_label: bar
`, `{foo="a",bar="a"}`, true, `{}`)

	// dropequal-miss
	f(`
- action: dropequal
  source_labels: [foo]
  target_label: bar
`, `{foo="a",bar="x"}`, true, `{bar="x",foo="a"}`)

	// keep-miss
	f(`
- action: keep
  source_labels: [foo]
  regex: ".+"
`, `{}`, true, `{}`)
	f(`
- action: keep
  source_labels: [foo]
  regex: ".+"
`, `{xxx="yyy"}`, true, `{}`)

	// keep-if-miss
	f(`
- action: keep
  if: '{foo="bar"}'
`, `{foo="yyy"}`, false, `{}`)

	// keep-if-hit
	f(`
- action: keep
  if: ['foobar', '{foo="yyy"}', '{a="b"}']
`, `{foo="yyy"}`, false, `{foo="yyy"}`)

	// keep-hit
	f(`
- action: keep
  source_labels: [foo]
  regex: "yyy"
`, `{foo="yyy"}`, false, `{foo="yyy"}`)

	// keep-hit-regexp
	f(`
- action: keep
  source_labels: ["foo"]
  regex: ".+"
`, `{foo="yyy"}`, false, `{foo="yyy"}`)

	// keep_metrics-miss
	f(`
- action: keep_metrics
  regex:
  - foo
  - bar
`, `xxx`, true, `{}`)

	// keep_metrics-if-miss
	f(`
- action: keep_metrics
  if: 'bar'
`, `foo`, true, `{}`)

	// keep_metrics-if-hit
	f(`
- action: keep_metrics
  if: 'foo'
`, `foo`, true, `foo`)

	// keep_metrics-hit
	f(`
- action: keep_metrics
  regex:
  - foo
  - bar
`, `foo`, true, `foo`)

	// drop-miss
	f(`
- action: drop
  source_labels: [foo]
  regex: ".+"
`, `{}`, false, `{}`)
	f(`
- action: drop
  source_labels: [foo]
  regex: ".+"
`, `{xxx="yyy"}`, true, `{xxx="yyy"}`)

	// drop-if-miss
	f(`
- action: drop
  if: '{foo="bar"}'
`, `{foo="yyy"}`, true, `{foo="yyy"}`)

	// drop-if-hit
	f(`
- action: drop
  if: '{foo="yyy"}'
`, `{foo="yyy"}`, true, `{}`)

	// drop-hit
	f(`
- action: drop
  source_labels: [foo]
  regex: yyy
`, `{foo="yyy"}`, true, `{}`)

	// drop-hit-regexp
	f(`
- action: drop
  source_labels: [foo]
  regex: ".+"
`, `{foo="yyy"}`, true, `{}`)

	// drop_metrics-miss
	f(`
- action: drop_metrics
  regex:
  - foo
  - bar
`, `xxx`, true, `xxx`)

	// drop_metrics-if-miss
	f(`
- action: drop_metrics
  if: bar
`, `foo`, true, `foo`)

	// drop_metrics-if-hit
	f(`
- action: drop_metrics
  if: foo
`, `foo`, true, `{}`)

	// drop_metrics-hit
	f(`
- action: drop_metrics
  regex:
  - foo
  - bar
`, `foo`, true, `{}`)

	// hashmod-miss
	f(`
- action: hashmod
  source_labels: [foo]
  target_label: aaa
  modulus: 123
`, `{xxx="yyy"}`, false, `{aaa="81",xxx="yyy"}`)

	// hashmod-if-miss
	f(`
- action: hashmod
  if: '{foo="bar"}'
  source_labels: [foo]
  target_label: aaa
  modulus: 123
`, `{foo="yyy"}`, true, `{foo="yyy"}`)

	// hashmod-if-hit
	f(`
- action: hashmod
  if: '{foo="yyy"}'
  source_labels: [foo]
  target_label: aaa
  modulus: 123
`, `{foo="yyy"}`, true, `{aaa="73",foo="yyy"}`)

	// hashmod-hit
	f(`
- action: hashmod
  source_labels: [foo]
  target_label: aaa
  modulus: 123
`, `{foo="yyy"}`, true, `{aaa="73",foo="yyy"}`)

	// labelmap-copy-label-if-miss
	f(`
- action: labelmap
  if: '{foo="yyy",foobar="aab"}'
  regex: "foo"
  replacement: "bar"
`, `{foo="yyy",foobar="aaa"}`, true, `{foo="yyy",foobar="aaa"}`)

	// labelmap-copy-label-if-hit
	f(`
- action: labelmap
  if: '{foo="yyy",foobar="aaa"}'
  regex: "foo"
  replacement: "bar"
`, `{foo="yyy",foobar="aaa"}`, true, `{bar="yyy",foo="yyy",foobar="aaa"}`)

	// labelmap-copy-label
	f(`
- action: labelmap
  regex: "foo"
  replacement: "bar"
`, `{foo="yyy",foobar="aaa"}`, true, `{bar="yyy",foo="yyy",foobar="aaa"}`)

	// labelmap-remove-prefix-dot-star
	f(`
- action: labelmap
  regex: "foo(.*)"
`, `{xoo="yyy",foobar="aaa"}`, true, `{bar="aaa",foobar="aaa",xoo="yyy"}`)

	// labelmap-remove-prefix-dot-plus
	f(`
- action: labelmap
  regex: "foo(.+)"
`, `{foo="yyy",foobar="aaa"}`, true, `{bar="aaa",foo="yyy",foobar="aaa"}`)

	// labelmap-regex
	f(`
- action: labelmap
  regex: "foo(.+)"
  replacement: "$1-x"
`, `{foo="yyy",foobar="aaa"}`, true, `{bar-x="aaa",foo="yyy",foobar="aaa"}`)

	// labelmap_all-if-miss
	f(`
- action: labelmap_all
  if: foobar
  regex: "\\."
  replacement: "-"
`, `{foo.bar.baz="yyy",foobar="aaa"}`, true, `{foo.bar.baz="yyy",foobar="aaa"}`)

	// labelmap_all-if-hit
	f(`
- action: labelmap_all
  if: '{foo.bar.baz="yyy"}'
  regex: "\\."
  replacement: "-"
`, `{foo.bar.baz="yyy",foobar="aaa"}`, true, `{foo-bar-baz="yyy",foobar="aaa"}`)

	// labelmap_all
	f(`
- action: labelmap_all
  regex: "\\."
  replacement: "-"
`, `{foo.bar.baz="yyy",foobar="aaa"}`, true, `{foo-bar-baz="yyy",foobar="aaa"}`)

	// labelmap_all-regexp
	f(`
- action: labelmap_all
  regex: "ba(.)"
  replacement: "${1}ss"
`, `{foo.bar.baz="yyy",foozar="aaa"}`, true, `{foo.rss.zss="yyy",foozar="aaa"}`)

	// labeldrop
	f(`
- action: labeldrop
  regex: dropme
`, `{aaa="bbb"}`, true, `{aaa="bbb"}`)

	// if-miss
	f(`
- action: labeldrop
  if: foo
  regex: dropme
`, `{xxx="yyy",dropme="aaa",foo="bar"}`, false, `{dropme="aaa",foo="bar",xxx="yyy"}`)

	// if-hit
	f(`
- action: labeldrop
  if: '{xxx="yyy"}'
  regex: dropme
`, `{xxx="yyy",dropme="aaa",foo="bar"}`, false, `{foo="bar",xxx="yyy"}`)
	f(`
- action: labeldrop
  regex: dropme
`, `{xxx="yyy",dropme="aaa",foo="bar"}`, false, `{foo="bar",xxx="yyy"}`)

	// regex in single quotes
	f(`
- action: labeldrop
  regex: 'dropme'
`, `{xxx="yyy",dropme="aaa"}`, false, `{xxx="yyy"}`)

	// regex in double quotes
	f(`
- action: labeldrop
  regex: "dropme"
`, `{xxx="yyy",dropme="aaa"}`, false, `{xxx="yyy"}`)

	// labeldrop-prefix
	f(`
- action: labeldrop
  regex: "dropme.*"
`, `{aaa="bbb"}`, true, `{aaa="bbb"}`)
	f(`
- action: labeldrop
  regex: "dropme(.+)"
`, `{xxx="yyy",dropme-please="aaa",foo="bar"}`, false, `{foo="bar",xxx="yyy"}`)

	// labeldrop-regexp
	f(`
- action: labeldrop
  regex: ".*dropme.*"
`, `{aaa="bbb"}`, true, `{aaa="bbb"}`)
	f(`
- action: labeldrop
  regex: ".*dropme.*"
`, `{xxx="yyy",dropme-please="aaa",foo="bar"}`, false, `{foo="bar",xxx="yyy"}`)

	// labelkeep
	f(`
- action: labelkeep
  regex: "keepme"
`, `{keepme="aaa"}`, true, `{keepme="aaa"}`)

	// if-miss
	f(`
- action: labelkeep
  if: '{aaaa="awefx"}'
  regex: keepme
`, `{keepme="aaa",aaaa="awef",keepme-aaa="234"}`, false, `{aaaa="awef",keepme="aaa",keepme-aaa="234"}`)

	// if-hit
	f(`
- action: labelkeep
  if: '{aaaa="awef"}'
  regex: keepme
`, `{keepme="aaa",aaaa="awef",keepme-aaa="234"}`, false, `{keepme="aaa"}`)
	f(`
- action: labelkeep
  regex: keepme
`, `{keepme="aaa",aaaa="awef",keepme-aaa="234"}`, false, `{keepme="aaa"}`)

	// labelkeep-regexp
	f(`
- action: labelkeep
  regex: "keepme.*"
`, `{keepme="aaa"}`, true, `{keepme="aaa"}`)
	f(`
- action: labelkeep
  regex: "keepme.*"
`, `{keepme="aaa",aaaa="awef",keepme-aaa="234"}`, false, `{keepme="aaa",keepme-aaa="234"}`)

	// upper-lower-case
	f(`
- action: uppercase
  source_labels: ["foo"]
  target_label: foo
`, `{foo="bar"}`, true, `{foo="BAR"}`)
	f(`
- action: lowercase
  source_labels: ["foo", "bar"]
  target_label: baz
- action: labeldrop
  regex: foo|bar
`, `{foo="BaR",bar="fOO"}`, true, `{baz="bar;foo"}`)
	f(`
- action: lowercase
  source_labels: ["foo"]
  target_label: baz
- action: uppercase
  source_labels: ["bar"]
  target_label: baz
`, `{qux="quux"}`, true, `{qux="quux"}`)

	// graphite-match
	f(`
- action: graphite
  match: foo.*.baz
  labels:
    __name__: aaa
    job: ${1}-zz
`, `foo.bar.baz`, true, `aaa{job="bar-zz"}`)

	// graphite-mismatch
	f(`
- action: graphite
  match: foo.*.baz
  labels:
    __name__: aaa
    job: ${1}-zz
`, `foo.bar.bazz`, true, `foo.bar.bazz`)

	// replacement-with-label-refs
	// no regex
	f(`
- target_label: abc
  replacement: "{{__name__}}.{{foo}}"
`, `qwe{foo="bar",baz="aaa"}`, true, `qwe{abc="qwe.bar",baz="aaa",foo="bar"}`)

	// with regex
	f(`
- target_label: abc
  replacement: "{{__name__}}.{{foo}}.$1"
  source_labels: [baz]
  regex: "a(.+)"
`, `qwe{foo="bar",baz="aaa"}`, true, `qwe{abc="qwe.bar.aa",baz="aaa",foo="bar"}`)

	// Check $ at the end of regex - see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3131
	f(`
- target_label: xyz
  regex: "foo\\$$"
  replacement: bar
  source_labels: [xyz]
`, `metric{xyz="foo$",a="b"}`, true, `metric{a="b",xyz="bar"}`)

	// issue-3251
	f(`
- source_labels: [instance, container_label_com_docker_swarm_task_name]
  separator: ';'
  #  regex: '(.*?)\..*;(.*?)\..*'
  regex: '([^.]+).[^;]+;([^.]+).+'
  replacement: '$2:$1'
  target_label: container_label_com_docker_swarm_task_name
  action: replace
`, `{instance="subdomain.domain.com",container_label_com_docker_swarm_task_name="myservice.h408nlaxmv8oqkn1pjjtd71to.nv987lz99rb27lkjjnfiay0g4"}`, true,
		`{container_label_com_docker_swarm_task_name="myservice:subdomain",instance="subdomain.domain.com"}`)
}

func TestFinalizeLabels(t *testing.T) {
	f := func(metric, resultExpected string) {
		t.Helper()
		labels := promutils.MustNewLabelsFromString(metric)
		resultLabels := FinalizeLabels(nil, labels.GetLabels())
		result := LabelsToString(resultLabels)
		if result != resultExpected {
			t.Fatalf("unexpected result; got\n%s\nwant\n%s", result, resultExpected)
		}
	}
	f(`{}`, `{}`)
	f(`{foo="bar",__aaa="ass",instance="foo.com"}`, `{foo="bar",instance="foo.com"}`)
	f(`{foo="bar",instance="ass",__address__="foo.com"}`, `{foo="bar",instance="ass"}`)
	f(`{foo="bar",abc="def",__address__="foo.com"}`, `{abc="def",foo="bar"}`)
}

func TestFillLabelReferences(t *testing.T) {
	f := func(replacement, metric, resultExpected string) {
		t.Helper()
		labels := promutils.MustNewLabelsFromString(metric)
		result := fillLabelReferences(nil, replacement, labels.GetLabels())
		if string(result) != resultExpected {
			t.Fatalf("unexpected result; got\n%q\nwant\n%q", result, resultExpected)
		}
	}
	f(``, `foo{bar="baz"}`, ``)
	f(`abc`, `foo{bar="baz"}`, `abc`)
	f(`foo{{bar`, `foo{bar="baz"}`, `foo{{bar`)
	f(`foo-$1`, `foo{bar="baz"}`, `foo-$1`)
	f(`foo{{bar}}`, `foo{bar="baz"}`, `foobaz`)
	f(`{{bar}}`, `foo{bar="baz"}`, `baz`)
	f(`{{bar}}-aa`, `foo{bar="baz"}`, `baz-aa`)
	f(`{{bar}}-aa{{__name__}}.{{bar}}{{non-existing-label}}`, `foo{bar="baz"}`, `baz-aafoo.baz`)
}

func TestRegexMatchStringSuccess(t *testing.T) {
	f := func(pattern, s string) {
		t.Helper()
		prc := newTestRegexRelabelConfig(pattern)
		if !prc.regex.MatchString(s) {
			t.Fatalf("unexpected MatchString(%q) result; got false; want true", s)
		}
	}
	f("", "")
	f("foo", "foo")
	f(".*", "")
	f(".*", "foo")
	f("foo.*", "foobar")
	f("foo.+", "foobar")
	f("f.+o", "foo")
	f("foo|bar", "bar")
	f("^(foo|bar)$", "foo")
	f("foo.+", "foobar")
	f("^foo$", "foo")
}

func TestRegexpMatchStringFailure(t *testing.T) {
	f := func(pattern, s string) {
		t.Helper()
		prc := newTestRegexRelabelConfig(pattern)
		if prc.regex.MatchString(s) {
			t.Fatalf("unexpected MatchString(%q) result; got true; want false", s)
		}
	}
	f("", "foo")
	f("foo", "")
	f("foo.*", "foa")
	f("foo.+", "foo")
	f("f.+o", "foor")
	f("foo|bar", "barz")
	f("^(foo|bar)$", "xfoo")
	f("foo.+", "foo")
	f("^foo$", "foobar")
}

func newTestRegexRelabelConfig(pattern string) *parsedRelabelConfig {
	rc := &RelabelConfig{
		Action: "labeldrop",
		Regex: &MultiLineRegex{
			S: pattern,
		},
	}
	prc, err := parseRelabelConfig(rc)
	if err != nil {
		panic(fmt.Errorf("unexpected error in parseRelabelConfig: %w", err))
	}
	return prc
}

func TestParsedRelabelConfigsApplyForMultipleSeries(t *testing.T) {
	f := func(config string, metrics []string, resultExpected []string) {
		t.Helper()
		pcs, err := ParseRelabelConfigsData([]byte(config))
		if err != nil {
			t.Fatalf("cannot parse %q: %s", config, err)
		}

		totalLabels := 0
		var labels []prompbmarshal.Label
		for _, metric := range metrics {
			labels = append(labels, promutils.MustNewLabelsFromString(metric).GetLabels()...)
			resultLabels := pcs.Apply(labels, totalLabels)
			SortLabels(resultLabels)
			totalLabels += len(resultLabels)
			labels = resultLabels
		}

		var result []string
		for i := range labels {
			result = append(result, LabelsToString(labels[i:i+1]))
		}

		if len(result) != len(resultExpected) {
			t.Fatalf("unexpected number of results; got\n%q\nwant\n%q", result, resultExpected)
		}

		for i := range result {
			if result[i] != resultExpected[i] {
				t.Fatalf("unexpected result[%d]; got\n%q\nwant\n%q", i, result[i], resultExpected[i])
			}
		}
	}

	// drops one of series
	f(`
- action: drop
  if: '{__name__!~"smth"}' 
`, []string{`smth`, `notthis`}, []string{`smth`})
	f(`
- action: drop
  if: '{__name__!~"smth"}'
`, []string{`notthis`, `smth`}, []string{`smth`})
}
