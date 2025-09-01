package uint64set

import (
	"fmt"
	"testing"
	"time"

	"github.com/RoaringBitmap/roaring/v2/roaring64"
	"github.com/valyala/fastrand"
)

func BenchmarkAddMulti(b *testing.B) {
	for _, itemsCount := range []int{1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7} {
		start := uint64(time.Now().UnixNano())
		a := createRangeSet(start, itemsCount).ToArray()
		b.Run(fmt.Sprintf("items_%d", itemsCount), func(b *testing.B) {
			benchmarkAddMulti(b, a)
		})
	}
}

func BenchmarkAdd(b *testing.B) {
	for _, itemsCount := range []int{1e3, 1e4, 1e5, 1e6, 1e7} {
		start := uint64(time.Now().UnixNano())
		a := createRangeSet(start, itemsCount).ToArray()
		b.Run(fmt.Sprintf("items_%d", itemsCount), func(b *testing.B) {
			benchmarkAdd(b, a)
		})
	}
}

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

func benchmarkAdd(b *testing.B, a []uint64) {
	b.ReportAllocs()
	b.SetBytes(int64(len(a)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s := roaring64.New()
			for _, x := range a {
				s.Add(x)
			}
		}
	})
}

func benchmarkAddMulti(b *testing.B, a []uint64) {
	b.ReportAllocs()
	b.SetBytes(int64(len(a)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s := roaring64.New()
			n := 0
			for n < len(a) {
				m := n + 64
				if m > len(a) {
					m = len(a)
				}
				s.AddMany(a[n:m])
				n = m
			}
		}
	})
}

func benchmarkUnion(b *testing.B, sa, sb *roaring64.Bitmap) {
	b.ReportAllocs()
	b.SetBytes(int64(sa.Stats().Cardinality + sb.Stats().Cardinality))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			saCopy := sa.Clone()
			sbCopy := sb.Clone()
			saCopy.Or(sb)
			sbCopy.Or(sa)
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

func benchmarkIntersect(b *testing.B, sa, sb *roaring64.Bitmap) {
	b.ReportAllocs()
	b.SetBytes(int64(sa.Stats().Cardinality + sb.Stats().Cardinality))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			saCopy := sa.Clone()
			sbCopy := sb.Clone()
			saCopy.And(sb)
			sbCopy.And(sa)
		}
	})
}

func BenchmarkSubtract(b *testing.B) {
	f := func(b *testing.B, startA, itemsCountA, startB, itemsCountB uint64) {
		sa := createRangeSet(startA, int(itemsCountA))
		sb := createRangeSet(startB, int(itemsCountB))
		b.ReportAllocs()
		b.SetBytes(int64(sa.Stats().Cardinality + sb.Stats().Cardinality))
		for b.Loop() {
			saCopy := sa.Clone()
			saCopy.AndNot(sb)
		}
	}

	start := uint64(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano())
	itemsACounts := []uint64{1e3, 1e4, 1e5, 1e6, 1e7}
	itemsBCounts := []uint64{1e7, 1e6, 1e5, 1e4, 1e3}
	for i := range len(itemsACounts) {
		itemsCountA := itemsACounts[i]
		itemsCountB := itemsBCounts[i]

		b.Run(fmt.Sprintf("-----NoOverlap-AbeforeB-A%d-B%d", itemsCountA, itemsCountB), func(b *testing.B) {
			f(b, start, itemsCountA, start+itemsCountA, itemsCountB)
		})
		b.Run(fmt.Sprintf("-----NoOverlap-AbeforeB-B%d-A%d", itemsCountB, itemsCountA), func(b *testing.B) {
			f(b, start+itemsCountA, itemsCountB, start, itemsCountA)
		})
		b.Run(fmt.Sprintf("-----NoOverlap-BbeforeA-A%d-B%d", itemsCountA, itemsCountB), func(b *testing.B) {
			f(b, start-itemsCountA, itemsCountA, start, itemsCountB)
		})
		b.Run(fmt.Sprintf("-----NoOverlap-BbeforeA-B%d-A%d", itemsCountB, itemsCountA), func(b *testing.B) {
			f(b, start, itemsCountB, start-itemsCountA, itemsCountA)
		})

		b.Run(fmt.Sprintf("PartialOverlap-AbeforeB-A%d-B%d", itemsCountA, itemsCountB), func(b *testing.B) {
			f(b, start, itemsCountA, start+itemsCountA-itemsCountB/2, itemsCountB)
		})
		b.Run(fmt.Sprintf("PartialOverlap-AbeforeB-B%d-A%d", itemsCountB, itemsCountA), func(b *testing.B) {
			f(b, start+itemsCountA-itemsCountB/2, itemsCountB, start, itemsCountA)
		})
		b.Run(fmt.Sprintf("PartialOverlap-BbeforeA-A%d-B%d", itemsCountA, itemsCountB), func(b *testing.B) {
			f(b, start+itemsCountB/2, itemsCountA, start, itemsCountB)
		})
		b.Run(fmt.Sprintf("PartialOverlap-BbeforeA-B%d-A%d", itemsCountB, itemsCountA), func(b *testing.B) {
			f(b, start, itemsCountB, start+itemsCountB/2, itemsCountA)
		})

		b.Run(fmt.Sprintf("---FullOverlap-AbeforeB-A%d-B%d", itemsCountA, itemsCountB), func(b *testing.B) {
			f(b, start, itemsCountA, start+itemsCountA-itemsCountB, itemsCountB)
		})
		b.Run(fmt.Sprintf("---FullOverlap-AbeforeB-B%d-A%d", itemsCountB, itemsCountA), func(b *testing.B) {
			f(b, start+itemsCountA-itemsCountB, itemsCountB, start, itemsCountA)
		})
		b.Run(fmt.Sprintf("---FullOverlap-BbeforeA-A%d-B%d", itemsCountA, itemsCountB), func(b *testing.B) {
			f(b, start+itemsCountB, itemsCountA, start, itemsCountB)
		})
		b.Run(fmt.Sprintf("---FullOverlap-BbeforeA-B%d-A%d", itemsCountB, itemsCountA), func(b *testing.B) {
			f(b, start, itemsCountB, start+itemsCountB, itemsCountA)
		})
	}
}

func createRangeSet(start uint64, itemsCount int) *roaring64.Bitmap {
	s := roaring64.New()
	for i := 0; i < itemsCount; i++ {
		n := start + uint64(i)
		s.Add(n)
	}
	return s
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
					s := roaring64.New()
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
					s := roaring64.New()
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
			s := roaring64.New()
			var rng fastrand.RNG
			for i := 0; i < itemsCount; i++ {
				n := start | (uint64(rng.Uint32()) & mask)
				s.Add(n)
			}
			a := s.ToArray()

			b.ResetTimer()
			b.ReportAllocs()
			b.SetBytes(int64(len(a)))
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					for _, n := range a {
						if !s.Contains(n) {
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
			s := roaring64.New()
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
						if !s.Contains(n) {
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
			s := roaring64.New()
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
						if s.Contains(n) {
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
