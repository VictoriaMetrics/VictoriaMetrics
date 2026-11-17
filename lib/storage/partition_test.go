package storage

import (
	"errors"
	"math"
	"math/rand"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestPartitionGetMaxOutBytes(t *testing.T) {
	n := getMaxOutBytes(fs.MustGetFreeSpace("."), 1)
	if n < 1e3 {
		t.Fatalf("too small free space remained in the current directory: %d", n)
	}
	if got := getMaxOutBytes(40<<20, 4); got != 10<<20 {
		t.Fatalf("unexpected getMaxOutBytes(40MiB, 4): got %d, want %d", got, uint64(10<<20))
	}
}

func TestAppendPartsToMerge(t *testing.T) {
	testAppendPartsToMerge(t, 2, []uint64{}, nil)
	testAppendPartsToMerge(t, 2, []uint64{123}, nil)
	testAppendPartsToMerge(t, 2, []uint64{4, 2}, nil)
	testAppendPartsToMerge(t, 2, []uint64{128, 64, 32, 16, 8, 4, 2, 1}, nil)
	testAppendPartsToMerge(t, 4, []uint64{128, 64, 32, 10, 9, 7, 3, 1}, []uint64{3, 7, 9, 10})
	testAppendPartsToMerge(t, 2, []uint64{128, 64, 32, 16, 8, 4, 2, 2}, []uint64{2, 2})
	testAppendPartsToMerge(t, 4, []uint64{128, 64, 32, 16, 8, 4, 2, 2}, []uint64{2, 2, 4, 8})
	testAppendPartsToMerge(t, 2, []uint64{1, 1}, []uint64{1, 1})
	testAppendPartsToMerge(t, 2, []uint64{2, 2, 2}, []uint64{2, 2})
	testAppendPartsToMerge(t, 2, []uint64{4, 2, 4}, []uint64{4, 4})
	testAppendPartsToMerge(t, 2, []uint64{1, 3, 7, 2}, nil)
	testAppendPartsToMerge(t, 3, []uint64{1, 3, 7, 2}, []uint64{1, 2, 3})
	testAppendPartsToMerge(t, 4, []uint64{1, 3, 7, 2}, []uint64{1, 2, 3})
	testAppendPartsToMerge(t, 5, []uint64{1, 3, 7, 2}, nil)
	testAppendPartsToMerge(t, 4, []uint64{1e6, 3e6, 7e6, 2e6}, []uint64{1e6, 2e6, 3e6})
	testAppendPartsToMerge(t, 4, []uint64{2, 3, 7, 2}, []uint64{2, 2, 3})
	testAppendPartsToMerge(t, 5, []uint64{2, 3, 7, 2}, nil)
	testAppendPartsToMerge(t, 3, []uint64{11, 1, 10, 100, 10}, []uint64{10, 10, 11})
}

func TestAppendPartsToMergeManyParts(t *testing.T) {
	// Verify that big number of parts are merged into minimal number of parts
	// using minimum merges.
	var sizes []uint64
	maxOutSize := uint64(0)
	r := rand.New(rand.NewSource(1))
	for range 1024 {
		n := uint64(uint32(r.NormFloat64() * 1e9))
		n++
		maxOutSize += n
		sizes = append(sizes, n)
	}
	pws := newTestPartWrappersForSizes(sizes)

	iterationsCount := 0
	sizeMergedTotal := uint64(0)
	for {
		pms := appendPartsToMerge(nil, pws, defaultPartsToMerge, maxOutSize)
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
				size += pw.p.size
			} else {
				pwsNew = append(pwsNew, pw)
			}
		}
		pw := &partWrapper{
			p: &part{
				size: size,
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

func testAppendPartsToMerge(t *testing.T, maxPartsToMerge int, initialSizes, expectedSizes []uint64) {
	t.Helper()

	pws := newTestPartWrappersForSizes(initialSizes)

	// Verify appending to nil.
	pms := appendPartsToMerge(nil, pws, maxPartsToMerge, 1e9)
	sizes := newTestSizesFromPartWrappers(pms)
	if !reflect.DeepEqual(sizes, expectedSizes) {
		t.Fatalf("unexpected size for maxPartsToMerge=%d, initialSizes=%d; got\n%d; want\n%d",
			maxPartsToMerge, initialSizes, sizes, expectedSizes)
	}

	// Verify appending to prefix
	prefix := []*partWrapper{
		{
			p: &part{
				size: 1234,
			},
		},
		{},
		{},
	}
	pms = appendPartsToMerge(prefix, pws, maxPartsToMerge, 1e9)
	if !reflect.DeepEqual(pms[:len(prefix)], prefix) {
		t.Fatalf("unexpected prefix for maxPartsToMerge=%d, initialSizes=%d; got\n%+v; want\n%+v",
			maxPartsToMerge, initialSizes, pms[:len(prefix)], prefix)
	}

	sizes = newTestSizesFromPartWrappers(pms[len(prefix):])
	if !reflect.DeepEqual(sizes, expectedSizes) {
		t.Fatalf("unexpected prefixed sizes for maxPartsToMerge=%d, initialSizes=%d; got\n%d; want\n%d",
			maxPartsToMerge, initialSizes, sizes, expectedSizes)
	}
}

func newTestSizesFromPartWrappers(pws []*partWrapper) []uint64 {
	var sizes []uint64
	for _, pw := range pws {
		sizes = append(sizes, pw.p.size)
	}
	return sizes
}

func newTestPartWrappersForSizes(sizes []uint64) []*partWrapper {
	var pws []*partWrapper
	for _, size := range sizes {
		pw := &partWrapper{
			p: &part{
				size: size,
			},
		}
		pws = append(pws, pw)
	}
	return pws
}

func TestMergeInMemoryPartsEmptyResult(t *testing.T) {
	defer testRemoveAll(t)

	s := newTestStorage()
	s.retentionMsecs = 1000
	defer stopTestStorage(s)

	timestamp := int64(0)
	pt := testCreatePartition(t, timestamp, s)
	defer pt.MustClose()

	var pws []*partWrapper
	const (
		inMemoryPartsCount = 5
		rowsCount          = 10
	)

	for range inMemoryPartsCount {
		rows := make([]rawRow, rowsCount)
		for i := range rowsCount {
			rows[i].TSID = TSID{
				MetricID: uint64(i),
			}
			rows[i].Value = float64(i)
			rows[i].Timestamp = timestamp + int64(i)
			rows[i].PrecisionBits = 64
		}

		mp := newTestInmemoryPart(rows)
		pw := newPartWrapperFromInmemoryPart(mp, time.Time{})
		pws = append(pws, pw)
	}

	pwsNew := pt.mustMergeInmemoryParts(pws)
	if len(pwsNew) != 0 {
		t.Fatalf("unexpected non-empty pwsNew: %d", len(pwsNew))
	}
}

func TestMergeInMemoryPartsFinal_pwsRefCount(t *testing.T) {
	defer testRemoveAll(t)

	generatePartWrappers := func(n int) []*partWrapper {
		var pws []*partWrapper
		for range n {
			var rows []rawRow
			for i := range 10 {
				row := rawRow{
					TSID:          TSID{MetricID: uint64(i)},
					Value:         float64(i),
					Timestamp:     time.Now().UnixMilli() + int64(i),
					PrecisionBits: 64,
				}
				rows = append(rows, row)
			}
			var mp inmemoryPart
			mp.InitFromRows(rows)
			pw := newPartWrapperFromInmemoryPart(&mp, time.Time{})
			pws = append(pws, pw)
		}
		return pws
	}

	assertRefCount := func(pws []*partWrapper, want int32) {
		t.Helper()
		for _, pw := range pws {
			if got := pw.refCount.Load(); got != want {
				t.Fatalf("unexpected inmemory part wrapper ref count: got %d, want %d", got, want)
			}
		}
	}

	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()
	ptw := s.tb.MustGetPartition(time.Now().UnixMilli())
	defer s.tb.PutPartition(ptw)
	pt := ptw.pt

	var (
		pwsSrc  []*partWrapper
		pwFinal *partWrapper
	)

	// single source part wrapper
	pwsSrc = generatePartWrappers(1)
	assertRefCount(pwsSrc, 1)
	pwFinal = pt.mustMergeInmemoryPartsFinal(pwsSrc)
	if pwFinal != pwsSrc[0] {
		t.Fatalf("mustMergeInmemoryPartsFinal must return the original wrapper for a single source part")
	}
	assertRefCount(pwsSrc, 1)
	assertRefCount([]*partWrapper{pwFinal}, 1)

	// many source part wrappers
	pwsSrc = generatePartWrappers(100)
	assertRefCount(pwsSrc, 1)
	pwFinal = pt.mustMergeInmemoryPartsFinal(pwsSrc)
	assertRefCount(pwsSrc, 0)
	assertRefCount([]*partWrapper{pwFinal}, 1)
}

func testCreatePartition(t *testing.T, timestamp int64, s *Storage) *partition {
	t.Helper()
	small := filepath.Join(t.Name(), smallDirname)
	big := filepath.Join(t.Name(), bigDirname)
	indexdb := filepath.Join(t.Name(), indexdbDirname)
	return mustCreatePartition(timestamp, small, big, indexdb, s)
}

func TestMustCreatePartition(t *testing.T) {
	defer testRemoveAll(t)

	ts := time.Date(2025, 3, 23, 14, 07, 56, 999_999_999, time.UTC).UnixMilli()
	smallPath := filepath.Join(t.Name(), "small")
	if fs.IsPathExist(smallPath) {
		t.Errorf("small partition directory must not exist: %s", smallPath)
	}
	bigPath := filepath.Join(t.Name(), "big")
	if fs.IsPathExist(bigPath) {
		t.Errorf("big partition directory must not exist: %s", bigPath)
	}
	indexDBPath := filepath.Join(t.Name(), "indexdb")
	if fs.IsPathExist(indexDBPath) {
		t.Errorf("indexdb parition directory must not exist: %s", indexDBPath)
	}
	s := &Storage{}

	got := mustCreatePartition(ts, smallPath, bigPath, indexDBPath, s)
	defer got.MustClose()

	wantSmallPartsPath := filepath.Join(smallPath, "2025_03")
	if got.smallPartsPath != wantSmallPartsPath {
		t.Errorf("unexpected small parts path: got %s, want %s", got.smallPartsPath, wantSmallPartsPath)
	}
	if !fs.IsPathExist(wantSmallPartsPath) {
		t.Errorf("small parts directory hasn't been created: %s", wantSmallPartsPath)
	}
	wantBigPartsPath := filepath.Join(bigPath, "2025_03")
	if got.bigPartsPath != wantBigPartsPath {
		t.Errorf("unexpected big parts path: got %s, want %s", got.bigPartsPath, wantBigPartsPath)
	}
	if !fs.IsPathExist(wantBigPartsPath) {
		t.Errorf("big parts directory hasn't been created: %s", wantBigPartsPath)
	}
	wantIndexDBPartsPath := filepath.Join(indexDBPath, "2025_03")
	if got.indexDBPartsPath != wantIndexDBPartsPath {
		t.Errorf("unexpected indexDB parts path: got %s, want %s", got.indexDBPartsPath, wantIndexDBPartsPath)
	}
	if !fs.IsPathExist(wantIndexDBPartsPath) {
		t.Errorf("indexDB parts directory hasn't been created: %s", wantIndexDBPartsPath)
	}

	wantStorage := s
	if got.s != wantStorage {
		t.Errorf("unexpected storage: got %v, want %v", got.s, wantStorage)
	}
	wantName := "2025_03"
	if got.name != wantName {
		t.Errorf("unexpected name: got %s, want %s", got.name, wantName)
	}
	wantTR := TimeRange{
		MinTimestamp: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 3, 31, 23, 59, 59, 999_000_000, time.UTC).UnixMilli(),
	}
	if got.tr != wantTR {
		t.Errorf("unexpected time range: got %v, want %v", &got.tr, &wantTR)
	}
}

func TestMustOpenPartition(t *testing.T) {
	defer testRemoveAll(t)

	smallPartsPath := filepath.Join(t.Name(), "small", "2025_03")
	bigPartsPath := filepath.Join(t.Name(), "big", "2025_03")
	indexDBPartsPath := filepath.Join(t.Name(), "indexdb", "2025_03")

	s := &Storage{}

	got := mustOpenPartition(smallPartsPath, bigPartsPath, indexDBPartsPath, s)
	defer got.MustClose()

	if got.smallPartsPath != smallPartsPath {
		t.Errorf("unexpected small parts path: got %s, want %s", got.smallPartsPath, smallPartsPath)
	}
	if !fs.IsPathExist(smallPartsPath) {
		t.Errorf("small parts directory hasn't been created: %s", smallPartsPath)
	}
	if got.bigPartsPath != bigPartsPath {
		t.Errorf("unexpected big parts path: got %s, want %s", got.bigPartsPath, bigPartsPath)
	}
	if !fs.IsPathExist(bigPartsPath) {
		t.Errorf("big parts directory hasn't been created: %s", bigPartsPath)
	}
	if got.indexDBPartsPath != indexDBPartsPath {
		t.Errorf("unexpected indexDB parts path: got %s, want %s", got.indexDBPartsPath, indexDBPartsPath)
	}
	if !fs.IsPathExist(indexDBPartsPath) {
		t.Errorf("indexDB parts directory hasn't been created: %s", indexDBPartsPath)
	}
	if got.s != s {
		t.Errorf("unexpected storage: got %v, want %v", got.s, s)
	}
	wantName := "2025_03"
	if got.name != wantName {
		t.Errorf("unexpected name: got %s, want %s", got.name, wantName)
	}
	wantTR := TimeRange{
		MinTimestamp: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 3, 31, 23, 59, 59, 999_000_000, time.UTC).UnixMilli(),
	}
	if got.tr != wantTR {
		t.Errorf("unexpected time range: got %v, want %v", &got.tr, &wantTR)
	}

}

func TestMustOpenPartition_invalidPartitionName(t *testing.T) {
	defer testRemoveAll(t)

	smallPartsPath := filepath.Join(t.Name(), "small", "2025_03_invalid")
	bigPartsPath := filepath.Join(t.Name(), "big", "2025_03_invalid")
	indexDBPartsPath := filepath.Join(t.Name(), "indexdb", "2025_03_invalid")

	defer func() {
		if err := recover(); err == nil {
			t.Fatalf("expected panic on invalid partition name in smallPartsPath but it did not happen: %q", smallPartsPath)
		}
	}()

	s := &Storage{}
	_ = mustOpenPartition(smallPartsPath, bigPartsPath, indexDBPartsPath, s)

}

func TestMustOpenPartition_smallAndBigPartsPathsAreNotTheSame(t *testing.T) {
	defer testRemoveAll(t)

	smallPartsPath := filepath.Join(t.Name(), "small", "2025_03")
	bigPartsPath := filepath.Join(t.Name(), "big", "2025_04")
	indexDBPartsPath := filepath.Join(t.Name(), "indexDB", "2025_04")

	defer func() {
		if err := recover(); err == nil {
			t.Fatalf("expected panic on different partition name in smallPartsPath=%q and bigPartsPath=%q indexDBPartsPath=%q but it did not happen", smallPartsPath, bigPartsPath, indexDBPartsPath)
		}
	}()

	s := &Storage{}
	_ = mustOpenPartition(smallPartsPath, bigPartsPath, indexDBPartsPath, s)
}

func newTestDiskSpacePartWrappers(baseTS int64, count, metricsCount, rowsPerTS int, partSize uint64, idOffset int) []*partWrapper {
	pws := make([]*partWrapper, 0, count)
	for j := range count {
		rows := make([]rawRow, 0, metricsCount*rowsPerTS)
		for m := range metricsCount {
			for r := range rowsPerTS {
				rows = append(rows, rawRow{TSID: TSID{MetricID: uint64((idOffset+j)*metricsCount + m + 1)}, Value: float64(r), Timestamp: baseTS + int64(r*1000), PrecisionBits: 64})
			}
		}
		var mp inmemoryPart
		mp.InitFromRows(rows)
		pw := newPartWrapperFromInmemoryPart(&mp, time.Time{})
		pw.p.size = partSize
		pws = append(pws, pw)
	}
	return pws
}

func newTestPartitionWithBigParts(s *Storage, ts int64, count, metricsCount, rowsPerTS int, partSize uint64, idOffset int, inMerge bool) (*partitionWrapper, []*partWrapper) {
	ptw := s.tb.MustGetPartition(ts)
	pws := newTestDiskSpacePartWrappers(ts, count, metricsCount, rowsPerTS, partSize, idOffset)
	ptw.pt.partsLock.Lock()
	for _, pw := range pws {
		pw.isInMerge = inMerge
	}
	ptw.pt.bigParts = append(ptw.pt.bigParts, pws...)
	ptw.pt.partsLock.Unlock()
	return ptw, pws
}

func testPartitionPartCounts(pt *partition) (inmem, small, big int) {
	pt.partsLock.Lock()
	defer pt.partsLock.Unlock()
	return len(pt.inmemoryParts), len(pt.smallParts), len(pt.bigParts)
}

func putTestPartitions(s *Storage, ptws []*partitionWrapper) {
	for _, ptw := range ptws {
		s.tb.PutPartition(ptw)
	}
}

func assertNoPartsInMerge(t *testing.T, pt *partition) {
	t.Helper()
	pt.partsLock.Lock()
	defer pt.partsLock.Unlock()
	for _, pws := range [][]*partWrapper{pt.inmemoryParts, pt.smallParts, pt.bigParts} {
		for _, pw := range pws {
			if pw.isInMerge {
				t.Fatalf("unexpected part %q left with isInMerge=true", pw.p.path)
			}
		}
	}
}

func mustPanic(t *testing.T, msg string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s", msg)
		}
	}()
	fn()
}

func runConcurrentlyWithPeak(t *testing.T, workers int, work func(i int) error, sample func() uint64) uint64 {
	t.Helper()
	startCh, obsStop := make(chan struct{}), make(chan struct{})
	var wg, obsWG sync.WaitGroup
	errs := make(chan error, workers)
	for i := range workers {
		wg.Go(func() { <-startCh; errs <- work(i) })
	}
	var peak atomic.Uint64
	obsWG.Go(func() {
		for {
			cur := sample()
			for prev := peak.Load(); cur > prev && !peak.CompareAndSwap(prev, cur); prev = peak.Load() {
			}
			select {
			case <-obsStop:
				return
			default:
			}
		}
	})
	close(startCh)
	wg.Wait()
	close(obsStop)
	obsWG.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent worker failed: %s", err)
		}
	}
	return peak.Load()
}

func TestDiskSpaceReservationArithmetic(t *testing.T) {
	if availableDiskSpaceForFree(100, 40) != 60 || availableDiskSpaceForFree(100, 150) != 0 ||
		!canReserveDiskSpace(100, 40, 60) || canReserveDiskSpace(100, 40, 61) ||
		canReserveDiskSpace(100, 10, math.MaxUint64) || canReserveDiskSpace(100, 120, 1) {
		t.Fatalf("unexpected disk space reservation arithmetic result")
	}
	defer testRemoveAll(t)
	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()
	if !s.tryReserveDiskSpace(s.path, 1<<20) || s.reservedDiskSpace.Load() != 1<<20 {
		t.Fatalf("expected tryReserveDiskSpace(1MiB) to succeed")
	}
	s.releaseDiskSpace(1 << 20)
	if s.reservedDiskSpace.Load() != 0 {
		t.Fatalf("unexpected reservedDiskSpace after release: %d", s.reservedDiskSpace.Load())
	}
	mustPanic(t, "expected releaseDiskSpace underflow to panic", func() { s.releaseDiskSpace(1) })
}

func TestForceMergeAllParts_ConcurrentDiskSpaceLimit(t *testing.T) {
	defer testRemoveAll(t)
	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()

	free := fs.MustGetFreeSpace(t.Name())
	partSize := free / 3
	ptws := make([]*partitionWrapper, 4)
	for i := range ptws {
		ts := time.Date(2025, time.Month(i+1), 15, 0, 0, 0, 0, time.UTC).UnixMilli()
		ptws[i], _ = newTestPartitionWithBigParts(s, ts, 2, 500, 100, partSize, i*2, false)
	}
	defer putTestPartitions(s, ptws)

	stopCh := make(chan struct{})
	defer close(stopCh)
	peak := runConcurrentlyWithPeak(t, len(ptws),
		func(i int) error { return ptws[i].pt.ForceMergeAllParts(stopCh) },
		func() uint64 {
			var active uint64
			for _, ptw := range ptws {
				if ptw.pt.activeBigMerges.Load() > 0 {
					active += 2 * partSize
				}
			}
			return active
		},
	)
	if peak > free || s.reservedDiskSpace.Load() != 0 {
		t.Fatalf("unexpected peak=%d (free=%d) or reserved=%d", peak, free, s.reservedDiskSpace.Load())
	}
	for _, ptw := range ptws {
		assertNoPartsInMerge(t, ptw.pt)
	}
}

func TestStorage_ConcurrentMergesRespectDiskSpace(t *testing.T) {
	defer testRemoveAll(t)
	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()
	free := fs.MustGetFreeSpace(t.Name())
	maxOut := getMaxOutBytes(free, 4)
	const partitions = 8
	ptws := make([]*partitionWrapper, partitions)
	selected := make([][]*partWrapper, partitions)
	for i := range partitions {
		ts := time.Date(2025, time.Month(i+1), 15, 0, 0, 0, 0, time.UTC).UnixMilli()
		ptws[i], _ = newTestPartitionWithBigParts(s, ts, 10, 50, 10, maxOut/10, i*10, false)
		ptws[i].pt.partsLock.Lock()
		selected[i] = getPartsToMerge(ptws[i].pt.bigParts, ptws[i].pt.getMaxBigPartSize())
		ptws[i].pt.partsLock.Unlock()
	}
	defer putTestPartitions(s, ptws)
	stopCh := make(chan struct{})
	defer close(stopCh)
	if peak := runConcurrentlyWithPeak(t, partitions, func(i int) error {
		return ptws[i].pt.mergeParts(selected[i], stopCh, false, false, mergeDiskReserveRequired)
	}, s.reservedDiskSpace.Load); peak > free {
		t.Fatalf("concurrent merges reserved %d bytes > free %d", peak, free)
	}
	for _, ptw := range ptws {
		assertNoPartsInMerge(t, ptw.pt)
	}

	// Deterministic TOCTOU decline and recovery via bigPartsMerger.
	ptwD, pwsD := newTestPartitionWithBigParts(s, time.Date(2025, 9, 15, 0, 0, 0, 0, time.UTC).UnixMilli(), 10, 50, 10, maxOut/10, 100, true)
	defer s.tb.PutPartition(ptwD)
	ptD := ptwD.pt
	hold := fs.MustGetFreeSpace(ptD.bigPartsPath) - maxOut/2
	if !s.tryReserveDiskSpace(ptD.bigPartsPath, hold) {
		t.Fatalf("expected tryReserveDiskSpace(%d) to succeed", hold)
	}
	if err := ptD.mergeParts(pwsD, stopCh, false, false, mergeDiskReserveRequired); err != nil {
		t.Fatalf("declined mergeParts failed: %s", err)
	}
	assertNoPartsInMerge(t, ptD)
	ptD.bigPartsMerger()
	if _, _, big := testPartitionPartCounts(ptD); big != 10 {
		t.Fatalf("expected 10 unmerged parts while space is held, got %d", big)
	}
	s.releaseDiskSpace(hold)
	ptD.bigPartsMerger()
	if _, small, big := testPartitionPartCounts(ptD); small+big != 1 || s.reservedDiskSpace.Load() != 0 {
		t.Fatalf("expected 1 merged part and 0 reserved, got parts=%d reserved=%d", small+big, s.reservedDiskSpace.Load())
	}
}

func TestForceMergeAllParts_ConcurrentWithBackgroundMerge(t *testing.T) {
	defer testRemoveAll(t)
	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()
	stopCh := make(chan struct{})
	defer close(stopCh)

	free := fs.MustGetFreeSpace(t.Name())
	ptw1, _ := newTestPartitionWithBigParts(s, time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC).UnixMilli(), 2, 50, 10, free/4, 0, false)
	defer s.tb.PutPartition(ptw1)
	pt1 := ptw1.pt
	bgReserve := fs.MustGetFreeSpace(pt1.bigPartsPath) - free/8
	if !s.tryReserveDiskSpace(pt1.bigPartsPath, bgReserve) {
		t.Fatalf("expected background reservation to succeed")
	}
	if err := pt1.ForceMergeAllParts(stopCh); err != nil {
		t.Fatalf("skipped ForceMergeAllParts returned error: %s", err)
	}
	if _, _, big := testPartitionPartCounts(pt1); big != 2 || s.reservedDiskSpace.Load() != bgReserve {
		t.Fatalf("unexpected skipped state: big=%d reserved=%d", big, s.reservedDiskSpace.Load())
	}
	assertNoPartsInMerge(t, pt1)
	s.releaseDiskSpace(bgReserve)
	if err := pt1.ForceMergeAllParts(stopCh); err != nil {
		t.Fatalf("ForceMergeAllParts after release failed: %s", err)
	}

	// Multi-chunk ForceMergeAllParts (> defaultPartsToMerge=15) releases all pre-reserved chunks.
	ptw2, _ := newTestPartitionWithBigParts(s, time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC).UnixMilli(), 20, 50, 10, free/40, 10, false)
	defer s.tb.PutPartition(ptw2)
	if err := ptw2.pt.ForceMergeAllParts(stopCh); err != nil {
		t.Fatalf("multi-chunk ForceMergeAllParts failed: %s", err)
	}
	if _, small, big := testPartitionPartCounts(ptw2.pt); small+big != 2 || s.reservedDiskSpace.Load() != 0 {
		t.Fatalf("expected 2 merged chunks and 0 reserved, got parts=%d reserved=%d", small+big, s.reservedDiskSpace.Load())
	}
	assertNoPartsInMerge(t, ptw2.pt)
}

func TestMergeParts_ReservationReleasePaths(t *testing.T) {
	defer testRemoveAll(t)
	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()
	free := fs.MustGetFreeSpace(t.Name())

	ptw1, pws := newTestPartitionWithBigParts(s, time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC).UnixMilli(), 20, 50, 10, free/40, 0, false)
	defer s.tb.PutPartition(ptw1)
	stopCh := make(chan struct{})
	close(stopCh)
	ptw1.pt.partsLock.Lock()
	pws[0].isInMerge, pws[1].isInMerge = true, true
	ptw1.pt.partsLock.Unlock()
	if err := ptw1.pt.mergeParts(pws[:2], stopCh, true, false, mergeDiskReserveRequired); !errors.Is(err, errForciblyStopped) {
		t.Fatalf("unexpected err: %v", err)
	}
	if err := ptw1.pt.ForceMergeAllParts(stopCh); err != nil {
		t.Fatalf("stopped ForceMergeAllParts failed: %s", err)
	}
	assertNoPartsInMerge(t, ptw1.pt)
	if s.reservedDiskSpace.Load() != 0 {
		t.Fatalf("unexpected reservedDiskSpace: %d", s.reservedDiskSpace.Load())
	}

	// Final flush of in-memory parts proceeds even when disk reservation fails.
	ts2 := time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC).UnixMilli()
	ptw2 := s.tb.MustGetPartition(ts2)
	defer s.tb.PutPartition(ptw2)
	pt2 := ptw2.pt
	partSize := free / 4
	hold := fs.MustGetFreeSpace(pt2.smallPartsPath) - partSize/2
	if !s.tryReserveDiskSpace(pt2.smallPartsPath, hold) {
		t.Fatalf("expected tryReserveDiskSpace to succeed")
	}
	for _, count := range []int{1, 2} {
		pt2.partsLock.Lock()
		pt2.inmemoryParts = append(pt2.inmemoryParts, newTestDiskSpacePartWrappers(ts2, count, 50, 10, partSize, count*10)...)
		pt2.partsLock.Unlock()
		pt2.flushInmemoryPartsToFiles(true)
	}
	if inmem, small, big := testPartitionPartCounts(pt2); inmem != 0 || small+big != 2 || s.reservedDiskSpace.Load() != hold {
		t.Fatalf("unexpected flush state: inmem=%d disk=%d reserved=%d", inmem, small+big, s.reservedDiskSpace.Load())
	}
	s.releaseDiskSpace(hold)

	// Reservation releases before swapSrcWithDstParts so spawned bigPartsMerger merges queued parts.
	ts3 := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC).UnixMilli()
	ptw3, pwsMerge := newTestPartitionWithBigParts(s, ts3, 2, 50, 10, free/3, 0, true)
	defer s.tb.PutPartition(ptw3)
	pt3 := ptw3.pt
	pt3.partsLock.Lock()
	pt3.bigParts = append(pt3.bigParts, newTestDiskSpacePartWrappers(ts3, 10, 50, 10, free/40, 10)...)
	pt3.partsLock.Unlock()
	stopOpen := make(chan struct{})
	defer close(stopOpen)
	if err := pt3.mergeParts(pwsMerge, stopOpen, true, false, mergeDiskReserveRequired); err != nil {
		t.Fatalf("mergeParts failed: %s", err)
	}
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		if _, _, big := testPartitionPartCounts(pt3); big <= 2 && s.reservedDiskSpace.Load() == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for spawned bigPartsMerger; reserved=%d", s.reservedDiskSpace.Load())
		}
	}
	pwsInmem := newTestDiskSpacePartWrappers(ts3, 1, 10, 5, 1024, 100)
	pt3.partsLock.Lock()
	pwsInmem[0].isInMerge = true
	pt3.inmemoryParts = append(pt3.inmemoryParts, pwsInmem...)
	pt3.partsLock.Unlock()
	mustPanic(t, "expected mergeDiskPreReserved with partInmemory to panic", func() {
		_ = pt3.mergeParts(pwsInmem, stopOpen, false, false, mergeDiskPreReserved)
	})
	assertNoPartsInMerge(t, pt3)
	// Empty retention result removes parts and releases reservation.
	sRet := MustOpenStorage(filepath.Join(t.Name(), "ret"), OpenOptions{Retention: 24 * time.Hour})
	defer sRet.MustClose()
	ptwR, pwsR := newTestPartitionWithBigParts(sRet, time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC).UnixMilli(), 2, 50, 10, free/8, 0, true)
	defer sRet.tb.PutPartition(ptwR)
	if err := ptwR.pt.mergeParts(pwsR, stopOpen, true, false, mergeDiskReserveRequired); err != nil {
		t.Fatalf("empty retention merge failed: %s", err)
	}
	if _, small, big := testPartitionPartCounts(ptwR.pt); small+big != 0 || sRet.reservedDiskSpace.Load() != 0 {
		t.Fatalf("expected 0 parts and 0 reserved, got parts=%d reserved=%d", small+big, sRet.reservedDiskSpace.Load())
	}
}
