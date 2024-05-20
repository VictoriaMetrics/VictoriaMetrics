package logstorage

import (
	"testing"
)

func TestPipeExtractUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("extract from x '<foo>'", "*", "", "*", "foo")

	// all the needed fields, unneeded fields do not intersect with fromField and output fields
	f("extract from x '<foo>'", "*", "f1,f2", "*", "f1,f2,foo")

	// all the needed fields, unneeded fields intersect with fromField
	f("extract from x '<foo>'", "*", "f2,x", "*", "f2,foo")

	// all the needed fields, unneeded fields intersect with output fields
	f("extract from x '<foo>x<bar>'", "*", "f2,foo", "*", "bar,f2,foo")

	// all the needed fields, unneeded fields intersect with all the output fields
	f("extract from x '<foo>x<bar>'", "*", "f2,foo,bar", "*", "bar,f2,foo,x")

	// needed fields do not intersect with fromField and output fields
	f("extract from x '<foo>x<bar>'", "f1,f2", "", "f1,f2", "")

	// needed fields intersect with fromField
	f("extract from x '<foo>x<bar>'", "f2,x", "", "f2,x", "")

	// needed fields intersect with output fields
	f("extract from x '<foo>x<bar>'", "f2,foo", "", "f2,x", "")

	// needed fields intersect with fromField and output fields
	f("extract from x '<foo>x<bar>'", "f2,foo,x,y", "", "f2,x,y", "")
}
