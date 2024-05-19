package logstorage

import (
	"testing"
)

func TestPipeRenameUpdateNeededFields(t *testing.T) {
	f := func(s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("rename s1 d1, s2 d2", "*", "", "*", "d1,d2")

	// all the needed fields, unneeded fields do not intersect with src and dst
	f("rename s1 d1, s2 d2", "*", "f1,f2", "*", "d1,d2,f1,f2")

	// all the needed fields, unneeded fields intersect with src
	// mv s1 d1, s2 d2 | rm s1, f1, f2   (d1, d2, f1, f2)
	f("rename s1 d1, s2 d2", "*", "s1,f1,f2", "*", "d1,d2,f1,f2")

	// all the needed fields, unneeded fields intersect with dst
	f("rename s1 d1, s2 d2", "*", "d2,f1,f2", "*", "d1,d2,f1,f2,s2")

	// all the needed fields, unneeded fields intersect with src and dst
	f("rename s1 d1, s2 d2", "*", "s1,d1,f1,f2", "*", "d1,d2,f1,f2,s1")
	f("rename s1 d1, s2 d2", "*", "s1,d2,f1,f2", "*", "d1,d2,f1,f2,s2")

	// needed fields do not intersect with src and dst
	f("rename s1 d1, s2 d2", "f1,f2", "", "f1,f2", "")

	// needed fields intersect with src
	f("rename s1 d1, s2 d2", "s1,f1,f2", "", "f1,f2", "")

	// needed fields intersect with dst
	f("rename s1 d1, s2 d2", "d1,f1,f2", "", "f1,f2,s1", "")

	// needed fields intersect with src and dst
	f("rename s1 d1, s2 d2", "s1,d1,f1,f2", "", "s1,f1,f2", "")
	f("rename s1 d1, s2 d2", "s1,d2,f1,f2", "", "s2,f1,f2", "")
	f("rename s1 d1, s2 d2", "s2,d1,f1,f2", "", "s1,f1,f2", "")
}
