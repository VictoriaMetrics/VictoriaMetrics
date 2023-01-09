package netstorage

import (
	"fmt"
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
		sbh := getSortBlocksHeap()
		for pb.Next() {
			result.reset()
			sbs := sbh.sbs[:0]
			for _, b := range blocks {
				sb := getSortBlock()
				sb.Timestamps = b.Timestamps
				sb.Values = b.Values
				sbs = append(sbs, sb)
			}
			sbh.sbs = sbs
			mergeSortBlocks(&result, sbh, dedupInterval)
		}
	})
}
