package bloomfilter

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestFilter(t *testing.T) {
	for _, maxItems := range []int{1e0, 1e1, 1e2, 1e3, 1e4, 1e5} {
		testFilter(t, maxItems)
	}
}

func testFilter(t *testing.T, maxItems int) {
	r := rand.New(rand.NewSource(int64(0)))
	f := newFilter(maxItems)
	items := make(map[uint64]struct{}, maxItems)

	// Populate f with maxItems
	collisions := 0
	for i := 0; i < maxItems; i++ {
		h := r.Uint64()
		items[h] = struct{}{}
		if !f.Add(h) {
			collisions++
		}
		if f.Add(h) {
			t.Fatalf("unexpected double addition of item %d on iteration %d for maxItems %d", h, i, maxItems)
		}
	}
	p := float64(collisions) / float64(maxItems)
	if p > 0.0006 {
		t.Fatalf("too big collision share for maxItems=%d: %.5f, collisions: %d", maxItems, p, collisions)
	}

	// Verify that the added items exist in f.
	i := 0
	for h := range items {
		if !f.Has(h) {
			t.Fatalf("cannot find item %d on iteration %d for maxItems %d", h, i, maxItems)
		}
		i++
	}

	// Verify false hits rate.
	i = 0
	falseHits := 0
	for i < maxItems {
		h := r.Uint64()
		if _, ok := items[h]; ok {
			continue
		}
		i++
		if f.Has(h) {
			falseHits++
		}
	}
	p = float64(falseHits) / float64(maxItems)
	if p > 0.003 {
		t.Fatalf("too big false hits share for maxItems=%d: %.5f, falseHits: %d", maxItems, p, falseHits)
	}

	// Check filter reset
	f.Reset()
	for i := 0; i < maxItems; i++ {
		h := r.Uint64()
		if f.Has(h) {
			t.Fatalf("unexpected item found in empty filter: %d", h)
		}
	}
}

func TestFilterConcurrent(t *testing.T) {
	concurrency := 3
	maxItems := 10000
	doneCh := make(chan struct{}, concurrency)
	f := newFilter(maxItems)
	for i := 0; i < concurrency; i++ {
		go func(randSeed int) {
			r := rand.New(rand.NewSource(int64(randSeed)))
			for i := 0; i < maxItems; i++ {
				h := r.Uint64()
				f.Add(h)
				if !f.Has(h) {
					panic(fmt.Errorf("the item %d must exist", h))
				}
			}
			doneCh <- struct{}{}
		}(i)
	}
	tC := time.After(time.Second * 5)
	for i := 0; i < concurrency; i++ {
		select {
		case <-doneCh:
		case <-tC:
			t.Fatalf("timeout!")
		}
	}
}
