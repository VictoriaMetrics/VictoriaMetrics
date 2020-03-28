package config

import (
	"testing"
)

func TestParseGood(t *testing.T) {
	if _, err := Parse([]string{"testdata/*good.rules", "testdata/dir/*good.*"}, true); err != nil {
		t.Errorf("error parsing files %s", err)
	}
}

func TestParseBad(t *testing.T) {
	if _, err := Parse([]string{"testdata/rules0-bad.rules"}, true); err == nil {
		t.Errorf("expected syntaxt error")
	}
	if _, err := Parse([]string{"testdata/dir/rules0-bad.rules"}, true); err == nil {
		t.Errorf("expected template annotation error")
	}
	if _, err := Parse([]string{"testdata/dir/rules1-bad.rules"}, true); err == nil {
		t.Errorf("expected same group error")
	}
	if _, err := Parse([]string{"testdata/*.yaml"}, true); err == nil {
		t.Errorf("expected empty group")
	}
}
