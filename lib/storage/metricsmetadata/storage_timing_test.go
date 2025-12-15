package metricsmetadata

import (
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func BenchmarkStorageWrite(b *testing.B) {

	for _, p := range []int{10, 100} {
		for _, rowsCount := range []int{10e3, 100e3} {
			rows := getRows(0, 0, rowsCount)
			b.Run(fmt.Sprintf("singletenant/parallel=%d,rows=%d,no_eviction=true", p, rowsCount), func(b *testing.B) {
				// allocate storage without eviction
				s := NewStorage(rowsCount * int(perItemOverhead) * bucketsCount)
				defer s.MustClose()
				b.SetParallelism(p)
				b.ReportAllocs()
				b.SetBytes(int64(len(rows)))
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						s.Add(rows)
					}
				})
			})
		}
	}
}

func BenchmarkStorageWriteMultitenant(b *testing.B) {

	tenants := [][2]uint32{{1, 2}, {0, 0}, {3, 3}}
	for _, rowsCount := range []int{10e3, 100e3} {
		var rows []Row
		for _, tenant := range tenants {
			rows = append(rows, getRows(tenant[0], tenant[1], rowsCount)...)
		}
		b.Run(fmt.Sprintf("multitenant/parallel=10,rows=%d,", rowsCount), func(b *testing.B) {
			// allocate storage without eviction
			s := NewStorage(rowsCount * len(tenants) * int(perItemOverhead) * bucketsCount)
			defer s.MustClose()
			b.SetParallelism(10)
			b.ReportAllocs()
			b.SetBytes(int64(len(rows)))
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					s.Add(rows)
				}
			})
		})

	}
}

func BenchmarkStorageRead(b *testing.B) {
	s := NewStorage(512 * 1024)
	defer s.MustClose()

	rows := getRows(0, 0, 10e3)
	s.Add(rows)

	for _, l := range []int{-1, 100, 20e3} {
		b.Run(fmt.Sprintf("limit=%d", l), func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_ = s.Get(l, "")
				}
			})
		})
	}
}

func BenchmarkStorageReadMultitenant(b *testing.B) {

	var rows []Row
	tenants := [][2]uint32{{0, 0}, {1, 1}, {2, 2}}
	for _, tenant := range tenants {
		rows = append(rows, getRows(tenant[0], tenant[1], 10e3)...)
	}

	s := NewStorage(10e3 * int(perItemOverhead) * len(tenants))
	defer s.MustClose()
	s.Add(rows)

	for _, l := range []int{-1, 100, 20e3} {
		b.Run(fmt.Sprintf("limit=%d", l), func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				var i int
				for pb.Next() {
					if i >= len(tenants) {
						i = 0
					}
					tenant := tenants[i]
					i++
					_ = s.GetForTenant(tenant[0], tenant[1], l, "")
				}
			})
		})
	}
}

func getRows(accountID, projectID uint32, n int) []Row {
	rows := make([]Row, n)
	for i := range rows {
		rows[i] = Row{
			AccountID:        accountID,
			ProjectID:        projectID,
			MetricFamilyName: []byte(fmt.Sprintf("metric_%d_%d", i, n)),
			Type:             prompb.MetricType(i % 3),
			Help:             []byte("help text for metric"),
			Unit:             []byte("seconds"),
		}
	}

	return rows
}
