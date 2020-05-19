package flagutil

import (
	"flag"
	"os"
	"reflect"
	"testing"
)

var fooFlag Array

func init() {
	os.Args = append(os.Args, "--fooFlag=foo", "--fooFlag=bar")
	flag.Var(&fooFlag, "fooFlag", "test")
}

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func TestArray(t *testing.T) {
	expected := map[string]struct{}{
		"foo": {},
		"bar": {},
	}
	if len(expected) != len(fooFlag) {
		t.Errorf("len array flag (%d) is not equal to %d", len(fooFlag), len(expected))
	}
	for _, i := range fooFlag {
		if _, ok := expected[i]; !ok {
			t.Errorf("unexpected item in array %v", i)
		}
	}
}

func TestArraySet(t *testing.T) {
	f := func(s string, expectedValues []string) {
		t.Helper()
		var a Array
		_ = a.Set(s)
		if !reflect.DeepEqual([]string(a), expectedValues) {
			t.Fatalf("unexpected values parsed;\ngot\n%q\nwant\n%q", a, expectedValues)
		}
	}
	f("", nil)
	f(`foo`, []string{`foo`})
	f(`foo,b ar,baz`, []string{`foo`, `b ar`, `baz`})
	f(`foo,b\"'ar,"baz,d`, []string{`foo`, `b\"'ar`, `"baz,d`})
	f(`,foo,,ba"r,`, []string{``, `foo`, ``, `ba"r`, ``})
	f(`""`, []string{``})
	f(`"foo,b\nar"`, []string{`foo,b` + "\n" + `ar`})
	f(`"foo","bar",baz`, []string{`foo`, `bar`, `baz`})
	f(`,fo,"\"b, a'\\",,r,`, []string{``, `fo`, `"b, a'\`, ``, `r`, ``})
}

func TestArrayGetOptionalArg(t *testing.T) {
	f := func(s string, argIdx int, expectedValue string) {
		t.Helper()
		var a Array
		_ = a.Set(s)
		v := a.GetOptionalArg(argIdx)
		if v != expectedValue {
			t.Fatalf("unexpected value; got %q; want %q", v, expectedValue)
		}
	}
	f("", 0, "")
	f("", 1, "")
	f("foo", 0, "foo")
	f("foo", 23, "foo")
	f("foo,bar", 0, "foo")
	f("foo,bar", 1, "bar")
	f("foo,bar", 2, "")
}

func TestArrayString(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var a Array
		_ = a.Set(s)
		result := a.String()
		if result != s {
			t.Fatalf("unexpected string;\ngot\n%s\nwant\n%s", result, s)
		}
	}
	f("")
	f("foo")
	f("foo,bar")
	f(",")
	f(",foo,")
	f(`", foo","b\"ar",`)
	f(`,"\nfoo\\",bar`)
}
