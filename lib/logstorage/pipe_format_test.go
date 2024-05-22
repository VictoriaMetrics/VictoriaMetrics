package logstorage

import (
	"testing"
)

func TestParsePipeFormatSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`format "foo<bar>"`)
	f(`format "" as x`)
	f(`format "<>" as x`)
	f(`format foo as x`)
	f(`format "<foo>"`)
	f(`format "<foo>bar<baz>"`)
	f(`format "bar<baz><xyz>bac"`)
	f(`format if (x:y) "bar<baz><xyz>bac"`)
}

func TestParsePipeFormatFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`format`)
	f(`format if`)
	f(`format foo bar`)
	f(`format foo if`)
	f(`format foo as x if (x:y)`)
}

func TestPipeFormat(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// plain string into a single field
	f(`format '{"foo":<q:foo>,"bar":"<bar>"}' as x`, [][]Field{
		{
			{"foo", `"abc"`},
			{"bar", `cde`},
		},
	}, [][]Field{
		{
			{"foo", `"abc"`},
			{"bar", `cde`},
			{"x", `{"foo":"\"abc\"","bar":"cde"}`},
		},
	})

	// plain string into a single field
	f(`format foo as x`, [][]Field{
		{
			{"_msg", `foobar`},
			{"a", "x"},
		},
	}, [][]Field{
		{
			{"_msg", `foobar`},
			{"a", "x"},
			{"x", `foo`},
		},
	})

	// plain string with html escaping into a single field
	f(`format "&lt;foo&gt;" as x`, [][]Field{
		{
			{"_msg", `foobar`},
			{"a", "x"},
		},
	}, [][]Field{
		{
			{"_msg", `foobar`},
			{"a", "x"},
			{"x", `<foo>`},
		},
	})

	// format with empty placeholders into existing field
	f(`format "<_>foo<_>" as _msg`, [][]Field{
		{
			{"_msg", `foobar`},
			{"a", "x"},
		},
	}, [][]Field{
		{
			{"_msg", `foo`},
			{"a", "x"},
		},
	})

	// format with various placeholders into new field
	f(`format "a<foo>aa<_msg>xx<a>x" as x`, [][]Field{
		{
			{"_msg", `foobar`},
			{"a", "b"},
		},
	}, [][]Field{
		{
			{"_msg", `foobar`},
			{"a", "b"},
			{"x", `aaafoobarxxbx`},
		},
	})

	// format into existing field
	f(`format "a<foo>aa<_msg>xx<a>x"`, [][]Field{
		{
			{"_msg", `foobar`},
			{"a", "b"},
		},
	}, [][]Field{
		{
			{"_msg", `aaafoobarxxbx`},
			{"a", "b"},
		},
	})

	// conditional format over multiple rows
	f(`format if (!c:*) "a: <a>, b: <b>, x: <a>" as c`, [][]Field{
		{
			{"b", "bar"},
			{"a", "foo"},
			{"c", "keep-me"},
		},
		{
			{"c", ""},
			{"a", "f"},
		},
		{
			{"b", "x"},
		},
	}, [][]Field{
		{
			{"b", "bar"},
			{"a", "foo"},
			{"c", "keep-me"},
		},
		{
			{"a", "f"},
			{"c", "a: f, b: , x: f"},
		},
		{
			{"b", "x"},
			{"c", "a: , b: x, x: "},
		},
	})
}

func TestPipeFormatUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f(`format "foo" as x`, "*", "", "*", "x")
	f(`format "<f1>foo" as x`, "*", "", "*", "x")
	f(`format if (f2:z) "<f1>foo" as x`, "*", "", "*", "x")

	// unneeded fields do not intersect with pattern and output field
	f(`format "foo" as x`, "*", "f1,f2", "*", "f1,f2,x")
	f(`format "<f3>foo" as x`, "*", "f1,f2", "*", "f1,f2,x")
	f(`format if (f4:z) "<f3>foo" as x`, "*", "f1,f2", "*", "f1,f2,x")
	f(`format if (f1:z) "<f3>foo" as x`, "*", "f1,f2", "*", "f2,x")

	// unneeded fields intersect with pattern
	f(`format "<f1>foo" as x`, "*", "f1,f2", "*", "f2,x")
	f(`format if (f4:z) "<f1>foo" as x`, "*", "f1,f2", "*", "f2,x")
	f(`format if (f2:z) "<f1>foo" as x`, "*", "f1,f2", "*", "x")

	// unneeded fields intersect with output field
	f(`format "<f1>foo" as x`, "*", "x,y", "*", "x,y")
	f(`format if (f2:z) "<f1>foo" as x`, "*", "x,y", "*", "x,y")
	f(`format if (y:z) "<f1>foo" as x`, "*", "x,y", "*", "x,y")

	// needed fields do not intersect with pattern and output field
	f(`format "<f1>foo" as f2`, "x,y", "", "x,y", "")
	f(`format if (f3:z) "<f1>foo" as f2`, "x,y", "", "x,y", "")
	f(`format if (x:z) "<f1>foo" as f2`, "x,y", "", "x,y", "")

	// needed fields intersect with pattern field
	f(`format "<f1>foo" as f2`, "f1,y", "", "f1,y", "")
	f(`format if (f3:z) "<f1>foo" as f2`, "f1,y", "", "f1,y", "")
	f(`format if (x:z) "<f1>foo" as f2`, "f1,y", "", "f1,y", "")

	// needed fields intersect with output field
	f(`format "<f1>foo" as f2`, "f2,y", "", "f1,y", "")
	f(`format if (f3:z) "<f1>foo" as f2`, "f2,y", "", "f1,f3,y", "")
	f(`format if (x:z or y:w) "<f1>foo" as f2`, "f2,y", "", "f1,x,y", "")

	// needed fields intersect with pattern and output fields
	f(`format "<f1>foo" as f2`, "f1,f2,y", "", "f1,y", "")
	f(`format if (f3:z) "<f1>foo" as f2`, "f1,f2,y", "", "f1,f3,y", "")
	f(`format if (x:z or y:w) "<f1>foo" as f2`, "f1,f2,y", "", "f1,x,y", "")
}
