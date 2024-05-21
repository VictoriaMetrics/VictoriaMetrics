package logstorage

import (
	"testing"
)

func TestPipeOffsetUpdateNeededFields(t *testing.T) {
	f := func(s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("offset 10", "*", "", "*", "")

	// all the needed fields, plus unneeded fields
	f("offset 10", "*", "f1,f2", "*", "f1,f2")

	// needed fields
	f("offset 10", "f1,f2", "", "f1,f2", "")
}
