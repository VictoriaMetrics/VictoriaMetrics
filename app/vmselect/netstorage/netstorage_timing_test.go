package netstorage

import (
	"fmt"
	"reflect"
	"testing"
)

func BenchmarkMergeSortBlocks(b *testing.B) {
	for _, replicationFactor := range []int{1, 2, 3, 4, 5} {
		b.Run(fmt.Sprintf("replicationFactor-%d", replicationFactor), func(b *testing.B) {
			const samplesPerBlock = 8192
			var blocks []*sortBlock
			for j := 0; j < 10; j++ {
				timestamps := make([]int64, samplesPerBlock)
				values := make([]float64, samplesPerBlock)
				for i := range timestamps {
					timestamps[i] = int64(j*samplesPerBlock + i)
					values[i] = float64(j*samplesPerBlock + i)
				}
				for i := 0; i < replicationFactor; i++ {
					blocks = append(blocks, &sortBlock{
						Timestamps: timestamps,
						Values:     values,
					})
				}
			}
			benchmarkMergeSortBlocks(b, blocks)
		})
	}
	b.Run("overlapped-blocks-bestcase", func(b *testing.B) {
		const samplesPerBlock = 8192
		var blocks []*sortBlock
		for j := 0; j < 10; j++ {
			timestamps := make([]int64, samplesPerBlock)
			values := make([]float64, samplesPerBlock)
			for i := range timestamps {
				timestamps[i] = int64(j*samplesPerBlock + i)
				values[i] = float64(j*samplesPerBlock + i)
			}
			blocks = append(blocks, &sortBlock{
				Timestamps: timestamps,
				Values:     values,
			})
		}
		for j := 1; j < len(blocks); j++ {
			prev := blocks[j-1].Timestamps
			curr := blocks[j].Timestamps
			for i := 0; i < samplesPerBlock/2; i++ {
				prev[i+samplesPerBlock/2], curr[i] = curr[i], prev[i+samplesPerBlock/2]
			}
		}
		benchmarkMergeSortBlocks(b, blocks)
	})
	b.Run("overlapped-blocks-worstcase", func(b *testing.B) {
		const samplesPerBlock = 8192
		var blocks []*sortBlock
		for j := 0; j < 5; j++ {
			timestamps := make([]int64, samplesPerBlock)
			values := make([]float64, samplesPerBlock)
			for i := range timestamps {
				timestamps[i] = int64(2 * (j*samplesPerBlock + i))
				values[i] = float64(2 * (j*samplesPerBlock + i))
			}
			blocks = append(blocks, &sortBlock{
				Timestamps: timestamps,
				Values:     values,
			})
			timestamps = make([]int64, samplesPerBlock)
			values = make([]float64, samplesPerBlock)
			for i := range timestamps {
				timestamps[i] = int64(2*(j*samplesPerBlock+i) + 1)
				values[i] = float64(2*(j*samplesPerBlock+i) + 1)
			}
			blocks = append(blocks, &sortBlock{
				Timestamps: timestamps,
				Values:     values,
			})
		}
		benchmarkMergeSortBlocks(b, blocks)
	})
}

func benchmarkMergeSortBlocks(b *testing.B, blocks []*sortBlock) {
	dedupInterval := int64(1)
	samples := 0
	for _, b := range blocks {
		samples += len(b.Timestamps)
	}
	b.SetBytes(int64(samples))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var result Result
		sbs := make(sortBlocksHeap, len(blocks))
		for pb.Next() {
			result.reset()
			for i, b := range blocks {
				sb := getSortBlock()
				sb.Timestamps = b.Timestamps
				sb.Values = b.Values
				sbs[i] = sb
			}
			mergeSortBlocks(&result, sbs, dedupInterval)
		}
	})
}

func BenchmarkMergeResults(b *testing.B) {
	b.ReportAllocs()
	f := func(name string, dst, update, expect *Result) {
		if len(dst.Timestamps) != len(dst.Values) {
			b.Fatalf("bad input data, timestamps and values lens must match")
		}
		if len(update.Values) != len(update.Timestamps) {
			b.Fatalf("bad input data, update timestamp and values must match")
		}
		var toMerge Result
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				toMerge.reset()
				toMerge.Values = append(toMerge.Values, dst.Values...)
				toMerge.Timestamps = append(toMerge.Timestamps, dst.Timestamps...)
				mergeResult(&toMerge, update)
				if !reflect.DeepEqual(&toMerge, expect) {
					b.Fatalf("unexpected result, got: \n%v\nwant: \n%v", &toMerge, expect)
				}
			}
		})
	}
	f("update at the start",
		&Result{
			Timestamps: []int64{10, 20, 30, 40, 50, 60, 90},
			Values:     []float64{2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.9},
		},
		&Result{
			Timestamps: []int64{0, 20, 40},
			Values:     []float64{0.0, 5.2, 5.4},
		},
		&Result{
			Timestamps: []int64{0, 20, 40, 50, 60, 90},
			Values:     []float64{0.0, 5.2, 5.4, 2.5, 2.6, 2.9},
		})
	f("update at the end",
		&Result{
			Timestamps: []int64{10, 20, 30, 40, 50, 60, 90},
			Values:     []float64{2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.9},
		},
		&Result{
			Timestamps: []int64{50, 70, 100},
			Values:     []float64{0.0, 5.7, 5.1},
		},
		&Result{
			Timestamps: []int64{10, 20, 30, 40, 50, 70, 100},
			Values:     []float64{2.1, 2.2, 2.3, 2.4, 0.0, 5.7, 5.1},
		})
	f("update at the middle",
		&Result{
			Timestamps: []int64{10, 20, 30, 40, 50, 60, 90},
			Values:     []float64{2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.9},
		},
		&Result{
			Timestamps: []int64{30, 40, 50, 60},
			Values:     []float64{5.3, 5.4, 5.5, 5.6},
		},
		&Result{
			Timestamps: []int64{10, 20, 30, 40, 50, 60, 90},
			Values:     []float64{2.1, 2.2, 5.3, 5.4, 5.5, 5.6, 2.9},
		})
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
