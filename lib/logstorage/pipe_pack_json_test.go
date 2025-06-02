package logstorage

import (
	"testing"
)

func TestParsePipePackJSONSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`pack_json`)
	f(`pack_json as x`)
	f(`pack_json fields (a, b)`)
	f(`pack_json fields (a, b) as x`)
	f(`pack_json fields (foo.*, bar, baz.abc.*) as x`)
}

func TestParsePipePackJSONFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`pack_json foo bar`)
	f(`pack_json fields`)
	f(`pack_json as *`)
	f(`pack_json as x*`)
}

func TestPipePackJSON(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// pack into _msg
	f(`pack_json`, [][]Field{
		{
			{"_msg", "x"},
			{"foo", `abc`},
			{"bar", `cde`},
		},
		{
			{"a", "b"},
			{"c", "d"},
		},
	}, [][]Field{
		{
			{"_msg", `{"_msg":"x","foo":"abc","bar":"cde"}`},
			{"foo", `abc`},
			{"bar", `cde`},
		},
		{
			{"_msg", `{"a":"b","c":"d"}`},
			{"a", "b"},
			{"c", "d"},
		},
	})

	// pack into other field
	f(`pack_json as a`, [][]Field{
		{
			{"_msg", "x"},
			{"foo", `abc`},
			{"bar", `cde`},
		},
		{
			{"a", "b"},
			{"c", "d"},
		},
	}, [][]Field{
		{
			{"_msg", `x`},
			{"foo", `abc`},
			{"bar", `cde`},
			{"a", `{"_msg":"x","foo":"abc","bar":"cde"}`},
		},
		{
			{"a", `{"a":"b","c":"d"}`},
			{"c", "d"},
		},
	})

	// pack only the needed fields
	f(`pack_json fields (foo, baz) a`, [][]Field{
		{
			{"_msg", "x"},
			{"foo", `abc`},
			{"bar", `cde`},
		},
		{
			{"a", "b"},
			{"c", "d"},
		},
	}, [][]Field{
		{
			{"_msg", `x`},
			{"foo", `abc`},
			{"bar", `cde`},
			{"a", `{"foo":"abc"}`},
		},
		{
			{"a", `{}`},
			{"c", "d"},
		},
	})

	// pack only the needed wildcard fields
	f(`pack_json fields (x*,y) a`, [][]Field{
		{
			{"x", `abc`},
			{"xx", `xabc`},
			{"yy", `cde`},
			{"y", `xcde`},
		},
	}, [][]Field{
		{
			{"x", `abc`},
			{"xx", `xabc`},
			{"yy", `cde`},
			{"y", `xcde`},
			{"a", `{"x":"abc","xx":"xabc","y":"xcde"}`},
		},
	})
}

func TestPipePackJSONUpdateNeededFields(t *testing.T) {
	f := func(s string, allowFilters, denyFilters, allowFiltersExpected, denyFiltersExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, allowFilters, denyFilters, allowFiltersExpected, denyFiltersExpected)
	}

	// all the needed fields
	f(`pack_json as x`, "*", "", "*", "")

	// unneeded fields do not intersect with output
	f(`pack_json as x`, "*", "f1,f2", "*", "")
	f(`pack_json as x`, "*", "f*", "*", "")

	// unneeded fields intersect with output
	f(`pack_json as f1`, "*", "f1,f2", "*", "f1,f2")
	f(`pack_json as f1`, "*", "f*", "*", "f*")

	// needed fields do not intersect with output
	f(`pack_json f1`, "x,y", "", "x,y", "")
	f(`pack_json f1`, "x*,y", "", "x*,y", "")

	// needed fields intersect with output
	f(`pack_json as f2`, "f2,y", "", "*", "")
	f(`pack_json as f2`, "f*,y", "", "*", "")
}
