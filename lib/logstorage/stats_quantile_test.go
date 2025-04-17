package logstorage

import (
	"reflect"
	"testing"
)

func TestParseStatsQuantileSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncSuccess(t, pipeStr)
	}

	f(`quantile(0.3)`)
	f(`quantile(1, a)`)
	f(`quantile(0.99, a, b)`)
}

func TestParseStatsQuantileFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncFailure(t, pipeStr)
	}

	f(`quantile`)
	f(`quantile(a)`)
	f(`quantile(a, b)`)
	f(`quantile(10, b)`)
	f(`quantile(-1, b)`)
	f(`quantile(0.5, b) c`)
}

func TestStatsQuantile(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("stats quantile(0.9) as x", [][]Field{
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
			{"x", "def"},
		},
	})

	f("stats quantile(0.9, a) as x", [][]Field{
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
			{"x", "3"},
		},
	})

	f("stats quantile(0.9, a, b) as x", [][]Field{
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
			{"x", "54"},
		},
	})

	f("stats quantile(0.9, b) as x", [][]Field{
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
			{"x", "54"},
		},
	})

	f("stats quantile(0.9, c) as x", [][]Field{
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
			{"x", ""},
		},
	})

	f("stats quantile(0.9, a) if (b:*) as x", [][]Field{
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
			{"x", "3"},
		},
	})

	f("stats by (b) quantile(0.9, a) if (b:*) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
			{"b", "3"},
		},
		{
			{"a", `3`},
			{"c", `54`},
		},
	}, [][]Field{
		{
			{"b", "3"},
			{"x", "2"},
		},
		{
			{"b", ""},
			{"x", ""},
		},
	})

	f("stats by (a) quantile(0.9, b) as x", [][]Field{
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
			{"x", "3"},
		},
		{
			{"a", "3"},
			{"x", "7"},
		},
	})

	f("stats by (a) quantile(0.9) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
			{"c", "3"},
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
			{"x", "def"},
		},
		{
			{"a", "3"},
			{"x", "7"},
		},
	})

	f("stats by (a) quantile(0.9, c) as x", [][]Field{
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
			{"c", `5`},
		},
		{
			{"a", `3`},
			{"b", `7`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"x", ""},
		},
		{
			{"a", "3"},
			{"x", "5"},
		},
	})

	f("stats by (a) quantile(0.9, a, b, c) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
			{"c", "3"},
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
			{"x", "3"},
		},
		{
			{"a", "3"},
			{"x", "7"},
		},
	})

	f("stats by (a, b) quantile(0.9, a) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
			{"c", "3"},
		},
		{
			{"a", `3`},
			{"b", `5`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"b", "3"},
			{"x", "1"},
		},
		{
			{"a", "1"},
			{"b", ""},
			{"x", "1"},
		},
		{
			{"a", "3"},
			{"b", "5"},
			{"x", "3"},
		},
	})

	f("stats by (a, b) quantile(0.9, c) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
			{"c", "3"},
		},
		{
			{"a", `3`},
			{"b", `5`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"b", "3"},
			{"x", ""},
		},
		{
			{"a", "1"},
			{"b", ""},
			{"x", "3"},
		},
		{
			{"a", "3"},
			{"b", "5"},
			{"x", ""},
		},
	})
}

func TestHistogramQuantile(t *testing.T) {
	f := func(a []string, phi float64, qExpected string) {
		t.Helper()

		var h histogram
		for _, v := range a {
			h.update(v)
		}
		q := h.quantile(phi)

		if q != qExpected {
			t.Fatalf("unexpected result for q=%v, phi=%v; got %q; want %q", a, phi, q, qExpected)
		}
	}

	f(nil, -1, "")
	f(nil, 0, "")
	f(nil, 0.5, "")
	f(nil, 1, "")
	f(nil, 10, "")

	f([]string{"123"}, -1, "123")
	f([]string{"123"}, 0, "123")
	f([]string{"123"}, 0.5, "123")
	f([]string{"123"}, 1, "123")
	f([]string{"123"}, 10, "123")

	f([]string{"5", "1"}, -1, "1")
	f([]string{"5", "10"}, 0, "5")
	f([]string{"5", "1"}, 0.5-1e-5, "1")
	f([]string{"5", "1"}, 0.5, "5")
	f([]string{"5", "10"}, 1, "10")
	f([]string{"5", "1"}, 10, "5")

	f([]string{"5", "1", "3"}, -1, "1")
	f([]string{"5", "10", "3"}, 0, "3")
	f([]string{"5", "10", "3"}, 1.0/3-1e-5, "3")
	f([]string{"5", "1", "3"}, 1.0/3, "3")
	f([]string{"5", "1", "3"}, 2.0/3-1e-5, "3")
	f([]string{"5", "1", "3"}, 2.0/3, "5")
	f([]string{"5", "1", "3"}, 1-1e-5, "5")
	f([]string{"5", "1", "3"}, 1, "5")
	f([]string{"10", "5", "3"}, 10, "10")
}

func TestStatsQuantile_ExportImportState(t *testing.T) {
	f := func(sqp *statsQuantileProcessor, dataLenExpected, stateSizeExpected int) {
		t.Helper()

		data := sqp.exportState(nil, nil)
		dataLen := len(data)
		if dataLen != dataLenExpected {
			t.Fatalf("unexpected dataLen; got %d; want %d", dataLen, dataLenExpected)
		}

		var sqp2 statsQuantileProcessor
		stateSize, err := sqp2.importState(data, nil)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if stateSize != stateSizeExpected {
			t.Fatalf("unexpected state size; got %d bytes; want %d bytes", stateSize, stateSizeExpected)
		}

		if !reflect.DeepEqual(sqp, &sqp2) {
			t.Fatalf("unexpected state imported; got %#v; want %#v", &sqp2, sqp)
		}
	}

	var sqp statsQuantileProcessor

	// zero state
	f(&sqp, 4, 0)
	/*
	      See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8710
	   	// non-zero state
	   	sqp = statsQuantileProcessor{}
	   	sqp.h.update("foo")
	   	sqp.h.update("bar")
	   	sqp.h.update("baz")
	   	f(&sqp, 22, 63)*/
}
