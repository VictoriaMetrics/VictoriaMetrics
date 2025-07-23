package metricsmetadata

import "testing"

func BenchmarkStoreWrite(b *testing.B) {
	s := NewStore()
	defer s.MustClose()

	rows := make([]Row, 100)
	for i := range rows {
		rows[i] = Row{
			MetricFamilyName: []byte("metric_" + string(rune(i))),
			Type:             uint32(i % 3),
			Help:             []byte("help text for metric"),
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(len(rows)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := s.Add(rows); err != nil {
				b.Fatalf("unexpected error during Add: %s", err)
			}
		}
	})
}

func BenchmarkStoreRead(b *testing.B) {
	s := NewStore()
	defer s.MustClose()

	// Pre-populate store
	for i := 0; i < 1000; i++ {
		row := Row{
			MetricFamilyName: []byte("metric_" + string(rune(i%100))),
			Type:             uint32(i % 3),
			Help:             []byte("help"),
		}
		s.Add([]Row{row})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		s.Get(100, 10, "")
	}
}
