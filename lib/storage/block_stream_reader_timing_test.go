package storage

import (
	"fmt"
	"testing"
)

func BenchmarkBlockStreamReaderBlocksWorstCase(b *testing.B) {
	benchmarkBlockStreamReader(b, benchInmemoryPartWorstCase, false)
}

func BenchmarkBlockStreamReaderBlocksBestCase(b *testing.B) {
	benchmarkBlockStreamReader(b, benchInmemoryPartBestCase, false)
}

func BenchmarkBlockStreamReaderRowsWorstCase(b *testing.B) {
	benchmarkBlockStreamReader(b, benchInmemoryPartWorstCase, true)
}

func BenchmarkBlockStreamReaderRowsBestCase(b *testing.B) {
	benchmarkBlockStreamReader(b, benchInmemoryPartBestCase, true)
}

func benchmarkBlockStreamReader(b *testing.B, mp *inmemoryPart, readRows bool) {
	b.ReportAllocs()
	b.SetBytes(int64(mp.ph.RowsCount))
	b.RunParallel(func(pb *testing.PB) {
		var bsr blockStreamReader
		blockNum := 0
		for pb.Next() {
			bsr.InitFromInmemoryPart(mp)
			for bsr.NextBlock() {
				if !readRows {
					continue
				}
				if err := bsr.Block.UnmarshalData(); err != nil {
					panic(fmt.Errorf("unexpected error when unmarshaling rows on block %d: %w", blockNum, err))
				}
				for bsr.Block.nextRow() {
				}
			}
			if err := bsr.Error(); err != nil {
				panic(fmt.Errorf("unexpected error when reading block %d: %w", blockNum, err))
			}
			blockNum++
		}
	})
}

var benchInmemoryPartWorstCase = newTestInmemoryPart(benchRawRowsWorstCase)
var benchInmemoryPartBestCase = newTestInmemoryPart(benchRawRowsBestCase)

func newTestInmemoryPart(rows []rawRow) *inmemoryPart {
	var mp inmemoryPart
	mp.InitFromRows(rows)
	return &mp
}
