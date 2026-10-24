package netstorage

import (
	"fmt"
	"testing"
)

const (
	benchScrapeMs = 30_000
	benchBaseTs   = 1_790_000_000_000
)

// benchSlot returns the timestamp of the scrape number k. The jitter is the same for all the replicas.
func benchSlot(k int) int64 {
	return benchBaseTs + int64(k)*benchScrapeMs + int64(k*7%11)
}

func benchValue(k int) float64 {
	return float64(k)*1.25 + float64(k%13)*0.001
}

// blockSpec is the immutable content of a block.
type blockSpec struct {
	ts []int64
	vs []float64
}

// slots returns n consecutive scrapes starting at slot start, shifted by shiftMs.
func slots(start, n int, shiftMs int64, vmul float64) blockSpec {
	s := blockSpec{ts: make([]int64, n), vs: make([]float64, n)}
	for i := range n {
		s.ts[i] = benchSlot(start+i) + shiftMs
		s.vs[i] = benchValue(start+i) * vmul
	}
	return s
}

// chain splits [0, total) scrapes into blocks of rows samples; the first block has firstRows samples (0 = rows).
func chain(total, rows, firstRows int, shiftMs int64, vmul float64) []blockSpec {
	var bs []blockSpec
	pos := 0
	n := rows
	if firstRows > 0 {
		n = firstRows
	}
	for pos < total {
		n = min(n, total-pos)
		bs = append(bs, slots(pos, n, shiftMs, vmul))
		pos += n
		n = rows
	}
	return bs
}

// real14 is a call captured on the kind RF=2 cluster (1 h instant query, tenant 4): two replica chains,
// one chain with interleaved head blocks.
func real14() []blockSpec {
	bs := []blockSpec{
		slots(0, 49, 0, 1), slots(0, 35, 0, 1), slots(35, 13, 0, 1), slots(48, 14, 0, 1), slots(49, 17, 0, 1),
		slots(62, 19, 0, 1), slots(66, 23, 0, 1), slots(81, 18, 0, 1), slots(89, 23, 0, 1), slots(99, 19, 0, 1),
	}
	pick := func(ks ...int) blockSpec {
		var s blockSpec
		for _, k := range ks {
			s.ts = append(s.ts, benchSlot(k))
			s.vs = append(s.vs, benchValue(k))
		}
		return s
	}
	return append(bs, pick(112, 115, 116), pick(113, 114), pick(117, 118, 119), pick(118, 119))
}

type mergeFunc func(dst *Result, sbh *sortBlocksHeap, dedupInterval int64)

var mergeVariants = []struct {
	name string
	f    mergeFunc
}{
	{"cur", mergeSortBlocksRef},
	{"new", mergeSortBlocks},
}

// BenchmarkMerge measures mergeSortBlocks variants, including a copy of the source blocks (same for all the variants, see "copy").
func BenchmarkMerge(b *testing.B) {
	type scenario struct {
		name   string
		blocks []blockSpec
	}
	var scs []scenario
	add := func(name string, bs ...[]blockSpec) {
		var all []blockSpec
		for _, x := range bs {
			all = append(all, x...)
		}
		scs = append(scs, scenario{name, all})
	}
	for _, rows := range []int{30, 1200, 8192} {
		add(fmt.Sprintf("a_single/R=%d", rows), []blockSpec{slots(0, rows, 0, 1)})
	}
	grid := [][2]int{{30, 3}, {30, 10}, {30, 30}, {1200, 3}, {1200, 10}, {8192, 3}}
	for _, g := range grid {
		rows, nb := g[0], g[1]
		total := rows * nb
		add(fmt.Sprintf("b_disjoint/R=%d/B=%d", rows, nb), chain(total, rows, 0, 0, 1))
		add(fmt.Sprintf("c_rf2_same/R=%d/B=%d", rows, nb), chain(total, rows, 0, 0, 1), chain(total, rows, 0, 0, 1))
		add(fmt.Sprintf("d_rf2_diff/R=%d/B=%d", rows, nb), chain(total, rows, 0, 0, 1), chain(total, rows, rows/3+1, 0, 1))
	}
	for _, g := range [][2]int{{30, 10}, {1200, 3}} {
		rows, nb := g[0], g[1]
		total := rows * nb
		// Same scrapes, timestamps shifted by a few ms and values slightly different.
		add(fmt.Sprintf("e_rf2_jitter/R=%d/B=%d", rows, nb), chain(total, rows, 0, 0, 1), chain(total, rows, rows/3+1, 4, 1.0000001))
		// Two out-of-order parts: the second one covers the second half of the first one.
		half := slots(total/2, total, 7, 1)
		var second []blockSpec
		for pos := 0; pos < total; pos += rows {
			second = append(second, blockSpec{half.ts[pos:min(pos+rows, total)], half.vs[pos:min(pos+rows, total)]})
		}
		add(fmt.Sprintf("f_partial/R=%d/B=%d", rows, nb), chain(total, rows, 0, 0, 1), second)
	}
	add("g_real14", real14())
	// Worst cases for the sweep merge: 3 replicas with different boundaries, and many small blocks nested in one big block.
	add("h_rf3_diff/R=30/B=10", chain(300, 30, 0, 0, 1), chain(300, 30, 11, 0, 1), chain(300, 30, 21, 0, 1))
	nested := []blockSpec{slots(0, 8192, 0, 1)}
	for i := range 100 {
		nested = append(nested, slots(i*80, 30, 3, 1))
	}
	add("i_nested/big=8192/small=100x30", nested)

	for _, sc := range scs {
		samples := 0
		for _, s := range sc.blocks {
			samples += len(s.ts)
		}
		for _, dedupInterval := range []int64{0, 30_000} {
			for _, v := range append([]struct {
				name string
				f    mergeFunc
			}{{"copy", nil}}, mergeVariants...) {
				name := fmt.Sprintf("%s/dedup=%d/%s", sc.name, dedupInterval/1000, v.name)
				b.Run(name, func(b *testing.B) {
					var dst Result
					sbh := getSortBlocksHeap()
					defer putSortBlocksHeap(sbh)
					b.ReportAllocs()
					for b.Loop() {
						dst.reset()
						sbs := sbh.sbs[:0]
						for _, s := range sc.blocks {
							sb := getSortBlock()
							sb.Timestamps = append(sb.Timestamps[:0], s.ts...)
							sb.Values = append(sb.Values[:0], s.vs...)
							sbs = append(sbs, sb)
						}
						sbh.sbs = sbs
						if v.f == nil {
							for _, sb := range sbs {
								putSortBlock(sb)
							}
							sbh.sbs = sbs[:0]
							continue
						}
						v.f(&dst, sbh, dedupInterval)
					}
					el := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
					b.ReportMetric(el/float64(samples), "ns/sample")
					b.ReportMetric(el/float64(len(sc.blocks)), "ns/block")
				})
			}
		}
	}
}
