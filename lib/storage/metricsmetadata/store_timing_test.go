package metricsmetadata

import (
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
)

func BenchmarkStoreWrite(b *testing.B) {
	s := NewStore(memory.Allowed())
	defer s.MustClose()

	rows := getRows(10e3)

	for _, p := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("parallel=%d", p), func(b *testing.B) {
			b.SetParallelism(p)
			b.ReportAllocs()
			b.SetBytes(int64(len(rows)))
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if err := s.Add(rows); err != nil {
						b.Fatalf("unexpected error during Add: %s", err)
					}
				}
			})
		})
	}
}

func BenchmarkStoreRead(b *testing.B) {
	s := NewStore(memory.Allowed())
	defer s.MustClose()

	rows := getRows(10e3)
	_ = s.Add(rows)

	for _, l := range []int64{-1, 100, 1000} {
		b.Run(fmt.Sprintf("limit=%d", l), func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_ = s.Get(l, l, "")
				}
			})

		})
	}
}

func getRows(n int) []Row {
	rows := make([]Row, n)
	for i := range rows {
		rows[i] = Row{
			MetricFamilyName: []byte(fmt.Sprintf("metric_%d_%d", n, i)),
			Type:             uint32(i % 3),
			Help:             []byte("help text for metric"),
		}
	}

	return rows
}
