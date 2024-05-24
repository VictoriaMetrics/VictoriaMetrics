package logstorage

import (
	"math/rand"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestParsePipeUnpackJSONSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`unpack_json`)
	f(`unpack_json keep_original_fields`)
	f(`unpack_json fields (a)`)
	f(`unpack_json fields (a, b, c)`)
	f(`unpack_json fields (a, b, c) keep_original_fields`)
	f(`unpack_json if (a:x)`)
	f(`unpack_json if (a:x) keep_original_fields`)
	f(`unpack_json from x`)
	f(`unpack_json from x keep_original_fields`)
	f(`unpack_json from x fields (a, b)`)
	f(`unpack_json if (a:x) from x fields (a, b)`)
	f(`unpack_json if (a:x) from x fields (a, b) keep_original_fields`)
	f(`unpack_json from x result_prefix abc`)
	f(`unpack_json if (a:x) from x fields (a, b) result_prefix abc`)
	f(`unpack_json if (a:x) from x fields (a, b) result_prefix abc keep_original_fields`)
	f(`unpack_json result_prefix abc`)
	f(`unpack_json if (a:x) fields (a, b) result_prefix abc`)
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

	// no keep original fields fields
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
			{"x", ""},
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
			{"y", ""},
			{"z", `foobar`},
			{"x", `{"z":["bar",123]}`},
		},
	})
}

func expectPipeResults(t *testing.T, pipeStr string, rows, rowsExpected [][]Field) {
	t.Helper()

	lex := newLexer(pipeStr)
	p, err := parsePipe(lex)
	if err != nil {
		t.Fatalf("unexpected error when parsing %q: %s", pipeStr, err)
	}

	workersCount := 5
	stopCh := make(chan struct{})
	cancel := func() {}
	ppTest := newTestPipeProcessor()
	pp := p.newPipeProcessor(workersCount, stopCh, cancel, ppTest)

	brw := newTestBlockResultWriter(workersCount, pp)
	for _, row := range rows {
		brw.writeRow(row)
	}
	brw.flush()
	pp.flush()

	ppTest.expectRows(t, rowsExpected)
}

func newTestBlockResultWriter(workersCount int, ppBase pipeProcessor) *testBlockResultWriter {
	return &testBlockResultWriter{
		workersCount: workersCount,
		ppBase:       ppBase,
	}
}

type testBlockResultWriter struct {
	workersCount int
	ppBase       pipeProcessor
	rcs          []resultColumn
	br           blockResult

	rowsCount int
}

func (brw *testBlockResultWriter) writeRow(row []Field) {
	if !brw.areSameFields(row) {
		brw.flush()

		brw.rcs = brw.rcs[:0]
		for _, field := range row {
			brw.rcs = appendResultColumnWithName(brw.rcs, field.Name)
		}
	}

	for i, field := range row {
		brw.rcs[i].addValue(field.Value)
	}
	brw.rowsCount++
	if rand.Intn(5) == 0 {
		brw.flush()
	}
}

func (brw *testBlockResultWriter) areSameFields(row []Field) bool {
	if len(brw.rcs) != len(row) {
		return false
	}
	for i, rc := range brw.rcs {
		if rc.name != row[i].Name {
			return false
		}
	}
	return true
}

func (brw *testBlockResultWriter) flush() {
	brw.br.setResultColumns(brw.rcs, brw.rowsCount)
	brw.rowsCount = 0
	workerID := rand.Intn(brw.workersCount)
	brw.ppBase.writeBlock(uint(workerID), &brw.br)
	brw.br.reset()
	for i := range brw.rcs {
		brw.rcs[i].resetValues()
	}
}

func newTestPipeProcessor() *testPipeProcessor {
	return &testPipeProcessor{}
}

type testPipeProcessor struct {
	resultRowsLock sync.Mutex
	resultRows     [][]Field
}

func (pp *testPipeProcessor) writeBlock(_ uint, br *blockResult) {
	cs := br.getColumns()
	var columnValues [][]string
	for _, c := range cs {
		values := c.getValues(br)
		columnValues = append(columnValues, values)
	}

	for i := range br.timestamps {
		row := make([]Field, len(columnValues))
		for j, values := range columnValues {
			r := &row[j]
			r.Name = strings.Clone(cs[j].name)
			r.Value = strings.Clone(values[i])
		}
		pp.resultRowsLock.Lock()
		pp.resultRows = append(pp.resultRows, row)
		pp.resultRowsLock.Unlock()
	}
}

func (pp *testPipeProcessor) flush() error {
	return nil
}

func (pp *testPipeProcessor) expectRows(t *testing.T, expectedRows [][]Field) {
	t.Helper()

	if len(pp.resultRows) != len(expectedRows) {
		t.Fatalf("unexpected number of rows; got %d; want %d\nrows got\n%s\nrows expected\n%s",
			len(pp.resultRows), len(expectedRows), rowsToString(pp.resultRows), rowsToString(expectedRows))
	}

	sortTestRows(pp.resultRows)
	sortTestRows(expectedRows)

	for i, resultRow := range pp.resultRows {
		expectedRow := expectedRows[i]
		if len(resultRow) != len(expectedRow) {
			t.Fatalf("unexpected number of fields at row #%d; got %d; want %d\nrow got\n%s\nrow expected\n%s",
				i, len(resultRow), len(expectedRow), rowToString(resultRow), rowToString(expectedRow))
		}
		for j, resultField := range resultRow {
			expectedField := expectedRow[j]
			if resultField.Name != expectedField.Name {
				t.Fatalf("unexpected field name at row #%d; got %q; want %q\nrow got\n%s\nrow expected\n%s",
					i, resultField.Name, expectedField.Name, rowToString(resultRow), rowToString(expectedRow))
			}
			if resultField.Value != expectedField.Value {
				t.Fatalf("unexpected value for field %q at row #%d; got %q; want %q\nrow got\n%s\nrow expected\n%s",
					resultField.Name, i, resultField.Value, expectedField.Value, rowToString(resultRow), rowToString(expectedRow))
			}
		}
	}
}

func sortTestRows(rows [][]Field) {
	for _, row := range rows {
		sortTestFields(row)
	}
	slices.SortFunc(rows, func(a, b []Field) int {
		reverse := false
		if len(a) > len(b) {
			reverse = true
			a, b = b, a
		}
		for i, fA := range a {
			fB := b[i]
			result := cmpTestFields(fA, fB)
			if result == 0 {
				continue
			}
			if reverse {
				result = -result
			}
			return result
		}
		if len(a) == len(b) {
			return 0
		}
		if reverse {
			return 1
		}
		return -1
	})
}

func sortTestFields(fields []Field) {
	slices.SortFunc(fields, cmpTestFields)
}

func cmpTestFields(a, b Field) int {
	if a.Name == b.Name {
		if a.Value == b.Value {
			return 0
		}
		if a.Value < b.Value {
			return -1
		}
		return 1
	}
	if a.Name < b.Name {
		return -1
	}
	return 1
}

func rowsToString(rows [][]Field) string {
	a := make([]string, len(rows))
	for i, row := range rows {
		a[i] = rowToString(row)
	}
	return strings.Join(a, "\n")
}

func rowToString(row []Field) string {
	a := make([]string, len(row))
	for i, f := range row {
		a[i] = f.String()
	}
	return "{" + strings.Join(a, ",") + "}"
}

func TestPipeUnpackJSONUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("unpack_json from x", "*", "", "*", "")
	f("unpack_json from x keep_original_fields", "*", "", "*", "")
	f("unpack_json if (y:z) from x", "*", "", "*", "")
	f("unpack_json if (y:z) from x fields (a, b)", "*", "", "*", "a,b")
	f("unpack_json if (y:z) from x fields (a, b) keep_original_fields", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("unpack_json from x", "*", "f1,f2", "*", "f1,f2")
	f("unpack_json from x keep_original_fields", "*", "f1,f2", "*", "f1,f2")
	f("unpack_json if (y:z) from x", "*", "f1,f2", "*", "f1,f2")
	f("unpack_json if (f1:z) from x", "*", "f1,f2", "*", "f2")
	f("unpack_json if (y:z) from x fields (f3)", "*", "f1,f2", "*", "f1,f2,f3")
	f("unpack_json if (y:z) from x fields (f1)", "*", "f1,f2", "*", "f1,f2")
	f("unpack_json if (y:z) from x fields (f1) keep_original_fields", "*", "f1,f2", "*", "f1,f2")

	// all the needed fields, unneeded fields intersect with src
	f("unpack_json from x", "*", "f2,x", "*", "f2")
	f("unpack_json from x keep_original_fields", "*", "f2,x", "*", "f2")
	f("unpack_json if (y:z) from x", "*", "f2,x", "*", "f2")
	f("unpack_json if (f2:z) from x", "*", "f1,f2,x", "*", "f1")
	f("unpack_json if (f2:z) from x fields (f3)", "*", "f1,f2,x", "*", "f1,f3")
	f("unpack_json if (f2:z) from x fields (f3) keep_original_fields", "*", "f1,f2,x", "*", "f1")

	// needed fields do not intersect with src
	f("unpack_json from x", "f1,f2", "", "f1,f2,x", "")
	f("unpack_json from x keep_original_fields", "f1,f2", "", "f1,f2,x", "")
	f("unpack_json if (y:z) from x", "f1,f2", "", "f1,f2,x,y", "")
	f("unpack_json if (f1:z) from x", "f1,f2", "", "f1,f2,x", "")
	f("unpack_json if (y:z) from x fields (f3)", "f1,f2", "", "f1,f2", "")
	f("unpack_json if (y:z) from x fields (f3) keep_original_fields", "f1,f2", "", "f1,f2", "")
	f("unpack_json if (y:z) from x fields (f2)", "f1,f2", "", "f1,x,y", "")
	f("unpack_json if (f2:z) from x fields (f2)", "f1,f2", "", "f1,f2,x", "")
	f("unpack_json if (f2:z) from x fields (f2) keep_original_fields", "f1,f2", "", "f1,f2,x", "")

	// needed fields intersect with src
	f("unpack_json from x", "f2,x", "", "f2,x", "")
	f("unpack_json from x keep_original_fields", "f2,x", "", "f2,x", "")
	f("unpack_json if (y:z) from x", "f2,x", "", "f2,x,y", "")
	f("unpack_json if (f2:z y:qwe) from x", "f2,x", "", "f2,x,y", "")
	f("unpack_json if (y:z) from x fields (f1)", "f2,x", "", "f2,x", "")
	f("unpack_json if (y:z) from x fields (f1) keep_original_fields", "f2,x", "", "f2,x", "")
	f("unpack_json if (y:z) from x fields (f2)", "f2,x", "", "x,y", "")
	f("unpack_json if (y:z) from x fields (f2) keep_original_fields", "f2,x", "", "f2,x,y", "")
	f("unpack_json if (y:z) from x fields (x)", "f2,x", "", "f2,x,y", "")
	f("unpack_json if (y:z) from x fields (x) keep_original_fields", "f2,x", "", "f2,x,y", "")
}
