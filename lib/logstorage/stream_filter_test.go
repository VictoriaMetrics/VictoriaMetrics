package logstorage

import (
	"fmt"
	"testing"
)

func TestNewTestStreamFilterSuccess(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()
		sf, err := newTestStreamFilter(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := sf.String()
		if result != resultExpected {
			t.Fatalf("unexpected StreamFilter; got %s; want %s", result, resultExpected)
		}
	}

	f("{}", "{}")
	f(`{foo="bar"}`, `{foo="bar"}`)
	f(`{ "foo" =~ "bar.+" , baz!="a" or x="y"}`, `{foo=~"bar.+",baz!="a" or x="y"}`)
	f(`{"a b"='c}"d' OR de="aaa"}`, `{"a b"="c}\"d" or de="aaa"}`)
	f(`{a="b", c="d" or x="y"}`, `{a="b",c="d" or x="y"}`)
}

func TestNewTestStreamFilterFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		sf, err := newTestStreamFilter(s)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if sf != nil {
			t.Fatalf("expecting nil sf; got %v", sf)
		}
	}

	f("")
	f("}")
	f("{")
	f("{foo")
	f("{foo}")
	f("{'foo")
	f("{foo=")
	f("{foo or bar}")
	f("{foo=bar")
	f("{foo=bar baz}")
	f("{foo='bar' baz='x'}")
}

func mustNewTestStreamFilter(s string) *StreamFilter {
	sf, err := newTestStreamFilter(s)
	if err != nil {
		panic(fmt.Errorf("unexpected error in newTestStreamFilter(%q): %w", s, err))
	}
	return sf
}

func newTestStreamFilter(s string) (*StreamFilter, error) {
	lex := newLexer(s)
	fs, err := parseFilterStream(lex)
	if err != nil {
		return nil, err
	}
	return fs.f, nil
}
