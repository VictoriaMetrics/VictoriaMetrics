package logstorage

import (
	"testing"
)

func TestPipeDeleteUpdateNeededFields(t *testing.T) {
	f := func(s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()

		nfs := newTestFieldsSet(neededFields)
		unfs := newTestFieldsSet(unneededFields)

		lex := newLexer(s)
		p, err := parsePipeDelete(lex)
		if err != nil {
			t.Fatalf("cannot parse %s: %s", s, err)
		}
		p.updateNeededFields(nfs, unfs)

		assertNeededFields(t, nfs, unfs, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("del s1,s2", "*", "", "*", "s1,s2")

	// all the needed fields, unneeded fields do not intersect with src
	f("del s1,s2", "*", "f1,f2", "*", "s1,s2,f1,f2")

	// all the needed fields, unneeded fields intersect with src
	f("del s1,s2", "*", "s1,f1,f2", "*", "s1,s2,f1,f2")

	// needed fields do not intersect with src
	f("del s1,s2", "f1,f2", "", "f1,f2", "")

	// needed fields intersect with src
	f("del s1,s2", "s1,f1,f2", "", "f1,f2", "")
}
