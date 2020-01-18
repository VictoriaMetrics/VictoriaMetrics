package uint64set

import (
	"fmt"
	"testing"
	"time"

	"github.com/valyala/fastrand"
)

func BenchmarkUnionNoOverlap(b *testing.B) {
	for _, itemsCount := range []int{1e3, 1e4, 1e5, 1e6, 1e7} {
		start := uint64(time.Now().UnixNano())
		sa := createRangeSet(start, itemsCount)
		sb := createRangeSet(start+uint64(itemsCount), itemsCount)
		b.Run(fmt.Sprintf("items_%d", itemsCount), func(b *testing.B) {
			benchmarkUnion(b, sa, sb)
		})
	}
}

func BenchmarkUnionPartialOverlap(b *testing.B) {
	for _, itemsCount := range []int{1e3, 1e4, 1e5, 1e6, 1e7} {
		start := uint64(time.Now().UnixNano())
		sa := createRangeSet(start, itemsCount)
		sb := createRangeSet(start+uint64(itemsCount/2), itemsCount)
		b.Run(fmt.Sprintf("items_%d", itemsCount), func(b *testing.B) {
			benchmarkUnion(b, sa, sb)
		})
	}
}

func BenchmarkUnionFullOverlap(b *testing.B) {
	for _, itemsCount := range []int{1e3, 1e4, 1e5, 1e6, 1e7} {
		start := uint64(time.Now().UnixNano())
		sa := createRangeSet(start, itemsCount)
		sb := createRangeSet(start, itemsCount)
		b.Run(fmt.Sprintf("items_%d", itemsCount), func(b *testing.B) {
			benchmarkUnion(b, sa, sb)
		})
	}
}

func benchmarkUnion(b *testing.B, sa, sb *Set) {
	b.ReportAllocs()
	b.SetBytes(int64(sa.Len() + sb.Len()))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			saCopy := sa.Clone()
			sbCopy := sb.Clone()
			saCopy.Union(sb)
			sbCopy.Union(sa)
		}
	})
}

func BenchmarkIntersectNoOverlap(b *testing.B) {
	for _, itemsCount := range []int{1e3, 1e4, 1e5, 1e6, 1e7} {
		start := uint64(time.Now().UnixNano())
		sa := createRangeSet(start, itemsCount)
		sb := createRangeSet(start+uint64(itemsCount), itemsCount)
		b.Run(fmt.Sprintf("items_%d", itemsCount), func(b *testing.B) {
			benchmarkIntersect(b, sa, sb)
		})
	}
}

func BenchmarkIntersectPartialOverlap(b *testing.B) {
	for _, itemsCount := range []int{1e3, 1e4, 1e5, 1e6, 1e7} {
		start := uint64(time.Now().UnixNano())
		sa := createRangeSet(start, itemsCount)
		sb := createRangeSet(start+uint64(itemsCount/2), itemsCount)
		b.Run(fmt.Sprintf("items_%d", itemsCount), func(b *testing.B) {
			benchmarkIntersect(b, sa, sb)
		})
	}
}

func BenchmarkIntersectFullOverlap(b *testing.B) {
	for _, itemsCount := range []int{1e3, 1e4, 1e5, 1e6, 1e7} {
		start := uint64(time.Now().UnixNano())
		sa := createRangeSet(start, itemsCount)
		sb := createRangeSet(start, itemsCount)
		b.Run(fmt.Sprintf("items_%d", itemsCount), func(b *testing.B) {
			benchmarkIntersect(b, sa, sb)
		})
	}
}

func benchmarkIntersect(b *testing.B, sa, sb *Set) {
	b.ReportAllocs()
	b.SetBytes(int64(sa.Len() + sb.Len()))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			saCopy := sa.Clone()
			sbCopy := sb.Clone()
			saCopy.Intersect(sb)
			sbCopy.Intersect(sa)
		}
	})
}

func createRangeSet(start uint64, itemsCount int) *Set {
	var s Set
	for i := 0; i < itemsCount; i++ {
		n := start + uint64(i)
		s.Add(n)
	}
	return &s
}

func BenchmarkSetAddRandomLastBits(b *testing.B) {
	const itemsCount = 1e5
	for _, lastBits := range []uint64{20, 24, 28, 32} {
		mask := (uint64(1) << lastBits) - 1
		b.Run(fmt.Sprintf("lastBits_%d", lastBits), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(itemsCount))
			b.RunParallel(func(pb *testing.PB) {
				var rng fastrand.RNG
				for pb.Next() {
					start := uint64(time.Now().UnixNano())
					var s Set
					for i := 0; i < itemsCount; i++ {
						n := start | (uint64(rng.Uint32()) & mask)
						s.Add(n)
					}
				}
			})
		})
	}
}

func BenchmarkMapAddRandomLastBits(b *testing.B) {
	const itemsCount = 1e5
	for _, lastBits := range []uint64{20, 24, 28, 32} {
		mask := (uint64(1) << lastBits) - 1
		b.Run(fmt.Sprintf("lastBits_%d", lastBits), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(itemsCount))
			b.RunParallel(func(pb *testing.PB) {
				var rng fastrand.RNG
				for pb.Next() {
					start := uint64(time.Now().UnixNano())
					m := make(map[uint64]struct{})
					for i := 0; i < itemsCount; i++ {
						n := start | (uint64(rng.Uint32()) & mask)
						m[n] = struct{}{}
					}
				}
			})
		})
	}
}

func BenchmarkSetAddWithAllocs(b *testing.B) {
	for _, itemsCount := range []uint64{1e3, 1e4, 1e5, 1e6, 1e7} {
		b.Run(fmt.Sprintf("items_%d", itemsCount), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(itemsCount))
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					start := uint64(time.Now().UnixNano())
					end := start + itemsCount
					var s Set
					n := start
					for n < end {
						s.Add(n)
						n++
					}
				}
			})
		})
	}
}

func BenchmarkMapAddWithAllocs(b *testing.B) {
	for _, itemsCount := range []uint64{1e3, 1e4, 1e5, 1e6, 1e7} {
		b.Run(fmt.Sprintf("items_%d", itemsCount), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(itemsCount))
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					start := uint64(time.Now().UnixNano())
					end := start + itemsCount
					m := make(map[uint64]struct{})
					n := start
					for n < end {
						m[n] = struct{}{}
						n++
					}
				}
			})
		})
	}
}

func BenchmarkMapAddNoAllocs(b *testing.B) {
	for _, itemsCount := range []uint64{1e3, 1e4, 1e5, 1e6, 1e7} {
		b.Run(fmt.Sprintf("items_%d", itemsCount), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(itemsCount))
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					start := uint64(time.Now().UnixNano())
					end := start + itemsCount
					m := make(map[uint64]struct{}, itemsCount)
					n := start
					for n < end {
						m[n] = struct{}{}
						n++
					}
				}
			})
		})
	}
}

func BenchmarkMapAddReuse(b *testing.B) {
	for _, itemsCount := range []uint64{1e3, 1e4, 1e5, 1e6, 1e7} {
		b.Run(fmt.Sprintf("items_%d", itemsCount), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(itemsCount))
			b.RunParallel(func(pb *testing.PB) {
				m := make(map[uint64]struct{}, itemsCount)
				for pb.Next() {
					start := uint64(time.Now().UnixNano())
					end := start + itemsCount
					for k := range m {
						delete(m, k)
					}
					n := start
					for n < end {
						m[n] = struct{}{}
						n++
					}
				}
			})
		})
	}
}

func BenchmarkSetHasHitRandomLastBits(b *testing.B) {
	const itemsCount = 1e5
	for _, lastBits := range []uint64{20, 24, 28, 32} {
		mask := (uint64(1) << lastBits) - 1
		b.Run(fmt.Sprintf("lastBits_%d", lastBits), func(b *testing.B) {
			start := uint64(time.Now().UnixNano())
			var s Set
			var rng fastrand.RNG
			for i := 0; i < itemsCount; i++ {
				n := start | (uint64(rng.Uint32()) & mask)
				s.Add(n)
			}
			a := s.AppendTo(nil)

			b.ResetTimer()
			b.ReportAllocs()
			b.SetBytes(int64(len(a)))
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					for _, n := range a {
						if !s.Has(n) {
							panic("unexpected miss")
						}
					}
				}
			})
		})
	}
}

func BenchmarkMapHasHitRandomLastBits(b *testing.B) {
	const itemsCount = 1e5
	for _, lastBits := range []uint64{20, 24, 28, 32} {
		mask := (uint64(1) << lastBits) - 1
		b.Run(fmt.Sprintf("lastBits_%d", lastBits), func(b *testing.B) {
			start := uint64(time.Now().UnixNano())
			m := make(map[uint64]struct{})
			var rng fastrand.RNG
			for i := 0; i < itemsCount; i++ {
				n := start | (uint64(rng.Uint32()) & mask)
				m[n] = struct{}{}
			}

			b.ResetTimer()
			b.ReportAllocs()
			b.SetBytes(int64(len(m)))
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					for n := range m {
						if _, ok := m[n]; !ok {
							panic("unexpected miss")
						}
					}
				}
			})
		})
	}
}

func BenchmarkSetHasHit(b *testing.B) {
	for _, itemsCount := range []uint64{1e3, 1e4, 1e5, 1e6, 1e7} {
		b.Run(fmt.Sprintf("items_%d", itemsCount), func(b *testing.B) {
			start := uint64(time.Now().UnixNano())
			end := start + itemsCount
			var s Set
			n := start
			for n < end {
				s.Add(n)
				n++
			}

			b.ResetTimer()
			b.ReportAllocs()
			b.SetBytes(int64(itemsCount))
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					n := start
					for n < end {
						if !s.Has(n) {
							panic("unexpected miss")
						}
						n++
					}
				}
			})
		})
	}
}

func BenchmarkMapHasHit(b *testing.B) {
	for _, itemsCount := range []uint64{1e3, 1e4, 1e5, 1e6, 1e7} {
		b.Run(fmt.Sprintf("items_%d", itemsCount), func(b *testing.B) {
			start := uint64(time.Now().UnixNano())
			end := start + itemsCount
			m := make(map[uint64]struct{}, itemsCount)
			n := start
			for n < end {
				m[n] = struct{}{}
				n++
			}

			b.ResetTimer()
			b.ReportAllocs()
			b.SetBytes(int64(itemsCount))
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					n := start
					for n < end {
						if _, ok := m[n]; !ok {
							panic("unexpected miss")
						}
						n++
					}
				}
			})
		})
	}
}

func BenchmarkSetHasMiss(b *testing.B) {
	for _, itemsCount := range []uint64{1e3, 1e4, 1e5, 1e6, 1e7} {
		b.Run(fmt.Sprintf("items_%d", itemsCount), func(b *testing.B) {
			start := uint64(time.Now().UnixNano())
			end := start + itemsCount
			var s Set
			n := start
			for n < end {
				s.Add(n)
				n++
			}

			b.ResetTimer()
			b.ReportAllocs()
			b.SetBytes(int64(itemsCount))
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					n := end
					nEnd := end + itemsCount
					for n < nEnd {
						if s.Has(n) {
							panic("unexpected hit")
						}
						n++
					}
				}
			})
		})
	}
}

func BenchmarkMapHasMiss(b *testing.B) {
	for _, itemsCount := range []uint64{1e3, 1e4, 1e5, 1e6, 1e7} {
		b.Run(fmt.Sprintf("items_%d", itemsCount), func(b *testing.B) {
			start := uint64(time.Now().UnixNano())
			end := start + itemsCount
			m := make(map[uint64]struct{}, itemsCount)
			n := start
			for n < end {
				m[n] = struct{}{}
				n++
			}

			b.ResetTimer()
			b.ReportAllocs()
			b.SetBytes(int64(itemsCount))
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					n := end
					nEnd := end + itemsCount
					for n < nEnd {
						if _, ok := m[n]; ok {
							panic("unexpected hit")
						}
						n++
					}
				}
			})
		})
	}
}
