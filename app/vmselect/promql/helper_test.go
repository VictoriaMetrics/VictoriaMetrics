package promql

import (
	"testing"
)

func TestParseMetricSelectorSuccess(t *testing.T) {
	f := func(s string) {
		t.Helper()
		tfs, err := ParseMetricSelector(s)
		if err != nil {
			t.Fatalf("unexpected error when parsing %q: %s", s, err)
		}
		if tfs == nil {
			t.Fatalf("expecting non-nil tfs when parsing %q", s)
		}
	}
	f("foo")
	f(":foo")
	f("  :fo:bar.baz")
	f(`a{}`)
	f(`{foo="bar"}`)
	f(`{:f:oo=~"bar.+"}`)
	f(`foo {bar != "baz"}`)
	f(` foo { bar !~ "^ddd(x+)$", a="ss", __name__="sffd"}  `)
	f(`(foo)`)
}

func TestParseMetricSelectorError(t *testing.T) {
	f := func(s string) {
		t.Helper()
		tfs, err := ParseMetricSelector(s)
		if err == nil {
			t.Fatalf("expecting non-nil error when parsing %q", s)
		}
		if tfs != nil {
			t.Fatalf("expecting nil tfs when parsing %q", s)
		}
	}
	f("")
	f(`{}`)
	f(`foo bar`)
	f(`foo+bar`)
	f(`sum(bar)`)
	f(`x{y}`)
	f(`x{y+z}`)
	f(`foo[5m]`)
	f(`foo offset 5m`)
}
