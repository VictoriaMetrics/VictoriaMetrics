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
		lr := GetLogRows(nil, nil, nil, "")
		defer PutLogRows(lr)

		tenantID := TenantID{
			AccountID: 123,
			ProjectID: 456,
		}
		ct := time.Now().UnixNano() - nsecsPerHour
		for i := 0; i < rowsPerInsert; i++ {
			timestamp := ct + int64(i)
			fields := make([]Field, 20)
			for j := range fields {
				fields[j] = Field{
					Name:  fmt.Sprintf("field-%d", j),
					Value: fmt.Sprintf("value-%d-%d", i, j),
				}
			}
			lr.MustAdd(tenantID, timestamp, fields, nil)
		}

		for pb.Next() {
			s.MustAddRows(lr)
		}
	})

	s.MustClose()
	fs.MustRemoveAll(testName)
}
