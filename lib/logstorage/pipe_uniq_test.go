package logstorage

import (
	"testing"
)

func TestPipeUniqUpdateNeededFields(t *testing.T) {
	f := func(s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("uniq", "*", "", "*", "")
	f("uniq by()", "*", "", "*", "")
	f("uniq by(*)", "*", "", "*", "")
	f("uniq by(f1,f2)", "*", "", "f1,f2", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("uniq by(s1, s2)", "*", "f1,f2", "s1,s2", "")
	f("uniq", "*", "f1,f2", "*", "")

	// all the needed fields, unneeded fields intersect with src
	f("uniq by(s1, s2)", "*", "s1,f1,f2", "s1,s2", "")
	f("uniq by(*)", "*", "s1,f1,f2", "*", "")
	f("uniq by(s1, s2)", "*", "s1,s2,f1", "s1,s2", "")

	// needed fields do not intersect with src
	f("uniq by (s1, s2)", "f1,f2", "", "s1,s2", "")

	// needed fields intersect with src
	f("uniq by (s1, s2)", "s1,f1,f2", "", "s1,s2", "")
	f("uniq by (*)", "s1,f1,f2", "", "*", "")
}
