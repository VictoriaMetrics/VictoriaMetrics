package graphite

import (
	"reflect"
	"testing"
)

func TestGetRegexpForQuery(t *testing.T) {
	f := func(query string, delimiter byte, reExpected string) {
		t.Helper()
		re, err := getRegexpForQuery(query, delimiter)
		if err != nil {
			t.Fatalf("unexpected error in getRegexpForQuery(%q): %s", query, err)
		}
		reStr := re.String()
		if reStr != reExpected {
			t.Fatalf("unexpected regexp for query=%q, delimiter=%c; got %s; want %s", query, delimiter, reStr, reExpected)
		}
	}
	f("", '.', "")
	f("foobar", '.', "foobar")
	f("*", '.', `[^\.]*`)
	f("*", '_', `[^_]*`)
	f("foo.*.bar", '.', `foo\.[^\.]*\.bar`)
	f("fo*b{ar,aaa}[a-z]xx*.d", '.', `fo[^\.]*b(?:ar|aaa)[a-z]xx[^\.]*\.d`)
	f("fo*b{ar,aaa}[a-z]xx*_d", '_', `fo[^_]*b(?:ar|aaa)[a-z]xx[^_]*_d`)
}

func TestSortPaths(t *testing.T) {
	f := func(paths []string, delimiter string, pathsSortedExpected []string) {
		t.Helper()
		sortPaths(paths, delimiter)
		if !reflect.DeepEqual(paths, pathsSortedExpected) {
			t.Fatalf("unexpected sortPaths result;\ngot\n%q\nwant\n%q", paths, pathsSortedExpected)
		}
	}
	f([]string{"foo", "bar"}, ".", []string{"bar", "foo"})
	f([]string{"foo.", "bar", "aa", "ab."}, ".", []string{"ab.", "foo.", "aa", "bar"})
	f([]string{"foo.", "bar", "aa", "ab."}, "_", []string{"aa", "ab.", "bar", "foo."})
}

func TestFilterLeaves(t *testing.T) {
	f := func(paths []string, delimiter string, leavesExpected []string) {
		t.Helper()
		leaves := filterLeaves(paths, delimiter)
		if !reflect.DeepEqual(leaves, leavesExpected) {
			t.Fatalf("unexpected leaves; got\n%q\nwant\n%q", leaves, leavesExpected)
		}
	}
	f([]string{"foo", "bar"}, ".", []string{"foo", "bar"})
	f([]string{"a.", ".", "bc"}, ".", []string{"bc"})
	f([]string{"a.", ".", "bc"}, "_", []string{"a.", ".", "bc"})
	f([]string{"a_", "_", "bc"}, "_", []string{"bc"})
	f([]string{"foo.", "bar."}, ".", []string{})
}

func TestAddAutomaticVariants(t *testing.T) {
	f := func(query, delimiter, resultExpected string) {
		t.Helper()
		result := addAutomaticVariants(query, delimiter)
		if result != resultExpected {
			t.Fatalf("unexpected result for addAutomaticVariants(%q, delimiter=%q); got %q; want %q", query, delimiter, result, resultExpected)
		}
	}
	f("", ".", "")
	f("foobar", ".", "foobar")
	f("foo,bar.baz", ".", "{foo,bar}.baz")
	f("foo,bar.baz", "_", "{foo,bar.baz}")
	f("foo,bar_baz*", "_", "{foo,bar}_baz*")
	f("foo.bar,baz,aa.bb,cc", ".", "foo.{bar,baz,aa}.{bb,cc}")
}
