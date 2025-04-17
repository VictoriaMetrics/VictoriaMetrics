package logstorage

import (
	"reflect"
	"testing"
)

func TestParseStatsCountUniqSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncSuccess(t, pipeStr)
	}

	f(`count_uniq(*)`)
	f(`count_uniq(a)`)
	f(`count_uniq(a, b)`)
	f(`count_uniq(*) limit 10`)
	f(`count_uniq(a) limit 20`)
	f(`count_uniq(a, b) limit 5`)
}

func TestParseStatsCountUniqFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncFailure(t, pipeStr)
	}

	f(`count_uniq`)
	f(`count_uniq(a b)`)
	f(`count_uniq(x) y`)
	f(`count_uniq(x) limit`)
	f(`count_uniq(x) limit N`)
}

func TestStatsCountUniq(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("stats count_uniq(*) as x", [][]Field{
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
			{"x", "3"},
		},
	})

	f("stats count_uniq(*) limit 2 as x", [][]Field{
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
			{"x", "2"},
		},
	})

	f("stats count_uniq(*) limit 10 as x", [][]Field{
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
			{"x", "3"},
		},
	})

	f("stats count_uniq(b) as x", [][]Field{
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
			{"x", "2"},
		},
	})

	f("stats count_uniq(a, b) as x", [][]Field{
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
			{"aa", `3`},
			{"bb", `54`},
		},
	}, [][]Field{
		{
			{"x", "2"},
		},
	})

	f("stats count_uniq(c) as x", [][]Field{
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
			{"x", "0"},
		},
	})

	f("stats count_uniq(a) if (b:*) as x", [][]Field{
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
			{"b", `54`},
		},
	}, [][]Field{
		{
			{"x", "1"},
		},
	})

	f("stats by (a) count_uniq(b) as x", [][]Field{
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
			{"x", "1"},
		},
		{
			{"a", "3"},
			{"x", "2"},
		},
	})

	f("stats by (a) count_uniq(b) if (!c:foo) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
			{"b", "aadf"},
			{"c", "foo"},
		},
		{
			{"a", `3`},
			{"b", `5`},
			{"c", "bar"},
		},
		{
			{"a", `3`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"x", "1"},
		},
		{
			{"a", "3"},
			{"x", "1"},
		},
	})

	f("stats by (a) count_uniq(*) as x", [][]Field{
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
		{},
		{
			{"a", `3`},
			{"b", `5`},
		},
	}, [][]Field{
		{
			{"a", ""},
			{"x", "0"},
		},
		{
			{"a", "1"},
			{"x", "2"},
		},
		{
			{"a", "3"},
			{"x", "1"},
		},
	})

	f("stats by (a) count_uniq(c) as x", [][]Field{
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
			{"x", "0"},
		},
		{
			{"a", "3"},
			{"x", "1"},
		},
	})

	f("stats by (a) count_uniq(a, b, c) as x", [][]Field{
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
			{"foo", "bar"},
		},
		{
			{"a", `3`},
			{"b", `7`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"x", "2"},
		},
		{
			{"a", ""},
			{"x", "0"},
		},
		{
			{"a", "3"},
			{"x", "2"},
		},
	})

	f("stats by (a, b) count_uniq(a) as x", [][]Field{
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
			{"c", `3`},
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
			{"a", ""},
			{"b", "5"},
			{"x", "0"},
		},
	})
}

func TestStatsCountUniq_ExportImportState(t *testing.T) {
	var a chunkedAllocator
	newStatsCountUniqProcessor := func() *statsCountUniqProcessor {
		sup := a.newStatsCountUniqProcessor()
		sup.a = &a
		sup.concurrency = 2
		return sup
	}

	f := func(sup *statsCountUniqProcessor, dataLenExpected, stateSizeExpected, entriesCountExpected int) {
		t.Helper()

		data := sup.exportState(nil, nil)
		dataLen := len(data)
		if dataLen != dataLenExpected {
			t.Fatalf("unexpected dataLen; got %d; want %d", dataLen, dataLenExpected)
		}

		entriesCount := sup.entriesCount()
		if entriesCount != uint64(entriesCountExpected) {
			t.Fatalf("unexpected entries count; got %d; want %d", entriesCount, entriesCountExpected)
		}

		sup2 := newStatsCountUniqProcessor()
		stateSize, err := sup2.importState(data, nil)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if stateSize != stateSizeExpected {
			t.Fatalf("unexpected state size; got %d bytes; want %d bytes", stateSize, stateSizeExpected)
		}

		entriesCount = sup2.entriesCount()
		if entriesCount != uint64(entriesCountExpected) {
			t.Fatalf("unexpected items count; got %d; want %d", entriesCount, entriesCountExpected)
		}

		sup.a = nil
		sup2.a = nil
		if !reflect.DeepEqual(sup, sup2) {
			t.Fatalf("unexpected state imported\ngot\n%#v\nwant\n%#v", sup2, sup)
		}
	}

	sup := newStatsCountUniqProcessor()

	// Zero state
	f(sup, 5, 0, 0)
	/*
	      See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8710
	   	// uniqValues initialized
	   	sup = newStatsCountUniqProcessor()
	   	sup.uniqValues = statsCountUniqSet{
	   		timestamps: map[uint64]struct{}{
	   			123: {},
	   			0:   {},
	   		},
	   		u64: map[uint64]struct{}{
	   			43: {},
	   		},
	   		negative64: map[uint64]struct{}{
	   			8234932: {},
	   		},
	   		strings: map[string]struct{}{
	   			"foo": {},
	   			"bar": {},
	   		},
	   	}
	   	f(sup, 45, 70, 6)

	   	// shards initialized
	   	sup = newStatsCountUniqProcessor()
	   	sup.shards = []statsCountUniqSet{
	   		{
	   			timestamps: map[uint64]struct{}{
	   				123: {},
	   				0:   {},
	   			},
	   			u64: map[uint64]struct{}{
	   				43: {},
	   			},
	   			negative64: map[uint64]struct{}{
	   				8234932: {},
	   			},
	   			strings: map[string]struct{}{
	   				"foo": {},
	   				"bar": {},
	   			},
	   		},
	   		{
	   			timestamps: map[uint64]struct{}{
	   				10:      {},
	   				1123:    {},
	   				3234324: {},
	   			},
	   			u64: map[uint64]struct{}{
	   				42: {},
	   			},
	   		},
	   	}
	   	f(sup, 81, 166, 10)

	   	// shardss initialized
	   	sup = newStatsCountUniqProcessor()
	   	sup.shardss = [][]statsCountUniqSet{
	   		{
	   			{
	   				strings: map[string]struct{}{
	   					"afoo": {},
	   					"bar":  {},
	   				},
	   			},
	   			{
	   				negative64: map[uint64]struct{}{
	   					10:      {},
	   					1123:    {},
	   					3234324: {},
	   				},
	   			},
	   		},
	   		{
	   			{
	   				timestamps: map[uint64]struct{}{
	   					123: {},
	   					0:   {},
	   				},
	   				u64: map[uint64]struct{}{
	   					43: {},
	   				},
	   				strings: map[string]struct{}{
	   					"foo": {},
	   					"bar": {},
	   					"baz": {},
	   				},
	   			},
	   			{
	   				timestamps: map[uint64]struct{}{
	   					10: {},
	   				},
	   			},
	   		},
	   	}
	   	f(sup, 82, 197, 11)

	   	// boths shards and shardss initialized
	   	sup = newStatsCountUniqProcessor()
	   	sup.shardss = [][]statsCountUniqSet{
	   		{
	   			{
	   				strings: map[string]struct{}{
	   					"afoo": {},
	   					"bar":  {},
	   				},
	   			},
	   			{
	   				strings: map[string]struct{}{
	   					"foo":  {},
	   					"abar": {},
	   				},
	   			},
	   		},
	   		{
	   			{
	   				strings: map[string]struct{}{
	   					"afoo": {},
	   					"bar":  {},
	   					"baz":  {},
	   				},
	   			},
	   			{
	   				strings: map[string]struct{}{
	   					"foo":  {},
	   					"abar": {},
	   					"abaz": {},
	   				},
	   			},
	   		},
	   	}
	   	sup.shards = []statsCountUniqSet{
	   		{
	   			strings: map[string]struct{}{
	   				"bar": {},
	   			},
	   		},
	   		{
	   			strings: map[string]struct{}{
	   				"foo":   {},
	   				"abarz": {},
	   			},
	   		},
	   	}
	   	f(sup, 42, 202, 7)*/
}
