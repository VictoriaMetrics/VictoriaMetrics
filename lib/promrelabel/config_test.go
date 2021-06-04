package promrelabel

import (
	"reflect"
	"testing"
)

func TestLoadRelabelConfigsSuccess(t *testing.T) {
	path := "testdata/relabel_configs_valid.yml"
	pcs, err := LoadRelabelConfigs(path, false)
	if err != nil {
		t.Fatalf("cannot load relabel configs from %q: %s", path, err)
	}
	if n := pcs.Len(); n != 9 {
		t.Fatalf("unexpected number of relabel configs loaded from %q; got %d; want %d", path, n, 9)
	}
}

func TestLoadRelabelConfigsFailure(t *testing.T) {
	f := func(path string) {
		t.Helper()
		rcs, err := LoadRelabelConfigs(path, false)
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

func TestParseRelabelConfigsSuccess(t *testing.T) {
	f := func(rcs []RelabelConfig, pcsExpected *ParsedConfigs) {
		t.Helper()
		pcs, err := ParseRelabelConfigs(rcs, false)
		if err != nil {
			t.Fatalf("unexected error: %s", err)
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
				SourceLabels: []string{"foo", "bar"},
				Separator:    ";",
				TargetLabel:  "xxx",
				Regex:        defaultRegexForRelabelConfig,
				Replacement:  "$1",
				Action:       "replace",

				regexOriginal:                defaultOriginalRegexForRelabelConfig,
				hasCaptureGroupInReplacement: true,
			},
		},
	})
}

func TestParseRelabelConfigsFailure(t *testing.T) {
	f := func(rcs []RelabelConfig) {
		t.Helper()
		pcs, err := ParseRelabelConfigs(rcs, false)
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
				Regex:        strPtr("foo[bar"),
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
}

func strPtr(s string) *string {
	return &s
}
