package bloomfilter

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	for _, maxItems := range []int{1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6} {
		testLimiter(t, maxItems)
	}
}

func testLimiter(t *testing.T, maxItems int) {
	r := rand.New(rand.NewSource(int64(0)))
	l := NewLimiter(maxItems, time.Hour)
	if n := l.MaxItems(); n != maxItems {
		t.Fatalf("unexpected maxItems returned; got %d; want %d", n, maxItems)
	}
	items := make(map[uint64]struct{}, maxItems)

	// Populate the l with new items.
	for i := 0; i < maxItems; i++ {
		h := r.Uint64()
		if !l.Add(h) {
			t.Fatalf("cannot add item %d on iteration %d out of %d", h, i, maxItems)
		}
		items[h] = struct{}{}
	}

	// Verify that already registered items can be added.
	i := 0
	for h := range items {
		if !l.Add(h) {
			t.Fatalf("cannot add already existing item %d on iteration %d out of %d", h, i, maxItems)
		}
		i++
	}

	// Verify that new items are rejected with high probability.
	falseAdditions := 0
	for i := 0; i < maxItems; i++ {
		h := r.Uint64()
		if l.Add(h) {
			falseAdditions++
		}
	}
	p := float64(falseAdditions) / float64(maxItems)
	if p > 0.003 {
		t.Fatalf("too big false additions share=%.5f: %d out of %d", p, falseAdditions, maxItems)
	}
}

func TestLimiterConcurrent(t *testing.T) {
	concurrency := 3
	maxItems := 10000
	l := NewLimiter(maxItems, time.Hour)
	doneCh := make(chan struct{}, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			if n := l.MaxItems(); n != maxItems {
				panic(fmt.Errorf("unexpected maxItems returned; got %d; want %d", n, maxItems))
			}
			r := rand.New(rand.NewSource(0))
			for i := 0; i < maxItems; i++ {
				h := r.Uint64()
				// Do not check whether the item is added, since less than maxItems can be added to l
				// due to passible (expected) race in l.f.Add
				l.Add(h)
			}
			// Verify that new items are rejected with high probability.
			falseAdditions := 0
			for i := 0; i < maxItems; i++ {
				h := r.Uint64()
				if l.Add(h) {
					falseAdditions++
				}
			}
			p := float64(falseAdditions) / float64(maxItems)
			if p > 0.0035 {
				panic(fmt.Errorf("too big false additions share=%.5f: %d out of %d", p, falseAdditions, maxItems))
			}
			doneCh <- struct{}{}
		}()
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
