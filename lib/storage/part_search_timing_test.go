package storage

import (
	"fmt"
	"testing"
)

func BenchmarkPartSearch(b *testing.B) {
	for _, sparseness := range []int{1, 2, 10, 100} {
		b.Run(fmt.Sprintf("sparseness-%d", sparseness), func(b *testing.B) {
			benchmarkPartSearchWithSparseness(b, sparseness)
		})
	}
}

func benchmarkPartSearchWithSparseness(b *testing.B, sparseness int) {
	blocksCount := 100000
	rows := make([]rawRow, blocksCount)
	for i := 0; i < blocksCount; i++ {
		r := &rows[i]
		r.PrecisionBits = defaultPrecisionBits
		r.TSID.MetricID = uint64(i * sparseness)
		r.Timestamp = int64(i) * 1000
		r.Value = float64(i)
	}
	tr := TimeRange{
		MinTimestamp: rows[0].Timestamp,
		MaxTimestamp: rows[len(rows)-1].Timestamp,
	}
	p := newTestPart(rows)
	for _, tsidsCount := range []int{100, 1000, 10000, 100000} {
		b.Run(fmt.Sprintf("tsids-%d", tsidsCount), func(b *testing.B) {
			tsids := make([]TSID, tsidsCount)
			for i := 0; i < tsidsCount; i++ {
				tsids[i].MetricID = uint64(i)
			}
			benchmarkPartSearch(b, p, tsids, tr, sparseness)
		})
	}
}

func benchmarkPartSearch(b *testing.B, p *part, tsids []TSID, tr TimeRange, sparseness int) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var ps partSearch
		for pb.Next() {
			blocksRead := 0
			ps.Init(p, tsids, tr)
			for ps.NextBlock() {
				blocksRead++
			}
			if err := ps.Error(); err != nil {
				panic(fmt.Errorf("BUG: unexpected error: %w", err))
			}
			blocksWant := len(tsids) / sparseness
			if blocksRead != blocksWant {
				panic(fmt.Errorf("BUG: unexpected blocks read; got %d; want %d", blocksRead, blocksWant))
			}
		}
	})
}
