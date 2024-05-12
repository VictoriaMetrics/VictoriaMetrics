package logstorage

import (
	"testing"
)

func TestPipeStatsUpdateNeededFields(t *testing.T) {
	f := func(s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()

		nfs := newTestFieldsSet(neededFields)
		unfs := newTestFieldsSet(unneededFields)

		lex := newLexer(s)
		p, err := parsePipeStats(lex)
		if err != nil {
			t.Fatalf("unexpected error when parsing %s: %s", s, err)
		}
		p.updateNeededFields(nfs, unfs)

		assertNeededFields(t, nfs, unfs, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("stats count() r1", "*", "", "", "")
	f("stats count(*) r1", "*", "", "", "")
	f("stats count(f1,f2) r1", "*", "", "f1,f2", "")
	f("stats count(f1,f2) r1, sum(f3,f4) r2", "*", "", "f1,f2,f3,f4", "")
	f("stats by (b1,b2) count(f1,f2) r1", "*", "", "b1,b2,f1,f2", "")
	f("stats by (b1,b2) count(f1,f2) r1, count(f1,f3) r2", "*", "", "b1,b2,f1,f2,f3", "")

	// all the needed fields, unneeded fields do not intersect with stats fields
	f("stats count() r1", "*", "f1,f2", "", "")
	f("stats count(*) r1", "*", "f1,f2", "", "")
	f("stats count(f1,f2) r1", "*", "f3,f4", "f1,f2", "")
	f("stats count(f1,f2) r1, sum(f3,f4) r2", "*", "f5,f6", "f1,f2,f3,f4", "")
	f("stats by (b1,b2) count(f1,f2) r1", "*", "f3,f4", "b1,b2,f1,f2", "")
	f("stats by (b1,b2) count(f1,f2) r1, count(f1,f3) r2", "*", "f4,f5", "b1,b2,f1,f2,f3", "")

	// all the needed fields, unneeded fields intersect with stats fields
	f("stats count() r1", "*", "r1,r2", "", "")
	f("stats count(*) r1", "*", "r1,r2", "", "")
	f("stats count(f1,f2) r1", "*", "r1,r2", "", "")
	f("stats count(f1,f2) r1, sum(f3,f4) r2", "*", "r1,r3", "f3,f4", "")
	f("stats by (b1,b2) count(f1,f2) r1", "*", "r1,r2", "b1,b2", "")
	f("stats by (b1,b2) count(f1,f2) r1", "*", "r1,r2,b1", "b1,b2", "")
	f("stats by (b1,b2) count(f1,f2) r1", "*", "r1,r2,b1,b2", "", "")
	f("stats by (b1,b2) count(f1,f2) r1, count(f1,f3) r2", "*", "r1,r3", "b1,b2,f1,f3", "")

	// needed fields do not intersect with stats fields
	f("stats count() r1", "r2", "", "", "")
	f("stats count(*) r1", "r2", "", "", "")
	f("stats count(f1,f2) r1", "r2", "", "", "")
	f("stats count(f1,f2) r1, sum(f3,f4) r2", "r3", "", "", "")
	f("stats by (b1,b2) count(f1,f2) r1", "r2", "", "", "")
	f("stats by (b1,b2) count(f1,f2) r1, count(f1,f3) r2", "r3", "", "", "")

	// needed fields intersect with stats fields
	f("stats count() r1", "r1,r2", "", "", "")
	f("stats count(*) r1", "r1,r2", "", "", "")
	f("stats count(f1,f2) r1", "r1,r2", "", "f1,f2", "")
	f("stats count(f1,f2) r1, sum(f3,f4) r2", "r1,r3", "", "f1,f2", "")
	f("stats by (b1,b2) count(f1,f2) r1", "r1,r2", "", "b1,b2,f1,f2", "")
	f("stats by (b1,b2) count(f1,f2) r1, count(f1,f3) r2", "r1,r3", "", "b1,b2,f1,f2", "")
}
