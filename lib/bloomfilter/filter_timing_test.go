package bloomfilter

import (
	"fmt"
	"testing"
)

func BenchmarkFilterAdd(b *testing.B) {
	for _, maxItems := range []int{1e3, 1e4, 1e5, 1e6, 1e7} {
		b.Run(fmt.Sprintf("maxItems=%d", maxItems), func(b *testing.B) {
			benchmarkFilterAdd(b, maxItems)
		})
	}
}

func benchmarkFilterAdd(b *testing.B, maxItems int) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		f := newFilter(maxItems)
		for pb.Next() {
			h := uint64(0)
			for i := 0; i < 10000; i++ {
				h += uint64(maxItems)
				f.Add(h)
			}
			f.Reset()
		}
	})
}

func BenchmarkFilterHasHit(b *testing.B) {
	for _, maxItems := range []int{1e3, 1e4, 1e5, 1e6, 1e7} {
		b.Run(fmt.Sprintf("maxItems=%d", maxItems), func(b *testing.B) {
			benchmarkFilterHasHit(b, maxItems)
		})
	}
}

func benchmarkFilterHasHit(b *testing.B, maxItems int) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		f := newFilter(maxItems)
		h := uint64(0)
		for i := 0; i < 10000; i++ {
			h += uint64(maxItems)
			f.Add(h)
		}
		for pb.Next() {
			h = 0
			for i := 0; i < 10000; i++ {
				h += uint64(maxItems)
				if !f.Has(h) {
					panic(fmt.Errorf("missing item %d", h))
				}
			}
		}
	})
}

func BenchmarkFilterHasMiss(b *testing.B) {
	for _, maxItems := range []int{1e3, 1e4, 1e5, 1e6, 1e7} {
		b.Run(fmt.Sprintf("maxItems=%d", maxItems), func(b *testing.B) {
			benchmarkFilterHasMiss(b, maxItems)
		})
	}
}

func benchmarkFilterHasMiss(b *testing.B, maxItems int) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		f := newFilter(maxItems)
		for pb.Next() {
			h := uint64(0)
			for i := 0; i < 10000; i++ {
				h += uint64(maxItems)
				if f.Has(h) {
					panic(fmt.Errorf("unexpected item %d", h))
				}
			}
		}
	})
}
