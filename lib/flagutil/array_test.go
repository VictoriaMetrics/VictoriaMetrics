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
	f := func(s, expectedResult string) {
		t.Helper()
		var a ArrayString
		if err := a.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := a.String()
		if result != expectedResult {
			t.Fatalf("unexpected values parsed;\ngot\n%s\nwant\n%s", result, expectedResult)
		}
	}
	// Zero args
	f("", "")

	// Single arg
	f(`foo`, `foo`)
	f(`fo"o`, `"fo\"o"`)
	f(`fo'o`, `"fo'o"`)
	f(`fo{o`, `"fo{o"`)
	f(`fo[o`, `"fo[o"`)
	f(`fo(o`, `"fo(o"`)

	// Single arg with Prometheus label filters
	f(`foo{bar="baz",x="y"}`, `"foo{bar=\"baz\",x=\"y\"}"`)
	f(`foo{bar="ba}z",x="y"}`, `"foo{bar=\"ba}z\",x=\"y\"}"`)
	f(`foo{bar='baz',x="y"}`, `"foo{bar='baz',x=\"y\"}"`)
	f(`foo{bar='baz',x='y'}`, `"foo{bar='baz',x='y'}"`)
	f(`foo{bar='ba}z',x='y'}`, `"foo{bar='ba}z',x='y'}"`)
	f(`{foo="ba[r",baz='a'}`, `"{foo=\"ba[r\",baz='a'}"`)

	// Single arg with JSON
	f(`[1,2,3]`, `"[1,2,3]"`)
	f(`{"foo":"ba,r",baz:x}`, `"{\"foo\":\"ba,r\",baz:x}"`)

	// Single quoted arg
	f(`"foo"`, `foo`)
	f(`"fo,'o"`, `"fo,'o"`)
	f(`"f\\o,\'\"o"`, `"f\\o,\\'\"o"`)
	f(`"foo{bar='baz',x='y'}"`, `"foo{bar='baz',x='y'}"`)
	f(`'foo'`, `foo`)
	f(`'fo,"o'`, `"fo,\"o"`)
	f(`'f\\o,\'\"o'`, `"f\\o,'\\\"o"`)
	f(`'foo{bar="baz",x="y"}'`, `"foo{bar=\"baz\",x=\"y\"}"`)

	// Multiple args
	f(`foo,bar,baz`, `foo,bar,baz`)
	f(`"foo",'bar',{[(ba'",z"`, `foo,bar,"{[(ba'\",z\""`)
	f(`foo,b"'ar,"baz,d`, `foo,"b\"'ar,\"baz",d`)
	f(`{foo="b,ar"},baz{x="y",z="d"}`, `"{foo=\"b,ar\"}","baz{x=\"y\",z=\"d\"}"`)

	// Empty args
	f(`""`, ``)
	f(`''`, ``)
	f(`,`, `,`)
	f(`,foo,,ba"r,`, `,foo,,"ba\"r",`)

	// Special chars inside double quotes
	f(`"foo,b\nar"`, `"foo,b\nar"`)
	f(`"foo\x23bar"`, "foo\x23bar")
}

func TestArrayString_GetOptionalArg(t *testing.T) {
	f := func(s string, argIdx int, expectedValue string) {
		t.Helper()
		var a ArrayString
		if err := a.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
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
		if err := a.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
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
	f(`"foo{bar=~\"baz\",a!=\"b\"}","{a='b,{[(c'}"`)
}

func TestArrayDuration(t *testing.T) {
	expected := ArrayDuration{
		a: []time.Duration{
			time.Second * 10,
			time.Minute * 5,
		},
	}
	if !reflect.DeepEqual(expected, fooFlagDuration) {
		t.Fatalf("unexpected flag values; got\n%s\nwant\n%s", &fooFlagDuration, &expected)
	}
}

func TestArrayDuration_Set(t *testing.T) {
	f := func(s, expectedResult string) {
		t.Helper()
		var a ArrayDuration
		a.defaultValue = 42 * time.Second
		if err := a.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := a.String()
		if result != expectedResult {
			t.Fatalf("unexpected values parsed;\ngot\n%q\nwant\n%q", result, expectedResult)
		}
	}
	f("", "42s")
	f(`1m`, `1m0s`)
	f(`5m,1s,1h`, `5m0s,1s,1h0m0s`)
	f(`5m,,1h`, `5m0s,42s,1h0m0s`)
}

func TestArrayDuration_GetOptionalArg(t *testing.T) {
	f := func(s string, argIdx int, defaultValue, expectedValue time.Duration) {
		t.Helper()
		var a ArrayDuration
		a.defaultValue = defaultValue
		if err := a.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		v := a.GetOptionalArg(argIdx)
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
		if err := a.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := a.String()
		if result != s {
			t.Fatalf("unexpected string;\ngot\n%s\nwant\n%s", result, s)
		}
	}
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
	f := func(s, expectedResult string) {
		t.Helper()
		var a ArrayBool
		if err := a.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := a.String()
		if result != expectedResult {
			t.Fatalf("unexpected values parsed;\ngot\n%v\nwant\n%v", result, expectedResult)
		}
	}
	f("", "false")
	f(`true`, `true`)
	f(`false,True,False`, `false,true,false`)
	f(`1,,False`, `true,false,false`)
}

func TestArrayBool_GetOptionalArg(t *testing.T) {
	f := func(s string, argIdx int, expectedValue bool) {
		t.Helper()
		var a ArrayBool
		if err := a.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
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
		if err := a.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := a.String()
		if result != s {
			t.Fatalf("unexpected string;\ngot\n%s\nwant\n%s", result, s)
		}
	}
	f("true")
	f("true,false")
	f("false,true")
}

func TestArrayInt(t *testing.T) {
	expected := ArrayInt{
		a: []int{1, 2, 3},
	}
	if !reflect.DeepEqual(expected, fooFlagInt) {
		t.Fatalf("unexpected flag values; got\n%d\nwant\n%d", fooFlagInt, expected)
	}
}

func TestArrayInt_Set(t *testing.T) {
	f := func(s, expectedResult string, expectedValues []int) {
		t.Helper()
		var a ArrayInt
		a.defaultValue = 42
		if err := a.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := a.String()
		if result != expectedResult {
			t.Fatalf("unexpected values parsed;\ngot\n%q\nwant\n%q", result, expectedResult)
		}
		values := a.Values()
		if !reflect.DeepEqual(values, expectedValues) {
			t.Fatalf("unexpected values;\ngot\n%d\nwant\n%d", values, expectedValues)
		}
	}
	f("", "42", []int{42})
	f(`1`, `1`, []int{1})
	f(`-2,3,-64`, `-2,3,-64`, []int{-2, 3, -64})
	f(`,,-64,`, `42,42,-64,42`, []int{42, 42, -64, 42})
}

func TestArrayInt_GetOptionalArg(t *testing.T) {
	f := func(s string, argIdx, defaultValue, expectedValue int) {
		t.Helper()
		var a ArrayInt
		a.defaultValue = defaultValue
		if err := a.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		v := a.GetOptionalArg(argIdx)
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
		if err := a.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := a.String()
		if result != s {
			t.Fatalf("unexpected string;\ngot\n%s\nwant\n%s", result, s)
		}
	}
	f("10,1")
	f("-5,1,123")
}

func TestArrayBytes(t *testing.T) {
	expected := []int64{10000000, 23, 10240}
	result := make([]int64, len(fooFlagBytes.a))
	for i, b := range fooFlagBytes.a {
		result[i] = b.N
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("unexpected flag values; got\n%d\nwant\n%d", result, expected)
	}
}

func TestArrayBytes_Set(t *testing.T) {
	f := func(s, expectedResult string) {
		t.Helper()
		var a ArrayBytes
		a.defaultValue = 42
		if err := a.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := a.String()
		if result != expectedResult {
			t.Fatalf("unexpected values parsed;\ngot\n%s\nwant\n%s", result, expectedResult)
		}
	}
	f("", "42")
	f(`1`, `1`)
	f(`-2,3,10kb`, `-2,3,10KB`)
	f(`,,10kb`, `42,42,10KB`)
}

func TestArrayBytes_GetOptionalArg(t *testing.T) {
	f := func(s string, argIdx int, defaultValue, expectedValue int64) {
		t.Helper()
		var a ArrayBytes
		a.defaultValue = defaultValue
		if err := a.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		v := a.GetOptionalArg(argIdx)
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
		if err := a.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := a.String()
		if result != s {
			t.Fatalf("unexpected string;\ngot\n%s\nwant\n%s", result, s)
		}
	}
	f("10.5KiB,1")
	f("-5,1,123MB")
}
