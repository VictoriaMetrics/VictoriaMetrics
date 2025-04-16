package logstorage

import (
	"reflect"
	"testing"
)

func TestParseStatsRowAnySuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncSuccess(t, pipeStr)
	}

	f(`row_any(*)`)
	f(`row_any(foo)`)
	f(`row_any(foo, bar)`)
}

func TestParseStatsRowAnyFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncFailure(t, pipeStr)
	}

	f(`row_any`)
	f(`row_any(x) bar`)
}

func TestStatsRowAny(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("row_any()", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
	}, [][]Field{
		{
			{"row_any(*)", `{"_msg":"abc","a":"2","b":"3"}`},
		},
	})

	f("stats row_any(a) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
	}, [][]Field{
		{
			{"x", `{"a":"2"}`},
		},
	})

	f("stats row_any(a, x, b) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
	}, [][]Field{
		{
			{"x", `{"a":"2","b":"3"}`},
		},
	})

	f("stats row_any(a) if (b:'') as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
		},
	}, [][]Field{
		{
			{"x", `{"a":"1"}`},
		},
	})

	f("stats by (b) row_any(a) if (b:*) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"a", `3`},
			{"c", `54`},
		},
	}, [][]Field{
		{
			{"b", "3"},
			{"x", `{"a":"2"}`},
		},
		{
			{"b", ""},
			{"x", `{}`},
		},
	})

	f("stats by (a) row_any(b) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"a", `3`},
			{"b", `5`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"x", `{"b":"3"}`},
		},
		{
			{"a", "3"},
			{"x", `{"b":"5"}`},
		},
	})

	f("stats by (a) row_any(c) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"a", `3`},
			{"c", `foo`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"x", `{}`},
		},
		{
			{"a", "3"},
			{"x", `{"c":"foo"}`},
		},
	})

	f("stats by (a, b) row_any(c) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
			{"c", "foo"},
		},
		{
			{"a", `3`},
			{"b", `5`},
			{"c", "4"},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"b", "3"},
			{"x", `{}`},
		},
		{
			{"a", "1"},
			{"b", ""},
			{"x", `{"c":"foo"}`},
		},
		{
			{"a", "3"},
			{"b", "5"},
			{"x", `{"c":"4"}`},
		},
	})
}

func TestStatsRowAny_ExportImportState(t *testing.T) {
	f := func(sap *statsRowAnyProcessor, dataLenExpected, stateSizeExpected int) {
		t.Helper()

		data := sap.exportState(nil, nil)
		dataLen := len(data)
		if dataLen != dataLenExpected {
			t.Fatalf("unexpected dataLen; got %d; want %d", dataLen, dataLenExpected)
		}

		var sap2 statsRowAnyProcessor
		stateSize, err := sap2.importState(data, nil)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if stateSize != stateSizeExpected {
			t.Fatalf("unexpected state size; got %d bytes; want %d bytes", stateSize, stateSizeExpected)
		}

		if !reflect.DeepEqual(sap, &sap2) {
			t.Fatalf("unexpected state imported; got %#v; want %#v", &sap2, sap)
		}
	}

	var sap statsRowAnyProcessor

	// zero state
	f(&sap, 1, 0)
	/*
	      See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8710
	   	// non-zero state
	   	sap = statsRowAnyProcessor{
	   		captured: true,

	   		fields: []Field{
	   			{
	   				Name:  "foo",
	   				Value: "bar",
	   			},
	   			{
	   				Name:  "abc",
	   				Value: "de",
	   			},
	   		},
	   	}
	   	f(&sap, 17, 75)*/
}
