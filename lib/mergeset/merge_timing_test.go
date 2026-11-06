package mergeset

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func BenchmarkMergeBlockStreams(b *testing.B) {
	for _, partsCount := range []int{2, 5, 10, 20} {
		b.Run(fmt.Sprintf("overlapping/parts-%d", partsCount), func(b *testing.B) {
			mps, itemsCount := newBenchmarkInmemoryParts(partsCount, false)
			benchmarkMergeBlockStreams(b, mps, itemsCount)
		})
		b.Run(fmt.Sprintf("disjoint/parts-%d", partsCount), func(b *testing.B) {
			mps, itemsCount := newBenchmarkInmemoryParts(partsCount, true)
			benchmarkMergeBlockStreams(b, mps, itemsCount)
		})
	}
}

func benchmarkMergeBlockStreams(b *testing.B, mps []*inmemoryPart, itemsCount int) {
	b.ReportAllocs()
	b.SetBytes(int64(itemsCount))
	b.ResetTimer()
	for range b.N {
		bsrs := make([]*blockStreamReader, len(mps))
		for i, mp := range mps {
			bsrs[i] = newTestBlockStreamReader(mp)
		}
		var dstIP inmemoryPart
		var bsw blockStreamWriter
		bsw.MustInitFromInmemoryPart(&dstIP, -5)
		var itemsMerged atomic.Uint64
		if err := mergeBlockStreams(&dstIP.ph, &bsw, bsrs, nil, nil, &itemsMerged); err != nil {
			b.Fatalf("cannot merge block streams: %s", err)
		}
		if n := itemsMerged.Load(); n != uint64(itemsCount) {
			b.Fatalf("unexpected itemsMerged; got %d; want %d", n, itemsCount)
		}
	}
}

// newBenchmarkInmemoryParts returns partsCount multi-block parts
// together with the total number of items in them.
//
// If disjoint is set, then every part covers its own key range, so the merge
// hits the optimization, which skips per-item comparison. Otherwise all the
// parts cover the same key range, so their items are interleaved during the merge.
func newBenchmarkInmemoryParts(partsCount int, disjoint bool) ([]*inmemoryPart, int) {
	const blocksPerPart = 20
	const itemsPerBlock = 1000

	mps := make([]*inmemoryPart, 0, partsCount)
	itemsCount := 0
	for i := range partsCount {
		// Build blocksPerPart single-block parts and merge them into a single
		// multi-block part, since inmemoryPart.Init creates a part with a single block.
		bsrs := make([]*blockStreamReader, 0, blocksPerPart)
		for j := range blocksPerPart {
			var ib inmemoryBlock
			for k := range itemsPerBlock {
				n := j*itemsPerBlock + k
				var item string
				if disjoint {
					item = fmt.Sprintf("prefix_%03d_%016d", i, n)
				} else {
					// Interleave items between the parts.
					item = fmt.Sprintf("prefix_%016d_%03d", n, i)
				}
				if !ib.Add([]byte(item)) {
					break
				}
				itemsCount++
			}
			var ip inmemoryPart
			ip.Init(&ib)
			bsrs = append(bsrs, newTestBlockStreamReader(&ip))
		}
		mp := &inmemoryPart{}
		var bsw blockStreamWriter
		bsw.MustInitFromInmemoryPart(mp, -5)
		var itemsMerged atomic.Uint64
		if err := mergeBlockStreams(&mp.ph, &bsw, bsrs, nil, nil, &itemsMerged); err != nil {
			panic(fmt.Errorf("cannot prepare part %d: %w", i, err))
		}
		mps = append(mps, mp)
	}
	return mps, itemsCount
}
