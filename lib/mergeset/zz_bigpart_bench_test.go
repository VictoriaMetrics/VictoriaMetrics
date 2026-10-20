package mergeset

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

// The benchmarks in this file cover the sizes of the parts which are reported in
// production: index parts with millions of series, where the filter size limit
// decides the number of bits per key, and the segment size decides the amount of
// data read on a filter cache miss.

// BenchmarkPartFilterSegmentReadCost measures the cost of the segment read and the
// segment checksum, which is paid on a part filter cache miss.
func BenchmarkPartFilterSegmentReadCost(b *testing.B) {
	const path = "BenchmarkPartFilterSegmentReadCost"
	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)
	fs.MustMkdirIfNotExist(path)

	// Build a filter of the maximum size, so segments have the target size.
	pf := newPartFilter(partFilterMaxBytes * 8 / partFilterBitsPerItem)
	for i := uint64(0); i < 4_220_000; i++ {
		pf.addKey(fmt.Appendf(nil, "metric-%012d:value", i))
	}
	pf.mustWriteFile(path)
	pfRead, err := mustOpenPartFilter(path)
	if err != nil || pfRead == nil {
		b.Fatalf("cannot open the filter: pf=%v, err=%s", pfRead, err)
	}
	defer pfRead.mustClose()
	b.Logf("segments=%d segmentBytes=%d fileBytes=%d", pfRead.segmentsCount(), pfRead.segments[0].sizeBytes, pfRead.sizeBytes())

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		si := &pfRead.segments[i%len(pfRead.segments)]
		segment, err := pfRead.readSegment(si)
		if err != nil {
			b.Fatalf("cannot read the segment: %s", err)
		}
		if xxhash.Sum64(segment) != si.checksum {
			b.Fatalf("unexpected checksum")
		}
	}
}

// BenchmarkPartFilterSegmentCacheHitCost measures the cost of the filter decision when
// all the needed segments are in the shared cache.
func BenchmarkPartFilterSegmentCacheHitCost(b *testing.B) {
	const path = "BenchmarkPartFilterSegmentCacheHitCost"
	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)
	fs.MustMkdirIfNotExist(path)

	pf := newPartFilter(partFilterMaxBytes * 8 / partFilterBitsPerItem)
	for i := uint64(0); i < 4_220_000; i++ {
		pf.addKey(fmt.Appendf(nil, "metric-%012d:value", i))
	}
	pf.mustWriteFile(path)
	pfRead, err := mustOpenPartFilter(path)
	if err != nil || pfRead == nil {
		b.Fatalf("cannot open the filter: pf=%v, err=%s", pfRead, err)
	}
	defer pfRead.mustClose()

	var tmpPart part
	tmpPart.ph.filter = pfRead
	// Drop the temporary part's segments from the shared cache, so the following
	// benchmarks are not measured against a budget occupied by this one.
	defer pfbCache.RemoveBlocksForPart(&tmpPart)
	keys := make([][]byte, pfRead.segmentsCount())
	for i := range keys {
		keys[i] = fmt.Appendf(nil, "absent-%012d:value", i)
	}
	// Warm up the cache.
	for i := range keys {
		h, h1, h2 := partFilterHashes(keys[i])
		tmpPart.filterDecision(h, h1, h2)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[i%len(keys)]
		h, h1, h2 := partFilterHashes(key)
		tmpPart.filterDecision(h, h1, h2)
	}
}

// BenchmarkPartFilterSizeLimit compares the filter size limits for parts with millions
// of series. It reports the false positive rate, the filter cache activity and the
// filter size on disk for every limit.
func BenchmarkPartFilterSizeLimit(b *testing.B) {
	for _, keysCount := range []uint64{4_220_000, 8_000_000} {
		b.Run(fmt.Sprintf("keys=%d", keysCount), func(b *testing.B) {
			for _, tc := range []struct {
				name        string
				maxBytes    uint64
				maxSegments int
			}{
				{name: "limit=4MiB", maxBytes: 4 << 20, maxSegments: 64},
				{name: "limit=8MiB", maxBytes: 8 << 20, maxSegments: 128},
				{name: "limit=16MiB", maxBytes: 16 << 20, maxSegments: 256},
			} {
				b.Run(tc.name, func(b *testing.B) {
					restore := SetPartFilterSizeLimit(tc.maxBytes, tc.maxSegments)
					defer restore()
					benchmarkPartFilterSizeLimit(b, keysCount)
				})
			}
		})
	}
}

func benchmarkPartFilterSizeLimit(b *testing.B, keysCount uint64) {
	path, err := filepath.Abs(b.Name())
	if err != nil {
		b.Fatalf("cannot obtain the absolute path: %s", err)
	}
	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)
	fs.MustMkdirIfNotExist(path)

	pf := newPartFilter(keysCount)
	for i := uint64(0); i < keysCount; i++ {
		pf.addKey(fmt.Appendf(nil, "metric-%012d:value", i))
	}
	pf.mustWriteFile(path)
	pfRead, err := mustOpenPartFilter(path)
	if err != nil || pfRead == nil {
		b.Fatalf("cannot open the filter: pf=%v, err=%s", pfRead, err)
	}
	defer pfRead.mustClose()

	var tmpPart part
	tmpPart.ph.filter = pfRead
	// Use many distinct absent keys, so the reported false positive rate is close
	// to the real one.
	keys := make([][]byte, 64*1024)
	for i := range keys {
		keys[i] = fmt.Appendf(nil, "absent-%012d:value", i)
	}
	b.ReportAllocs()
	b.ReportMetric(float64(pfRead.sizeBytes()), "filter-bytes")
	b.ReportMetric(float64(pfRead.segmentsCount()), "filter-segments")
	b.ReportMetric(8*float64(pfRead.sizeBytes())/float64(keysCount), "filter-bits-per-key")

	cacheRequestsBefore := pfbCache.Requests()
	cacheMissesBefore := pfbCache.Misses()

	possible := 0
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[i%len(keys)]
		h, h1, h2 := partFilterHashes(key)
		if tmpPart.filterDecision(h, h1, h2) == partFilterPossible {
			possible++
		}
	}
	b.StopTimer()
	if b.N > 0 {
		b.ReportMetric(float64(possible)/float64(b.N), "filter-possible/op")
	}
	b.ReportMetric(float64(pfbCache.Requests()-cacheRequestsBefore)/float64(b.N), "filter-cache-requests/op")
	b.ReportMetric(float64(pfbCache.Misses()-cacheMissesBefore)/float64(b.N), "filter-cache-misses/op")
}

// TestPartFilterSizeLimitKeepsSegmentSize verifies that the segment size stays at the
// target size for every supported filter size limit, since a bigger segment increases
// the amount of data read on a filter cache miss.
func TestPartFilterSizeLimitKeepsSegmentSize(t *testing.T) {
	for _, tc := range []struct {
		maxBytes    uint64
		maxSegments int
	}{
		{maxBytes: 4 << 20, maxSegments: 64},
		{maxBytes: 8 << 20, maxSegments: 128},
		{maxBytes: 16 << 20, maxSegments: 256},
	} {
		restore := SetPartFilterSizeLimit(tc.maxBytes, tc.maxSegments)
		defer restore()

		segmentsCount, segmentBytes := partFilterSizes(tc.maxBytes * 8 / partFilterBitsPerItem)
		if segmentsCount != tc.maxSegments {
			t.Fatalf("unexpected segments count for maxBytes=%d; got %d; want %d", tc.maxBytes, segmentsCount, tc.maxSegments)
		}
		if segmentBytes != partFilterTargetSegmentBytes {
			t.Fatalf("unexpected segment size for maxBytes=%d; got %d; want %d", tc.maxBytes, segmentBytes, partFilterTargetSegmentBytes)
		}
		if uint64(segmentsCount)*uint64(segmentBytes) != tc.maxBytes {
			t.Fatalf("the maximum size filter bitmap must be %d bytes; got %d", tc.maxBytes, uint64(segmentsCount)*uint64(segmentBytes))
		}
	}
}
