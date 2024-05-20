package logstorage

import (
	"testing"
)

func TestParsePipeUnpackLogfmtSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`unpack_logfmt`)
	f(`unpack_logfmt from x`)
	f(`unpack_logfmt from x result_prefix abc`)
	f(`unpack_logfmt result_prefix abc`)
}

func TestParsePipeUnpackLogfmtFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`unpack_logfmt foo`)
	f(`unpack_logfmt from`)
	f(`unpack_logfmt from x y`)
	f(`unpack_logfmt from x result_prefix`)
	f(`unpack_logfmt from x result_prefix a b`)
	f(`unpack_logfmt result_prefix`)
	f(`unpack_logfmt result_prefix a b`)
}

func TestPipeUnpackLogfmt(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// single row, unpack from _msg
	f("unpack_logfmt", [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
		},
	}, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
			{"foo", "bar"},
			{"baz", "x y=z"},
			{"a", "b"},
		},
	})

	// single row, unpack from _msg into _msg
	f("unpack_logfmt", [][]Field{
		{
			{"_msg", `_msg=bar`},
		},
	}, [][]Field{
		{
			{"_msg", "bar"},
		},
	})

	// single row, unpack from missing field
	f("unpack_logfmt from x", [][]Field{
		{
			{"_msg", `foo=bar`},
		},
	}, [][]Field{
		{
			{"_msg", `foo=bar`},
		},
	})

	// single row, unpack from non-json field
	f("unpack_logfmt from x", [][]Field{
		{
			{"x", `foobar`},
		},
	}, [][]Field{
		{
			{"x", `foobar`},
		},
	})

	// single row, unpack from non-logfmt
	f("unpack_logfmt from x", [][]Field{
		{
			{"x", `foobar`},
		},
	}, [][]Field{
		{
			{"x", `foobar`},
		},
	})

	// unpack empty value
	f("unpack_logfmt from x", [][]Field{
		{
			{"x", `foobar=`},
		},
	}, [][]Field{
		{
			{"x", `foobar=`},
			{"foobar", ""},
		},
	})
	f("unpack_logfmt from x", [][]Field{
		{
			{"x", `foo="" bar= baz=`},
		},
	}, [][]Field{
		{
			{"x", `foo="" bar= baz=`},
			{"foo", ""},
			{"bar", ""},
			{"baz", ""},
		},
	})

	// multiple rows with distinct number of fields
	f("unpack_logfmt from x", [][]Field{
		{
			{"x", `foo=bar baz=xyz`},
			{"y", `abc`},
		},
		{
			{"y", `abc`},
		},
		{
			{"z", `foobar`},
			{"x", `z=bar`},
		},
	}, [][]Field{
		{
			{"x", `foo=bar baz=xyz`},
			{"y", "abc"},
			{"foo", "bar"},
			{"baz", "xyz"},
		},
		{
			{"y", `abc`},
		},
		{
			{"z", `bar`},
			{"x", `z=bar`},
		},
	})

	// multiple rows with distinct number of fields, with result_prefix
	f("unpack_logfmt from x result_prefix qwe_", [][]Field{
		{
			{"x", `foo=bar baz=xyz`},
			{"y", `abc`},
		},
		{
			{"y", `abc`},
		},
		{
			{"z", `foobar`},
			{"x", `z=bar`},
		},
	}, [][]Field{
		{
			{"x", `foo=bar baz=xyz`},
			{"y", "abc"},
			{"qwe_foo", "bar"},
			{"qwe_baz", "xyz"},
		},
		{
			{"y", `abc`},
		},
		{
			{"z", `foobar`},
			{"x", `z=bar`},
			{"qwe_z", `bar`},
		},
	})
}

func TestPipeUnpackLogfmtUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("unpack_logfmt from x", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("unpack_logfmt from x", "*", "f1,f2", "*", "f1,f2")

	// all the needed fields, unneeded fields intersect with src
	f("unpack_logfmt from x", "*", "f2,x", "*", "f2")

	// needed fields do not intersect with src
	f("unpack_logfmt from x", "f1,f2", "", "f1,f2,x", "")

	// needed fields intersect with src
	f("unpack_logfmt from x", "f2,x", "", "f2,x", "")
}
