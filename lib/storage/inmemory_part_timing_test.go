package storage

import (
	"math/rand"
	"testing"
)

func BenchmarkInmemoryPartInitFromRowsWorstCase(b *testing.B) {
	benchmarkInmemoryPartInitFromRows(b, benchRawRowsWorstCase)
}

func BenchmarkInmemoryPartInitFromRowsBestCase(b *testing.B) {
	benchmarkInmemoryPartInitFromRows(b, benchRawRowsBestCase)
}

func benchmarkInmemoryPartInitFromRows(b *testing.B, rows []rawRow) {
	b.ReportAllocs()
	b.SetBytes(int64(len(rows)))
	b.RunParallel(func(pb *testing.PB) {
		var mp inmemoryPart
		for pb.Next() {
			mp.InitFromRows(rows)
		}
	})
}

// Each row belongs to an unique TSID
var benchRawRowsWorstCase = func() []rawRow {
	var rows []rawRow
	var r rawRow
	for i := 0; i < 1e5; i++ {
		r.TSID.MetricID = uint64(i)
		r.Timestamp = rand.Int63()
		r.Value = rand.NormFloat64()
		r.PrecisionBits = uint8(i%64) + 1
		rows = append(rows, r)
	}
	return rows
}()

// All the rows belong to a single TSID, values are zeros, timestamps
// are delimited by const delta.
var benchRawRowsBestCase = func() []rawRow {
	var rows []rawRow
	var r rawRow
	r.PrecisionBits = defaultPrecisionBits
	for i := 0; i < 1e5; i++ {
		r.Timestamp += 30e3
		rows = append(rows, r)
	}
	return rows
}()
