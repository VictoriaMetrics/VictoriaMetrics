package storage

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestPartitionMaxRowsByPath(t *testing.T) {
	n := maxRowsByPath(".")
	if n < 1e3 {
		t.Fatalf("too small number of rows can be created in the current directory: %d", n)
	}
}

func TestAppendPartsToMerge(t *testing.T) {
	testAppendPartsToMerge(t, 2, []int{}, nil)
	testAppendPartsToMerge(t, 2, []int{123}, nil)
	testAppendPartsToMerge(t, 2, []int{4, 2}, nil)
	testAppendPartsToMerge(t, 2, []int{128, 64, 32, 16, 8, 4, 2, 1}, nil)
	testAppendPartsToMerge(t, 4, []int{128, 64, 32, 10, 9, 7, 2, 1}, []int{2, 7, 9, 10})
	testAppendPartsToMerge(t, 2, []int{128, 64, 32, 16, 8, 4, 2, 2}, []int{2, 2})
	testAppendPartsToMerge(t, 4, []int{128, 64, 32, 16, 8, 4, 2, 2}, []int{2, 2, 4, 8})
	testAppendPartsToMerge(t, 2, []int{1, 1}, []int{1, 1})
	testAppendPartsToMerge(t, 2, []int{2, 2, 2}, []int{2, 2})
	testAppendPartsToMerge(t, 2, []int{4, 2, 4}, []int{4, 4})
	testAppendPartsToMerge(t, 2, []int{1, 3, 7, 2}, nil)
	testAppendPartsToMerge(t, 3, []int{1, 3, 7, 2}, []int{1, 2, 3})
	testAppendPartsToMerge(t, 4, []int{1, 3, 7, 2}, []int{1, 2, 3})
	testAppendPartsToMerge(t, 3, []int{11, 1, 10, 100, 10}, []int{10, 10, 11})
}

func TestAppendPartsToMergeManyParts(t *testing.T) {
	// Verify that big number of parts are merged into minimal number of parts
	// using minimum merges.
	var a []int
	maxOutPartRows := uint64(0)
	for i := 0; i < 1024; i++ {
		n := int(rand.NormFloat64() * 1e9)
		if n < 0 {
			n = -n
		}
		n++
		maxOutPartRows += uint64(n)
		a = append(a, n)
	}
	pws := newTestPartWrappersForRowsCount(a)

	iterationsCount := 0
	rowsMerged := uint64(0)
	for {
		pms := appendPartsToMerge(nil, pws, defaultPartsToMerge, maxOutPartRows)
		if len(pms) == 0 {
			break
		}
		m := make(map[*partWrapper]bool)
		for _, pw := range pms {
			m[pw] = true
		}
		var pwsNew []*partWrapper
		rowsCount := uint64(0)
		for _, pw := range pws {
			if m[pw] {
				rowsCount += pw.p.ph.RowsCount
			} else {
				pwsNew = append(pwsNew, pw)
			}
		}
		pw := &partWrapper{
			p: &part{
				ph: partHeader{
					RowsCount: rowsCount,
				},
			},
		}
		rowsMerged += rowsCount
		pwsNew = append(pwsNew, pw)
		pws = pwsNew
		iterationsCount++
	}
	rowsCount := newTestRowsCountFromPartWrappers(pws)
	rowsTotal := uint64(0)
	for _, rc := range rowsCount {
		rowsTotal += uint64(rc)
	}
	overhead := float64(rowsMerged) / float64(rowsTotal)
	if overhead > 2.96 {
		t.Fatalf("too big overhead; rowsCount=%d, iterationsCount=%d, rowsTotal=%d, rowsMerged=%d, overhead=%f",
			rowsCount, iterationsCount, rowsTotal, rowsMerged, overhead)
	}
	if len(rowsCount) > 40 {
		t.Fatalf("too many rowsCount %d; rowsCount=%d, iterationsCount=%d, rowsTotal=%d, rowsMerged=%d, overhead=%f",
			len(rowsCount), rowsCount, iterationsCount, rowsTotal, rowsMerged, overhead)
	}
}

func testAppendPartsToMerge(t *testing.T, maxPartsToMerge int, initialRowsCount, expectedRowsCount []int) {
	t.Helper()

	pws := newTestPartWrappersForRowsCount(initialRowsCount)

	// Verify appending to nil.
	pms := appendPartsToMerge(nil, pws, maxPartsToMerge, 1e9)
	rowsCount := newTestRowsCountFromPartWrappers(pms)
	if !reflect.DeepEqual(rowsCount, expectedRowsCount) {
		t.Fatalf("unexpected rowsCount for maxPartsToMerge=%d, initialRowsCount=%d; got\n%d; want\n%d",
			maxPartsToMerge, initialRowsCount, rowsCount, expectedRowsCount)
	}

	// Verify appending to prefix
	prefix := []*partWrapper{
		{
			p: &part{
				ph: partHeader{
					RowsCount: 1234,
				},
			},
		},
		{},
		{},
	}
	pms = appendPartsToMerge(prefix, pws, maxPartsToMerge, 1e9)
	if !reflect.DeepEqual(pms[:len(prefix)], prefix) {
		t.Fatalf("unexpected prefix for maxPartsToMerge=%d, initialRowsCount=%d; got\n%+v; want\n%+v",
			maxPartsToMerge, initialRowsCount, pms[:len(prefix)], prefix)
	}

	rowsCount = newTestRowsCountFromPartWrappers(pms[len(prefix):])
	if !reflect.DeepEqual(rowsCount, expectedRowsCount) {
		t.Fatalf("unexpected prefixed rowsCount for maxPartsToMerge=%d, initialRowsCount=%d; got\n%d; want\n%d",
			maxPartsToMerge, initialRowsCount, rowsCount, expectedRowsCount)
	}
}

func newTestRowsCountFromPartWrappers(pws []*partWrapper) []int {
	var rowsCount []int
	for _, pw := range pws {
		rowsCount = append(rowsCount, int(pw.p.ph.RowsCount))
	}
	return rowsCount
}

func newTestPartWrappersForRowsCount(rowsCount []int) []*partWrapper {
	var pws []*partWrapper
	for _, rc := range rowsCount {
		pw := &partWrapper{
			p: &part{
				ph: partHeader{
					RowsCount: uint64(rc),
				},
			},
		}
		pws = append(pws, pw)
	}
	return pws
}
