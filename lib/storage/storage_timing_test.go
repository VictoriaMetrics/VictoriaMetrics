package storage

import (
	"fmt"
	"os"
	"sync/atomic"
	"testing"
)

func BenchmarkStorageAddRows(b *testing.B) {
	for _, rowsPerBatch := range []int{1, 10, 100, 1000} {
		b.Run(fmt.Sprintf("rowsPerBatch_%d", rowsPerBatch), func(b *testing.B) {
			benchmarkStorageAddRows(b, rowsPerBatch)
		})
	}
}

func benchmarkStorageAddRows(b *testing.B, rowsPerBatch int) {
	path := fmt.Sprintf("BenchmarkStorageAddRows_%d", rowsPerBatch)
	s, err := OpenStorage(path, 0)
	if err != nil {
		b.Fatalf("cannot open storage at %q: %s", path, err)
	}
	defer func() {
		s.MustClose()
		if err := os.RemoveAll(path); err != nil {
			b.Fatalf("cannot remove storage at %q: %s", path, err)
		}
	}()

	var globalOffset uint64

	b.SetBytes(int64(rowsPerBatch))
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		mrs := make([]MetricRow, rowsPerBatch)
		var mn MetricName
		mn.MetricGroup = []byte("rps")
		mn.Tags = []Tag{
			{[]byte("job"), []byte("webservice")},
			{[]byte("instance"), []byte("1.2.3.4")},
		}
		for pb.Next() {
			offset := int(atomic.AddUint64(&globalOffset, uint64(rowsPerBatch)))
			for i := 0; i < rowsPerBatch; i++ {
				mr := &mrs[i]
				mr.MetricNameRaw = mn.marshalRaw(mr.MetricNameRaw[:0])
				mr.Timestamp = int64(offset + i)
				mr.Value = float64(offset + i)
			}
			if err := s.AddRows(mrs, defaultPrecisionBits); err != nil {
				panic(fmt.Errorf("cannot add rows to storage: %w", err))
			}
		}
	})
	b.StopTimer()
}
