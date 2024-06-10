package graphite

import (
	"reflect"
	"testing"
)

func TestUnmarshalTags(t *testing.T) {
	f := func(s string, tagsExpected map[string]string) {
		t.Helper()
		tags := unmarshalTags(s)
		if !reflect.DeepEqual(tags, tagsExpected) {
			t.Fatalf("unexpected tags unmarshaled for s=%q\ngot\n%s\nwant\n%s", s, tags, tagsExpected)
		}
	}
	f("", map[string]string{})
	f("foo.bar", map[string]string{
		"name": "foo.bar",
	})
	f("foo;bar=baz", map[string]string{
		"name": "foo",
		"bar":  "baz",
	})
	f("foo.bar;bar;x=aa;baz=aaa;x=y", map[string]string{
		"name": "foo.bar",
		"baz":  "aaa",
		"x":    "y",
	})
}

func TestMarshalTags(t *testing.T) {
	f := func(s, sExpected string) {
		t.Helper()
		tags := unmarshalTags(s)
		sMarshaled := marshalTags(tags)
		if sMarshaled != sExpected {
			t.Fatalf("unexpected marshaled tags for s=%q\ngot\n%s\nwant\n%s", s, sMarshaled, sExpected)
		}
	}
	f("", "")
	f("foo", "foo")
	f("foo;bar=baz", "foo;bar=baz")
	f("foo.bar;baz;xx=yy;a=b", "foo.bar;a=b;xx=yy")
	f("foo.bar;a=bb;a=ccc;d=a.b.c", "foo.bar;a=ccc;d=a.b.c")
}

func TestGetPathFromName(t *testing.T) {
	f := func(name, pathExpected string) {
		t.Helper()
		path := getPathFromName(name)
		if path != pathExpected {
			t.Fatalf("unexpected path extracted from name %q; got %q; want %q", name, path, pathExpected)
		}
	}
	f("", "")
	f("foo", "foo")
	f("foo.bar", "foo.bar")
	f("foo.bar,baz.aa", "foo.bar,baz.aa")
	f("foo(bar.baz,aa.bb)", "bar.baz")
	f("foo(1, 'foo', aaa )", "aaa")
	f("foo|bar(baz)", "foo")
	f("a(b(c.d.e))", "c.d.e")
	f("foo()", "foo()")
	f("123", "123")
	f("foo(123)", "123")
	f("fo(bar", "fo(bar")
}

func TestGraphiteToGolangRegexpReplace(t *testing.T) {
	f := func(s, replaceExpected string) {
		t.Helper()
		replace := graphiteToGolangRegexpReplace(s)
		if replace != replaceExpected {
			t.Fatalf("unexpected result for graphiteToGolangRegexpReplace(%q); got %q; want %q", s, replace, replaceExpected)
		}
	}
	f("", "")
	f("foo", "foo")
	f(`a\d+`, `a\d+`)
	f(`\1f\\oo\2`, `$1f\\oo$2`)
}

func TestGetAbsoluteNodeIndex(t *testing.T) {
	f := func(index, size, expectedIndex int) {
		t.Helper()
		absoluteIndex := getAbsoluteNodeIndex(index, size)
		if absoluteIndex != expectedIndex {
			t.Fatalf("unexpected result for getAbsoluteNodeIndex(%d, %d); got %d; want %d", index, size, expectedIndex, absoluteIndex)
		}
	}
	f(1, 1, -1)
	f(0, 1, 0)
	f(-1, 3, 2)
	f(-3, 1, -1)
	f(-1, 1, 0)
	f(-2, 1, -1)
	f(3, 2, -1)
	f(2, 2, -1)
	f(1, 2, 1)
	f(0, 2, 0)
	f(-1, 2, 1)
	f(-2, 2, 0)
	f(-3, 2, -1)
	f(-5, 2, -1)
	f(-1, 100, 99)
	f(-99, 100, 1)
	f(-100, 100, 0)
	f(-101, 100, -1)
}
