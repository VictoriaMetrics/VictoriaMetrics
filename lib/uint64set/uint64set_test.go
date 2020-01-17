package uint64set

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestSetBasicOps(t *testing.T) {
	for _, itemsCount := range []int{1, 2, 3, 4, 5, 6, 1e2, 1e3, 1e4, 1e5, 1e6, maxUnsortedBuckets * bitsPerBucket * 2} {
		t.Run(fmt.Sprintf("items_%d", itemsCount), func(t *testing.T) {
			testSetBasicOps(t, itemsCount)
		})
	}
}

func testSetBasicOps(t *testing.T, itemsCount int) {
	var s Set

	offset := uint64(time.Now().UnixNano())

	// Verify operations on nil set
	{
		var sNil *Set
		if n := sNil.SizeBytes(); n != 0 {
			t.Fatalf("sNil.SizeBytes must return 0; got %d", n)
		}
		if sNil.Has(123) {
			t.Fatalf("sNil shouldn't contain any item; found 123")
		}
		if n := sNil.Len(); n != 0 {
			t.Fatalf("unexpected sNil.Len(); got %d; want 0", n)
		}
		result := sNil.AppendTo(nil)
		if result != nil {
			t.Fatalf("sNil.AppendTo(nil) must return nil")
		}
		buf := []uint64{1, 2, 3}
		result = sNil.AppendTo(buf)
		if !reflect.DeepEqual(result, buf) {
			t.Fatalf("sNil.AppendTo(buf) must return buf")
		}
		sCopy := sNil.Clone()
		if n := sCopy.Len(); n != 0 {
			t.Fatalf("unexpected sCopy.Len() from nil set; got %d; want 0", n)
		}
		sCopy.Add(123)
		if n := sCopy.Len(); n != 1 {
			t.Fatalf("unexpected sCopy.Len() after adding an item; got %d; want 1", n)
		}
		sCopy.Add(123)
		if n := sCopy.Len(); n != 1 {
			t.Fatalf("unexpected sCopy.Len() after adding an item twice; got %d; want 1", n)
		}
		if !sCopy.Has(123) {
			t.Fatalf("sCopy must contain 123")
		}
		sCopy.Del(123)
		if n := sCopy.Len(); n != 0 {
			t.Fatalf("unexpected sCopy.Len() after deleting the item; got %d; want 0", n)
		}
		sCopy.Del(123)
		if n := sCopy.Len(); n != 0 {
			t.Fatalf("unexpected sCopy.Len() after double deleting the item; got %d; want 0", n)
		}
	}

	// Verify forward Add
	itemsCount = (itemsCount / 2) * 2
	for i := 0; i < itemsCount/2; i++ {
		s.Add(uint64(i) + offset)
	}
	if n := s.Len(); n != itemsCount/2 {
		t.Fatalf("unexpected s.Len() after forward Add; got %d; want %d", n, itemsCount/2)
	}
	if n := s.SizeBytes(); n == 0 {
		t.Fatalf("s.SizeBytes() must be greater than 0")
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
			t.Fatalf("missing bit %d", uint64(i)+offset)
		}
	}

	// Verify Has on missing bits
	for i := itemsCount; i < 2*itemsCount; i++ {
		if s.Has(uint64(i) + offset) {
			t.Fatalf("unexpected bit found: %d", uint64(i)+offset)
		}
	}

	// Verify Clone and Equal
	sCopy := s.Clone()
	if n := sCopy.Len(); n != itemsCount {
		t.Fatalf("unexpected sCopy.Len(); got %d; want %d", n, itemsCount)
	}
	for i := 0; i < itemsCount; i++ {
		if !sCopy.Has(uint64(i) + offset) {
			t.Fatalf("missing bit %d on sCopy", uint64(i)+offset)
		}
	}
	if !sCopy.Equal(&s) {
		t.Fatalf("s must equal to sCopy")
	}
	if !s.Equal(sCopy) {
		t.Fatalf("sCopy must equal to s")
	}
	if s.Len() > 0 {
		var sEmpty Set
		if s.Equal(&sEmpty) {
			t.Fatalf("s mustn't equal to sEmpty")
		}
		sNew := s.Clone()
		sNew.Del(offset)
		if sNew.Equal(&s) {
			t.Fatalf("sNew mustn't equal to s")
		}
		if s.Equal(sNew) {
			t.Fatalf("s mustn't equal to sNew")
		}
		sNew.Add(offset - 123)
		if sNew.Equal(&s) {
			t.Fatalf("sNew mustn't equal to s")
		}
		if s.Equal(sNew) {
			t.Fatalf("s mustn't equal to sNew")
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
			t.Fatalf("missing bit %d in the exported bits; array:\n%d", uint64(i)+offset, a)
		}
	}

	// Verify ForEach
	{
		var s Set
		m := make(map[uint64]bool)
		for i := 0; i < itemsCount; i++ {
			v := uint64(i) + offset
			s.Add(v)
			m[v] = true
		}

		// Verify visiting all the items.
		s.ForEach(func(part []uint64) bool {
			for _, v := range part {
				if !m[v] {
					t.Fatalf("unexpected value v=%d passed to ForEach", v)
				}
				delete(m, v)
			}
			return true
		})
		if len(m) != 0 {
			t.Fatalf("ForEach didn't visit %d items; items: %v", len(m), m)
		}

		// Verify fast stop
		calls := 0
		s.ForEach(func(part []uint64) bool {
			calls++
			return false
		})
		if itemsCount > 0 && calls != 1 {
			t.Fatalf("Unexpected number of ForEach callback calls; got %d; want %d", calls, 1)
		}

		// Verify ForEach on nil set.
		var s1 *Set
		s1.ForEach(func(part []uint64) bool {
			t.Fatalf("callback shouldn't be called on empty set")
			return true
		})
	}

	// Verify union
	{
		const unionOffset = 12345
		var s1, s2 Set
		for i := 0; i < itemsCount; i++ {
			s1.Add(uint64(i) + offset)
			s2.Add(uint64(i) + offset + unionOffset)
		}
		s1.Union(&s2)
		expectedLen := 2 * itemsCount
		if itemsCount > unionOffset {
			expectedLen = itemsCount + unionOffset
		}
		if n := s1.Len(); n != expectedLen {
			t.Fatalf("unexpected s1.Len() after union; got %d; want %d", n, expectedLen)
		}

		// Verify union on empty set.
		var s3 Set
		s3.Union(&s1)
		expectedLen = s1.Len()
		if n := s3.Len(); n != expectedLen {
			t.Fatalf("unexpected s3.Len() after union with empty set; got %d; want %d", n, expectedLen)
		}
		var s4 Set
		expectedLen = s3.Len()
		s3.Union(&s4)
		if n := s3.Len(); n != expectedLen {
			t.Fatalf("unexpected s3.Len() after union with empty set; got %d; want %d", n, expectedLen)
		}
	}

	// Verify UnionMayOwn
	{
		const unionOffset = 12345
		var s1, s2 Set
		for i := 0; i < itemsCount; i++ {
			s1.Add(uint64(i) + offset)
			s2.Add(uint64(i) + offset + unionOffset)
		}
		s1.UnionMayOwn(&s2)
		expectedLen := 2 * itemsCount
		if itemsCount > unionOffset {
			expectedLen = itemsCount + unionOffset
		}
		if n := s1.Len(); n != expectedLen {
			t.Fatalf("unexpected s1.Len() after union; got %d; want %d", n, expectedLen)
		}

		// Verify union on empty set.
		var s3 Set
		expectedLen = s1.Len()
		s3.UnionMayOwn(&s1)
		if n := s3.Len(); n != expectedLen {
			t.Fatalf("unexpected s3.Len() after union with empty set; got %d; want %d", n, expectedLen)
		}
		var s4 Set
		expectedLen = s3.Len()
		s3.UnionMayOwn(&s4)
		if n := s3.Len(); n != expectedLen {
			t.Fatalf("unexpected s3.Len() after union with empty set; got %d; want %d", n, expectedLen)
		}
	}

	// Verify intersect
	{
		// Verify s1.Intersect(s2) and s2.Intersect(s1)
		var s1, s2 Set
		for _, intersectOffset := range []uint64{123, 12345, 1<<32 + 4343} {
			s1 = Set{}
			s2 = Set{}
			for i := 0; i < itemsCount; i++ {
				s1.Add(uint64(i) + offset)
				s2.Add(uint64(i) + offset + intersectOffset)
			}
			expectedLen := 0
			if uint64(itemsCount) > intersectOffset {
				expectedLen = int(uint64(itemsCount) - intersectOffset)
			}
			s1Copy := s1.Clone()
			s1Copy.Intersect(&s2)
			if n := s1Copy.Len(); n != expectedLen {
				t.Fatalf("unexpected s1.Len() after intersect; got %d; want %d", n, expectedLen)
			}
			s2.Intersect(&s1)
			if n := s2.Len(); n != expectedLen {
				t.Fatalf("unexpected s2.Len() after intersect; got %d; want %d", n, expectedLen)
			}
		}

		// Verify intersect on empty set.
		var s3 Set
		s2.Intersect(&s3)
		expectedLen := 0
		if n := s2.Len(); n != expectedLen {
			t.Fatalf("unexpected s3.Len() after intersect with empty set; got %d; want %d", n, expectedLen)
		}
		var s4 Set
		s4.Intersect(&s1)
		if n := s4.Len(); n != expectedLen {
			t.Fatalf("unexpected s4.Len() after intersect with empty set; got %d; want %d", n, expectedLen)
		}
	}

	// Verify subtract
	{
		const subtractOffset = 12345
		var s1, s2 Set
		for i := 0; i < itemsCount; i++ {
			s1.Add(uint64(i) + offset)
			s2.Add(uint64(i) + offset + subtractOffset)
		}
		s1.Subtract(&s2)
		expectedLen := itemsCount
		if itemsCount > subtractOffset {
			expectedLen = subtractOffset
		}
		if n := s1.Len(); n != expectedLen {
			t.Fatalf("unexpected s1.Len() after subtract; got %d; want %d", n, expectedLen)
		}

		// Verify subtract from empty set.
		var s3 Set
		s3.Subtract(&s2)
		expectedLen = 0
		if n := s3.Len(); n != 0 {
			t.Fatalf("unexpected s3.Len() after subtract from empty set; got %d; want %d", n, expectedLen)
		}
	}

	// Verify Del
	itemsDeleted := 0
	for i := itemsCount / 2; i < itemsCount-itemsCount/4; i++ {
		s.Del(uint64(i) + offset)
		itemsDeleted++
	}
	if n := s.Len(); n != itemsCount-itemsDeleted {
		t.Fatalf("unexpected s.Len() after Del; got %d; want %d", n, itemsCount-itemsDeleted)
	}
	a = s.AppendTo(a[:0])
	if len(a) != itemsCount-itemsDeleted {
		t.Fatalf("unexpected len of exported array; got %d; want %d", len(a), itemsCount-itemsDeleted)
	}
	m = make(map[uint64]bool)
	for _, x := range a {
		m[x] = true
	}
	for i := 0; i < itemsCount; i++ {
		if i >= itemsCount/2 && i < itemsCount-itemsCount/4 {
			if m[uint64(i)+offset] {
				t.Fatalf("unexpected bit found after deleting: %d", uint64(i)+offset)
			}
		} else {
			if !m[uint64(i)+offset] {
				t.Fatalf("missing bit %d in the exported bits after deleting", uint64(i)+offset)
			}
		}
	}

	// Try Del for non-existing items
	for i := itemsCount / 2; i < itemsCount-itemsCount/4; i++ {
		s.Del(uint64(i) + offset)
		s.Del(uint64(i) + offset)
		s.Del(uint64(i) + offset + uint64(itemsCount))
	}
	if n := s.Len(); n != itemsCount-itemsDeleted {
		t.Fatalf("unexpected s.Len() after Del for non-existing items; got %d; want %d", n, itemsCount-itemsDeleted)
	}

	// Verify sCopy has the original data
	if n := sCopy.Len(); n != itemsCount {
		t.Fatalf("unexpected sCopy.Len(); got %d; want %d", n, itemsCount)
	}
	for i := 0; i < itemsCount; i++ {
		if !sCopy.Has(uint64(i) + offset) {
			t.Fatalf("missing bit %d on sCopy", uint64(i)+offset)
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
	if n := s.SizeBytes(); n == 0 {
		t.Fatalf("SizeBytes() must return value greater than 0")
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
