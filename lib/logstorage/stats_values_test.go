package logstorage

import (
	"testing"
)

func TestParseStatsValuesSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncSuccess(t, pipeStr)
	}

	f(`values(*)`)
	f(`values(a)`)
	f(`values(a, b)`)
	f(`values(a, b) limit 10`)
}

func TestParseStatsValuesFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncFailure(t, pipeStr)
	}

	f(`values`)
	f(`values(a b)`)
	f(`values(x) y`)
	f(`values(a, b) limit`)
	f(`values(a, b) limit foo`)
}
