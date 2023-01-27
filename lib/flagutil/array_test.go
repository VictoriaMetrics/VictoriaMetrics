package flagutil

import (
	"flag"
	"os"
	"reflect"
	"testing"
	"time"
)

var (
	fooFlagString   ArrayString
	fooFlagDuration ArrayDuration
	fooFlagBool     ArrayBool
	fooFlagInt      ArrayInt
	fooFlagBytes    ArrayBytes
)

func init() {
	os.Args = append(os.Args, "-fooFlagString=foo", "-fooFlagString=bar")
	os.Args = append(os.Args, "-fooFlagDuration=10s", "-fooFlagDuration=5m")
	os.Args = append(os.Args, "-fooFlagBool=true", "-fooFlagBool=false,true", "-fooFlagBool")
	os.Args = append(os.Args, "-fooFlagInt=1", "-fooFlagInt=2,3")
	os.Args = append(os.Args, "-fooFlagBytes=10MB", "-fooFlagBytes=23,10kib")
	flag.Var(&fooFlagString, "fooFlagString", "test")
	flag.Var(&fooFlagDuration, "fooFlagDuration", "test")
	flag.Var(&fooFlagBool, "fooFlagBool", "test")
	flag.Var(&fooFlagInt, "fooFlagInt", "test")
	flag.Var(&fooFlagBytes, "fooFlagBytes", "test")
}

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func TestArrayString(t *testing.T) {
	expected := ArrayString{
		"foo",
		"bar",
	}
	if !reflect.DeepEqual(expected, fooFlagString) {
		t.Fatalf("unexpected flag values; got\n%q\nwant\n%q", fooFlagString, expected)
	}
}

func TestArrayString_Set(t *testing.T) {
	f := func(s string, expectedValues []string) {
		t.Helper()
		var a ArrayString
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

func TestArrayString_GetOptionalArg(t *testing.T) {
	f := func(s string, argIdx int, expectedValue string) {
		t.Helper()
		var a ArrayString
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

func TestArrayString_String(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var a ArrayString
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
	expected := ArrayDuration{
		time.Second * 10,
		time.Minute * 5,
	}
	if !reflect.DeepEqual(expected, fooFlagDuration) {
		t.Fatalf("unexpected flag values; got\n%s\nwant\n%s", fooFlagDuration, expected)
	}
}

func TestArrayDuration_Set(t *testing.T) {
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

func TestArrayDuration_GetOptionalArg(t *testing.T) {
	f := func(s string, argIdx int, defaultValue, expectedValue time.Duration) {
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
	f("10s", 3, time.Minute, time.Second*10)
}

func TestArrayDuration_String(t *testing.T) {
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
	expected := ArrayBool{
		true, false, true, true,
	}
	if !reflect.DeepEqual(expected, fooFlagBool) {
		t.Fatalf("unexpected flag values; got\n%v\nwant\n%v", fooFlagBool, expected)
	}
}

func TestArrayBool_Set(t *testing.T) {
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

func TestArrayBool_GetOptionalArg(t *testing.T) {
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

func TestArrayBool_String(t *testing.T) {
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

func TestArrayInt(t *testing.T) {
	expected := ArrayInt{1, 2, 3}
	if !reflect.DeepEqual(expected, fooFlagInt) {
		t.Fatalf("unexpected flag values; got\n%d\nwant\n%d", fooFlagInt, expected)
	}
}

func TestArrayInt_Set(t *testing.T) {
	f := func(s string, expectedValues []int) {
		t.Helper()
		var a ArrayInt
		_ = a.Set(s)
		if !reflect.DeepEqual([]int(a), expectedValues) {
			t.Fatalf("unexpected values parsed;\ngot\n%q\nwant\n%q", a, expectedValues)
		}
	}
	f("", nil)
	f(`1`, []int{1})
	f(`-2,3,-64`, []int{-2, 3, -64})
}

func TestArrayInt_GetOptionalArg(t *testing.T) {
	f := func(s string, argIdx, defaultValue, expectedValue int) {
		t.Helper()
		var a ArrayInt
		_ = a.Set(s)
		v := a.GetOptionalArgOrDefault(argIdx, defaultValue)
		if v != expectedValue {
			t.Fatalf("unexpected value; got %d; want %d", v, expectedValue)
		}
	}
	f("", 0, 123, 123)
	f("", 1, -34, -34)
	f("10,1", 1, 234, 1)
	f("10", 3, -34, 10)
}

func TestArrayInt_String(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var a ArrayInt
		_ = a.Set(s)
		result := a.String()
		if result != s {
			t.Fatalf("unexpected string;\ngot\n%s\nwant\n%s", result, s)
		}
	}
	f("")
	f("10,1")
	f("-5,1,123")
}

func TestArrayBytes(t *testing.T) {
	expected := []int64{10000000, 23, 10240}
	result := make([]int64, len(fooFlagBytes))
	for i, b := range fooFlagBytes {
		result[i] = b.N
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("unexpected flag values; got\n%d\nwant\n%d", result, expected)
	}
}

func TestArrayBytes_Set(t *testing.T) {
	f := func(s string, expectedValues []int64) {
		t.Helper()
		var a ArrayBytes
		_ = a.Set(s)
		values := make([]int64, len(a))
		for i, v := range a {
			values[i] = v.N
		}
		if !reflect.DeepEqual(values, expectedValues) {
			t.Fatalf("unexpected values parsed;\ngot\n%d\nwant\n%d", values, expectedValues)
		}
	}
	f("", []int64{})
	f(`1`, []int64{1})
	f(`-2,3,10kb`, []int64{-2, 3, 10000})
}

func TestArrayBytes_GetOptionalArg(t *testing.T) {
	f := func(s string, argIdx int, defaultValue, expectedValue int64) {
		t.Helper()
		var a ArrayBytes
		_ = a.Set(s)
		v := a.GetOptionalArgOrDefault(argIdx, defaultValue)
		if v != expectedValue {
			t.Fatalf("unexpected value; got %d; want %d", v, expectedValue)
		}
	}
	f("", 0, 123, 123)
	f("", 1, -34, -34)
	f("10,1", 1, 234, 1)
	f("10,1", 3, 234, 234)
	f("10Kb", 3, -34, 10000)
}

func TestArrayBytes_String(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var a ArrayBytes
		_ = a.Set(s)
		result := a.String()
		if result != s {
			t.Fatalf("unexpected string;\ngot\n%s\nwant\n%s", result, s)
		}
	}
	f("")
	f("10.5KiB,1")
	f("-5,1,123MB")
}
