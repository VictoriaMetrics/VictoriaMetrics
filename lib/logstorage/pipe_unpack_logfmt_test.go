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
	f(`unpack_logfmt skip_empty_results`)
	f(`unpack_logfmt keep_original_fields`)
	f(`unpack_logfmt fields (a, b)`)
	f(`unpack_logfmt fields (a, b) skip_empty_results`)
	f(`unpack_logfmt fields (a, b) keep_original_fields`)
	f(`unpack_logfmt if (a:x)`)
	f(`unpack_logfmt if (a:x) skip_empty_results`)
	f(`unpack_logfmt if (a:x) keep_original_fields`)
	f(`unpack_logfmt if (a:x) fields (a, b)`)
	f(`unpack_logfmt from x`)
	f(`unpack_logfmt from x skip_empty_results`)
	f(`unpack_logfmt from x keep_original_fields`)
	f(`unpack_logfmt from x fields (a, b)`)
	f(`unpack_logfmt from x fields (a, b) skip_empty_results`)
	f(`unpack_logfmt from x fields (a, b) keep_original_fields`)
	f(`unpack_logfmt if (a:x) from x`)
	f(`unpack_logfmt if (a:x) from x fields (a, b)`)
	f(`unpack_logfmt from x result_prefix abc`)
	f(`unpack_logfmt if (a:x) from x result_prefix abc`)
	f(`unpack_logfmt if (a:x) from x fields (a, b) result_prefix abc`)
	f(`unpack_logfmt if (a:x) from x fields (a, b) result_prefix abc skip_empty_results`)
	f(`unpack_logfmt if (a:x) from x fields (a, b) result_prefix abc keep_original_fields`)
	f(`unpack_logfmt result_prefix abc`)
	f(`unpack_logfmt if (a:x) result_prefix abc`)
	f(`unpack_logfmt if (a:x) fields (a, b) result_prefix abc`)
	f(`unpack_logfmt if (a:x) fields (a, b) result_prefix abc skip_empty_results`)
	f(`unpack_logfmt if (a:x) fields (a, b) result_prefix abc keep_original_fields`)
}

func TestParsePipeUnpackLogfmtFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`unpack_logfmt foo`)
	f(`unpack_logfmt fields`)
	f(`unpack_logfmt if`)
	f(`unpack_logfmt if (x:y) foobar`)
	f(`unpack_logfmt from`)
	f(`unpack_logfmt from x y`)
	f(`unpack_logfmt from x if`)
	f(`unpack_logfmt from x result_prefix`)
	f(`unpack_logfmt from x result_prefix a b`)
	f(`unpack_logfmt from x result_prefix a if`)
	f(`unpack_logfmt result_prefix`)
	f(`unpack_logfmt result_prefix a b`)
	f(`unpack_logfmt result_prefix a if`)
}

func TestPipeUnpackLogfmt(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// unpack a subset of fields
	f("unpack_logfmt fields (foo, a, b)", [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
			{"a", "xxx"},
		},
	}, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
			{"foo", "bar"},
			{"a", "b"},
			{"b", ""},
		},
	})

	// no skip empty results
	f("unpack_logfmt", [][]Field{
		{
			{"_msg", `foo= baz="x y=z" a=b`},
			{"foo", "321"},
			{"baz", "abcdef"},
		},
	}, [][]Field{
		{
			{"_msg", `foo= baz="x y=z" a=b`},
			{"foo", ""},
			{"baz", "x y=z"},
			{"a", "b"},
		},
	})

	// skip empty results
	f("unpack_logfmt skip_empty_results", [][]Field{
		{
			{"_msg", `foo= baz="x y=z" a=b`},
			{"foo", "321"},
			{"baz", "abcdef"},
		},
	}, [][]Field{
		{
			{"_msg", `foo= baz="x y=z" a=b`},
			{"foo", "321"},
			{"baz", "x y=z"},
			{"a", "b"},
		},
	})

	// keep original fields
	f("unpack_logfmt keep_original_fields", [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
			{"baz", "abcdef"},
		},
	}, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
			{"foo", "bar"},
			{"baz", "abcdef"},
			{"a", "b"},
		},
	})

	// single row, unpack from _msg
	f("unpack_logfmt", [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
			{"baz", "abcdef"},
		},
	}, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
			{"foo", "bar"},
			{"baz", "x y=z"},
			{"a", "b"},
		},
	})

	// failed if condition
	f("unpack_logfmt if (foo:bar)", [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
		},
	}, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
		},
	})

	// matched if condition
	f("unpack_logfmt if (foo)", [][]Field{
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

	// single row, unpack from non-logfmt
	f("unpack_logfmt from x", [][]Field{
		{
			{"x", `foobar`},
		},
	}, [][]Field{
		{
			{"x", `foobar`},
			{"foobar", ""},
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

	// multiple rows with distinct number of fields, with result_prefix and if condition
	f("unpack_logfmt if (y:abc) from x result_prefix qwe_", [][]Field{
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
		},
	})
}

func TestPipeUnpackLogfmtUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("unpack_logfmt", "*", "", "*", "")
	f("unpack_logfmt fields (f1, f2)", "*", "", "*", "f1,f2")
	f("unpack_logfmt fields (f1, f2) skip_empty_results", "*", "", "*", "")
	f("unpack_logfmt fields (f1, f2) keep_original_fields", "*", "", "*", "")
	f("unpack_logfmt keep_original_fields", "*", "", "*", "")
	f("unpack_logfmt if (y:z) from x", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("unpack_logfmt from x", "*", "f1,f2", "*", "f1,f2")
	f("unpack_logfmt if (y:z) from x", "*", "f1,f2", "*", "f1,f2")
	f("unpack_logfmt if (f1:z) from x", "*", "f1,f2", "*", "f2")

	// all the needed fields, unneeded fields intersect with src
	f("unpack_logfmt from x", "*", "f2,x", "*", "f2")
	f("unpack_logfmt if (y:z) from x", "*", "f2,x", "*", "f2")
	f("unpack_logfmt if (f2:z) from x", "*", "f1,f2,x", "*", "f1")

	// needed fields do not intersect with src
	f("unpack_logfmt from x", "f1,f2", "", "f1,f2,x", "")
	f("unpack_logfmt if (y:z) from x", "f1,f2", "", "f1,f2,x,y", "")
	f("unpack_logfmt if (f1:z) from x", "f1,f2", "", "f1,f2,x", "")

	// needed fields intersect with src
	f("unpack_logfmt from x", "f2,x", "", "f2,x", "")
	f("unpack_logfmt if (y:z) from x", "f2,x", "", "f2,x,y", "")
	f("unpack_logfmt if (f2:z y:qwe) from x", "f2,x", "", "f2,x,y", "")
}
