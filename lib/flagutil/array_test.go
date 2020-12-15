package flagutil

import (
	"flag"
	"os"
	"reflect"
	"testing"
	"time"
)

var (
	fooFlag         Array
	fooFlagDuration ArrayDuration
	fooFlagBool     ArrayBool
)

func init() {
	os.Args = append(os.Args, "--fooFlag=foo", "--fooFlag=bar", "--fooFlagDuration=10s", "--fooFlagDuration=5m")
	os.Args = append(os.Args, "--fooFlagBool=true", "--fooFlagBool=false")
	flag.Var(&fooFlag, "fooFlag", "test")
	flag.Var(&fooFlagDuration, "fooFlagDuration", "test")
	flag.Var(&fooFlagBool, "fooFlagBool", "test")
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

func TestArrayDuration(t *testing.T) {
	expected := map[time.Duration]struct{}{
		time.Second * 10: {},
		time.Minute * 5:  {},
	}
	if len(expected) != len(fooFlagDuration) {
		t.Errorf("len array flag (%d) is not equal to %d", len(fooFlag), len(expected))
	}
	for _, i := range fooFlagDuration {
		if _, ok := expected[i]; !ok {
			t.Errorf("unexpected item in array %v", i)
		}
	}
}

func TestArrayDurationSet(t *testing.T) {
	f := func(s string, expectedValues []time.Duration) {
		t.Helper()
		var a ArrayDuration
		_ = a.Set(s)
		if !reflect.DeepEqual([]time.Duration(a), expectedValues) {
			t.Fatalf("unexpected values parsed;\ngot\n%q\nwant\n%q", a, expectedValues)
		}
	}
	f("", nil)
	f(`1m`, []time.Duration{time.Minute})
	f(`5m,1s,1h`, []time.Duration{time.Minute * 5, time.Second, time.Hour})
}

func TestArrayDurationGetOptionalArg(t *testing.T) {
	f := func(s string, argIdx int, expectedValue time.Duration, defaultValue time.Duration) {
		t.Helper()
		var a ArrayDuration
		_ = a.Set(s)
		v := a.GetOptionalArgOrDefault(argIdx, defaultValue)
		if v != expectedValue {
			t.Fatalf("unexpected value; got %q; want %q", v, expectedValue)
		}
	}
	f("", 0, time.Second, time.Second)
	f("", 1, time.Minute, time.Minute)
	f("10s,1m", 1, time.Minute, time.Minute)
	f("10s", 3, time.Second*10, time.Minute)
}

func TestArrayDurationString(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var a ArrayDuration
		_ = a.Set(s)
		result := a.String()
		if result != s {
			t.Fatalf("unexpected string;\ngot\n%s\nwant\n%s", result, s)
		}
	}
	f("")
	f("10s,1m0s")
	f("5m0s,1s")
}

func TestArrayBool(t *testing.T) {
	expected := map[bool]struct{}{
		true:  {},
		false: {},
	}
	if len(expected) != len(fooFlagBool) {
		t.Errorf("len array flag (%d) is not equal to %d", len(fooFlag), len(expected))
	}
	for _, i := range fooFlagBool {
		if _, ok := expected[i]; !ok {
			t.Errorf("unexpected item in array %v", i)
		}
	}
}

func TestArrayBoolSet(t *testing.T) {
	f := func(s string, expectedValues []bool) {
		t.Helper()
		var a ArrayBool
		_ = a.Set(s)
		if !reflect.DeepEqual([]bool(a), expectedValues) {
			t.Fatalf("unexpected values parsed;\ngot\n%v\nwant\n%v", a, expectedValues)
		}
	}
	f("", nil)
	f(`true`, []bool{true})
	f(`false,True,False`, []bool{false, true, false})
}

func TestArrayBoolGetOptionalArg(t *testing.T) {
	f := func(s string, argIdx int, expectedValue bool) {
		t.Helper()
		var a ArrayBool
		_ = a.Set(s)
		v := a.GetOptionalArg(argIdx)
		if v != expectedValue {
			t.Fatalf("unexpected value; got %v; want %v", v, expectedValue)
		}
	}
	f("", 0, false)
	f("", 1, false)
	f("true,true,false", 1, true)
	f("true", 2, true)
}

func TestArrayBoolString(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var a ArrayBool
		_ = a.Set(s)
		result := a.String()
		if result != s {
			t.Fatalf("unexpected string;\ngot\n%s\nwant\n%s", result, s)
		}
	}
	f("")
	f("true")
	f("true,false")
	f("false,true")
}
