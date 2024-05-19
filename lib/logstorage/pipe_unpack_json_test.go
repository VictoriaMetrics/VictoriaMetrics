package logstorage

import (
	"testing"
)

func TestPipeUnpackJSONUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()

		nfs := newTestFieldsSet(neededFields)
		unfs := newTestFieldsSet(unneededFields)

		lex := newLexer(s)
		p, err := parsePipeUnpackJSON(lex)
		if err != nil {
			t.Fatalf("cannot parse %s: %s", s, err)
		}
		p.updateNeededFields(nfs, unfs)

		assertNeededFields(t, nfs, unfs, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("unpack_json from x", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("unpack_json from x", "*", "f1,f2", "*", "f1,f2")

	// all the needed fields, unneeded fields intersect with src
	f("unpack_json from x", "*", "f2,x", "*", "f2")

	// needed fields do not intersect with src
	f("unpack_json from x", "f1,f2", "", "f1,f2,x", "")

	// needed fields intersect with src
	f("unpack_json from x", "f2,x", "", "f2,x", "")
}
