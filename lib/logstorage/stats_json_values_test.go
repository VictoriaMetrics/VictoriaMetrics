package logstorage

import (
	"reflect"
	"testing"
)

func TestParseStatsJSONValuesSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncSuccess(t, pipeStr)
	}

	f(`json_values(*)`)
	f(`json_values(a)`)
	f(`json_values(a, b)`)
	f(`json_values(a, b) limit 10`)
}

func TestParseStatsJSONValuesFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncFailure(t, pipeStr)
	}

	f(`json_values`)
	f(`json_values(a b)`)
	f(`json_values(x) y`)
	f(`json_values(a, b) limit`)
	f(`json_values(a, b) limit foo`)
}

func TestStatsJSONValues_ExportImportState(t *testing.T) {
	var a chunkedAllocator
	newStatsJSONValuesProcessor := func() *statsJSONValuesProcessor {
		sjp := a.newStatsJSONValuesProcessor()
		sjp.a = &a
		return sjp
	}

	f := func(sjp *statsJSONValuesProcessor, dataLenExpected, stateSizeExpected int) {
		t.Helper()

		data := sjp.exportState(nil, nil)
		dataLen := len(data)
		if dataLen != dataLenExpected {
			t.Fatalf("unexpected dataLen; got %d; want %d", dataLen, dataLenExpected)
		}

		sjp2 := newStatsJSONValuesProcessor()
		stateSize, err := sjp2.importState(data, nil)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if stateSize != stateSizeExpected {
			t.Fatalf("unexpected state size; got %d bytes; want %d bytes", stateSize, stateSizeExpected)
		}

		sjp.a = nil
		sjp2.a = nil
		if !reflect.DeepEqual(sjp, sjp2) {
			t.Fatalf("unexpected state imported\ngot\n%#v\nwant\n%#v", sjp2, sjp)
		}
	}

	// empty state
	sjp := newStatsJSONValuesProcessor()
	f(sjp, 1, 0)
	/*
	      See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8710
	   	// non-empty state
	   	sjp = newStatsJSONValuesProcessor()
	   	sjp.values = []string{"foo", "bar", "baz"}
	   	f(sjp, 13, 57)*/
}
