package logstorage

import (
	"strings"
	"testing"
)

func TestParsePipeCopySuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`copy foo as bar`)
	f(`copy foo as bar, a as b`)
}

func TestParsePipeCopyFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`copy`)
	f(`copy x`)
	f(`copy x as`)
	f(`copy x y z`)
}

func TestPipeCopy(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// single row, copy from existing field
	f("copy a as b", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"b", `test`},
		},
	})

	// single row, copy from existing field to multiple fields
	f("copy a as b, a as c, _msg as d", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"b", `test`},
			{"c", `test`},
			{"d", `{"foo":"bar"}`},
		},
	})

	// single row, copy from non-exsiting field
	f("copy x as b", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"b", ``},
		},
	})

	// copy to existing field
	f("copy _msg as a", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `{"foo":"bar"}`},
		},
	})

	// copy to itself
	f("copy a as a", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	})

	// swap copy
	f("copy a as b, _msg as a, b as _msg", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `test`},
			{"a", `{"foo":"bar"}`},
			{"b", `test`},
		},
	})

	// copy to the same field multiple times
	f("copy a as b, _msg as b", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"b", `{"foo":"bar"}`},
		},
	})

	// chain copy
	f("copy a as b, b as c", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"b", `test`},
			{"c", `test`},
		},
	})

	// Multiple rows
	f("copy a as b", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
		{
			{"a", `foobar`},
		},
		{
			{"b", `baz`},
			{"c", "d"},
			{"e", "afdf"},
		},
		{
			{"c", "dss"},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"b", `test`},
		},
		{
			{"a", `foobar`},
			{"b", `foobar`},
		},
		{
			{"b", ``},
			{"c", "d"},
			{"e", "afdf"},
		},
		{
			{"c", "dss"},
			{"b", ""},
		},
	})
}

func TestPipeCopyUpdateNeededFields(t *testing.T) {
	f := func(s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("copy s1 d1, s2 d2", "*", "", "*", "d1,d2")
	f("copy a a", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src and dst
	f("copy s1 d1 ,s2 d2", "*", "f1,f2", "*", "d1,d2,f1,f2")

	// all the needed fields, unneeded fields intersect with src
	f("copy s1 d1 ,s2 d2", "*", "s1,f1,f2", "*", "d1,d2,f1,f2")

	// all the needed fields, unneeded fields intersect with dst
	f("copy s1 d1, s2 d2", "*", "d2,f1,f2", "*", "d1,d2,f1,f2")

	// all the needed fields, unneeded fields intersect with src and dst
	f("copy s1 d1, s2 d2", "*", "s1,d1,f1,f2", "*", "d1,d2,f1,f2,s1")
	f("copy s1 d1, s2 d2", "*", "s1,d2,f1,f2", "*", "d1,d2,f1,f2")

	// needed fields do not intersect with src and dst
	f("copy s1 d1, s2 d2", "f1,f2", "", "f1,f2", "")

	// needed fields intersect with src
	f("copy s1 d1, s2 d2", "s1,f1,f2", "", "s1,f1,f2", "")

	// needed fields intersect with dst
	f("copy s1 d1, s2 d2", "d1,f1,f2", "", "f1,f2,s1", "")

	// needed fields intersect with src and dst
	f("copy s1 d1, s2 d2", "s1,d1,f1,f2", "", "s1,f1,f2", "")
	f("copy s1 d1, s2 d2", "s1,d2,f1,f2", "", "s1,s2,f1,f2", "")
	f("copy s1 d1, s2 d2", "s2,d1,f1,f2", "", "s1,s2,f1,f2", "")
}

func expectPipeNeededFields(t *testing.T, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
	t.Helper()

	nfs := newTestFieldsSet(neededFields)
	unfs := newTestFieldsSet(unneededFields)

	lex := newLexer(s)
	p, err := parsePipe(lex)
	if err != nil {
		t.Fatalf("cannot parse %s: %s", s, err)
	}
	p.updateNeededFields(nfs, unfs)

	assertNeededFields(t, nfs, unfs, neededFieldsExpected, unneededFieldsExpected)
}

func assertNeededFields(t *testing.T, nfs, unfs fieldsSet, neededFieldsExpected, unneededFieldsExpected string) {
	t.Helper()

	nfsStr := nfs.String()
	unfsStr := unfs.String()

	nfsExpected := newTestFieldsSet(neededFieldsExpected)
	unfsExpected := newTestFieldsSet(unneededFieldsExpected)
	nfsExpectedStr := nfsExpected.String()
	unfsExpectedStr := unfsExpected.String()

	if nfsStr != nfsExpectedStr {
		t.Fatalf("unexpected needed fields; got %s; want %s", nfsStr, nfsExpectedStr)
	}
	if unfsStr != unfsExpectedStr {
		t.Fatalf("unexpected unneeded fields; got %s; want %s", unfsStr, unfsExpectedStr)
	}
}

func newTestFieldsSet(fields string) fieldsSet {
	fs := newFieldsSet()
	if fields != "" {
		fs.addFields(strings.Split(fields, ","))
	}
	return fs
}
