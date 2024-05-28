package logstorage

import (
	"testing"
)

func TestParsePipeReplaceSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`replace (foo, bar)`)
	f(`replace (" ", "") at x`)
	f(`replace if (x:y) ("-", ":") at a`)
	f(`replace (" ", "") at x limit 10`)
	f(`replace if (x:y) (" ", "") at foo limit 10`)
}

func TestParsePipeReplaceFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`replace`)
	f(`replace if`)
	f(`replace foo`)
	f(`replace (`)
	f(`replace (foo`)
	f(`replace (foo,`)
	f(`replace(foo,bar`)
	f(`replace(foo,bar,baz)`)
	f(`replace(foo,bar) abc`)
	f(`replace(bar,baz) limit`)
	f(`replace(bar,baz) limit N`)
}

func TestPipeReplace(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// replace without limits at _msg
	f(`replace ("_", "-")`, [][]Field{
		{
			{"_msg", `a_bc_def`},
			{"bar", `cde`},
		},
		{
			{"_msg", `1234`},
		},
	}, [][]Field{
		{
			{"_msg", `a-bc-def`},
			{"bar", `cde`},
		},
		{
			{"_msg", `1234`},
		},
	})

	// replace with limit 1 at foo
	f(`replace ("_", "-") at foo limit 1`, [][]Field{
		{
			{"foo", `a_bc_def`},
			{"bar", `cde`},
		},
		{
			{"foo", `1234`},
		},
	}, [][]Field{
		{
			{"foo", `a-bc_def`},
			{"bar", `cde`},
		},
		{
			{"foo", `1234`},
		},
	})

	// replace with limit 100 at foo
	f(`replace ("_", "-") at foo limit 100`, [][]Field{
		{
			{"foo", `a_bc_def`},
			{"bar", `cde`},
		},
		{
			{"foo", `1234`},
		},
	}, [][]Field{
		{
			{"foo", `a-bc-def`},
			{"bar", `cde`},
		},
		{
			{"foo", `1234`},
		},
	})

	// conditional replace at foo
	f(`replace if (bar:abc) ("_", "") at foo`, [][]Field{
		{
			{"foo", `a_bc_def`},
			{"bar", `cde`},
		},
		{
			{"foo", `123_456`},
			{"bar", "abc"},
		},
	}, [][]Field{
		{
			{"foo", `a_bc_def`},
			{"bar", `cde`},
		},
		{
			{"foo", `123456`},
			{"bar", "abc"},
		},
	})
}

func TestPipeReplaceUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f(`replace ("a", "b") at x`, "*", "", "*", "")
	f(`replace if (f1:q) ("a", "b") at x`, "*", "", "*", "")

	// unneeded fields do not intersect with at field
	f(`replace ("a", "b") at x`, "*", "f1,f2", "*", "f1,f2")
	f(`replace if (f3:q) ("a", "b") at x`, "*", "f1,f2", "*", "f1,f2")
	f(`replace if (f2:q) ("a", "b") at x`, "*", "f1,f2", "*", "f1")

	// unneeded fields intersect with at field
	f(`replace ("a", "b") at x`, "*", "x,y", "*", "x,y")
	f(`replace if (f1:q) ("a", "b") at x`, "*", "x,y", "*", "x,y")
	f(`replace if (x:q) ("a", "b") at x`, "*", "x,y", "*", "x,y")
	f(`replace if (y:q) ("a", "b") at x`, "*", "x,y", "*", "x,y")

	// needed fields do not intersect with at field
	f(`replace ("a", "b") at x`, "f2,y", "", "f2,y", "")
	f(`replace if (f1:q) ("a", "b") at x`, "f2,y", "", "f2,y", "")

	// needed fields intersect with at field
	f(`replace ("a", "b") at y`, "f2,y", "", "f2,y", "")
	f(`replace if (f1:q) ("a", "b") at y`, "f2,y", "", "f1,f2,y", "")
}

func TestAppendReplace(t *testing.T) {
	f := func(s, oldSubstr, newSubstr string, limit int, resultExpected string) {
		t.Helper()

		result := appendReplace(nil, s, oldSubstr, newSubstr, uint64(limit))
		if string(result) != resultExpected {
			t.Fatalf("unexpected result for appendReplace(%q, %q, %q, %d)\ngot\n%s\nwant\n%s", s, oldSubstr, newSubstr, limit, result, resultExpected)
		}
	}

	f("", "", "", 0, "")
	f("", "foo", "bar", 0, "")
	f("abc", "foo", "bar", 0, "abc")
	f("foo", "foo", "bar", 0, "bar")
	f("foox", "foo", "bar", 0, "barx")
	f("afoo", "foo", "bar", 0, "abar")
	f("afoox", "foo", "bar", 0, "abarx")
	f("foo-bar-baz", "-", "_", 0, "foo_bar_baz")
	f("foo bar baz  ", " ", "", 1, "foobar baz  ")
}
