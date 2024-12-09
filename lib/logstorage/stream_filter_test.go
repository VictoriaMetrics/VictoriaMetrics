package logstorage

import (
	"fmt"
	"testing"
)

func TestStreamFilterMatchStreamName(t *testing.T) {
	f := func(filter, streamName string, resultExpected bool) {
		t.Helper()
		sf := mustNewTestStreamFilter(filter)
		result := sf.matchStreamName(streamName)
		if result != resultExpected {
			t.Fatalf("unexpected result for matching %s against %s; got %v; want %v", streamName, sf, result, resultExpected)
		}
	}

	// Empty filter matches anything
	f(`{}`, `{}`, true)
	f(`{}`, `{foo="bar"}`, true)
	f(`{}`, `{foo="bar",a="b",c="d"}`, true)

	// empty '=' filter
	f(`{foo=""}`, `{}`, true)
	f(`{foo=""}`, `{foo="bar"}`, false)
	f(`{foo=""}`, `{a="b",c="d"}`, true)

	// non-empty '=' filter
	f(`{foo="bar"}`, `{}`, false)
	f(`{foo="bar"}`, `{foo="bar"}`, true)
	f(`{foo="bar"}`, `{foo="barbaz"}`, false)
	f(`{foo="bar"}`, `{foo="bazbar"}`, false)
	f(`{foo="bar"}`, `{a="b",foo="bar"}`, true)
	f(`{foo="bar"}`, `{foo="bar",a="b"}`, true)
	f(`{foo="bar"}`, `{a="b",foo="bar",c="d"}`, true)
	f(`{foo="bar"}`, `{foo="baz"}`, false)
	f(`{foo="bar"}`, `{foo="baz",a="b"}`, false)
	f(`{foo="bar"}`, `{a="b",foo="baz"}`, false)
	f(`{foo="bar"}`, `{a="b",foo="baz",b="c"}`, false)
	f(`{foo="bar"}`, `{zoo="bar"}`, false)
	f(`{foo="bar"}`, `{a="b",zoo="bar"}`, false)

	// empty '!=' filter
	f(`{foo!=""}`, `{}`, false)
	f(`{foo!=""}`, `{foo="bar"}`, true)
	f(`{foo!=""}`, `{a="b",c="d"}`, false)

	// non-empty '!=' filter
	f(`{foo!="bar"}`, `{}`, true)
	f(`{foo!="bar"}`, `{foo="bar"}`, false)
	f(`{foo!="bar"}`, `{foo="barbaz"}`, true)
	f(`{foo!="bar"}`, `{foo="bazbar"}`, true)
	f(`{foo!="bar"}`, `{a="b",foo="bar"}`, false)
	f(`{foo!="bar"}`, `{foo="bar",a="b"}`, false)
	f(`{foo!="bar"}`, `{a="b",foo="bar",c="d"}`, false)
	f(`{foo!="bar"}`, `{foo="baz"}`, true)
	f(`{foo!="bar"}`, `{foo="baz",a="b"}`, true)
	f(`{foo!="bar"}`, `{a="b",foo="baz"}`, true)
	f(`{foo!="bar"}`, `{a="b",foo="baz",b="c"}`, true)
	f(`{foo!="bar"}`, `{zoo="bar"}`, true)
	f(`{foo!="bar"}`, `{a="b",zoo="bar"}`, true)

	// empty '=~' filter
	f(`{foo=~""}`, `{}`, true)
	f(`{foo=~""}`, `{foo="bar"}`, false)
	f(`{foo=~""}`, `{a="b",c="d"}`, true)
	f(`{foo=~".*"}`, `{}`, true)
	f(`{foo=~".*"}`, `{foo="bar"}`, true)
	f(`{foo=~".*"}`, `{a="b",c="d"}`, true)

	// non-empty '=~` filter

	f(`{foo=~".+"}`, `{}`, false)
	f(`{foo=~".+"}`, `{foo="bar"}`, true)
	f(`{foo=~".+"}`, `{a="b",c="d"}`, false)

	f(`{foo=~"bar"}`, `{foo="bar"}`, true)
	f(`{foo=~"bar"}`, `{foo="barbaz"}`, false)
	f(`{foo=~"bar"}`, `{foo="bazbar"}`, false)
	f(`{foo=~"bar"}`, `{a="b",foo="bar"}`, true)
	f(`{foo=~"bar"}`, `{foo="bar",a="b"}`, true)
	f(`{foo=~"bar"}`, `{a="b",foo="bar",b="c"}`, true)
	f(`{foo=~"bar"}`, `{foo="baz"}`, false)
	f(`{foo=~"bar"}`, `{foo="baz",a="b"}`, false)
	f(`{foo=~"bar"}`, `{zoo="bar"}`, false)
	f(`{foo=~"bar"}`, `{a="b",zoo="bar"}`, false)

	f(`{foo=~".*a.+"}`, `{foo="bar"}`, true)
	f(`{foo=~".*a.+"}`, `{foo="barboz"}`, true)
	f(`{foo=~".*a.+"}`, `{foo="bazbor"}`, true)
	f(`{foo=~".*a.+"}`, `{a="b",foo="bar"}`, true)
	f(`{foo=~".*a.+"}`, `{foo="bar",a="b"}`, true)
	f(`{foo=~".*a.+"}`, `{a="b",foo="bar",b="c"}`, true)
	f(`{foo=~".*a.+"}`, `{foo="boz"}`, false)
	f(`{foo=~".*a.+"}`, `{foo="boz",a="b"}`, false)
	f(`{foo=~".*a.+"}`, `{zoo="bar"}`, false)
	f(`{foo=~".*a.+"}`, `{a="b",zoo="bar"}`, false)

	// empty '!~' filter
	f(`{foo!~""}`, `{}`, false)
	f(`{foo!~""}`, `{foo="bar"}`, true)
	f(`{foo!~""}`, `{a="b",c="d"}`, false)
	f(`{foo!~".*"}`, `{}`, false)
	f(`{foo!~".*"}`, `{foo="bar"}`, false)
	f(`{foo!~".*"}`, `{a="b",c="d"}`, false)

	f(`{foo!~"bar"}`, `{foo="bar"}`, false)
	f(`{foo!~"bar"}`, `{foo="barbaz"}`, true)
	f(`{foo!~"bar"}`, `{foo="bazbar"}`, true)
	f(`{foo!~"bar"}`, `{a="b",foo="bar"}`, false)
	f(`{foo!~"bar"}`, `{foo="bar",a="b"}`, false)
	f(`{foo!~"bar"}`, `{a="b",foo="bar",b="c"}`, false)
	f(`{foo!~"bar"}`, `{foo="baz"}`, true)
	f(`{foo!~"bar"}`, `{foo="baz",a="b"}`, true)
	f(`{foo!~"bar"}`, `{zoo="bar"}`, true)
	f(`{foo!~"bar"}`, `{a="b",zoo="bar"}`, true)

	f(`{foo!~".*a.+"}`, `{foo="bar"}`, false)
	f(`{foo!~".*a.+"}`, `{foo="barboz"}`, false)
	f(`{foo!~".*a.+"}`, `{foo="bazbor"}`, false)
	f(`{foo!~".*a.+"}`, `{a="b",foo="bar"}`, false)
	f(`{foo!~".*a.+"}`, `{foo="bar",a="b"}`, false)
	f(`{foo!~".*a.+"}`, `{a="b",foo="bar",b="c"}`, false)
	f(`{foo!~".*a.+"}`, `{foo="boz"}`, true)
	f(`{foo!~".*a.+"}`, `{foo="boz",a="b"}`, true)
	f(`{foo!~".*a.+"}`, `{zoo="bar"}`, true)
	f(`{foo!~".*a.+"}`, `{a="b",zoo="bar"}`, true)

	// multiple 'and' filters
	f(`{a="b",b="c"}`, `{a="b"}`, false)
	f(`{a="b",b="c"}`, `{b="c",a="b"}`, true)
	f(`{a="b",b="c"}`, `{x="y",b="c",a="b",d="e"}`, true)
	f(`{a=~"foo.+",a!~".+bar"}`, `{a="foobar"}`, false)
	f(`{a=~"foo.+",a!~".+bar"}`, `{a="foozar"}`, true)

	// multple `or` filters
	f(`{a="b" or b="c"}`, `{x="y"}`, false)
	f(`{a="b" or b="c"}`, `{x="y",b="c"}`, true)
	f(`{a="b" or b="c"}`, `{a="b",x="y",b="c"}`, true)
	f(`{a="b",b="c" or a=~"foo.+"}`, `{}`, false)
	f(`{a="b",b="c" or a=~"foo.+"}`, `{x="y",a="foobar"}`, true)
	f(`{a="b",b="c" or a=~"foo.+"}`, `{x="y",a="b"}`, false)
	f(`{a="b",b="c" or a=~"foo.+"}`, `{x="y",b="c",a="b"}`, true)
	f(`{a="b" or c=""}`, `{}`, true)
	f(`{a="b" or c=""}`, `{c="x"}`, false)
	f(`{a="b" or c=""}`, `{a="b"}`, true)
}

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
	f(`{a-q:w.z="b", c="d" or 'x a'=y-z=q}`, `{"a-q:w.z"="b",c="d" or "x a"="y-z=q"}`)
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
