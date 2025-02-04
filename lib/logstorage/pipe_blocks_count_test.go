package logstorage

import (
	"testing"
)

func TestParsePipeBlocksCountSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`blocks_count`)
	f(`blocks_count as x`)
}

func TestParsePipeBlocksCountFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`blocks_count(foo)`)
	f(`blocks_count a b`)
	f(`blocks_count as`)
}

func TestPipeBlocksCountUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("blocks_count as f1", "*", "", "", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("blocks_count as f3", "*", "f1,f2", "", "")

	// all the needed fields, unneeded fields intersect with src
	f("blocks_count as f1", "*", "s1,f1,f2", "", "")

	// needed fields do not intersect with src
	f("blocks_count as f3", "f1,f2", "", "", "")

	// needed fields intersect with src
	f("blocks_count as f1", "s1,f1,f2", "", "", "")
}
