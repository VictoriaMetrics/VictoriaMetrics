package promrelabel

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestMultiLineRegexUnmarshalMarshal(t *testing.T) {
	f := func(data, resultExpected string) {
		t.Helper()
		var mlr MultiLineRegex
		if err := yaml.UnmarshalStrict([]byte(data), &mlr); err != nil {
			t.Fatalf("cannot unmarshal %q: %s", data, err)
		}
		result, err := yaml.Marshal(&mlr)
		if err != nil {
			t.Fatalf("cannot marshal %q: %s", data, err)
		}
		if string(result) != resultExpected {
			t.Fatalf("unexpected marshaled data; got\n%q\nwant\n%q", result, resultExpected)
		}
	}
	f(``, `""`+"\n")
	f(`foo`, "foo\n")
	f(`a|b||c`, "- a\n- b\n- \"\"\n- c\n")
	f(`(a|b)`, "(a|b)\n")
	f(`a|b[c|d]`, "a|b[c|d]\n")
	f("- a\n- b", "- a\n- b\n")
	f("- a\n- (b)", "a|(b)\n")
}

func TestRelabelConfigMarshalUnmarshal(t *testing.T) {
	f := func(data, resultExpected string) {
		t.Helper()
		var rcs []RelabelConfig
		if err := yaml.UnmarshalStrict([]byte(data), &rcs); err != nil {
			t.Fatalf("cannot unmarshal %q: %s", data, err)
		}
		result, err := yaml.Marshal(&rcs)
		if err != nil {
			t.Fatalf("cannot marshal %q: %s", data, err)
		}
		if string(result) != resultExpected {
			t.Fatalf("unexpected marshaled data; got\n%q\nwant\n%q", result, resultExpected)
		}
	}
	f(``, "[]\n")
	f(`
- action: keep
  regex: foobar
  `, "- action: keep\n  regex: foobar\n")
	f(`
- regex:
  - 'fo.+'
  - '.*ba[r-z]a'
`, "- regex: fo.+|.*ba[r-z]a\n")
	f(`- regex: foo|bar`, "- regex:\n  - foo\n  - bar\n")
	f(`- regex: True`, `- regex: "true"`+"\n")
	f(`- regex: true`, `- regex: "true"`+"\n")
	f(`- regex: 123`, `- regex: "123"`+"\n")
	f(`- regex: 1.23`, `- regex: "1.23"`+"\n")
	f(`- regex: [null]`, `- regex: "null"`+"\n")
	f(`
- regex:
  - -1.23
  - False
  - null
  - nan
`, "- regex:\n  - \"-1.23\"\n  - \"false\"\n  - \"null\"\n  - nan\n")
	f(`
- action: graphite
  match: 'foo.*.*.aaa'
  labels:
    instance: '$1-abc'
    job: '${2}'
`, "- action: graphite\n  match: foo.*.*.aaa\n  labels:\n    instance: $1-abc\n    job: ${2}\n")
}

func TestLoadRelabelConfigsSuccess(t *testing.T) {
	path := "testdata/relabel_configs_valid.yml"
	pcs, err := LoadRelabelConfigs(path)
	if err != nil {
		t.Fatalf("cannot load relabel configs from %q: %s", path, err)
	}
	nExpected := 18
	if n := pcs.Len(); n != nExpected {
		t.Fatalf("unexpected number of relabel configs loaded from %q; got %d; want %d", path, n, nExpected)
	}
}

func TestLoadRelabelConfigsFailure(t *testing.T) {
	f := func(path string) {
		t.Helper()
		rcs, err := LoadRelabelConfigs(path)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if rcs.Len() != 0 {
			t.Fatalf("unexpected non-empty rcs: %#v", rcs)
		}
	}
	t.Run("non-existing-file", func(t *testing.T) {
		f("testdata/non-exsiting-file")
	})
	t.Run("invalid-file", func(t *testing.T) {
		f("testdata/invalid_config.yml")
	})
}

func TestParsedConfigsString(t *testing.T) {
	f := func(rcs []RelabelConfig, sExpected string) {
		t.Helper()
		pcs, err := ParseRelabelConfigs(rcs)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		s := pcs.String()
		if s != sExpected {
			t.Fatalf("unexpected string representation for ParsedConfigs;\ngot\n%s\nwant\n%s", s, sExpected)
		}
	}
	f([]RelabelConfig{
		{
			TargetLabel:  "foo",
			SourceLabels: []string{"aaa"},
		},
	}, "- source_labels: [aaa]\n  target_label: foo\n")
	var ie IfExpression
	if err := ie.Parse("{foo=~'bar'}"); err != nil {
		t.Fatalf("unexpected error when parsing if expression: %s", err)
	}
	f([]RelabelConfig{
		{
			Action: "graphite",
			Match:  "foo.*.bar",
			Labels: map[string]string{
				"job": "$1-zz",
			},
			If: &ie,
		},
	}, "- if: '{foo=~''bar''}'\n  action: graphite\n  match: foo.*.bar\n  labels:\n    job: $1-zz\n")
	replacement := "foo"
	f([]RelabelConfig{
		{
			Action:       "replace",
			SourceLabels: []string{"foo", "bar"},
			TargetLabel:  "x",
			If:           &ie,
		},
		{
			TargetLabel: "x",
			Replacement: &replacement,
		},
	}, "- if: '{foo=~''bar''}'\n  action: replace\n  source_labels: [foo, bar]\n  target_label: x\n- target_label: x\n  replacement: foo\n")
}

func TestParseRelabelConfigsSuccess(t *testing.T) {
	f := func(rcs []RelabelConfig, pcsExpected *ParsedConfigs) {
		t.Helper()
		pcs, err := ParseRelabelConfigs(rcs)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if pcs != nil {
			for _, prc := range pcs.prcs {
				prc.ruleOriginal = ""
				prc.stringReplacer = nil
				prc.submatchReplacer = nil
			}
		}
		if !reflect.DeepEqual(pcs, pcsExpected) {
			t.Fatalf("unexpected pcs; got\n%#v\nwant\n%#v", pcs, pcsExpected)
		}
	}
	f(nil, nil)
	f([]RelabelConfig{
		{
			SourceLabels: []string{"foo", "bar"},
			TargetLabel:  "xxx",
		},
	}, &ParsedConfigs{
		prcs: []*parsedRelabelConfig{
			{
				SourceLabels:  []string{"foo", "bar"},
				Separator:     ";",
				TargetLabel:   "xxx",
				RegexAnchored: defaultRegexForRelabelConfig,
				Replacement:   "$1",
				Action:        "replace",

				regex:                        defaultPromRegex,
				regexOriginal:                defaultOriginalRegexForRelabelConfig,
				hasCaptureGroupInReplacement: true,
			},
		},
	})
}

func TestParseRelabelConfigsFailure(t *testing.T) {
	f := func(rcs []RelabelConfig) {
		t.Helper()
		pcs, err := ParseRelabelConfigs(rcs)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if pcs.Len() > 0 {
			t.Fatalf("unexpected non-empty pcs: %#v", pcs)
		}
	}
	t.Run("invalid-regex", func(t *testing.T) {
		f([]RelabelConfig{
			{
				SourceLabels: []string{"aaa"},
				TargetLabel:  "xxx",
				Regex: &MultiLineRegex{
					S: "foo[bar",
				},
			},
		})
	})
	t.Run("replace-missing-target-label", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "replace",
				SourceLabels: []string{"foo"},
			},
		})
	})
	t.Run("replace_all-missing-source-labels", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:      "replace_all",
				TargetLabel: "xxx",
			},
		})
	})
	t.Run("replace_all-missing-target-label", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "replace_all",
				SourceLabels: []string{"foo"},
			},
		})
	})
	t.Run("keep-missing-source-labels", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action: "keep",
			},
		})
	})
	t.Run("keep_if_contains-missing-target-label", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "keep_if_contains",
				SourceLabels: []string{"foo"},
			},
		})
	})
	t.Run("keep_if_contains-missing-source-labels", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:      "keep_if_contains",
				TargetLabel: "foo",
			},
		})
	})
	t.Run("keep_if_contains-unused-regex", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "keep_if_contains",
				TargetLabel:  "foo",
				SourceLabels: []string{"bar"},
				Regex: &MultiLineRegex{
					S: "bar",
				},
			},
		})
	})
	t.Run("drop_if_contains-missing-target-label", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "drop_if_contains",
				SourceLabels: []string{"foo"},
			},
		})
	})
	t.Run("drop_if_contains-missing-source-labels", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:      "drop_if_contains",
				TargetLabel: "foo",
			},
		})
	})
	t.Run("drop_if_contains-unused-regex", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "drop_if_contains",
				TargetLabel:  "foo",
				SourceLabels: []string{"bar"},
				Regex: &MultiLineRegex{
					S: "bar",
				},
			},
		})
	})
	t.Run("keep_if_equal-missing-source-labels", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action: "keep_if_equal",
			},
		})
	})
	t.Run("keep_if_equal-single-source-label", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "keep_if_equal",
				SourceLabels: []string{"foo"},
			},
		})
	})
	t.Run("keep_if_equal-unused-target-label", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "keep_if_equal",
				SourceLabels: []string{"foo", "bar"},
				TargetLabel:  "foo",
			},
		})
	})
	t.Run("keep_if_equal-unused-regex", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "keep_if_equal",
				SourceLabels: []string{"foo", "bar"},
				Regex: &MultiLineRegex{
					S: "bar",
				},
			},
		})
	})
	t.Run("drop_if_equal-missing-source-labels", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action: "drop_if_equal",
			},
		})
	})
	t.Run("drop_if_equal-single-source-label", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "drop_if_equal",
				SourceLabels: []string{"foo"},
			},
		})
	})
	t.Run("drop_if_equal-unused-target-label", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "drop_if_equal",
				SourceLabels: []string{"foo", "bar"},
				TargetLabel:  "foo",
			},
		})
	})
	t.Run("drop_if_equal-unused-regex", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "drop_if_equal",
				SourceLabels: []string{"foo", "bar"},
				Regex: &MultiLineRegex{
					S: "bar",
				},
			},
		})
	})
	t.Run("keepequal-missing-source-labels", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action: "keepequal",
			},
		})
	})
	t.Run("keepequal-missing-target-label", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "keepequal",
				SourceLabels: []string{"foo"},
			},
		})
	})
	t.Run("keepequal-unused-regex", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "keepequal",
				SourceLabels: []string{"foo"},
				TargetLabel:  "foo",
				Regex: &MultiLineRegex{
					S: "bar",
				},
			},
		})
	})
	t.Run("dropequal-missing-source-labels", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action: "dropequal",
			},
		})
	})
	t.Run("dropequal-missing-target-label", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "dropequal",
				SourceLabels: []string{"foo"},
			},
		})
	})
	t.Run("dropequal-unused-regex", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "dropequal",
				SourceLabels: []string{"foo"},
				TargetLabel:  "foo",
				Regex: &MultiLineRegex{
					S: "bar",
				},
			},
		})
	})
	t.Run("drop-missing-source-labels", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action: "drop",
			},
		})
	})
	t.Run("hashmod-missing-source-labels", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:      "hashmod",
				TargetLabel: "aaa",
				Modulus:     123,
			},
		})
	})
	t.Run("hashmod-missing-target-label", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "hashmod",
				SourceLabels: []string{"aaa"},
				Modulus:      123,
			},
		})
	})
	t.Run("hashmod-missing-modulus", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "hashmod",
				SourceLabels: []string{"aaa"},
				TargetLabel:  "xxx",
			},
		})
	})
	t.Run("invalid-action", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action: "invalid-action",
			},
		})
	})
	t.Run("drop_metrics-missing-regex", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action: "drop_metrics",
			},
		})
	})
	t.Run("drop_metrics-non-empty-source-labels", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "drop_metrics",
				SourceLabels: []string{"foo"},
				Regex: &MultiLineRegex{
					S: "bar",
				},
			},
		})
	})
	t.Run("keep_metrics-missing-regex", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action: "keep_metrics",
			},
		})
	})
	t.Run("keep_metrics-non-empty-source-labels", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "keep_metrics",
				SourceLabels: []string{"foo"},
				Regex: &MultiLineRegex{
					S: "bar",
				},
			},
		})
	})
	t.Run("uppercase-missing-sourceLabels", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:      "uppercase",
				TargetLabel: "foobar",
			},
		})
	})
	t.Run("lowercase-missing-targetLabel", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "lowercase",
				SourceLabels: []string{"foobar"},
			},
		})
	})
	t.Run("graphite-missing-match", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action: "graphite",
				Labels: map[string]string{
					"foo": "bar",
				},
			},
		})
	})
	t.Run("graphite-missing-labels", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action: "graphite",
				Match:  "foo.*.bar",
			},
		})
	})
	t.Run("graphite-superflouous-sourceLabels", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action: "graphite",
				Match:  "foo.*.bar",
				Labels: map[string]string{
					"foo": "bar",
				},
				SourceLabels: []string{"foo"},
			},
		})
	})
	t.Run("graphite-superflouous-targetLabel", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action: "graphite",
				Match:  "foo.*.bar",
				Labels: map[string]string{
					"foo": "bar",
				},
				TargetLabel: "foo",
			},
		})
	})
	replacement := "foo"
	t.Run("graphite-superflouous-replacement", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action: "graphite",
				Match:  "foo.*.bar",
				Labels: map[string]string{
					"foo": "bar",
				},
				Replacement: &replacement,
			},
		})
	})
	var re MultiLineRegex
	t.Run("graphite-superflouous-regex", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action: "graphite",
				Match:  "foo.*.bar",
				Labels: map[string]string{
					"foo": "bar",
				},
				Regex: &re,
			},
		})
	})
	t.Run("non-graphite-superflouos-match", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "uppercase",
				SourceLabels: []string{"foo"},
				TargetLabel:  "foo",
				Match:        "aaa",
			},
		})
	})
	t.Run("non-graphite-superflouos-labels", func(t *testing.T) {
		f([]RelabelConfig{
			{
				Action:       "uppercase",
				SourceLabels: []string{"foo"},
				TargetLabel:  "foo",
				Labels: map[string]string{
					"foo": "Bar",
				},
			},
		})
	})
}

func TestIsDefaultRegex(t *testing.T) {
	f := func(s string, resultExpected bool) {
		t.Helper()
		result := isDefaultRegex(s)
		if result != resultExpected {
			t.Fatalf("unexpected result for isDefaultRegex(%q); got %v; want %v", s, result, resultExpected)
		}
	}
	f("", false)
	f("foo", false)
	f(".+", false)
	f("a.*", false)
	f(".*", true)
	f("(.*)", true)
	f("^.*$", true)
	f("(?:.*)", true)
}
