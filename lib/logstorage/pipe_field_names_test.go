package logstorage

import (
	"testing"
)

func TestPipeFieldNamesUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()

		nfs := newTestFieldsSet(neededFields)
		unfs := newTestFieldsSet(unneededFields)

		lex := newLexer(s)
		p, err := parsePipeFieldNames(lex)
		if err != nil {
			t.Fatalf("cannot parse %s: %s", s, err)
		}
		p.updateNeededFields(nfs, unfs)

		assertNeededFields(t, nfs, unfs, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("field_names as f1", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("field_names as f3", "*", "f1,f2", "*", "")

	// all the needed fields, unneeded fields intersect with src
	f("field_names as f1", "*", "s1,f1,f2", "*", "")

	// needed fields do not intersect with src
	f("field_names as f3", "f1,f2", "", "*", "")

	// needed fields intersect with src
	f("field_names as f1", "s1,f1,f2", "", "*", "")
}
