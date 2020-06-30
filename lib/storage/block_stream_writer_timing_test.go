package storage

import (
	"fmt"
	"testing"
)

func BenchmarkBlockStreamWriterBlocksWorstCase(b *testing.B) {
	benchmarkBlockStreamWriter(b, benchBlocksWorstCase, len(benchRawRowsWorstCase), false)
}

func BenchmarkBlockStreamWriterBlocksBestCase(b *testing.B) {
	benchmarkBlockStreamWriter(b, benchBlocksBestCase, len(benchRawRowsBestCase), false)
}

func BenchmarkBlockStreamWriterRowsWorstCase(b *testing.B) {
	benchmarkBlockStreamWriter(b, benchBlocksWorstCase, len(benchRawRowsWorstCase), true)
}

func BenchmarkBlockStreamWriterRowsBestCase(b *testing.B) {
	benchmarkBlockStreamWriter(b, benchBlocksBestCase, len(benchRawRowsBestCase), true)
}

func benchmarkBlockStreamWriter(b *testing.B, ebs []Block, rowsCount int, writeRows bool) {
	var rowsMerged uint64

	b.ReportAllocs()
	b.SetBytes(int64(rowsCount))
	b.RunParallel(func(pb *testing.PB) {
		var bsw blockStreamWriter
		var mp inmemoryPart
		var ph partHeader
		var ebsCopy []Block
		for i := range ebs {
			var ebCopy Block
			ebCopy.CopyFrom(&ebs[i])
			ebsCopy = append(ebsCopy, ebCopy)
		}
		loopCount := 0
		for pb.Next() {
			if writeRows {
				for i := range ebsCopy {
					eb := &ebsCopy[i]
					if err := eb.UnmarshalData(); err != nil {
						panic(fmt.Errorf("cannot unmarshal block %d on loop %d: %w", i, loopCount, err))
					}
				}
			}

			bsw.InitFromInmemoryPart(&mp)
			for i := range ebsCopy {
				bsw.WriteExternalBlock(&ebsCopy[i], &ph, &rowsMerged)
			}
			bsw.MustClose()
			mp.Reset()
			loopCount++
		}
	})
}

var benchBlocksWorstCase = newBenchBlocks(benchRawRowsWorstCase)
var benchBlocksBestCase = newBenchBlocks(benchRawRowsBestCase)

func newBenchBlocks(rows []rawRow) []Block {
	var ebs []Block

	mp := newTestInmemoryPart(rows)
	var bsr blockStreamReader
	bsr.InitFromInmemoryPart(mp)
	for bsr.NextBlock() {
		var eb Block
		eb.CopyFrom(&bsr.Block)
		ebs = append(ebs, eb)
	}
	if err := bsr.Error(); err != nil {
		panic(fmt.Errorf("unexpected error when reading inmemoryPart: %w", err))
	}
	return ebs
}
