package logstorage

import (
	"testing"
)

func TestPipeLimitUpdateNeededFields(t *testing.T) {
	f := func(s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()

		nfs := newTestFieldsSet(neededFields)
		unfs := newTestFieldsSet(unneededFields)

		lex := newLexer(s)
		p, err := parsePipeLimit(lex)
		if err != nil {
			t.Fatalf("cannot parse %s: %s", s, err)
		}
		p.updateNeededFields(nfs, unfs)

		assertNeededFields(t, nfs, unfs, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("limit 10", "*", "", "*", "")

	// all the needed fields, plus unneeded fields
	f("limit 10", "*", "f1,f2", "*", "f1,f2")

	// needed fields
	f("limit 10", "f1,f2", "", "f1,f2", "")
}
