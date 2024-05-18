package logstorage

import (
	"testing"
)

func TestPipeFilterUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()

		nfs := newTestFieldsSet(neededFields)
		unfs := newTestFieldsSet(unneededFields)

		lex := newLexer(s)
		p, err := parsePipeFilter(lex)
		if err != nil {
			t.Fatalf("cannot parse %s: %s", s, err)
		}
		p.updateNeededFields(nfs, unfs)

		assertNeededFields(t, nfs, unfs, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("filter foo f1:bar", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("filter foo f3:bar", "*", "f1,f2", "*", "f1,f2")

	// all the needed fields, unneeded fields intersect with src
	f("filter foo f1:bar", "*", "s1,f1,f2", "*", "s1,f2")

	// needed fields do not intersect with src
	f("filter foo f3:bar", "f1,f2", "", "_msg,f1,f2,f3", "")

	// needed fields intersect with src
	f("filter foo f1:bar", "s1,f1,f2", "", "_msg,f1,f2,s1", "")
}
