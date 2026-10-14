package mergeset

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/blockcache"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestPartFilterSizes(t *testing.T) {
	// maxFilterableItems is the number of keys which fits into the filter of the
	// maximum size at partFilterBitsPerItem bits per key.
	maxFilterableItems := partFilterMaxBytes * 8 / partFilterBitsPerItem

	f := func(keysCount uint64) {
		t.Helper()
		segmentsCount, segmentBytes := partFilterSizes(keysCount)
		if segmentsCount < 1 || segmentsCount > partFilterMaxSegments {
			t.Fatalf("unexpected segments count for keysCount=%d; got %d", keysCount, segmentsCount)
		}
		if segmentBytes < partFilterMinSegmentBytes || segmentBytes%8 != 0 {
			t.Fatalf("unexpected segment size for keysCount=%d; got %d", keysCount, segmentBytes)
		}
		bitmapBytes := uint64(segmentsCount) * uint64(segmentBytes)
		if bitmapBytes > partFilterMaxBytes+uint64(segmentsCount)*8 {
			t.Fatalf("the filter bitmap is too big for keysCount=%d; got %d bytes; max %d bytes", keysCount, bitmapBytes, partFilterMaxBytes)
		}
		bitsPerKey := 8 * float64(bitmapBytes) / float64(keysCount)
		if keysCount <= maxFilterableItems {
			// The filter isn't limited, so every key gets partFilterBitsPerItem bits.
			if bitsPerKey < float64(partFilterBitsPerItem)*0.97 {
				t.Fatalf("too few bits per key for keysCount=%d; got %.2f bits; want about %d", keysCount, bitsPerKey, partFilterBitsPerItem)
			}
		} else {
			// The filter is limited by partFilterMaxBytes, so the number of bits per key drops.
			if bitmapBytes != partFilterMaxBytes && bitmapBytes > partFilterMaxBytes {
				t.Fatalf("the limited filter must not exceed partFilterMaxBytes for keysCount=%d; got %d bytes", keysCount, bitmapBytes)
			}
			if bitsPerKey >= float64(partFilterBitsPerItem) {
				t.Fatalf("the number of bits per key must drop for keysCount=%d; got %.2f bits", keysCount, bitsPerKey)
			}
			if segmentsCount != partFilterMaxSegments {
				t.Fatalf("the limited filter must use all the segments for keysCount=%d; got %d; want %d", keysCount, segmentsCount, partFilterMaxSegments)
			}
		}
		// Segments must stay close to the target size, so a filter cache miss reads
		// a single target-sized segment.
		if bitmapBytes >= uint64(partFilterTargetSegmentBytes) {
			if segmentBytes > partFilterTargetSegmentBytes*2 || segmentBytes < partFilterTargetSegmentBytes/2 {
				t.Fatalf("unexpected segment size for keysCount=%d; got %d; want about %d", keysCount, segmentBytes, partFilterTargetSegmentBytes)
			}
		}
	}

	for _, keysCount := range []uint64{32_768, 65_536, 262_144, 524_288, 1_048_576, 2_097_152, 4_194_304, 4_220_000, 8_000_000, 16_000_000} {
		f(keysCount)
	}

	// The maximum size filter must consist of exactly partFilterMaxSegments segments
	// of partFilterTargetSegmentBytes bytes each.
	segmentsCount, segmentBytes := partFilterSizes(maxFilterableItems)
	if segmentsCount != partFilterMaxSegments {
		t.Fatalf("unexpected segments count for the maximum size filter; got %d; want %d", segmentsCount, partFilterMaxSegments)
	}
	if segmentBytes != partFilterTargetSegmentBytes {
		t.Fatalf("unexpected segment size for the maximum size filter; got %d; want %d", segmentBytes, partFilterTargetSegmentBytes)
	}
	if uint64(segmentsCount)*uint64(segmentBytes) != partFilterMaxBytes {
		t.Fatalf("the maximum size filter bitmap must be %d bytes; got %d", partFilterMaxBytes, uint64(segmentsCount)*uint64(segmentBytes))
	}

	// A part with 4.22M series - the biggest part in the reported production
	// deployment - must get close to partFilterBitsPerItem bits per key.
	segmentsCount, segmentBytes = partFilterSizes(4_220_000)
	if segmentsCount != partFilterMaxSegments {
		t.Fatalf("unexpected segments count for 4.22M keys; got %d; want %d", segmentsCount, partFilterMaxSegments)
	}
	bitsPerKey := 8 * float64(uint64(segmentsCount)*uint64(segmentBytes)) / 4_220_000
	if bitsPerKey < 15.8 {
		t.Fatalf("too few bits per key for 4.22M keys; got %.2f; want at least 15.8", bitsPerKey)
	}
}

func TestInmemoryPartFilterSizesFromFilterableItems(t *testing.T) {
	var ib inmemoryBlock
	for i := range 100 {
		item := []byte("other-item")
		if i%10 == 0 {
			item = []byte("metric-name")
		}
		start := uint32(len(ib.data))
		ib.data = append(ib.data, item...)
		ib.items = append(ib.items, Item{Start: start, End: uint32(len(ib.data))})
	}

	filterKey := func(item []byte) []byte {
		if string(item) == "metric-name" {
			return item
		}
		return nil
	}
	var mp inmemoryPart
	mp.Init(&ib, filterKey)
	if got, want := mp.ph.filter.itemsCount, uint64(10); got != want {
		t.Fatalf("unexpected filter item count; got %d, want %d", got, want)
	}
	// The filter must be sized from the filterable items (10) instead of from all the items (100).
	got := mp.ph.filter.sizeBytes()
	want := uint64(partFilterMinSegmentBytes)
	if got >= uint64(100*partFilterBitsPerItem/8) {
		t.Fatalf("the filter was not sized from filterable items; got %d bytes", got)
	}
	if got < want {
		t.Fatalf("the filter is smaller than the minimum segment size; got %d bytes; want at least %d", got, want)
	}
	if got := mp.ph.filter.segmentsCount(); got != 1 {
		t.Fatalf("unexpected segments count; got %d; want 1", got)
	}
}

func TestPartFilterRoundtrip(t *testing.T) {
	const path = "TestPartFilterRoundtrip"
	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)
	fs.MustMkdirIfNotExist(path)

	filterKey := func(item []byte) []byte {
		n := strings.IndexByte(string(item), ':')
		if n < 0 {
			return nil
		}
		return item[:n+1]
	}

	const keysCount = 50_000
	pf := newPartFilter(keysCount)
	for i := range keysCount {
		pf.addItem(fmt.Appendf(nil, "metric-%06d:value", i), filterKey)
	}
	if got := pf.itemsCount; got != keysCount {
		t.Fatalf("unexpected items count; got %d; want %d", got, keysCount)
	}
	pf.mustWriteFile(path)

	filterFileSize := fs.MustFileSize(filepath.Join(path, filterFilename))
	if filterFileSize == 0 {
		t.Fatalf("unexpected empty filter file")
	}
	// The filter file contains the bitmap plus the header, the directory and the
	// directory checksum.
	maxFileSize := partFilterMaxBytes +
		uint64(partFilterHeaderSize+partFilterMaxSegments*partFilterSegmentEntrySize+partFilterChecksumSize)
	if filterFileSize > maxFileSize {
		t.Fatalf("the filter file is too big: %d bytes; max %d bytes", filterFileSize, maxFileSize)
	}

	pfRead, err := mustOpenPartFilter(path)
	if err != nil {
		t.Fatalf("cannot open the written filter: %s", err)
	}
	if pfRead == nil {
		t.Fatalf("cannot open the written filter")
	}
	defer pfRead.mustClose()

	if got, want := pfRead.itemsCount, uint64(keysCount); got != want {
		t.Fatalf("unexpected items count after reopen; got %d; want %d", got, want)
	}
	if got, want := pfRead.sizeBytes(), filterFileSize; got != want {
		t.Fatalf("unexpected filter size after reopen; got %d; want %d", got, want)
	}
	if got := pfRead.segmentsCount(); got < 2 {
		t.Fatalf("expected multiple segments for %d keys; got %d", keysCount, got)
	}

	// The part must be reported as possibly containing all the added keys.
	var tmpPart part
	tmpPart.ph.filter = pfRead
	for i := range keysCount {
		key := filterKey(fmt.Appendf(nil, "metric-%06d:value", i))
		h, h1, h2 := partFilterHashes(key)
		if decision := tmpPart.filterDecision(h, h1, h2); decision != partFilterPossible {
			t.Fatalf("the added key %q must be reported as possible; got decision %d", key, decision)
		}
	}

	// Absent keys must be reported as missing with a small false positive rate.
	falsePositives := 0
	const absentKeysCount = 100_000
	for i := range absentKeysCount {
		key := filterKey(fmt.Appendf(nil, "absent-%06d:value", i))
		h, h1, h2 := partFilterHashes(key)
		if decision := tmpPart.filterDecision(h, h1, h2); decision == partFilterPossible {
			falsePositives++
		}
	}
	falsePositiveRate := float64(falsePositives) / absentKeysCount
	if falsePositiveRate > 0.01 {
		t.Fatalf("too high false positive rate: %.4f; false positives %d out of %d", falsePositiveRate, falsePositives, absentKeysCount)
	}
	t.Logf("false positives: %d out of %d (%.5f)", falsePositives, absentKeysCount, falsePositiveRate)
}

func TestPartFilterInmemorySegmentsDontUseCache(t *testing.T) {
	var ib inmemoryBlock
	for i := range 100 {
		item := fmt.Appendf(nil, "metric-%d:value", i)
		start := uint32(len(ib.data))
		ib.data = append(ib.data, item...)
		ib.items = append(ib.items, Item{Start: start, End: uint32(len(ib.data))})
	}
	filterKey := func(item []byte) []byte {
		return item
	}
	var mp inmemoryPart
	mp.Init(&ib, filterKey)
	defer mp.Reset()

	requestsBefore := pfbCache.Requests()
	var tmpPart part
	tmpPart.ph.filter = mp.ph.filter
	for i := range 100 {
		key := fmt.Appendf(nil, "metric-%d:value", i)
		h, h1, h2 := partFilterHashes(key)
		if decision := tmpPart.filterDecision(h, h1, h2); decision != partFilterPossible {
			t.Fatalf("the added key %q must be reported as possible; got decision %d", key, decision)
		}
	}
	key := []byte("absent-key:value")
	h, h1, h2 := partFilterHashes(key)
	if decision := tmpPart.filterDecision(h, h1, h2); decision != partFilterNegative {
		t.Fatalf("the absent key must be reported as missing; got decision %d", decision)
	}
	if got := pfbCache.Requests() - requestsBefore; got != 0 {
		t.Fatalf("in-memory filters must not use the shared cache; got %d cache requests", got)
	}
}

func TestPartFilterInvalidFileFallsBack(t *testing.T) {
	f := func(name string, writeFile func(filterPath string)) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("TestPartFilterInvalidFileFallsBack", name)
			fs.MustRemoveDir(path)
			defer fs.MustRemoveDir(path)
			fs.MustMkdirIfNotExist(path)
			writeFile(filepath.Join(path, filterFilename))

			pf, err := mustOpenPartFilter(path)
			if pf != nil {
				pf.mustClose()
				t.Fatalf("expected the invalid filter to be ignored; got %+v", pf)
			}
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		})
	}

	f("missing", func(filterPath string) {})
	f("empty", func(filterPath string) {
		if err := os.WriteFile(filterPath, nil, 0o600); err != nil {
			t.Fatalf("cannot write the filter: %s", err)
		}
	})
	f("garbage", func(filterPath string) {
		if err := os.WriteFile(filterPath, make([]byte, 1000), 0o600); err != nil {
			t.Fatalf("cannot write the filter: %s", err)
		}
	})
	f("oversized", func(filterPath string) {
		if err := os.WriteFile(filterPath, make([]byte, partFilterMaxBytes*10), 0o600); err != nil {
			t.Fatalf("cannot write the filter: %s", err)
		}
	})
	f("truncated", func(filterPath string) {
		pf := newPartFilter(100_000)
		for i := range 100_000 {
			pf.addKey(fmt.Appendf(nil, "metric-%d", i))
		}
		pf.mustWriteFile(filepath.Dir(filterPath))
		// Truncate the file, so the segment offsets become out of the file range.
		fi := fs.MustFileSize(filterPath)
		if err := os.Truncate(filterPath, int64(fi/2)); err != nil {
			t.Fatalf("cannot truncate the filter: %s", err)
		}
	})
	f("corrupted directory", func(filterPath string) {
		pf := newPartFilter(1000)
		for i := range 1000 {
			pf.addKey(fmt.Appendf(nil, "metric-%d", i))
		}
		pf.mustWriteFile(filepath.Dir(filterPath))
		data, err := os.ReadFile(filterPath)
		if err != nil {
			t.Fatalf("cannot read the filter: %s", err)
		}
		data[partFilterHeaderSize+8] ^= 1
		if err := os.WriteFile(filterPath, data, 0o600); err != nil {
			t.Fatalf("cannot write the filter: %s", err)
		}
	})
	f("invalid segments count", func(filterPath string) {
		pf := newPartFilter(1000)
		pf.addKey([]byte("metric-1"))
		pf.mustWriteFile(filepath.Dir(filterPath))
		data, err := os.ReadFile(filterPath)
		if err != nil {
			t.Fatalf("cannot read the filter: %s", err)
		}
		binary.LittleEndian.PutUint32(data[len(partFilterMagic):], uint32(partFilterMaxSegments+1))
		if err := os.WriteFile(filterPath, data, 0o600); err != nil {
			t.Fatalf("cannot write the filter: %s", err)
		}
	})
}

func TestPartFilterCorruptedSegmentDisablesFilter(t *testing.T) {
	const path = "TestPartFilterCorruptedSegmentDisablesFilter"
	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)
	fs.MustMkdirIfNotExist(path)

	pf := newPartFilter(200_000)
	for i := range 200_000 {
		pf.addKey(fmt.Appendf(nil, "metric-%08d:value", i))
	}
	pf.mustWriteFile(path)

	pfRead, err := mustOpenPartFilter(path)
	if err != nil || pfRead == nil {
		t.Fatalf("cannot open the written filter: pf=%v, err=%s", pfRead, err)
	}
	defer pfRead.mustClose()

	// Corrupt the first segment, so its checksum stops matching.
	filterPath := filepath.Join(path, filterFilename)
	f, err := os.OpenFile(filterPath, os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("cannot open the filter for writing: %s", err)
	}
	si := pfRead.segments[0]
	if _, err := f.WriteAt([]byte{0xFF}, int64(si.offset)+int64(si.sizeBytes)-1); err != nil {
		t.Fatalf("cannot corrupt the filter segment: %s", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("cannot close the filter: %s", err)
	}

	// Find a key which maps to the corrupted segment.
	var tmpPart part
	tmpPart.ph.filter = pfRead
	var found bool
	for i := range 200_000 {
		key := fmt.Appendf(nil, "metric-%08d:value", i)
		h, h1, h2 := partFilterHashes(key)
		if h%pfRead.segmentsCount() != 0 {
			continue
		}
		found = true
		if decision := tmpPart.filterDecision(h, h1, h2); decision != partFilterFallbackVerify {
			t.Fatalf("the corrupted segment must lead to the fallback; got decision %d", decision)
		}
		break
	}
	if !found {
		t.Fatalf("cannot find a key for the corrupted segment")
	}

	// The filter must be disabled after the failed checksum, so it isn't re-read on
	// every lookup. The reported reason is preserved.
	otherKey := []byte("metric-00000000:value")
	if decision := tmpPart.filterDecision(partFilterHashes(otherKey)); decision != partFilterFallbackVerify {
		t.Fatalf("the filter must be disabled with the preserved reason after the checksum mismatch; got decision %d", decision)
	}
	// The disabled filter must not read segments from disk anymore.
	requestsBefore := pfbCache.Requests()
	h, h1, h2 := partFilterHashes(otherKey)
	if decision := tmpPart.filterDecision(h, h1, h2); decision != partFilterFallbackVerify {
		t.Fatalf("unexpected decision for the disabled filter; got %d", decision)
	}
	if got := pfbCache.Requests() - requestsBefore; got != 0 {
		t.Fatalf("the disabled filter must not use the cache; got %d requests", got)
	}
}

func TestPartFilterBudgetFallback(t *testing.T) {
	const path = "TestPartFilterBudgetFallback"
	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)
	fs.MustMkdirIfNotExist(path)

	pf := newPartFilter(100_000)
	for i := range 100_000 {
		pf.addKey(fmt.Appendf(nil, "metric-%08d:value", i))
	}
	pf.mustWriteFile(path)

	pfRead, err := mustOpenPartFilter(path)
	if err != nil || pfRead == nil {
		t.Fatalf("cannot open the written filter: pf=%v, err=%s", pfRead, err)
	}
	defer pfRead.mustClose()

	// Disable the cache budget, so the segments cannot be cached.
	cacheSizeBefore := getMaxPartFilterCacheSize()
	SetPartFilterCacheSize(1)
	defer SetPartFilterCacheSize(cacheSizeBefore)

	var tmpPart part
	tmpPart.ph.filter = pfRead
	key := []byte("metric-00000001:value")
	h, h1, h2 := partFilterHashes(key)
	if decision := tmpPart.filterDecision(h, h1, h2); decision != partFilterFallbackBudget {
		t.Fatalf("the filter must fall back when the cache budget is insufficient; got decision %d", decision)
	}

	// In-memory filters must keep working, since they don't need the cache.
	SetPartFilterCacheSize(1)
	var ib inmemoryBlock
	item := []byte("metric-00000001:value")
	ib.data = append(ib.data, item...)
	ib.items = append(ib.items, Item{Start: 0, End: uint32(len(ib.data))})
	var mp inmemoryPart
	mp.Init(&ib, func(item []byte) []byte { return item })
	defer mp.Reset()
	tmpPart.ph.filter = mp.ph.filter
	if decision := tmpPart.filterDecision(h, h1, h2); decision != partFilterPossible {
		t.Fatalf("in-memory filters must work with the disabled cache; got decision %d", decision)
	}
}

func TestPartFilterSegmentChecksum(t *testing.T) {
	const path = "TestPartFilterSegmentChecksum"
	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)
	fs.MustMkdirIfNotExist(path)

	pf := newPartFilter(10_000)
	for i := range 10_000 {
		pf.addKey(fmt.Appendf(nil, "metric-%d", i))
	}
	pf.mustWriteFile(path)

	data, err := os.ReadFile(filepath.Join(path, filterFilename))
	if err != nil {
		t.Fatalf("cannot read the filter: %s", err)
	}
	segmentsCount := int(pf.segmentsCount())
	dirEnd := partFilterHeaderSize + segmentsCount*partFilterSegmentEntrySize
	if got := binary.LittleEndian.Uint64(data[dirEnd:]); got != xxhash.Sum64(data[:dirEnd]) {
		t.Fatalf("unexpected directory checksum; got %x; want %x", got, xxhash.Sum64(data[:dirEnd]))
	}
	for i := range segmentsCount {
		be := data[partFilterHeaderSize+i*partFilterSegmentEntrySize:]
		offset := binary.LittleEndian.Uint64(be)
		sizeBytes := binary.LittleEndian.Uint32(be[8:])
		checksum := binary.LittleEndian.Uint64(be[16:])
		if got := xxhash.Sum64(data[offset : offset+uint64(sizeBytes)]); got != checksum {
			t.Fatalf("unexpected checksum for segment %d; got %x; want %x", i, got, checksum)
		}
	}
}

func TestTableMetricsCountPartFilters(t *testing.T) {
	const path = "TestTableMetricsCountPartFilters"
	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)

	filterKey := func(item []byte) []byte { return item }
	var isReadOnly atomic.Bool
	tb := MustOpenTable(path, 0, nil, 0, nil, &isReadOnly, filterKey)
	for i := range 1000 {
		item := fmt.Appendf(nil, "metric-%06d:value", i)
		tb.AddItems([][]byte{item})
	}
	tb.DebugFlush()
	tb.MustClose()

	tb = MustOpenTable(path, 0, nil, 0, nil, &isReadOnly, filterKey)
	defer tb.MustClose()
	var ts TableSearch
	ts.Init(tb, false)
	ts.SeekWithPartFilter([]byte("metric-000001:"))
	ts.MustClose()

	var tm TableMetrics
	tb.UpdateMetrics(&tm)
	if tm.FilePartsCount == 0 {
		t.Fatalf("expected file parts")
	}
	if tm.PartFilterFileSizeBytes == 0 {
		t.Fatalf("PartFilterFileSizeBytes must be non-zero for the persisted part filter")
	}
	if tm.PartFilterSegmentsCount == 0 {
		t.Fatalf("PartFilterSegmentsCount must be non-zero for the persisted part filter")
	}
	if tm.PartFilterMemorySizeBytes != 0 {
		t.Fatalf("unexpected in-memory part filter size for the file part: %d", tm.PartFilterMemorySizeBytes)
	}
	// The cache budget is split between cache shards, so the reported budget may
	// be slightly lower than the configured one.
	cacheSizeMaxBytes := getMaxPartFilterCacheSize()
	if tm.PartFilterCacheSizeMaxBytes > uint64(cacheSizeMaxBytes) || tm.PartFilterCacheSizeMaxBytes < uint64(cacheSizeMaxBytes)*9/10 {
		t.Fatalf("unexpected part filter cache budget; got %d; want about %d", tm.PartFilterCacheSizeMaxBytes, cacheSizeMaxBytes)
	}
	if tm.PartFilterCacheSizeBytes > tm.PartFilterCacheSizeMaxBytes {
		t.Fatalf("the part filter cache size %d exceeds its budget %d", tm.PartFilterCacheSizeBytes, tm.PartFilterCacheSizeMaxBytes)
	}
	if tm.PartFilterCacheRequests == 0 {
		t.Fatalf("expected non-zero part filter cache requests")
	}
}

func TestPartFilterFalsePositiveRate(t *testing.T) {
	// The upper bound for the false positive rate of a Bloom filter with k probes
	// and m/n bits per key.
	falsePositiveRateBound := func(keysCount uint64) float64 {
		bitsPerKey := min(float64(partFilterBitsPerItem), 8*float64(partFilterMaxBytes)/float64(keysCount))
		fill := 1 - math.Exp(-float64(partFilterProbes)/bitsPerKey)
		return math.Pow(fill, partFilterProbes)
	}

	// Cover both the number of segments which is a power of two - it may confine
	// the probes of all the keys from the same segment to a subset of the segment
	// bits - and the number of segments which is not a power of two.
	for _, keysCount := range []uint64{
		32_768,     // 1 segment
		65_536,     // 2 segments
		262_144,    // 8 segments
		524_288,    // 16 segments
		1_048_576,  // 32 segments
		2_097_152,  // 64 segments - the maximum for the previous filter size limit
		4_194_304,  // 128 segments at partFilterBitsPerItem bits per key
		4_220_000,  // 128 segments, the biggest part in the reported production deployment
		8_000_000,  // limited by partFilterMaxBytes
		16_000_000, // limited by partFilterMaxBytes
	} {
		pf := newPartFilter(keysCount)
		for i := uint64(0); i < keysCount; i++ {
			pf.addKey(fmt.Appendf(nil, "metric-%012d:value", i))
		}
		t.Run(fmt.Sprintf("keys=%d_segments=%d", keysCount, pf.segmentsCount()), func(t *testing.T) {
			bitmapBytes := pf.sizeBytes()
			bitsPerKey := 8 * float64(bitmapBytes) / float64(keysCount)
			if bitsPerKey > float64(partFilterBitsPerItem) {
				t.Fatalf("too many bits per key: %.2f; want at most %d", bitsPerKey, partFilterBitsPerItem)
			}
			t.Logf("bitmapBytes=%d bits per key=%.2f segments=%d", bitmapBytes, bitsPerKey, pf.segmentsCount())
			checkPartFilterFalsePositiveRate(t, pf, keysCount, falsePositiveRateBound(keysCount))
		})
	}

	// The production part with 4.22M series must get close to the full precision,
	// since it is the reason for the partFilterMaxBytes bump.
	pf := newPartFilter(4_220_000)
	for i := uint64(0); i < 4_220_000; i++ {
		pf.addKey(fmt.Appendf(nil, "metric-%012d:value", i))
	}
	bitsPerKey := 8 * float64(pf.sizeBytes()) / 4_220_000
	if bitsPerKey < 15.8 {
		t.Fatalf("too few bits per key for 4.22M keys; got %.2f; want at least 15.8", bitsPerKey)
	}
	if fp := falsePositiveRateBound(4_220_000); fp > 0.003 {
		t.Fatalf("too high theoretical false positive rate for 4.22M keys: %.5f; want at most 0.003", fp)
	}
}

func checkPartFilterFalsePositiveRate(t *testing.T, pf *partFilter, keysCount uint64, threshold float64) {
	t.Helper()

	// The present keys must never be reported as missing.
	var tmpPart part
	tmpPart.ph.filter = pf
	for i := uint64(0); i < keysCount; i += max(keysCount/1000, 1) {
		key := fmt.Appendf(nil, "metric-%012d:value", i)
		h, h1, h2 := partFilterHashes(key)
		if decision := tmpPart.filterDecision(h, h1, h2); decision != partFilterPossible {
			t.Fatalf("the present key %q must be reported as possible; got decision %d", key, decision)
		}
	}

	const absentKeysCount = 200_000
	falsePositives := 0
	for i := uint64(0); i < absentKeysCount; i++ {
		key := fmt.Appendf(nil, "absent-%012d:value", i)
		h, h1, h2 := partFilterHashes(key)
		if decision := tmpPart.filterDecision(h, h1, h2); decision == partFilterPossible {
			falsePositives++
		}
	}
	falsePositiveRate := float64(falsePositives) / absentKeysCount

	// Allow 25% slack for the sampling error, while catching the confinement of
	// the probes, which raises the rate by 2x and more.
	if falsePositiveRate > threshold*1.25 {
		t.Fatalf("too high false positive rate: %.5f; expected at most %.5f (%.5f + 25%%)", falsePositiveRate, threshold*1.25, threshold)
	}
	t.Logf("segments=%d bits per key=%.2f false positive rate=%.5f (bound %.5f)", pf.segmentsCount(),
		8*float64(pf.sizeBytes())/float64(keysCount), falsePositiveRate, threshold)
}

// TestPartFilterSegmentLimits verifies that the reader accepts the filters written
// with the previous segments count limit, accepts the filters written with the
// current limit, and rejects the filters which claim a too big segments count.
func TestPartFilterSegmentLimits(t *testing.T) {
	const path = "TestPartFilterSegmentLimits"
	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)
	fs.MustMkdirIfNotExist(path)

	putSegmentsCount := func(n uint32) func([]byte) []byte {
		return func(data []byte) []byte {
			binary.LittleEndian.PutUint32(data[len(partFilterMagic):], n)
			return data
		}
	}

	f := func(name string, keysCount uint64, wantSegmentsCount int, patch func([]byte) []byte, wantIgnored bool) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			partPath := filepath.Join(path, name)
			fs.MustMkdirIfNotExist(partPath)

			pf := newPartFilter(keysCount)
			for i := uint64(0); i < keysCount; i++ {
				pf.addKey(fmt.Appendf(nil, "metric-%012d:value", i))
			}
			if got := pf.segmentsCount(); got != uint64(wantSegmentsCount) {
				t.Fatalf("unexpected segments count; got %d; want %d", got, wantSegmentsCount)
			}
			pf.mustWriteFile(partPath)

			filterPath := filepath.Join(partPath, filterFilename)
			if patch != nil {
				data, err := os.ReadFile(filterPath)
				if err != nil {
					t.Fatalf("cannot read the filter: %s", err)
				}
				if err := os.WriteFile(filterPath, patch(data), 0o600); err != nil {
					t.Fatalf("cannot patch the filter: %s", err)
				}
			}

			pfRead, err := mustOpenPartFilter(partPath)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if wantIgnored {
				if pfRead != nil {
					pfRead.mustClose()
					t.Fatalf("expected the filter to be ignored")
				}
				return
			}
			if pfRead == nil {
				t.Fatalf("cannot open the written filter")
			}
			defer pfRead.mustClose()
			if got := pfRead.segmentsCount(); got != uint64(wantSegmentsCount) {
				t.Fatalf("unexpected segments count after reopen; got %d; want %d", got, wantSegmentsCount)
			}

			// All the added keys must be reported as possible.
			var tmpPart part
			tmpPart.ph.filter = pfRead
			for i := uint64(0); i < keysCount; i += max(keysCount/1000, 1) {
				key := fmt.Appendf(nil, "metric-%012d:value", i)
				h, h1, h2 := partFilterHashes(key)
				if decision := tmpPart.filterDecision(h, h1, h2); decision != partFilterPossible {
					t.Fatalf("the present key %q must be reported as possible; got decision %d", key, decision)
				}
			}
		})
	}

	// The filter of the previous maximum size (64 segments, 4 MiB) must stay readable.
	f("previous_max_size", 2_097_152, 64, nil, false)

	// The filter of the current maximum size (128 segments, 8 MiB) must be readable.
	f("current_max_size", 4_220_000, 128, nil, false)

	f("too_many_segments", 4_220_000, 128, putSegmentsCount(uint32(partFilterMaxSegments+1)), true)
	f("huge_segments_count", 4_220_000, 128, putSegmentsCount(1<<20), true)
	f("zero_segments", 4_220_000, 128, putSegmentsCount(0), true)

	// A directory which lies about the segments count must be rejected by the
	// directory checksum.
	f("stale_segments_count", 4_220_000, 128, putSegmentsCount(uint32(partFilterMaxSegments/2)), true)
}

// TestPartFilterShardBudgetFallback verifies that the filter falls back to the index
// lookup when the cache budget allows a segment to be read, but a single cache shard
// cannot hold it, since such a segment would be evicted right after being stored.
func TestPartFilterShardBudgetFallback(t *testing.T) {
	const path = "TestPartFilterShardBudgetFallback"
	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)
	fs.MustMkdirIfNotExist(path)

	pf := newPartFilter(100_000)
	for i := range 100_000 {
		pf.addKey(fmt.Appendf(nil, "metric-%08d:value", i))
	}
	pf.mustWriteFile(path)

	pfRead, err := mustOpenPartFilter(path)
	if err != nil || pfRead == nil {
		t.Fatalf("cannot open the written filter: pf=%v, err=%s", pfRead, err)
	}
	defer pfRead.mustClose()

	// Use a cache with a deterministic shards count, since the shard budget is what
	// the filter checks before reading a segment.
	const shardsCount = 4
	cacheBefore := pfbCache
	cacheTmp := blockcache.NewCacheWithShardsCount(getMaxPartFilterCacheSize, shardsCount)
	pfbCache = cacheTmp
	defer func() {
		pfbCache = cacheBefore
		cacheTmp.MustStop()
	}()

	var tmpPart part
	tmpPart.ph.filter = pfRead
	presentKey := []byte("metric-00000001:value")
	hPresent, h1Present, h2Present := partFilterHashes(presentKey)
	absentKey := []byte("absent-00000001:value")
	hAbsent, h1Absent, h2Absent := partFilterHashes(absentKey)

	defer SetPartFilterCacheSize(0)

	// The total budget fits two segments, but a single shard fits only half a segment.
	// The filter must fall back instead of reading the segment from disk on every lookup.
	SetPartFilterCacheSize(2 * partFilterTargetSegmentBytes)
	if pfbCache.CanStoreBlock(partFilterTargetSegmentBytes) {
		t.Fatalf("the segment must not fit a single shard with the budget of %d bytes", 2*partFilterTargetSegmentBytes)
	}
	cacheRequestsBefore := pfbCache.Requests()
	for _, key := range []struct {
		h, h1, h2 uint64
	}{
		{hPresent, h1Present, h2Present},
		{hAbsent, h1Absent, h2Absent},
	} {
		if decision := tmpPart.filterDecision(key.h, key.h1, key.h2); decision != partFilterFallbackBudget {
			t.Fatalf("the filter must report the budget fallback when a single cache shard cannot hold a segment; got decision %d", decision)
		}
	}
	if got := pfbCache.Requests() - cacheRequestsBefore; got != 0 {
		t.Fatalf("the filter must not read segments which cannot be cached; got %d cache requests", got)
	}

	// The filter must be used again as soon as a single shard can hold a segment, so the
	// usability check must not cache its previous result.
	SetPartFilterCacheSize(shardsCount * partFilterTargetSegmentBytes)
	if !pfbCache.CanStoreBlock(partFilterTargetSegmentBytes) {
		t.Fatalf("the segment must fit a single shard with the budget of %d bytes", shardsCount*partFilterTargetSegmentBytes)
	}
	if decision := tmpPart.filterDecision(hPresent, h1Present, h2Present); decision != partFilterPossible {
		t.Fatalf("the present key must be reported as possible after the budget is increased; got decision %d", decision)
	}
	if decision := tmpPart.filterDecision(hAbsent, h1Absent, h2Absent); decision != partFilterNegative {
		t.Fatalf("the absent key must be reported as missing after the budget is increased; got decision %d", decision)
	}

	// Falling back again after the budget is reduced must work too.
	SetPartFilterCacheSize(partFilterTargetSegmentBytes)
	if decision := tmpPart.filterDecision(hPresent, h1Present, h2Present); decision != partFilterFallbackBudget {
		t.Fatalf("the filter must report the budget fallback after the budget is reduced again; got decision %d", decision)
	}
}

// TestSetPartFilterSizeLimitRejectsOversizedSegments verifies that limits which would
// make a filter segment bigger than partFilterTargetSegmentBytes are rejected.
//
// isPartFilterCacheUsable checks the cache budget against partFilterTargetSegmentBytes,
// so a bigger segment passes that check even though a cache shard cannot hold it, and
// every lookup would read it from disk again without reporting any fallback.
func TestSetPartFilterSizeLimitRejectsOversizedSegments(t *testing.T) {
	defaultMaxBytes := partFilterMaxBytes
	defaultMaxSegments := partFilterMaxSegments

	f := func(maxBytes uint64, maxSegments int, wantPanic bool) {
		t.Helper()
		panicked := false
		func() {
			defer func() {
				panicked = recover() != nil
			}()
			restore := SetPartFilterSizeLimit(maxBytes, maxSegments)
			defer restore()

			segmentsCount, segmentBytes := partFilterSizes(maxBytes * 8 / partFilterBitsPerItem)
			if segmentsCount < 1 {
				t.Fatalf("unexpected segments count for maxBytes=%d, maxSegments=%d; got %d; want at least one segment", maxBytes, maxSegments, segmentsCount)
			}
			if segmentBytes > partFilterTargetSegmentBytes {
				t.Fatalf("the segment size must not exceed partFilterTargetSegmentBytes for maxBytes=%d, maxSegments=%d; got %d; want at most %d",
					maxBytes, maxSegments, segmentBytes, partFilterTargetSegmentBytes)
			}
		}()
		if panicked != wantPanic {
			t.Fatalf("unexpected panic for maxBytes=%d, maxSegments=%d; got %v; want %v", maxBytes, maxSegments, panicked, wantPanic)
		}
		if partFilterMaxBytes != defaultMaxBytes || partFilterMaxSegments != defaultMaxSegments {
			t.Fatalf("the limits must stay unchanged on panic; got maxBytes=%d, maxSegments=%d; want maxBytes=%d, maxSegments=%d",
				partFilterMaxBytes, partFilterMaxSegments, defaultMaxBytes, defaultMaxSegments)
		}
	}

	// The segment size stays at the target size.
	f(4<<20, 64, false)
	f(8<<20, 128, false)
	f(16<<20, 256, false)
	// Smaller segments are fine.
	f(8<<20, 256, false)
	f(partFilterTargetSegmentBytes, 1, false)
	// Segments bigger than the target size are rejected.
	f(12<<20, 128, true)
	f(16<<20, 128, true)
	f(8<<20, 64, true)
	// A filter must have at least one segment.
	f(partFilterTargetSegmentBytes, 0, true)
}
