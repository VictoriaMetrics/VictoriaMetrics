package logstorage

import (
	"testing"
)

func TestParsePipeBlockStatsSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`block_stats`)
}

func TestParsePipeBlockStatsFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`block_stats foo`)
	f(`block_stats ()`)
	f(`block_stats (foo)`)
}

func TestPipeBlockStatsUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("block_stats", "*", "", "*", "")

	// all the needed fields, plus unneeded fields
	f("block_stats", "*", "f1,f2", "*", "")

	// needed fields
	f("block_stats", "f1,f2", "", "*", "")
}
