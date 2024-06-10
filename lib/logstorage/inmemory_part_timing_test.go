package logstorage

import (
	"fmt"
	"testing"
)

func BenchmarkInmemoryPart_MustInitFromRows(b *testing.B) {
	for _, streams := range []int{1, 10, 100} {
		b.Run(fmt.Sprintf("streams_%d", streams), func(b *testing.B) {
			for _, rowsPerStream := range []int{1, 10, 100, 1000} {
				b.Run(fmt.Sprintf("rowsPerStream_%d", rowsPerStream), func(b *testing.B) {
					benchmarkInmemoryPartMustInitFromRows(b, streams, rowsPerStream)
				})
			}
		})
	}
}

func benchmarkInmemoryPartMustInitFromRows(b *testing.B, streams, rowsPerStream int) {
	b.ReportAllocs()
	b.SetBytes(int64(streams * rowsPerStream))
	b.RunParallel(func(pb *testing.PB) {
		lr := newTestLogRows(streams, rowsPerStream, 0)
		mp := getInmemoryPart()
		for pb.Next() {
			mp.mustInitFromRows(lr)
			if mp.ph.RowsCount != uint64(len(lr.timestamps)) {
				panic(fmt.Errorf("unexpecte number of entries in the output stream; got %d; want %d", mp.ph.RowsCount, len(lr.timestamps)))
			}
		}
		putInmemoryPart(mp)
	})
}
