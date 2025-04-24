package logstorage

import (
	"testing"
)

func TestParsePipeSampleSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`sample 10`)
	f(`sample 10000`)
}

func TestParsePipeSampleFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`sample`)
	f(`sample 0`)
	f(`sample -1`)
	f(`sample foo`)
}

func TestPipeSampleUpdateNeededFields(t *testing.T) {
	f := func(s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("sample 10", "*", "", "*", "")

	// all the needed fields, plus unneeded fields
	f("sample 10", "*", "f1,f2", "*", "f1,f2")

	// needed fields
	f("sample 10", "f1,f2", "", "f1,f2", "")
}
