package logstorage

import (
	"testing"
)

func TestParsePipeUnionSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`union (*)`)
	f(`union (foo)`)
	f(`union (foo | union (bar | stats count(*) as x))`)
}

func TestParsePipeUnionFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`union`)
	f(`union()`)
	f(`union(foo | count)`)
	f(`union (foo) bar`)
}

func TestPipeUnionUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("union (abc)", "*", "", "*", "")

	// all the needed fields, non-empty unneeded fields
	f("union (abc)", "*", "f1,f2", "*", "f1,f2")

	// non-empty needed fields
	f("union (abc)", "f1,f2", "", "f1,f2", "")
}
