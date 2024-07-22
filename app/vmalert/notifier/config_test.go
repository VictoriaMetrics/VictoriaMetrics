package notifier

import (
	"strings"
	"testing"
)

func TestParseConfig_Success(t *testing.T) {
	f := func(path string) {
		t.Helper()

		_, err := parseConfig(path)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}
	f("testdata/mixed.good.yaml")
	f("testdata/consul.good.yaml")
	f("testdata/dns.good.yaml")
	f("testdata/static.good.yaml")
}

func TestParseConfig_Failure(t *testing.T) {
	f := func(path, expErr string) {
		t.Helper()

		_, err := parseConfig(path)
		if err == nil {
			t.Fatalf("expected to get non-nil err for config %q", path)
		}
		if !strings.Contains(err.Error(), expErr) {
			t.Fatalf("expected err to contain %q; got %q instead", expErr, err)
		}
	}

	f("testdata/unknownFields.bad.yaml", "unknown field")
	f("non-existing-file", "error reading")
}
