package logstorage

import (
	"reflect"
	"testing"
)

func TestParseStatsCountUniqHashSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncSuccess(t, pipeStr)
	}

	f(`count_uniq_hash(*)`)
	f(`count_uniq_hash(a)`)
	f(`count_uniq_hash(a, b)`)
	f(`count_uniq_hash(*) limit 10`)
	f(`count_uniq_hash(a) limit 20`)
	f(`count_uniq_hash(a, b) limit 5`)
}

func TestParseStatsCountUniqHashFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncFailure(t, pipeStr)
	}

	f(`count_uniq_hash`)
	f(`count_uniq_hash(a b)`)
	f(`count_uniq_hash(x) y`)
	f(`count_uniq_hash(x) limit`)
	f(`count_uniq_hash(x) limit N`)
}

func TestStatsCountUniqHash(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("stats count_uniq_hash(*) as x", [][]Field{
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

	f("stats count_uniq_hash(*) limit 2 as x", [][]Field{
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

	f("stats count_uniq_hash(*) limit 10 as x", [][]Field{
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

	f("stats count_uniq_hash(b) as x", [][]Field{
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

	f("stats count_uniq_hash(a, b) as x", [][]Field{
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

	f("stats count_uniq_hash(c) as x", [][]Field{
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

	f("stats count_uniq_hash(a) if (b:*) as x", [][]Field{
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

	f("stats by (a) count_uniq_hash(b) as x", [][]Field{
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

	f("stats by (a) count_uniq_hash(b) if (!c:foo) as x", [][]Field{
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

	f("stats by (a) count_uniq_hash(*) as x", [][]Field{
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

	f("stats by (a) count_uniq_hash(c) as x", [][]Field{
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

	f("stats by (a) count_uniq_hash(a, b, c) as x", [][]Field{
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

	f("stats by (a, b) count_uniq_hash(a) as x", [][]Field{
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

func TestStatsCountUniqHash_ExportImportState(t *testing.T) {
	newStatsCountUniqHashProcessor := func() *statsCountUniqHashProcessor {
		sup := &statsCountUniqHashProcessor{
			concurrency: 2,
		}
		return sup
	}

	f := func(sup *statsCountUniqHashProcessor, dataLenExpected, stateSizeExpected, entriesCountExpected int) {
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

		sup2 := newStatsCountUniqHashProcessor()
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

	sup := newStatsCountUniqHashProcessor()

	// Zero state
	f(sup, 5, 0, 0)

	// uniqValues initialized
	sup = newStatsCountUniqHashProcessor()
	sup.uniqValues = statsCountUniqHashSet{
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
		strings: map[uint64]struct{}{
			1111: {},
			2222: {},
		},
	}
	f(sup, 53, 48, 6)
	/*
	      See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8710
	   	// shards initialized
	   	sup = newStatsCountUniqHashProcessor()
	   	sup.shards = []statsCountUniqHashSet{
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
	   			strings: map[uint64]struct{}{
	   				1111: {},
	   				2222: {},
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
	   	f(sup, 89, 144, 10)

	   	// shardss initialized
	   	sup = newStatsCountUniqHashProcessor()
	   	sup.shardss = [][]statsCountUniqHashSet{
	   		{
	   			{
	   				strings: map[uint64]struct{}{
	   					11111: {},
	   					22222: {},
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
	   				strings: map[uint64]struct{}{
	   					111:  {},
	   					222:  {},
	   					3333: {},
	   				},
	   			},
	   			{
	   				timestamps: map[uint64]struct{}{
	   					10: {},
	   				},
	   			},
	   		},
	   	}
	   	f(sup, 105, 160, 12)*/
}
