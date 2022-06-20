package netstorage

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestMergeSortBlocks(t *testing.T) {
	f := func(blocks []*sortBlock, dedupInterval int64, expectedResult *Result) {
		t.Helper()
		var result Result
		mergeSortBlocks(&result, blocks, dedupInterval)
		if !reflect.DeepEqual(result.Values, expectedResult.Values) {
			t.Fatalf("unexpected values;\ngot\n%v\nwant\n%v", result.Values, expectedResult.Values)
		}
		if !reflect.DeepEqual(result.Timestamps, expectedResult.Timestamps) {
			t.Fatalf("unexpected timestamps;\ngot\n%v\nwant\n%v", result.Timestamps, expectedResult.Timestamps)
		}
	}

	// Zero blocks
	f(nil, 1, &Result{})

	// Single block without samples
	f([]*sortBlock{{}}, 1, &Result{})

	// Single block with a single samples.
	f([]*sortBlock{
		{
			Timestamps: []int64{1},
			Values:     []float64{4.2},
		},
	}, 1, &Result{
		Timestamps: []int64{1},
		Values:     []float64{4.2},
	})

	// Single block with multiple samples.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 3},
			Values:     []float64{4.2, 2.1, 10},
		},
	}, 1, &Result{
		Timestamps: []int64{1, 2, 3},
		Values:     []float64{4.2, 2.1, 10},
	})

	// Single block with multiple samples with deduplication.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 3},
			Values:     []float64{4.2, 2.1, 10},
		},
	}, 2, &Result{
		Timestamps: []int64{2, 3},
		Values:     []float64{2.1, 10},
	})

	// Multiple blocks without time range intersection.
	f([]*sortBlock{
		{
			Timestamps: []int64{3, 5},
			Values:     []float64{5.2, 6.1},
		},
		{
			Timestamps: []int64{1, 2},
			Values:     []float64{4.2, 2.1},
		},
	}, 1, &Result{
		Timestamps: []int64{1, 2, 3, 5},
		Values:     []float64{4.2, 2.1, 5.2, 6.1},
	})

	// Multiple blocks with time range intersection.
	f([]*sortBlock{
		{
			Timestamps: []int64{3, 5},
			Values:     []float64{5.2, 6.1},
		},
		{
			Timestamps: []int64{1, 2, 4},
			Values:     []float64{4.2, 2.1, 42},
		},
	}, 1, &Result{
		Timestamps: []int64{1, 2, 3, 4, 5},
		Values:     []float64{4.2, 2.1, 5.2, 42, 6.1},
	})

	// Multiple blocks with time range inclusion.
	f([]*sortBlock{
		{
			Timestamps: []int64{0, 3, 5},
			Values:     []float64{9, 5.2, 6.1},
		},
		{
			Timestamps: []int64{1, 2, 4},
			Values:     []float64{4.2, 2.1, 42},
		},
	}, 1, &Result{
		Timestamps: []int64{0, 1, 2, 3, 4, 5},
		Values:     []float64{9, 4.2, 2.1, 5.2, 42, 6.1},
	})

	// Multiple blocks with identical timestamps.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 4},
			Values:     []float64{9, 5.2, 6.1},
		},
		{
			Timestamps: []int64{1, 2, 4},
			Values:     []float64{4.2, 2.1, 42},
		},
	}, 1, &Result{
		Timestamps: []int64{1, 2, 4},
		Values:     []float64{4.2, 2.1, 42},
	})

	// Multiple blocks with identical timestamps, disabled deduplication.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 4},
			Values:     []float64{9, 5.2, 6.1},
		},
		{
			Timestamps: []int64{1, 2, 4},
			Values:     []float64{4.2, 2.1, 42},
		},
	}, 0, &Result{
		Timestamps: []int64{1, 1, 2, 2, 4, 4},
		Values:     []float64{9, 4.2, 2.1, 5.2, 6.1, 42},
	})

	// Multiple blocks with identical timestamp ranges.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 5, 10, 11},
			Values:     []float64{9, 8, 7, 6, 5},
		},
		{
			Timestamps: []int64{1, 2, 4, 10, 11, 12},
			Values:     []float64{21, 22, 23, 24, 25, 26},
		},
	}, 1, &Result{
		Timestamps: []int64{1, 2, 4, 5, 10, 11, 12},
		Values:     []float64{21, 22, 23, 7, 24, 5, 26},
	})

	// Multiple blocks with identical timestamp ranges, no deduplication.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 5, 10, 11},
			Values:     []float64{9, 8, 7, 6, 5},
		},
		{
			Timestamps: []int64{1, 2, 4, 10, 11, 12},
			Values:     []float64{21, 22, 23, 24, 25, 26},
		},
	}, 0, &Result{
		Timestamps: []int64{1, 1, 2, 2, 4, 5, 10, 10, 11, 11, 12},
		Values:     []float64{9, 21, 22, 8, 23, 7, 6, 24, 25, 5, 26},
	})

	// Multiple blocks with identical timestamp ranges with deduplication.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 5, 10, 11},
			Values:     []float64{9, 8, 7, 6, 5},
		},
		{
			Timestamps: []int64{1, 2, 4, 10, 11, 12},
			Values:     []float64{21, 22, 23, 24, 25, 26},
		},
	}, 5, &Result{
		Timestamps: []int64{5, 10, 12},
		Values:     []float64{7, 24, 26},
	})
}

func TestMergeResult(t *testing.T) {
	f := func(name string, dst, update, expect *Result) {
		t.Run(name, func(t *testing.T) {
			mergeResult(dst, update)
			if !reflect.DeepEqual(dst, expect) {
				t.Fatalf(" unexpected result \ngot: \n%v\nwant: \n%v", dst, expect)
			}
		})
	}

	f("append and replace",
		&Result{Timestamps: []int64{1, 2}, Values: []float64{5.0, 6.0}},
		&Result{Timestamps: []int64{2, 3}, Values: []float64{10.0, 30.0}},
		&Result{Timestamps: []int64{1, 2, 3}, Values: []float64{5.0, 10.0, 30.0}})
	f("extend and replace",
		&Result{Timestamps: []int64{1, 2, 3}, Values: []float64{5.0, 6.0, 7.0}},
		&Result{Timestamps: []int64{0, 1, 2}, Values: []float64{10.0, 15.0, 30.0}},
		&Result{Timestamps: []int64{0, 1, 2, 3}, Values: []float64{10.0, 15.0, 30.0, 7.0}})
	f("extend",
		&Result{Timestamps: []int64{1, 2, 3}, Values: []float64{5.0, 6.0, 7.0}},
		&Result{Timestamps: []int64{6, 7, 8}, Values: []float64{10.0, 15.0, 30.0}},
		&Result{Timestamps: []int64{1, 2, 3, 6, 7, 8}, Values: []float64{5, 6, 7, 10, 15, 30}})
	f("fast path",
		&Result{},
		&Result{Timestamps: []int64{1, 2, 3}},
		&Result{})
	f("merge at the middle",
		&Result{Timestamps: []int64{1, 2, 5, 6, 10, 15}, Values: []float64{.1, .2, .3, .4, .5, .6}},
		&Result{Timestamps: []int64{2, 6, 9, 10}, Values: []float64{1.1, 1.2, 1.3, 1.4}},
		&Result{Timestamps: []int64{1, 2, 6, 9, 10, 15}, Values: []float64{.1, 1.1, 1.2, 1.3, 1.4, 0.6}})

	f("merge and re-allocate",
		&Result{
			Timestamps: []int64{10, 20, 30, 50, 60, 90},
			Values:     []float64{1.1, 1.2, 1.3, 1.4, 1.5, 1.6},
		},
		&Result{
			Timestamps: []int64{20, 30, 35, 45, 50, 55, 60},
			Values:     []float64{2.0, 2.3, 2.35, 2.45, 2.5, 2.55, 2.6},
		},
		&Result{
			Timestamps: []int64{10, 20, 30, 35, 45, 50, 55, 60, 90},
			Values:     []float64{1.1, 2.0, 2.3, 2.35, 2.45, 2.50, 2.55, 2.6, 1.6},
		})
}

func TestPackedTimeseries_Unpack(t *testing.T) {
	createBlock := func(ts []int64, vs []int64) *storage.Block {
		tsid := &storage.TSID{
			MetricID: 234211,
		}
		scale := int16(0)
		precisionBits := uint8(8)
		var b storage.Block
		b.Init(tsid, ts, vs, scale, precisionBits)
		_, _, _ = b.MarshalData(0, 0)
		return &b
	}
	tr := storage.TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: 1<<63 - 1,
	}
	var mn storage.MetricName
	mn.MetricGroup = []byte("foobar")
	metricName := string(mn.Marshal(nil))
	type blockData struct {
		timestamps []int64
		values     []int64
	}
	isValuesEqual := func(got, want []float64) bool {
		equal := true
		if len(got) != len(want) {
			return false
		}
		for i, v := range want {
			gotV := got[i]
			if v == gotV {
				continue
			}
			if decimal.IsStaleNaN(v) && decimal.IsStaleNaN(gotV) {
				continue
			}
			equal = false
		}
		return equal
	}
	f := func(name string, dataBlocks []blockData, updateBlocks []blockData, wantResult *Result) {
		t.Run(name, func(t *testing.T) {

			pts := packedTimeseries{
				metricName: metricName,
			}
			var dst Result
			tbf := tmpBlocksFile{
				buf: make([]byte, 0, 20*1024*1024),
			}
			for _, dataBlock := range dataBlocks {
				bb := createBlock(dataBlock.timestamps, dataBlock.values)
				addr, err := tbf.WriteBlockData(storage.MarshalBlock(nil, bb), 0)
				if err != nil {
					t.Fatalf("cannot write block: %s", err)
				}
				pts.addrs = append(pts.addrs, addr)
			}
			var updateAddrs []tmpBlockAddr
			for _, updateBlock := range updateBlocks {
				bb := createBlock(updateBlock.timestamps, updateBlock.values)
				addr, err := tbf.WriteBlockData(storage.MarshalBlock(nil, bb), 0)
				if err != nil {
					t.Fatalf("cannot write update block: %s", err)
				}
				updateAddrs = append(updateAddrs, addr)
			}
			if len(updateAddrs) > 0 {
				pts.updateAddrs = append(pts.updateAddrs, updateAddrs)
			}

			if err := pts.Unpack(&dst, []*tmpBlocksFile{&tbf}, tr); err != nil {
				t.Fatalf("unexpected error at series unpack: %s", err)
			}
			if !reflect.DeepEqual(wantResult, &dst) && !isValuesEqual(wantResult.Values, dst.Values) {
				t.Fatalf("unexpected result for unpack \nwant: \n%v\ngot: \n%v\n", wantResult, &dst)
			}
		})
	}
	f("2 blocks without updates",
		[]blockData{
			{
				timestamps: []int64{10, 15, 30},
				values:     []int64{1, 2, 3},
			},
			{
				timestamps: []int64{35, 40, 45},
				values:     []int64{4, 5, 6},
			},
		},
		nil,
		&Result{
			MetricName: mn,
			Values:     []float64{1, 2, 3, 4, 5, 6},
			Timestamps: []int64{10, 15, 30, 35, 40, 45},
		})
	f("2 blocks at the border of time range",
		[]blockData{
			{
				timestamps: []int64{10, 15, 30},
				values:     []int64{1, 2, 3},
			},
			{
				timestamps: []int64{35, 40, 45},
				values:     []int64{4, 5, 6},
			},
		},
		[]blockData{
			{
				timestamps: []int64{10},
				values:     []int64{16},
			},
		},
		&Result{
			MetricName: mn,
			Values:     []float64{16, 2, 3, 4, 5, 6},
			Timestamps: []int64{10, 15, 30, 35, 40, 45},
		})
	f("2 blocks with update",
		[]blockData{
			{
				timestamps: []int64{10, 15, 30},
				values:     []int64{1, 2, 3},
			},
			{
				timestamps: []int64{35, 40, 45},
				values:     []int64{4, 5, 6},
			},
		},
		[]blockData{
			{
				timestamps: []int64{15, 30},
				values:     []int64{11, 12},
			},
		},
		&Result{
			MetricName: mn,
			Values:     []float64{1, 11, 12, 4, 5, 6},
			Timestamps: []int64{10, 15, 30, 35, 40, 45},
		})
	f("2 blocks with 2 update blocks",
		[]blockData{
			{
				timestamps: []int64{10, 15, 30},
				values:     []int64{1, 2, 3},
			},
			{
				timestamps: []int64{35, 40, 65},
				values:     []int64{4, 5, 6},
			},
		},
		[]blockData{
			{
				timestamps: []int64{15, 30},
				values:     []int64{11, 12},
			},
			{
				timestamps: []int64{45, 55},
				values:     []int64{21, 22},
			},
		},
		&Result{
			MetricName: mn,
			Values:     []float64{1, 11, 12, 21, 22, 6},
			Timestamps: []int64{10, 15, 30, 45, 55, 65},
		})
}
