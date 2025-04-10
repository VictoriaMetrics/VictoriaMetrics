package logstorage

import (
	"reflect"
	"testing"
)

func TestParseStatsRateSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncSuccess(t, pipeStr)
	}

	f(`rate()`)
}

func TestParseStatsRateFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncFailure(t, pipeStr)
	}

	f(`rate`)
	f(`rate(x)`)
	f(`rate() y`)
}

func TestStatsRate(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("stats rate() as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
		},
		{},
		{
			{"a", `3`},
			{"b", `54`},
		},
	}, [][]Field{
		{
			{"x", "4"},
		},
	})
}

func TestStatsRate_ExportImportState(t *testing.T) {
	f := func(srp *statsRateProcessor, dataLenExpected, stateSizeExpected int) {
		t.Helper()

		data := srp.exportState(nil, nil)
		dataLen := len(data)
		if dataLen != dataLenExpected {
			t.Fatalf("unexpected dataLen; got %d; want %d", dataLen, dataLenExpected)
		}

		var srp2 statsRateProcessor
		stateSize, err := srp2.importState(data, nil)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if stateSize != stateSizeExpected {
			t.Fatalf("unexpected state size; got %d bytes; want %d bytes", stateSize, stateSizeExpected)
		}

		if !reflect.DeepEqual(srp, &srp2) {
			t.Fatalf("unexpected state imported; got %#v; want %#v", &srp2, srp)
		}
	}

	var srp statsRateProcessor

	f(&srp, 1, 0)

	srp = statsRateProcessor{
		rowsCount: 234,
	}
	f(&srp, 2, 0)
}
