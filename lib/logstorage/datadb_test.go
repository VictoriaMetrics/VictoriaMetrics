package logstorage

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
)

func TestRowsBuffer(t *testing.T) {
	var rowsFlushed atomic.Uint64
	flushFunc := func(lr *logRows) {
		rowsFlushed.Add(uint64(lr.Len()))
	}
	var wgBuffer sync.WaitGroup

	var rb rowsBuffer
	rb.init(&wgBuffer, flushFunc)

	const concurrency = 10
	const rowsPerInsert = 200
	const insertLoops = 30
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			lr := newTestLogRows(1, rowsPerInsert, 1)
			for i := 0; i < insertLoops; i++ {
				rb.mustAddRows(lr)
			}
		}()
	}
	wg.Wait()

	rb.flush()
	wgBuffer.Wait()

	rowsLen := rowsFlushed.Load()
	rowsLenExpected := uint64(concurrency * rowsPerInsert * insertLoops)
	if rowsLen != rowsLenExpected {
		t.Fatalf("unexpected number of rows; got %d; want %d", rowsLen, rowsLenExpected)
	}
}

func TestAppendPartsToMergeManyParts(t *testing.T) {
	// Verify that big number of parts are merged into minimal number of parts
	// using minimum merges.
	var sizes []uint64
	maxOutSize := uint64(0)
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 1024; i++ {
		n := uint64(uint32(r.NormFloat64() * 1e9))
		n++
		maxOutSize += n
		sizes = append(sizes, n)
	}
	pws := newTestPartWrappersForSizes(sizes)

	iterationsCount := 0
	sizeMergedTotal := uint64(0)
	for {
		pms := appendPartsToMerge(nil, pws, maxOutSize)
		if len(pms) == 0 {
			break
		}
		m := make(map[*partWrapper]bool)
		for _, pw := range pms {
			m[pw] = true
		}
		var pwsNew []*partWrapper
		size := uint64(0)
		for _, pw := range pws {
			if m[pw] {
				size += pw.p.ph.CompressedSizeBytes
			} else {
				pwsNew = append(pwsNew, pw)
			}
		}
		pw := &partWrapper{
			p: &part{
				ph: partHeader{
					CompressedSizeBytes: size,
				},
			},
		}
		sizeMergedTotal += size
		pwsNew = append(pwsNew, pw)
		pws = pwsNew
		iterationsCount++
	}
	sizes = newTestSizesFromPartWrappers(pws)
	sizeTotal := uint64(0)
	for _, size := range sizes {
		sizeTotal += uint64(size)
	}
	overhead := float64(sizeMergedTotal) / float64(sizeTotal)
	if overhead > 2.1 {
		t.Fatalf("too big overhead; sizes=%d, iterationsCount=%d, sizeTotal=%d, sizeMergedTotal=%d, overhead=%f",
			sizes, iterationsCount, sizeTotal, sizeMergedTotal, overhead)
	}
	if len(sizes) > 18 {
		t.Fatalf("too many sizes %d; sizes=%d, iterationsCount=%d, sizeTotal=%d, sizeMergedTotal=%d, overhead=%f",
			len(sizes), sizes, iterationsCount, sizeTotal, sizeMergedTotal, overhead)
	}
}

func newTestSizesFromPartWrappers(pws []*partWrapper) []uint64 {
	var sizes []uint64
	for _, pw := range pws {
		sizes = append(sizes, pw.p.ph.CompressedSizeBytes)
	}
	return sizes
}

func newTestPartWrappersForSizes(sizes []uint64) []*partWrapper {
	var pws []*partWrapper
	for _, size := range sizes {
		pw := &partWrapper{
			p: &part{
				ph: partHeader{
					CompressedSizeBytes: size,
				},
			},
		}
		pws = append(pws, pw)
	}
	return pws
}
