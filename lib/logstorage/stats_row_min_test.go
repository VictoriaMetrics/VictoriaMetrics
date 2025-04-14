package logstorage

import (
	"reflect"
	"testing"
)

func TestParseStatsRowMinSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncSuccess(t, pipeStr)
	}

	f(`row_min(foo)`)
	f(`row_min(foo, bar)`)
	f(`row_min(foo, bar, baz)`)
}

func TestParseStatsRowMinFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncFailure(t, pipeStr)
	}

	f(`row_min`)
	f(`row_min()`)
	f(`row_min(x) bar`)
}

func TestStatsRowMin(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("stats row_min(a) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
		},
		{
			{"a", `3`},
			{"b", `54`},
		},
	}, [][]Field{
		{
			{"x", `{"_msg":"def","a":"1"}`},
		},
	})

	f("stats row_min(foo) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
		},
		{
			{"a", `3`},
			{"b", `54`},
		},
	}, [][]Field{
		{
			{"x", `{}`},
		},
	})

	f("stats row_min(b, a) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
		},
		{
			{"a", `3`},
			{"b", `54`},
			{"c", "1232"},
		},
	}, [][]Field{
		{
			{"x", `{"a":"2"}`},
		},
	})

	f("stats row_min(b, a, x, b) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
		},
		{
			{"a", `3`},
			{"b", `54`},
			{"c", "1232"},
		},
	}, [][]Field{
		{
			{"x", `{"a":"2","b":"3"}`},
		},
	})

	f("stats row_min(a) if (b:*) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
		},
		{
			{"a", `3`},
			{"b", `54`},
		},
	}, [][]Field{
		{
			{"x", `{"_msg":"abc","a":"2","b":"3"}`},
		},
	})

	f("stats by (b) row_min(a) if (b:*) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `-12.34`},
			{"b", "3"},
		},
		{
			{"a", `3`},
			{"c", `54`},
		},
	}, [][]Field{
		{
			{"b", "3"},
			{"x", `{"_msg":"def","a":"-12.34","b":"3"}`},
		},
		{
			{"b", ""},
			{"x", `{}`},
		},
	})

	f("stats by (a) row_min(b) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
		},
		{
			{"a", `3`},
			{"b", `5`},
		},
		{
			{"a", `3`},
			{"b", `7`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"x", `{"_msg":"abc","a":"1","b":"3"}`},
		},
		{
			{"a", "3"},
			{"x", `{"a":"3","b":"5"}`},
		},
	})

	f("stats by (a) row_min(c) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
		},
		{
			{"a", `3`},
			{"c", `foo`},
		},
		{
			{"a", `3`},
			{"b", `7`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"x", `{}`},
		},
		{
			{"a", "3"},
			{"x", `{"a":"3","c":"foo"}`},
		},
	})

	f("stats by (a) row_min(b, c) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `34`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
			{"c", "3"},
		},
		{
			{"a", `3`},
			{"b", `5`},
			{"c", "foo"},
		},
		{
			{"a", `3`},
			{"b", `7`},
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

	f("stats by (a, b) row_min(c) as x", [][]Field{
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
			{"x", `{"_msg":"def","a":"1","c":"foo"}`},
		},
		{
			{"a", "3"},
			{"b", "5"},
			{"x", `{"a":"3","b":"5","c":"4"}`},
		},
	})
}

func TestStatsRowMin_ExportImportState(t *testing.T) {
	f := func(smp *statsRowMinProcessor, dataLenExpected, stateSizeExpected int) {
		t.Helper()

		data := smp.exportState(nil, nil)
		dataLen := len(data)
		if dataLen != dataLenExpected {
			t.Fatalf("unexpected dataLen; got %d; want %d", dataLen, dataLenExpected)
		}

		var smp2 statsRowMinProcessor
		stateSize, err := smp2.importState(data, nil)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if stateSize != stateSizeExpected {
			t.Fatalf("unexpected state size; got %d bytes; want %d bytes", stateSize, stateSizeExpected)
		}

		if !reflect.DeepEqual(smp, &smp2) {
			t.Fatalf("unexpected state imported; got %#v; want %#v", &smp2, smp)
		}
	}

	var smp statsRowMinProcessor

	// zero state
	f(&smp, 2, 0)
	/*
	      See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8710
	   	// non-zero state
	   	smp = statsRowMinProcessor{
	   		min: "abcded",

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
	   	f(&smp, 23, 81)*/
}
