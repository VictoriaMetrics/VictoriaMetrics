package netstorage

import (
	"math"
	"math/rand"
	"slices"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
)

type mergeResult struct {
	ts          []int64
	vs          []uint64
	rowsSkipped int
	dedupDelta  uint64
}

func runMerge(f mergeFunc, specs []blockSpec, skipped []int, dedupInterval int64) mergeResult {
	sbh := getSortBlocksHeap()
	defer putSortBlocksHeap(sbh)
	for i, s := range specs {
		sb := getSortBlock()
		sb.Timestamps = append(sb.Timestamps[:0], s.ts...)
		sb.Values = append(sb.Values[:0], s.vs...)
		sb.rowsSkipped = skipped[i]
		sbh.sbs = append(sbh.sbs, sb)
	}
	var dst Result
	before := dedupsDuringSelect.Get()
	f(&dst, sbh, dedupInterval)
	res := mergeResult{
		ts:          slices.Clone(dst.Timestamps),
		rowsSkipped: dst.rowsSkipped,
		dedupDelta:  dedupsDuringSelect.Get() - before,
	}
	for _, v := range dst.Values {
		res.vs = append(res.vs, math.Float64bits(v))
	}
	return res
}

func checkMerge(t *testing.T, specs []blockSpec, dedupInterval int64) (fast bool) {
	t.Helper()
	skipped := make([]int, len(specs))
	for i := range skipped {
		skipped[i] = i % 3
	}
	want := runMerge(mergeSortBlocksRef, specs, skipped, dedupInterval)
	got := runMerge(mergeSortBlocks, specs, skipped, dedupInterval)
	if !slices.Equal(want.ts, got.ts) || !slices.Equal(want.vs, got.vs) || want.rowsSkipped != got.rowsSkipped || want.dedupDelta != got.dedupDelta {
		t.Fatalf("mismatch for dedupInterval=%d, specs=%v\nwant ts=%v\n got ts=%v\nwant vs=%x\n got vs=%x\nrowsSkipped %d/%d, dedup counter %d/%d",
			dedupInterval, specs, want.ts, got.ts, want.vs, got.vs, want.rowsSkipped, got.rowsSkipped, want.dedupDelta, got.dedupDelta)
	}

	// Count the calls handled without the heap.
	sbh := getSortBlocksHeap()
	defer putSortBlocksHeap(sbh)
	for _, s := range specs {
		if len(s.ts) == 0 {
			continue
		}
		sb := getSortBlock()
		sb.Timestamps = append(sb.Timestamps[:0], s.ts...)
		sb.Values = append(sb.Values[:0], s.vs...)
		sbh.sbs = append(sbh.sbs, sb)
	}
	if len(sbh.sbs) == 0 {
		return false
	}
	var dst Result
	return mergeFast(&dst, sbh, dedupInterval)
}

func valueFor(ts int64, sel int) float64 {
	switch sel {
	case 6:
		return math.NaN()
	case 7:
		return decimal.StaleNaN
	case 5:
		return float64(ts)*0.5 + 1 // conflicting value
	}
	return float64(ts) * 0.5
}

// genSpecs generates blocks of replicated series: random cuts, deleted samples, conflicts, in-block duplicates, empty blocks.
func genSpecs(rng *rand.Rand) ([]blockSpec, int64) {
	scale := []int64{1, 1000, 30000}[rng.Intn(3)]
	off := []int64{0, 1 << 40, -(1 << 40), 1_790_000_000_000}[rng.Intn(4)]
	nBase := rng.Intn(70)
	base := make([]int64, nBase)
	cur := off
	for i := range base {
		step := int64(1 + rng.Intn(3))
		if rng.Intn(40) == 0 {
			step = 0 // equal timestamps in one block
		}
		cur += step * scale
		base[i] = cur
	}
	conflictPct, deletePct, jitterPct := 0, 0, 0
	switch rng.Intn(4) {
	case 1:
		deletePct = 10
	case 2:
		conflictPct = 5
	case 3:
		deletePct, jitterPct, conflictPct = 20, 20, 2
	}
	var specs []blockSpec
	for range 1 + rng.Intn(3) {
		var s blockSpec
		for _, ts := range base {
			if rng.Intn(100) < deletePct {
				continue
			}
			sel := 0
			if rng.Intn(100) < conflictPct {
				sel = 5
			}
			if rng.Intn(50) == 0 {
				sel = 6 + rng.Intn(2)
			}
			v := valueFor(ts, sel)
			if rng.Intn(100) < jitterPct {
				ts++
			}
			if n := len(s.ts); n > 0 {
				ts = max(ts, s.ts[n-1])
			}
			s.ts = append(s.ts, ts)
			s.vs = append(s.vs, v)
		}
		// Cut into blocks, sometimes sharing the boundary sample.
		for len(s.ts) > 0 {
			n := 1 + rng.Intn(len(s.ts))
			if rng.Intn(3) == 0 {
				n = len(s.ts)
			}
			specs = append(specs, blockSpec{slices.Clone(s.ts[:n]), slices.Clone(s.vs[:n])})
			if rng.Intn(4) == 0 && n > 1 {
				n--
			}
			s.ts, s.vs = s.ts[n:], s.vs[n:]
		}
	}
	if rng.Intn(10) == 0 {
		specs = append(specs, blockSpec{})
	}
	rng.Shuffle(len(specs), func(i, j int) { specs[i], specs[j] = specs[j], specs[i] })
	return specs, []int64{0, 1, 1000, 30000}[rng.Intn(4)]
}

func TestMergeSortBlocksDiff(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	fast, total := 0, 0
	for range 200_000 {
		specs, dedupInterval := genSpecs(rng)
		if checkMerge(t, specs, dedupInterval) {
			fast++
		}
		total++
	}
	t.Logf("fast path: %d of %d cases", fast, total)
	if fast < total/4 {
		t.Fatalf("fast path is too rare: %d of %d", fast, total)
	}
}

// decodeSpecs builds blocks from fuzz bytes.
func decodeSpecs(data []byte) ([]blockSpec, int64) {
	next := func() int {
		if len(data) == 0 {
			return 0
		}
		b := data[0]
		data = data[1:]
		return int(b)
	}
	scale := []int64{1, 1000, 30000, 1 << 20}[next()%4]
	off := []int64{0, 1 << 40, -(1 << 40)}[next()%3]
	dedupInterval := []int64{0, 1, 1000, 30000}[next()%4]
	var specs []blockSpec
	for range next() % 9 {
		var s blockSpec
		ts := off + int64(next()%16)*scale
		for range next() % 16 {
			ts += int64(next()%4) * scale
			s.ts = append(s.ts, ts)
			s.vs = append(s.vs, valueFor(ts, next()%12))
		}
		specs = append(specs, s)
	}
	return specs, dedupInterval
}

func FuzzMergeSortBlocks(f *testing.F) {
	rng := rand.New(rand.NewSource(2))
	for range 50 {
		data := make([]byte, 200)
		rng.Read(data)
		f.Add(data)
	}
	// Two replicas with different boundaries.
	f.Add([]byte{1, 0, 2, 2, 0, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 3, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		specs, dedupInterval := decodeSpecs(data)
		checkMerge(t, specs, dedupInterval)
	})
}
