package logstorage

import (
	"fmt"
	"sync"
	"testing"
)

func BenchmarkRowsBuffer(b *testing.B) {
	for _, rowsPerInsert := range []int{1, 10, 100, 1000, 10000} {
		b.Run(fmt.Sprintf("rowsPerInsert-%d", rowsPerInsert), func(b *testing.B) {
			benchmarkRowsBuffer(b, rowsPerInsert)
		})
	}
}

func benchmarkRowsBuffer(b *testing.B, rowsPerInsert int) {
	var rb rowsBuffer
	var wgBuffer sync.WaitGroup
	rb.init(&wgBuffer, func(_ *logRows) {})

	b.ReportAllocs()
	b.SetBytes(int64(rowsPerInsert))
	b.RunParallel(func(pb *testing.PB) {
		lr := newTestLogRows(1, rowsPerInsert, 1)

		for pb.Next() {
			rb.mustAddRows(lr)
		}
	})
	rb.flush()
	wgBuffer.Wait()
}
