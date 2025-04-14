package logstorage

import (
	"reflect"
	"testing"
)

func TestParseStatsRowMaxSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncSuccess(t, pipeStr)
	}

	f(`row_max(foo)`)
	f(`row_max(foo, bar)`)
	f(`row_max(foo, bar, baz)`)
}

func TestParseStatsRowMaxFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncFailure(t, pipeStr)
	}

	f(`row_max`)
	f(`row_max()`)
	f(`row_max(x) bar`)
}

func TestStatsRowMax(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("stats row_max(a) as x", [][]Field{
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
			{"x", `{"a":"3","b":"54"}`},
		},
	})

	f("stats row_max(foo) as x", [][]Field{
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

	f("stats row_max(b, a) as x", [][]Field{
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
			{"x", `{"a":"3"}`},
		},
	})

	f("stats row_max(b, a, x, b) as x", [][]Field{
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
			{"x", `{"a":"3","b":"54"}`},
		},
	})

	f("stats row_max(a) if (b:*) as x", [][]Field{
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
			{"x", `{"a":"3","b":"54"}`},
		},
	})

	f("stats by (b) row_max(a) if (b:*) as x", [][]Field{
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
			{"x", `{"_msg":"abc","a":"2","b":"3"}`},
		},
		{
			{"b", ""},
			{"x", `{}`},
		},
	})

	f("stats by (a) row_max(b) as x", [][]Field{
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
			{"x", `{"a":"3","b":"7"}`},
		},
	})

	f("stats by (a) row_max(c) as x", [][]Field{
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

	f("stats by (a) row_max(b, c) as x", [][]Field{
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
			{"c", "bar"},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"x", `{}`},
		},
		{
			{"a", "3"},
			{"x", `{"c":"bar"}`},
		},
	})

	f("stats by (a, b) row_max(c) as x", [][]Field{
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

func TestStatsRowMax_ExportImportState(t *testing.T) {
	f := func(smp *statsRowMaxProcessor, dataLenExpected, stateSizeExpected int) {
		t.Helper()

		data := smp.exportState(nil, nil)
		dataLen := len(data)
		if dataLen != dataLenExpected {
			t.Fatalf("unexpected dataLen; got %d; want %d", dataLen, dataLenExpected)
		}

		var smp2 statsRowMaxProcessor
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

	var smp statsRowMaxProcessor

	// zero state
	f(&smp, 2, 0)
	/*
	      See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8710
	   	// non-zero state
	   	smp = statsRowMaxProcessor{
	   		max: "abcded",

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
