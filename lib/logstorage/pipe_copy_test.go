package logstorage

import (
	"strings"
	"testing"
)

func TestPipeCopyUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()

		nfs := newTestFieldsSet(neededFields)
		unfs := newTestFieldsSet(unneededFields)

		lex := newLexer(s)
		p, err := parsePipeCopy(lex)
		if err != nil {
			t.Fatalf("cannot parse %s: %s", s, err)
		}
		p.updateNeededFields(nfs, unfs)

		assertNeededFields(t, nfs, unfs, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("copy s1 d1, s2 d2", "*", "", "*", "d1,d2")

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
