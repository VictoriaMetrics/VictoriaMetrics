package notifier

import (
	"strings"
	"testing"
)

func TestConfigParseGood(t *testing.T) {
	f := func(path string) {
		_, err := parseConfig(path)
		checkErr(t, err)
	}
	f("testdata/mixed.good.yaml")
	f("testdata/consul.good.yaml")
	f("testdata/dns.good.yaml")
	f("testdata/static.good.yaml")
}

func TestConfigParseBad(t *testing.T) {
	f := func(path, expErr string) {
		_, err := parseConfig(path)
		if err == nil {
			t.Fatalf("expected to get non-nil err for config %q", path)
		}
		if !strings.Contains(err.Error(), expErr) {
			t.Errorf("expected err to contain %q; got %q instead", expErr, err)
		}
	}

	f("testdata/unknownFields.bad.yaml", "unknown field")
	f("non-existing-file", "error reading")
}
