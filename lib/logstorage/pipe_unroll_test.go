package logstorage

import (
	"reflect"
	"testing"
)

func TestParsePipeUnrollSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`unroll by (foo)`)
	f(`unroll if (x:y) by (foo, bar)`)
}

func TestParsePipeUrollFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`unroll`)
	f(`unroll by ()`)
	f(`unroll by (*)`)
	f(`unroll by (f, *)`)
	f(`unroll by`)
	f(`unroll (`)
	f(`unroll by (foo) bar`)
	f(`unroll by (x) if (a:b)`)
}

func TestPipeUnroll(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// unroll by missing field
	f("unroll (x)", [][]Field{
		{
			{"a", `["foo",1,{"baz":"x"},[1,2],null,NaN]`},
			{"q", "w"},
		},
	}, [][]Field{
		{
			{"a", `["foo",1,{"baz":"x"},[1,2],null,NaN]`},
			{"q", "w"},
			{"x", ""},
		},
	})

	// unroll by field without JSON array
	f("unroll (q)", [][]Field{
		{
			{"a", `["foo",1,{"baz":"x"},[1,2],null,NaN]`},
			{"q", "w"},
		},
	}, [][]Field{
		{
			{"a", `["foo",1,{"baz":"x"},[1,2],null,NaN]`},
			{"q", ""},
		},
	})

	// unroll by a single field
	f("unroll (a)", [][]Field{
		{
			{"a", `["foo",1,{"baz":"x"},[1,2],null,NaN]`},
			{"q", "w"},
		},
		{
			{"a", "b"},
			{"c", "d"},
		},
	}, [][]Field{
		{
			{"a", "foo"},
			{"q", "w"},
		},
		{
			{"a", "1"},
			{"q", "w"},
		},
		{
			{"a", `{"baz":"x"}`},
			{"q", "w"},
		},
		{
			{"a", "[1,2]"},
			{"q", "w"},
		},
		{
			{"a", "null"},
			{"q", "w"},
		},
		{
			{"a", "NaN"},
			{"q", "w"},
		},
		{
			{"a", ""},
			{"c", "d"},
		},
	})

	// unroll by multiple fields
	f("unroll by (timestamp, value)", [][]Field{
		{
			{"timestamp", "[1,2,3]"},
			{"value", `["foo","bar","baz"]`},
			{"other", "abc"},
			{"x", "y"},
		},
		{
			{"timestamp", "[1]"},
			{"value", `["foo","bar"]`},
		},
		{
			{"timestamp", "[1]"},
			{"value", `bar`},
			{"q", "w"},
		},
	}, [][]Field{
		{
			{"timestamp", "1"},
			{"value", "foo"},
			{"other", "abc"},
			{"x", "y"},
		},
		{
			{"timestamp", "2"},
			{"value", "bar"},
			{"other", "abc"},
			{"x", "y"},
		},
		{
			{"timestamp", "3"},
			{"value", "baz"},
			{"other", "abc"},
			{"x", "y"},
		},
		{
			{"timestamp", "1"},
			{"value", "foo"},
		},
		{
			{"timestamp", ""},
			{"value", "bar"},
		},
		{
			{"timestamp", "1"},
			{"value", ""},
			{"q", "w"},
		},
	})

	// conditional unroll by missing field
	f("unroll if (q:abc) (a)", [][]Field{
		{
			{"a", `asd`},
			{"q", "w"},
		},
		{
			{"a", `["foo",123]`},
			{"q", "abc"},
		},
	}, [][]Field{
		{
			{"a", `asd`},
			{"q", "w"},
		},
		{
			{"a", "foo"},
			{"q", "abc"},
		},
		{
			{"a", "123"},
			{"q", "abc"},
		},
	})

	// unroll by non-existing field
	f("unroll (a)", [][]Field{
		{
			{"a", `asd`},
			{"q", "w"},
		},
		{
			{"a", `["foo",123]`},
			{"q", "abc"},
		},
	}, [][]Field{
		{
			{"a", ``},
			{"q", "w"},
		},
		{
			{"a", "foo"},
			{"q", "abc"},
		},
		{
			{"a", "123"},
			{"q", "abc"},
		},
	})

}

func TestPipeUnrollUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("unroll (x)", "*", "", "*", "")
	f("unroll (x, y)", "*", "", "*", "")
	f("unroll if (y:z) (a, b)", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("unroll (x)", "*", "f1,f2", "*", "f1,f2")
	f("unroll if (a:b) (x)", "*", "f1,f2", "*", "f1,f2")
	f("unroll if (f1:b) (x)", "*", "f1,f2", "*", "f2")

	// all the needed fields, unneeded fields intersect with src
	f("unroll (x)", "*", "f2,x", "*", "f2")
	f("unroll if (a:b) (x)", "*", "f2,x", "*", "f2")
	f("unroll if (f2:b) (x)", "*", "f2,x", "*", "")

	// needed fields do not intersect with src
	f("unroll (x)", "f1,f2", "", "f1,f2,x", "")
	f("unroll if (a:b) (x)", "f1,f2", "", "a,f1,f2,x", "")

	// needed fields intersect with src
	f("unroll (x)", "f2,x", "", "f2,x", "")
	f("unroll if (a:b) (x)", "f2,x", "", "a,f2,x", "")
}

func TestUnpackJSONArray(t *testing.T) {
	f := func(s string, resultExpected []string) {
		t.Helper()

		var a arena
		result := unpackJSONArray(nil, &a, s)
		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected result for unpackJSONArray(%q)\ngot\n%q\nwant\n%q", s, result, resultExpected)
		}
	}

	f("", nil)
	f("123", nil)
	f("foo", nil)
	f(`"foo"`, nil)
	f(`{"foo":"bar"}`, nil)
	f(`[foo`, nil)
	f(`[]`, nil)
	f(`[1]`, []string{"1"})
	f(`[1,"foo",["bar",12],{"baz":"x"},NaN,null]`, []string{"1", "foo", `["bar",12]`, `{"baz":"x"}`, "NaN", "null"})
}
