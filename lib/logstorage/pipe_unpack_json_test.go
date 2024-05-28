package logstorage

import (
	"testing"
)

func TestParsePipeUnpackJSONSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`unpack_json`)
	f(`unpack_json skip_empty_results`)
	f(`unpack_json keep_original_fields`)
	f(`unpack_json fields (a)`)
	f(`unpack_json fields (a, b, c)`)
	f(`unpack_json fields (a, b, c) skip_empty_results`)
	f(`unpack_json fields (a, b, c) keep_original_fields`)
	f(`unpack_json if (a:x)`)
	f(`unpack_json if (a:x) skip_empty_results`)
	f(`unpack_json if (a:x) keep_original_fields`)
	f(`unpack_json from x`)
	f(`unpack_json from x skip_empty_results`)
	f(`unpack_json from x keep_original_fields`)
	f(`unpack_json from x fields (a, b)`)
	f(`unpack_json if (a:x) from x fields (a, b)`)
	f(`unpack_json if (a:x) from x fields (a, b) skip_empty_results`)
	f(`unpack_json if (a:x) from x fields (a, b) keep_original_fields`)
	f(`unpack_json from x result_prefix abc`)
	f(`unpack_json if (a:x) from x fields (a, b) result_prefix abc`)
	f(`unpack_json if (a:x) from x fields (a, b) result_prefix abc skip_empty_results`)
	f(`unpack_json if (a:x) from x fields (a, b) result_prefix abc keep_original_fields`)
	f(`unpack_json result_prefix abc`)
	f(`unpack_json if (a:x) fields (a, b) result_prefix abc`)
	f(`unpack_json if (a:x) fields (a, b) result_prefix abc skip_empty_results`)
	f(`unpack_json if (a:x) fields (a, b) result_prefix abc keep_original_fields`)
}

func TestParsePipeUnpackJSONFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`unpack_json foo`)
	f(`unpack_json if`)
	f(`unpack_json fields`)
	f(`unpack_json fields x`)
	f(`unpack_json if (x:y) foobar`)
	f(`unpack_json from`)
	f(`unpack_json from x y`)
	f(`unpack_json from x if`)
	f(`unpack_json from x result_prefix`)
	f(`unpack_json from x result_prefix a b`)
	f(`unpack_json from x result_prefix a if`)
	f(`unpack_json result_prefix`)
	f(`unpack_json result_prefix a b`)
	f(`unpack_json result_prefix a if`)
}

func TestPipeUnpackJSON(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// skip empty results
	f("unpack_json skip_empty_results", [][]Field{
		{
			{"_msg", `{"foo":"bar","z":"q","a":""}`},
			{"foo", "x"},
			{"a", "foobar"},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar","z":"q","a":""}`},
			{"foo", "bar"},
			{"z", "q"},
			{"a", "foobar"},
		},
	})

	// no skip empty results
	f("unpack_json", [][]Field{
		{
			{"_msg", `{"foo":"bar","z":"q","a":""}`},
			{"foo", "x"},
			{"a", "foobar"},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar","z":"q","a":""}`},
			{"foo", "bar"},
			{"z", "q"},
			{"a", ""},
		},
	})

	// no keep original fields
	f("unpack_json", [][]Field{
		{
			{"_msg", `{"foo":"bar","z":"q","a":"b"}`},
			{"foo", "x"},
			{"a", ""},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar","z":"q","a":"b"}`},
			{"foo", "bar"},
			{"z", "q"},
			{"a", "b"},
		},
	})

	// keep original fields
	f("unpack_json keep_original_fields", [][]Field{
		{
			{"_msg", `{"foo":"bar","z":"q","a":"b"}`},
			{"foo", "x"},
			{"a", ""},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar","z":"q","a":"b"}`},
			{"foo", "x"},
			{"z", "q"},
			{"a", "b"},
		},
	})

	// unpack only the requested fields
	f("unpack_json fields (foo, b)", [][]Field{
		{
			{"_msg", `{"foo":"bar","z":"q","a":"b"}`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar","z":"q","a":"b"}`},
			{"foo", "bar"},
			{"b", ""},
		},
	})

	// single row, unpack from _msg
	f("unpack_json", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"foo", "bar"},
		},
	})

	// failed if condition
	f("unpack_json if (x:foo)", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
		},
	})

	// matched if condition
	f("unpack_json if (foo)", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"foo", "bar"},
		},
	})

	// single row, unpack from _msg into _msg
	f("unpack_json", [][]Field{
		{
			{"_msg", `{"_msg":"bar"}`},
		},
	}, [][]Field{
		{
			{"_msg", "bar"},
		},
	})

	// single row, unpack from missing field
	f("unpack_json from x", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
		},
	})

	// single row, unpack from non-json field
	f("unpack_json from x", [][]Field{
		{
			{"x", `foobar`},
		},
	}, [][]Field{
		{
			{"x", `foobar`},
		},
	})

	// single row, unpack from non-dict json
	f("unpack_json from x", [][]Field{
		{
			{"x", `["foobar"]`},
		},
	}, [][]Field{
		{
			{"x", `["foobar"]`},
		},
	})
	f("unpack_json from x", [][]Field{
		{
			{"x", `1234`},
		},
	}, [][]Field{
		{
			{"x", `1234`},
		},
	})
	f("unpack_json from x", [][]Field{
		{
			{"x", `"xxx"`},
		},
	}, [][]Field{
		{
			{"x", `"xxx"`},
		},
	})

	// single row, unpack from named field
	f("unpack_json from x", [][]Field{
		{
			{"x", `{"foo":"bar","baz":"xyz","a":123,"b":["foo","bar"],"x":NaN,"y":{"z":{"a":"b"}}}`},
		},
	}, [][]Field{
		{
			{"x", `NaN`},
			{"foo", "bar"},
			{"baz", "xyz"},
			{"a", "123"},
			{"b", `["foo","bar"]`},
			{"y.z.a", "b"},
		},
	})

	// multiple rows with distinct number of fields
	f("unpack_json from x", [][]Field{
		{
			{"x", `{"foo":"bar","baz":"xyz"}`},
			{"y", `abc`},
		},
		{
			{"y", `abc`},
		},
		{
			{"z", `foobar`},
			{"x", `{"z":["bar",123]}`},
		},
	}, [][]Field{
		{
			{"x", `{"foo":"bar","baz":"xyz"}`},
			{"y", "abc"},
			{"foo", "bar"},
			{"baz", "xyz"},
		},
		{
			{"y", `abc`},
		},
		{
			{"z", `["bar",123]`},
			{"x", `{"z":["bar",123]}`},
		},
	})

	// multiple rows with distinct number of fields with result_prefix and if condition
	f("unpack_json if (y:abc) from x result_prefix qwe_", [][]Field{
		{
			{"x", `{"foo":"bar","baz":"xyz"}`},
			{"y", `abc`},
		},
		{
			{"y", `abc`},
		},
		{
			{"z", `foobar`},
			{"x", `{"z":["bar",123]}`},
		},
	}, [][]Field{
		{
			{"x", `{"foo":"bar","baz":"xyz"}`},
			{"y", "abc"},
			{"qwe_foo", "bar"},
			{"qwe_baz", "xyz"},
		},
		{
			{"y", `abc`},
		},
		{
			{"z", `foobar`},
			{"x", `{"z":["bar",123]}`},
		},
	})
}

func TestPipeUnpackJSONUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("unpack_json from x", "*", "", "*", "")
	f("unpack_json from x skip_empty_results", "*", "", "*", "")
	f("unpack_json from x keep_original_fields", "*", "", "*", "")
	f("unpack_json if (y:z) from x", "*", "", "*", "")
	f("unpack_json if (y:z) from x fields (a, b)", "*", "", "*", "a,b")
	f("unpack_json if (y:z) from x fields (a, b) skip_empty_results", "*", "", "*", "")
	f("unpack_json if (y:z) from x fields (a, b) keep_original_fields", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("unpack_json from x", "*", "f1,f2", "*", "f1,f2")
	f("unpack_json from x skip_empty_results", "*", "f1,f2", "*", "f1,f2")
	f("unpack_json from x keep_original_fields", "*", "f1,f2", "*", "f1,f2")
	f("unpack_json if (y:z) from x", "*", "f1,f2", "*", "f1,f2")
	f("unpack_json if (f1:z) from x", "*", "f1,f2", "*", "f2")
	f("unpack_json if (y:z) from x fields (f3)", "*", "f1,f2", "*", "f1,f2,f3")
	f("unpack_json if (y:z) from x fields (f1)", "*", "f1,f2", "*", "f1,f2")
	f("unpack_json if (y:z) from x fields (f1) skip_empty_results", "*", "f1,f2", "*", "f1,f2")
	f("unpack_json if (y:z) from x fields (f1) keep_original_fields", "*", "f1,f2", "*", "f1,f2")

	// all the needed fields, unneeded fields intersect with src
	f("unpack_json from x", "*", "f2,x", "*", "f2")
	f("unpack_json from x skip_empty_results", "*", "f2,x", "*", "f2")
	f("unpack_json from x keep_original_fields", "*", "f2,x", "*", "f2")
	f("unpack_json if (y:z) from x", "*", "f2,x", "*", "f2")
	f("unpack_json if (f2:z) from x", "*", "f1,f2,x", "*", "f1")
	f("unpack_json if (f2:z) from x fields (f3)", "*", "f1,f2,x", "*", "f1,f3")
	f("unpack_json if (f2:z) from x fields (f3) skip_empty_results", "*", "f1,f2,x", "*", "f1")
	f("unpack_json if (f2:z) from x fields (f3) keep_original_fields", "*", "f1,f2,x", "*", "f1")

	// needed fields do not intersect with src
	f("unpack_json from x", "f1,f2", "", "f1,f2,x", "")
	f("unpack_json from x skip_empty_results", "f1,f2", "", "f1,f2,x", "")
	f("unpack_json from x keep_original_fields", "f1,f2", "", "f1,f2,x", "")
	f("unpack_json if (y:z) from x", "f1,f2", "", "f1,f2,x,y", "")
	f("unpack_json if (f1:z) from x", "f1,f2", "", "f1,f2,x", "")
	f("unpack_json if (y:z) from x fields (f3)", "f1,f2", "", "f1,f2", "")
	f("unpack_json if (y:z) from x fields (f3) skip_empty_results", "f1,f2", "", "f1,f2", "")
	f("unpack_json if (y:z) from x fields (f3) keep_original_fields", "f1,f2", "", "f1,f2", "")
	f("unpack_json if (y:z) from x fields (f2)", "f1,f2", "", "f1,x,y", "")
	f("unpack_json if (f2:z) from x fields (f2)", "f1,f2", "", "f1,f2,x", "")
	f("unpack_json if (f2:z) from x fields (f2) skip_empty_results", "f1,f2", "", "f1,f2,x", "")
	f("unpack_json if (f2:z) from x fields (f2) keep_original_fields", "f1,f2", "", "f1,f2,x", "")

	// needed fields intersect with src
	f("unpack_json from x", "f2,x", "", "f2,x", "")
	f("unpack_json from x skip_empty_results", "f2,x", "", "f2,x", "")
	f("unpack_json from x keep_original_fields", "f2,x", "", "f2,x", "")
	f("unpack_json if (y:z) from x", "f2,x", "", "f2,x,y", "")
	f("unpack_json if (f2:z y:qwe) from x", "f2,x", "", "f2,x,y", "")
	f("unpack_json if (y:z) from x fields (f1)", "f2,x", "", "f2,x", "")
	f("unpack_json if (y:z) from x fields (f1) skip_empty_results", "f2,x", "", "f2,x", "")
	f("unpack_json if (y:z) from x fields (f1) keep_original_fields", "f2,x", "", "f2,x", "")
	f("unpack_json if (y:z) from x fields (f2)", "f2,x", "", "x,y", "")
	f("unpack_json if (y:z) from x fields (f2) skip_empty_results", "f2,x", "", "f2,x,y", "")
	f("unpack_json if (y:z) from x fields (f2) keep_original_fields", "f2,x", "", "f2,x,y", "")
	f("unpack_json if (y:z) from x fields (x)", "f2,x", "", "f2,x,y", "")
	f("unpack_json if (y:z) from x fields (x) skip_empty_results", "f2,x", "", "f2,x,y", "")
	f("unpack_json if (y:z) from x fields (x) keep_original_fields", "f2,x", "", "f2,x,y", "")
}
