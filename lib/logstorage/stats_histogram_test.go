package logstorage

import (
	"reflect"
	"testing"
)

func TestParseStatsHistogramSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncSuccess(t, pipeStr)
	}

	f(`histogram(foo)`)
}

func TestParseStatsHistogramFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncFailure(t, pipeStr)
	}

	f(`histogram`)
	f(`histogram(a, b)`)
	f(`histogram(a) abc`)
}

func TestStatsHistogram(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("stats histogram(a) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1.9`},
		},
		{
			{"a", `3.05`},
			{"b", `54`},
		},
	}, [][]Field{
		{
			{"x", `[{"vmrange":"1.896e+00...2.154e+00","hits":2},{"vmrange":"2.783e+00...3.162e+00","hits":1}]`},
		},
	})
}

func TestStatsHistogram_ExportImportState(t *testing.T) {
	f := func(shp *statsHistogramProcessor, stateSizeExpected int) {
		t.Helper()

		data := shp.exportState(nil, nil)

		var shp2 statsHistogramProcessor
		stateSize, err := shp2.importState(data, nil)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if stateSize != stateSizeExpected {
			t.Fatalf("unexpected state size; got %d bytes; want %d bytes", stateSize, stateSizeExpected)
		}

		if !reflect.DeepEqual(shp, &shp2) {
			t.Fatalf("unexpected state imported\ngot\n%#v\nwant\n%#v", &shp2, shp)
		}
	}

	var shp statsHistogramProcessor

	// Zero state
	f(&shp, 0)
	/*
	      See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8710
	   	// Non-zero state
	   	shp = statsHistogramProcessor{
	   		bucketsMap: map[string]uint64{
	   			"1.896e+00...2.154e+00": 2344,
	   			"2.783e+00...3.162e+00": 3289,
	   		},
	   	}
	   	f(&shp, 90)*/
}
