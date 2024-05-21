package logstorage

import (
	"testing"
)

func TestPipeSortUpdateNeededFields(t *testing.T) {
	f := func(s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("sort by(s1,s2)", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("sort by(s1,s2)", "*", "f1,f2", "*", "f1,f2")

	// all the needed fields, unneeded fields intersect with src
	f("sort by(s1,s2)", "*", "s1,f1,f2", "*", "f1,f2")

	// needed fields do not intersect with src
	f("sort by(s1,s2)", "f1,f2", "", "s1,s2,f1,f2", "")

	// needed fields intersect with src
	f("sort by(s1,s2)", "s1,f1,f2", "", "s1,s2,f1,f2", "")
}
