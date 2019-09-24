package uint64set

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"
)

func TestSetBasicOps(t *testing.T) {
	for _, itemsCount := range []int{1e2, 1e3, 1e4, 1e5, 1e6, maxUnsortedBuckets * bitsPerBucket * 2} {
		t.Run(fmt.Sprintf("items_%d", itemsCount), func(t *testing.T) {
			testSetBasicOps(t, itemsCount)
		})
	}
}

func testSetBasicOps(t *testing.T, itemsCount int) {
	var s Set

	offset := uint64(time.Now().UnixNano())

	// Verify forward Add
	for i := 0; i < itemsCount/2; i++ {
		s.Add(uint64(i) + offset)
	}
	if n := s.Len(); n != itemsCount/2 {
		t.Fatalf("unexpected s.Len() after forward Add; got %d; want %d", n, itemsCount/2)
	}

	// Verify backward Add
	for i := 0; i < itemsCount/2; i++ {
		s.Add(uint64(itemsCount-i-1) + offset)
	}
	if n := s.Len(); n != itemsCount {
		t.Fatalf("unexpected s.Len() after backward Add; got %d; want %d", n, itemsCount)
	}

	// Verify repeated Add
	for i := 0; i < itemsCount/2; i++ {
		s.Add(uint64(i) + offset)
	}
	if n := s.Len(); n != itemsCount {
		t.Fatalf("unexpected s.Len() after repeated Add; got %d; want %d", n, itemsCount)
	}

	// Verify Has on existing bits
	for i := 0; i < itemsCount; i++ {
		if !s.Has(uint64(i) + offset) {
			t.Fatalf("missing bit %d", i)
		}
	}

	// Verify Has on missing bits
	for i := itemsCount; i < 2*itemsCount; i++ {
		if s.Has(uint64(i) + offset) {
			t.Fatalf("unexpected bit found: %d", i)
		}
	}

	// Verify Clone
	sCopy := s.Clone()
	if n := sCopy.Len(); n != itemsCount {
		t.Fatalf("unexpected sCopy.Len(); got %d; want %d", n, itemsCount)
	}
	for i := 0; i < itemsCount; i++ {
		if !sCopy.Has(uint64(i) + offset) {
			t.Fatalf("missing bit %d on sCopy", i)
		}
	}

	// Verify AppendTo
	a := s.AppendTo(nil)
	if len(a) != itemsCount {
		t.Fatalf("unexpected len of exported array; got %d; want %d; array:\n%d", len(a), itemsCount, a)
	}
	if !sort.SliceIsSorted(a, func(i, j int) bool { return a[i] < a[j] }) {
		t.Fatalf("unsorted result returned from AppendTo: %d", a)
	}
	m := make(map[uint64]bool)
	for _, x := range a {
		m[x] = true
	}
	for i := 0; i < itemsCount; i++ {
		if !m[uint64(i)+offset] {
			t.Fatalf("missing bit %d in the exported bits; array:\n%d", i, a)
		}
	}

	// Verify Del
	for i := itemsCount / 2; i < itemsCount-itemsCount/4; i++ {
		s.Del(uint64(i) + offset)
	}
	if n := s.Len(); n != itemsCount-itemsCount/4 {
		t.Fatalf("unexpected s.Len() after Del; got %d; want %d", n, itemsCount-itemsCount/4)
	}
	a = s.AppendTo(a[:0])
	if len(a) != itemsCount-itemsCount/4 {
		t.Fatalf("unexpected len of exported array; got %d; want %d", len(a), itemsCount-itemsCount/4)
	}
	m = make(map[uint64]bool)
	for _, x := range a {
		m[x] = true
	}
	for i := 0; i < itemsCount; i++ {
		if i >= itemsCount/2 && i < itemsCount-itemsCount/4 {
			if m[uint64(i)+offset] {
				t.Fatalf("unexpected bit found after deleting: %d", i)
			}
		} else {
			if !m[uint64(i)+offset] {
				t.Fatalf("missing bit %d in the exported bits after deleting", i)
			}
		}
	}

	// Try Del for non-existing items
	for i := itemsCount / 2; i < itemsCount-itemsCount/4; i++ {
		s.Del(uint64(i) + offset)
		s.Del(uint64(i) + offset)
		s.Del(uint64(i) + offset + uint64(itemsCount))
	}
	if n := s.Len(); n != itemsCount-itemsCount/4 {
		t.Fatalf("unexpected s.Len() after Del for non-existing items; got %d; want %d", n, itemsCount-itemsCount/4)
	}

	// Verify sCopy has the original data
	if n := sCopy.Len(); n != itemsCount {
		t.Fatalf("unexpected sCopy.Len(); got %d; want %d", n, itemsCount)
	}
	for i := 0; i < itemsCount; i++ {
		if !sCopy.Has(uint64(i) + offset) {
			t.Fatalf("missing bit %d on sCopy", i)
		}
	}
}

func TestSetSparseItems(t *testing.T) {
	for _, itemsCount := range []int{1e2, 1e3, 1e4} {
		t.Run(fmt.Sprintf("items_%d", itemsCount), func(t *testing.T) {
			testSetSparseItems(t, itemsCount)
		})
	}
}

func testSetSparseItems(t *testing.T, itemsCount int) {
	var s Set
	m := make(map[uint64]bool)
	for i := 0; i < itemsCount; i++ {
		x := rand.Uint64()
		s.Add(x)
		m[x] = true
	}
	if n := s.Len(); n != len(m) {
		t.Fatalf("unexpected Len(); got %d; want %d", n, len(m))
	}

	// Check Has
	for x := range m {
		if !s.Has(x) {
			t.Fatalf("missing item %d", x)
		}
	}
	for i := 0; i < itemsCount; i++ {
		x := uint64(i)
		if m[x] {
			continue
		}
		if s.Has(x) {
			t.Fatalf("unexpected item found %d", x)
		}
	}

	// Check Clone
	sCopy := s.Clone()
	if n := sCopy.Len(); n != len(m) {
		t.Fatalf("unexpected sCopy.Len(); got %d; want %d", n, len(m))
	}
	for x := range m {
		if !sCopy.Has(x) {
			t.Fatalf("missing item %d on sCopy", x)
		}
	}

	// Check AppendTo
	a := s.AppendTo(nil)
	if len(a) != len(m) {
		t.Fatalf("unexpected len for AppendTo result; got %d; want %d", len(a), len(m))
	}
	if !sort.SliceIsSorted(a, func(i, j int) bool { return a[i] < a[j] }) {
		t.Fatalf("unsorted result returned from AppendTo: %d", a)
	}
	for _, x := range a {
		if !m[x] {
			t.Fatalf("unexpected item found in AppendTo result: %d", x)
		}
	}

	// Check Del
	for x := range m {
		s.Del(x)
		s.Del(x)
		s.Del(x + 1)
		s.Del(x - 1)
	}
	if n := s.Len(); n != 0 {
		t.Fatalf("unexpected number of items left after Del; got %d; want 0", n)
	}
	a = s.AppendTo(a[:0])
	if len(a) != 0 {
		t.Fatalf("unexpected number of items returned from AppendTo after Del; got %d; want 0; items\n%d", len(a), a)
	}

	// Check items in sCopy
	if n := sCopy.Len(); n != len(m) {
		t.Fatalf("unexpected sCopy.Len() after Del; got %d; want %d", n, len(m))
	}
	for x := range m {
		if !sCopy.Has(x) {
			t.Fatalf("missing item %d on sCopy after Del", x)
		}
	}
}
