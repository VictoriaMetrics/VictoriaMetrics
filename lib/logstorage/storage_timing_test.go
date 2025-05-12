package logstorage

import (
	"fmt"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func BenchmarkStorageMustAddRows(b *testing.B) {
	for _, rowsPerInsert := range []int{1, 10, 100, 1000, 10000} {
		b.Run(fmt.Sprintf("rowsPerInsert-%d", rowsPerInsert), func(b *testing.B) {
			benchmarkStorageMustAddRows(b, rowsPerInsert)
		})
	}
}

func benchmarkStorageMustAddRows(b *testing.B, rowsPerInsert int) {
	cfg := &StorageConfig{
		Retention: 24 * time.Hour,
	}
	testName := b.Name()
	s := MustOpenStorage(testName, cfg)

	b.ReportAllocs()
	b.SetBytes(int64(rowsPerInsert))

	b.RunParallel(func(pb *testing.PB) {
		lr := newTestLogRows(1, rowsPerInsert, 1)

		ct := time.Now().UnixNano() - nsecsPerMinute
		timestamps := lr.timestamps
		for i := range timestamps {
			timestamps[i] = ct + int64(i)
		}

		for pb.Next() {
			s.MustAddRows(lr)
		}
	})

	s.MustClose()
	fs.MustRemoveAll(testName)
}
