package promrelabel

import (
	"reflect"
	"regexp"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestApplyRelabelConfigs(t *testing.T) {
	f := func(prcs []ParsedRelabelConfig, labels []prompbmarshal.Label, isFinalize bool, resultExpected []prompbmarshal.Label) {
		t.Helper()
		result := ApplyRelabelConfigs(labels, 0, prcs, isFinalize)
		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected result; got\n%v\nwant\n%v", result, resultExpected)
		}
	}
	t.Run("empty_relabel_configs", func(t *testing.T) {
		f(nil, nil, false, nil)
		f(nil, nil, true, nil)
		f(nil, []prompbmarshal.Label{
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
		f(nil, []prompbmarshal.Label{
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
		f([]ParsedRelabelConfig{
			{
				Action:                       "replace",
				TargetLabel:                  "bar",
				Regex:                        defaultRegexForRelabelConfig,
				Replacement:                  "$1",
				hasCaptureGroupInReplacement: true,
			},
		}, nil, false, []prompbmarshal.Label{})
		f([]ParsedRelabelConfig{
			{
				Action:                       "replace",
				SourceLabels:                 []string{"foo"},
				TargetLabel:                  "bar",
				Regex:                        defaultRegexForRelabelConfig,
				Replacement:                  "$1",
				hasCaptureGroupInReplacement: true,
			},
		}, nil, false, []prompbmarshal.Label{})
		f([]ParsedRelabelConfig{
			{
				Action:                       "replace",
				SourceLabels:                 []string{"foo"},
				TargetLabel:                  "bar",
				Regex:                        defaultRegexForRelabelConfig,
				Replacement:                  "$1",
				hasCaptureGroupInReplacement: true,
			},
		}, []prompbmarshal.Label{
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
		f([]ParsedRelabelConfig{
			{
				Action:                       "replace",
				SourceLabels:                 []string{"foo"},
				TargetLabel:                  "bar",
				Regex:                        regexp.MustCompile(".+"),
				Replacement:                  "$1",
				hasCaptureGroupInReplacement: true,
			},
		}, []prompbmarshal.Label{
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
		f([]ParsedRelabelConfig{
			{
				Action:                       "replace",
				SourceLabels:                 []string{"xxx", "foo"},
				Separator:                    ";",
				TargetLabel:                  "bar",
				Regex:                        defaultRegexForRelabelConfig,
				Replacement:                  "a-$1-b",
				hasCaptureGroupInReplacement: true,
			},
		}, []prompbmarshal.Label{
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
	t.Run("replace-hit-target-label-with-capture-group", func(t *testing.T) {
		f([]ParsedRelabelConfig{
			{
				Action:                       "replace",
				SourceLabels:                 []string{"xxx", "foo"},
				Separator:                    ";",
				TargetLabel:                  "bar-$1",
				Regex:                        defaultRegexForRelabelConfig,
				Replacement:                  "a-$1-b",
				hasCaptureGroupInTargetLabel: true,
				hasCaptureGroupInReplacement: true,
			},
		}, []prompbmarshal.Label{
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
		f([]ParsedRelabelConfig{
			{
				Action:                       "replace_all",
				TargetLabel:                  "bar",
				Regex:                        defaultRegexForRelabelConfig,
				Replacement:                  "$1",
				hasCaptureGroupInReplacement: true,
			},
		}, nil, false, []prompbmarshal.Label{})
		f([]ParsedRelabelConfig{
			{
				Action:                       "replace_all",
				SourceLabels:                 []string{"foo"},
				TargetLabel:                  "bar",
				Regex:                        defaultRegexForRelabelConfig,
				Replacement:                  "$1",
				hasCaptureGroupInReplacement: true,
			},
		}, nil, false, []prompbmarshal.Label{})
		f([]ParsedRelabelConfig{
			{
				Action:                       "replace_all",
				SourceLabels:                 []string{"foo"},
				TargetLabel:                  "bar",
				Regex:                        defaultRegexForRelabelConfig,
				Replacement:                  "$1",
				hasCaptureGroupInReplacement: true,
			},
		}, []prompbmarshal.Label{
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
		f([]ParsedRelabelConfig{
			{
				Action:                       "replace_all",
				SourceLabels:                 []string{"foo"},
				TargetLabel:                  "bar",
				Regex:                        regexp.MustCompile(".+"),
				Replacement:                  "$1",
				hasCaptureGroupInReplacement: true,
			},
		}, []prompbmarshal.Label{
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
		f([]ParsedRelabelConfig{
			{
				Action:                       "replace_all",
				SourceLabels:                 []string{"xxx", "foo"},
				Separator:                    ";",
				TargetLabel:                  "xxx",
				Regex:                        regexp.MustCompile("(;)"),
				Replacement:                  "-$1-",
				hasCaptureGroupInReplacement: true,
			},
		}, []prompbmarshal.Label{
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
		f([]ParsedRelabelConfig{
			{
				Action:                       "replace",
				SourceLabels:                 []string{"xxx"},
				TargetLabel:                  "bar",
				Regex:                        defaultRegexForRelabelConfig,
				Replacement:                  "a-$1",
				hasCaptureGroupInReplacement: true,
			},
			{
				Action:       "replace",
				SourceLabels: []string{"bar"},
				TargetLabel:  "zar",
				Regex:        defaultRegexForRelabelConfig,
				Replacement:  "b-$1",
			},
		}, []prompbmarshal.Label{
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
		f([]ParsedRelabelConfig{
			{
				Action:                       "replace",
				SourceLabels:                 []string{"foo"},
				TargetLabel:                  "foo",
				Regex:                        defaultRegexForRelabelConfig,
				Replacement:                  "a-$1",
				hasCaptureGroupInReplacement: true,
			},
		}, []prompbmarshal.Label{
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
		f([]ParsedRelabelConfig{
			{
				Action:      "replace",
				TargetLabel: "foo",
				Regex:       defaultRegexForRelabelConfig,
				Replacement: "foobar",
			},
		}, []prompbmarshal.Label{}, true, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "foobar",
			},
		})
	})
	t.Run("keep_if_equal-miss", func(t *testing.T) {
		f([]ParsedRelabelConfig{
			{
				Action:       "keep_if_equal",
				SourceLabels: []string{"foo", "bar"},
			},
		}, nil, true, nil)
		f([]ParsedRelabelConfig{
			{
				Action:       "keep_if_equal",
				SourceLabels: []string{"xxx", "bar"},
			},
		}, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		}, true, []prompbmarshal.Label{})
	})
	t.Run("keep_if_equal-hit", func(t *testing.T) {
		f([]ParsedRelabelConfig{
			{
				Action:       "keep_if_equal",
				SourceLabels: []string{"xxx", "bar"},
			},
		}, []prompbmarshal.Label{
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
		f([]ParsedRelabelConfig{
			{
				Action:       "drop_if_equal",
				SourceLabels: []string{"foo", "bar"},
			},
		}, nil, true, nil)
		f([]ParsedRelabelConfig{
			{
				Action:       "drop_if_equal",
				SourceLabels: []string{"xxx", "bar"},
			},
		}, []prompbmarshal.Label{
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
		f([]ParsedRelabelConfig{
			{
				Action:       "drop_if_equal",
				SourceLabels: []string{"xxx", "bar"},
			},
		}, []prompbmarshal.Label{
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
		f([]ParsedRelabelConfig{
			{
				Action:       "keep",
				SourceLabels: []string{"foo"},
				Regex:        regexp.MustCompile(".+"),
			},
		}, nil, true, nil)
		f([]ParsedRelabelConfig{
			{
				Action:       "keep",
				SourceLabels: []string{"foo"},
				Regex:        regexp.MustCompile(".+"),
			},
		}, []prompbmarshal.Label{
			{
				Name:  "xxx",
				Value: "yyy",
			},
		}, true, []prompbmarshal.Label{})
	})
	t.Run("keep-hit", func(t *testing.T) {
		f([]ParsedRelabelConfig{
			{
				Action:       "keep",
				SourceLabels: []string{"foo"},
				Regex:        regexp.MustCompile(".+"),
			},
		}, []prompbmarshal.Label{
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
	t.Run("drop-miss", func(t *testing.T) {
		f([]ParsedRelabelConfig{
			{
				Action:       "drop",
				SourceLabels: []string{"foo"},
				Regex:        regexp.MustCompile(".+"),
			},
		}, nil, false, nil)
		f([]ParsedRelabelConfig{
			{
				Action:       "drop",
				SourceLabels: []string{"foo"},
				Regex:        regexp.MustCompile(".+"),
			},
		}, []prompbmarshal.Label{
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
		f([]ParsedRelabelConfig{
			{
				Action:       "drop",
				SourceLabels: []string{"foo"},
				Regex:        regexp.MustCompile(".+"),
			},
		}, []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "yyy",
			},
		}, true, []prompbmarshal.Label{})
	})
	t.Run("hashmod-miss", func(t *testing.T) {
		f([]ParsedRelabelConfig{
			{
				Action:       "hashmod",
				SourceLabels: []string{"foo"},
				TargetLabel:  "aaa",
				Modulus:      123,
			},
		}, []prompbmarshal.Label{
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
		f([]ParsedRelabelConfig{
			{
				Action:       "hashmod",
				SourceLabels: []string{"foo"},
				TargetLabel:  "aaa",
				Modulus:      123,
			},
		}, []prompbmarshal.Label{
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
	t.Run("labelmap", func(t *testing.T) {
		f([]ParsedRelabelConfig{
			{
				Action:      "labelmap",
				Regex:       regexp.MustCompile("foo(.+)"),
				Replacement: "$1-x",
			},
		}, []prompbmarshal.Label{
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
		})
	})
	t.Run("labelmap_all", func(t *testing.T) {
		f([]ParsedRelabelConfig{
			{
				Action:      "labelmap_all",
				Regex:       regexp.MustCompile(`\.`),
				Replacement: "-",
			},
		}, []prompbmarshal.Label{
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
	t.Run("labeldrop", func(t *testing.T) {
		f([]ParsedRelabelConfig{
			{
				Action: "labeldrop",
				Regex:  regexp.MustCompile("dropme.*"),
			},
		}, []prompbmarshal.Label{
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
		f([]ParsedRelabelConfig{
			{
				Action: "labeldrop",
				Regex:  regexp.MustCompile("dropme.*"),
			},
		}, []prompbmarshal.Label{
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
		f([]ParsedRelabelConfig{
			{
				Action: "labelkeep",
				Regex:  regexp.MustCompile("keepme.*"),
			},
		}, []prompbmarshal.Label{
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
		f([]ParsedRelabelConfig{
			{
				Action: "labelkeep",
				Regex:  regexp.MustCompile("keepme.*"),
			},
		}, []prompbmarshal.Label{
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
