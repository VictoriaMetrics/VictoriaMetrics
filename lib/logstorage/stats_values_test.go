package logstorage

import (
	"reflect"
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

func TestStatsValues_ExportImportState(t *testing.T) {
	var a chunkedAllocator
	newStatsValuesProcessor := func() *statsValuesProcessor {
		return a.newStatsValuesProcessor()
	}

	f := func(svp *statsValuesProcessor, dataLenExpected, stateSizeExpected int) {
		t.Helper()

		data := svp.exportState(nil, nil)
		dataLen := len(data)
		if dataLen != dataLenExpected {
			t.Fatalf("unexpected dataLen; got %d; want %d", dataLen, dataLenExpected)
		}

		svp2 := newStatsValuesProcessor()
		stateSize, err := svp2.importState(data, nil)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if stateSize != stateSizeExpected {
			t.Fatalf("unexpected state size; got %d bytes; want %d bytes", stateSize, stateSizeExpected)
		}

		if !reflect.DeepEqual(svp, svp2) {
			t.Fatalf("unexpected state imported\ngot\n%#v\nwant\n%#v", svp2, svp)
		}
	}

	// empty state
	svp := newStatsValuesProcessor()
	f(svp, 1, 0)
	/*
	      See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8710
	   	// non-empty state
	   	svp = newStatsValuesProcessor()
	   	svp.values = []string{"foo", "bar", "baz"}
	   	f(svp, 13, 57)*/
}
