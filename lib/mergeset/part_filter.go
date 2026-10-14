package mergeset

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/blockcache"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
)

// The part filter file has the following layout:
//
//	magic (6 bytes) | segmentsCount (uint32) | itemsCount (uint64) | directory | directory checksum (uint64) | segments
//
// The layout is identified by partFilterMagic and isn't versioned. The reader
// accepts any segmentsCount up to partFilterMaxSegments, so filter files written
// with a smaller limit - for example, with 64 segments - stay readable.
const (
	// partFilterMagic is the signature of the part filter file.
	partFilterMagic = "VMBFS1"

	// partFilterHeaderSize is the size of the filter file header.
	partFilterHeaderSize = len(partFilterMagic) + 4 + 8

	// partFilterSegmentEntrySize is the size of a single segment entry in the filter file directory.
	partFilterSegmentEntrySize = 8 + 4 + 4 + 8

	// partFilterChecksumSize is the size of the filter file directory checksum.
	//
	// The checksum is stored after the directory, so it covers the directory and the header.
	partFilterChecksumSize = 8

	// partFilterBitsPerItem is the number of bits allocated per filter key.
	partFilterBitsPerItem = 16

	// partFilterProbes is the number of bits checked per key.
	partFilterProbes = 4

	// partFilterTargetSegmentBytes is the target size of a single filter segment.
	//
	// Segments are loaded from disk on demand and cached in pfbCache, so smaller
	// segments reduce the amount of data read and cached per lookup.
	partFilterTargetSegmentBytes = 64 << 10

	// partFilterMinSegmentBytes is the minimum size of a single filter segment.
	partFilterMinSegmentBytes = 64
)

var (
	// partFilterMaxSegments is the maximum number of segments in a single part filter.
	//
	// The value also limits the segments count accepted from the filter file, so
	// files written with a smaller limit stay readable.
	partFilterMaxSegments int = 128

	// partFilterMaxBytes is the maximum bitmap size of a single part filter.
	//
	// It is derived from partFilterMaxSegments instead of being maintained separately,
	// so the two cannot drift apart: a filter of the maximum size always consists of
	// segments no bigger than partFilterTargetSegmentBytes. A bigger segment would not
	// fit into a single cache shard, so it would be read from disk on every lookup.
	//
	// It is a variable instead of a constant only to let tests and benchmarks
	// compare different limits. Production code must not change it.
	partFilterMaxBytes uint64 = uint64(partFilterMaxSegments) * partFilterTargetSegmentBytes
)

// SetPartFilterSizeLimit overrides the maximum bitmap size and the maximum segments
// count of a single part filter, and returns a function which restores the defaults.
//
// It is used by tests and benchmarks which compare different limits.
//
// maxBytes must not exceed maxSegments*partFilterTargetSegmentBytes, otherwise a filter
// segment grows beyond the size a single cache shard can hold, so every lookup would read
// it from disk again without reporting any fallback. The function panics on such a limit
// instead of silently producing a slower filter.
func SetPartFilterSizeLimit(maxBytes uint64, maxSegments int) func() {
	mustValidatePartFilterLimits(maxBytes, maxSegments)
	prevMaxBytes, prevMaxSegments := partFilterMaxBytes, partFilterMaxSegments
	partFilterMaxBytes, partFilterMaxSegments = maxBytes, maxSegments
	return func() {
		partFilterMaxBytes, partFilterMaxSegments = prevMaxBytes, prevMaxSegments
	}
}

// mustValidatePartFilterLimits panics if the given limits would produce filter segments
// bigger than partFilterTargetSegmentBytes.
func mustValidatePartFilterLimits(maxBytes uint64, maxSegments int) {
	if maxSegments < 1 {
		logger.Panicf("BUG: a part filter must have at least one segment; got maxSegments=%d", maxSegments)
	}
	if maxBytes < uint64(partFilterMinSegmentBytes) {
		logger.Panicf("BUG: maxBytes=%d is smaller than the minimum segment size %d, so the filter would exceed the requested limit; "+
			"increase maxBytes or use partFilterMinSegmentBytes", maxBytes, partFilterMinSegmentBytes)
	}
	if maxBytes <= uint64(maxSegments)*uint64(partFilterTargetSegmentBytes) {
		return
	}
	logger.Panicf("BUG: partFilterMaxBytes=%d exceeds partFilterMaxSegments=%d multiplied by partFilterTargetSegmentBytes=%d, "+
		"so a filter segment would not fit into a single part filter cache shard and would be re-read from disk on every lookup; "+
		"either increase partFilterMaxSegments to at least %d or reduce partFilterMaxBytes",
		maxBytes, maxSegments, partFilterTargetSegmentBytes,
		(maxBytes+uint64(partFilterTargetSegmentBytes)-1)/uint64(partFilterTargetSegmentBytes))
}

// PartFilterKey extracts the lookup key represented by an item. Returning nil
// skips the item. The returned key is consumed before the next callback.
type PartFilterKey func(item []byte) []byte

// pfbCache caches part filter segments in memory. Its size is limited by
// getMaxPartFilterCacheSize.
//
// The cache is shared by all mergeset tables, so a single budget covers all
// part filters, and the total memory occupied by part filters stays bounded
// independently of the number of parts and of the retention period.
var pfbCache = blockcache.NewCache(getMaxPartFilterCacheSize)

// SetPartFilterCacheSize overrides the default size of the indexdb/partFilterSegments cache.
//
// Pass 0 in order to restore the default size.
func SetPartFilterCacheSize(size int) {
	maxPartFilterCacheSize.Store(int64(size))
}

func getMaxPartFilterCacheSize() int {
	maxPartFilterCacheSizeDefaultOnce.Do(func() {
		maxPartFilterCacheSizeDefault = int(0.02 * float64(memory.Allowed()))
	})
	if n := maxPartFilterCacheSize.Load(); n > 0 {
		return int(n)
	}
	return maxPartFilterCacheSizeDefault
}

var (
	maxPartFilterCacheSize            atomic.Int64
	maxPartFilterCacheSizeDefault     int
	maxPartFilterCacheSizeDefaultOnce sync.Once

	partFilterBudgetWarned atomic.Bool
)

// partFilterDecision describes the outcome of a part filter check.
type partFilterDecision uint8

const (
	// partFilterNegative means that the part definitely doesn't contain the searched item.
	partFilterNegative partFilterDecision = iota

	// partFilterPossible means that the part may contain the searched item.
	partFilterPossible

	// partFilterUnfiltered means that the part has no filter, so it must be searched.
	partFilterUnfiltered

	// partFilterFallbackBudget means that the filter cannot be checked because of the missing cache budget.
	partFilterFallbackBudget

	// partFilterFallbackRead means that the filter segment cannot be read.
	partFilterFallbackRead

	// partFilterFallbackVerify means that the filter segment checksum doesn't match.
	partFilterFallbackVerify
)

// filterDecision returns the filter decision for p for the item with the given hashes.
//
// The returned decision is never partFilterNegative for a part without a filter,
// since such a part must be searched.
func (p *part) filterDecision(h, h1, h2 uint64) partFilterDecision {
	pf := p.ph.filter
	if pf == nil {
		return partFilterUnfiltered
	}
	return pf.mayContainHashes(p, h, h1, h2)
}

// partFilterSegment is a single cached filter segment.
type partFilterSegment []byte

// SizeBytes implements blockcache.Block interface.
func (s partFilterSegment) SizeBytes() int {
	return len(s)
}

// mayContainHashes reports whether the segment may contain the item with the given hashes.
func (s partFilterSegment) mayContainHashes(h1, h2 uint64) bool {
	segBitsCount := uint64(len(s)) * 8
	for i := uint64(0); i < partFilterProbes; i++ {
		pos := (h1 + i*h2) % segBitsCount
		if s[pos>>3]&(byte(1)<<(pos&7)) == 0 {
			return false
		}
	}
	return true
}

// partFilter is a segmented negative lookup filter for a single part.
//
// A filter consists of a small directory which is kept in memory, and of
// segments with Bloom filter bits. Segments are loaded from disk on demand and
// are cached in pfbCache under a shared memory budget.
//
// Filters for in-memory parts keep segments in memory, since the part data
// itself is in memory in this case.
type partFilter struct {
	// itemsCount is the number of filter keys covered by the filter.
	itemsCount uint64

	// segmentItemsCount contains the number of filter keys per segment. It is used
	// for writing the directory and is not needed after that.
	segmentItemsCount []uint32

	// inmemorySegments contains segments for in-memory parts.
	inmemorySegments [][]byte

	// segments contains the directory for file-based filters.
	segments []partFilterSegmentInfo

	// filterFile is the open filter file for file-based filters.
	filterFile *os.File

	// fileSizeBytes is the size of the filter file.
	fileSizeBytes uint64

	// disabledReason contains the fallback decision after the first failed segment
	// read or checksum check, so the part is searched instead of re-reading the
	// broken filter on every lookup.
	//
	// Zero value means that the filter isn't disabled, since partFilterNegative is zero.
	disabledReason atomic.Uint32

	// loggedFallback is set to true after logging the first failed segment read or checksum check.
	loggedFallback atomic.Bool
}

// partFilterSegmentInfo describes a single segment in the filter file.
type partFilterSegmentInfo struct {
	offset     uint64
	sizeBytes  uint32
	itemsCount uint32
	checksum   uint64
}

// newPartFilter returns a part filter for at most maxItems filter keys.
//
// The filter is created in memory, so it can be filled with addItem and then
// stored to disk with mustWriteFile. When the number of the added items exceeds
// maxItems, then the number of bits per item drops, so the filter returns more
// false positives. This doesn't affect correctness, since false positives lead
// to the ordinary index search.
//
// The memory occupied by the returned filter is limited by partFilterMaxBytes.
func newPartFilter(maxItems uint64) *partFilter {
	segmentsCount, segmentBytes := partFilterSizes(maxItems)
	segments := make([][]byte, segmentsCount)
	for i := range segments {
		segments[i] = make([]byte, segmentBytes)
	}
	return &partFilter{
		inmemorySegments:  segments,
		segmentItemsCount: make([]uint32, segmentsCount),
	}
}

// partFilterSizes returns the number of segments and the size of every segment for the given number of filter keys.
func partFilterSizes(maxItems uint64) (int, int) {
	totalBytes := maxItems * partFilterBitsPerItem / 8
	if totalBytes > partFilterMaxBytes {
		totalBytes = partFilterMaxBytes
	}
	segmentsCount := int((totalBytes + partFilterTargetSegmentBytes - 1) / partFilterTargetSegmentBytes)
	segmentsCount = max(segmentsCount, 1)
	segmentsCount = min(segmentsCount, partFilterMaxSegments)
	segmentBytes := int((totalBytes + uint64(segmentsCount) - 1) / uint64(segmentsCount))
	segmentBytes = (segmentBytes + 7) &^ 7
	segmentBytes = max(segmentBytes, partFilterMinSegmentBytes)
	return segmentsCount, segmentBytes
}

// addItem adds the given item to the filter via filterKey.
func (pf *partFilter) addItem(item []byte, filterKey PartFilterKey) {
	if pf == nil || filterKey == nil {
		return
	}
	if key := filterKey(item); key != nil {
		pf.addKey(key)
	}
}

// addKey adds the given key to the filter.
func (pf *partFilter) addKey(key []byte) {
	if pf == nil {
		return
	}
	if pf.inmemorySegments == nil {
		logger.Panicf("BUG: cannot add keys to the already stored part filter")
	}
	h, h1, h2 := partFilterHashes(key)
	idx := h % uint64(len(pf.inmemorySegments))
	segment := pf.inmemorySegments[idx]
	segmentBitsCount := uint64(len(segment)) * 8
	for i := uint64(0); i < partFilterProbes; i++ {
		pos := (h1 + i*h2) % segmentBitsCount
		segment[pos>>3] |= byte(1) << (pos & 7)
	}
	pf.segmentItemsCount[idx]++
	pf.itemsCount++
}

// mayContainHashes reports whether the filter may contain the item with the given hashes.
//
// It returns partFilterNegative if the item is definitely missing, and one of the
// partFilterFallback* decisions if the filter cannot be checked. The caller must
// search the part in the latter case.
func (pf *partFilter) mayContainHashes(p *part, h, h1, h2 uint64) partFilterDecision {
	if d := partFilterDecision(pf.disabledReason.Load()); d != partFilterNegative {
		return d
	}
	if pf.inmemorySegments != nil {
		// In-memory filters don't need the cache, so they are checked regardless
		// of the cache budget.
		idx := h % uint64(len(pf.inmemorySegments))
		segment := partFilterSegment(pf.inmemorySegments[idx])
		if segment.mayContainHashes(h1, h2) {
			return partFilterPossible
		}
		return partFilterNegative
	}
	idx := h % uint64(len(pf.segments))
	si := &pf.segments[idx]
	// Check the actual segment size against the shard budget before touching the
	// cache. Segments of small parts are smaller than partFilterTargetSegmentBytes,
	// so checking the target size instead would reject segments which do fit and
	// fall back to the index search for parts which could be filtered.
	if !pfbCache.CanStoreBlock(int(si.sizeBytes)) {
		if partFilterBudgetWarned.CompareAndSwap(false, true) {
			logger.Warnf("part filters are disabled, since a single indexdb/partFilterSegments cache shard "+
				"cannot hold a %d byte filter segment; searched parts may contain series which do not exist there",
				si.sizeBytes)
		}
		return partFilterFallbackBudget
	}

	key := blockcache.Key{
		Part:   p,
		Offset: idx,
	}
	if b := pfbCache.GetBlock(key); b != nil {
		if b.(partFilterSegment).mayContainHashes(h1, h2) {
			return partFilterPossible
		}
		return partFilterNegative
	}

	segment, err := pf.readSegment(si)
	if err != nil {
		pf.disable(partFilterFallbackRead, "cannot read part filter segment", err)
		return partFilterFallbackRead
	}
	if xxhash.Sum64(segment) != si.checksum {
		pf.disable(partFilterFallbackVerify, "part filter segment checksum mismatch", nil)
		return partFilterFallbackVerify
	}
	pfbCache.TryPutBlock(key, segment)
	if segment.mayContainHashes(h1, h2) {
		return partFilterPossible
	}
	return partFilterNegative
}

func (pf *partFilter) disable(decision partFilterDecision, reason string, err error) {
	pf.disabledReason.CompareAndSwap(0, uint32(decision))
	if !pf.loggedFallback.CompareAndSwap(false, true) {
		return
	}
	if err != nil {
		logger.Warnf("%s for the part %q, so the part is searched instead of being filtered until the next restart: %s",
			reason, pf.partPath(), err)
		return
	}
	logger.Warnf("%s for the part %q, so the part is searched instead of being filtered until the next restart",
		reason, pf.partPath())
}

func (pf *partFilter) partPath() string {
	if pf.filterFile != nil {
		return filepath.Dir(pf.filterFile.Name())
	}
	return ""
}

func (pf *partFilter) readSegment(si *partFilterSegmentInfo) (partFilterSegment, error) {
	segment := make([]byte, si.sizeBytes)
	if _, err := pf.filterFile.ReadAt(segment, int64(si.offset)); err != nil {
		return nil, err
	}
	return partFilterSegment(segment), nil
}

// mustWriteFile stores the filter to partPath/filter.bin.
func (pf *partFilter) mustWriteFile(partPath string) {
	if pf == nil || pf.inmemorySegments == nil {
		return
	}
	dirEnd := partFilterHeaderSize + len(pf.inmemorySegments)*partFilterSegmentEntrySize
	bufLen := dirEnd + partFilterChecksumSize
	buf := make([]byte, bufLen)
	copy(buf, partFilterMagic)
	binary.LittleEndian.PutUint32(buf[len(partFilterMagic):], uint32(len(pf.inmemorySegments)))
	binary.LittleEndian.PutUint64(buf[len(partFilterMagic)+4:], pf.itemsCount)
	offset := uint64(bufLen)
	for i, segment := range pf.inmemorySegments {
		be := buf[partFilterHeaderSize+i*partFilterSegmentEntrySize:]
		binary.LittleEndian.PutUint64(be, offset)
		binary.LittleEndian.PutUint32(be[8:], uint32(len(segment)))
		binary.LittleEndian.PutUint32(be[12:], pf.segmentItemsCount[i])
		binary.LittleEndian.PutUint64(be[16:], xxhash.Sum64(segment))
		offset += uint64(len(segment))
	}
	binary.LittleEndian.PutUint64(buf[dirEnd:], xxhash.Sum64(buf[:dirEnd]))

	filterPath := filepath.Join(partPath, filterFilename)
	w := filestream.MustCreate(filterPath, false)
	fs.MustWriteData(w, buf)
	for _, segment := range pf.inmemorySegments {
		fs.MustWriteData(w, segment)
	}
	// MustClose flushes and fsyncs the written data.
	w.MustClose()

	pf.fileSizeBytes = offset
}

// sizeBytes returns the size of the filter in bytes.
//
// It returns the size of the filter file for file-based filters and the size of
// the in-memory segments for in-memory filters.
func (pf *partFilter) sizeBytes() uint64 {
	if pf == nil {
		return 0
	}
	if pf.inmemorySegments != nil {
		n := uint64(0)
		for _, segment := range pf.inmemorySegments {
			n += uint64(len(segment))
		}
		return n
	}
	return pf.fileSizeBytes
}

// segmentsCount returns the number of segments in the filter.
func (pf *partFilter) segmentsCount() uint64 {
	if pf == nil {
		return 0
	}
	if pf.inmemorySegments != nil {
		return uint64(len(pf.inmemorySegments))
	}
	return uint64(len(pf.segments))
}

// mustClose closes the filter file.
func (pf *partFilter) mustClose() {
	if pf == nil || pf.filterFile == nil {
		return
	}
	fs.MustClose(pf.filterFile)
	pf.filterFile = nil
}

// mustOpenPartFilter opens the filter at the given part path.
//
// It reads and validates only the filter directory. Segments are loaded on demand.
// It returns (nil, nil) if the filter cannot be used, so the part is searched instead.
func mustOpenPartFilter(partPath string) (*partFilter, error) {
	filterPath := filepath.Join(partPath, filterFilename)
	f, err := os.Open(filterPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		_ = f.Close()
		return nil, nil
	}
	fileSize := uint64(fi.Size())
	pf, err := readPartFilterDirectory(f, fileSize)
	if err != nil || pf == nil {
		_ = f.Close()
		return nil, err
	}
	pf.filterFile = f
	return pf, nil
}

// readPartFilterDirectory reads and validates the filter directory from f.
func readPartFilterDirectory(f *os.File, fileSize uint64) (*partFilter, error) {
	maxDirSize := partFilterHeaderSize + partFilterMaxSegments*partFilterSegmentEntrySize + partFilterChecksumSize
	if fileSize < uint64(partFilterHeaderSize)+partFilterChecksumSize {
		return nil, nil
	}
	readSize := min(fileSize, uint64(maxDirSize))
	buf := make([]byte, readSize)
	if _, err := f.ReadAt(buf, 0); err != nil {
		return nil, err
	}
	if string(buf[:len(partFilterMagic)]) != partFilterMagic {
		return nil, nil
	}
	segmentsCount := int(binary.LittleEndian.Uint32(buf[len(partFilterMagic):]))
	if segmentsCount < 1 || segmentsCount > partFilterMaxSegments {
		return nil, nil
	}
	dirEnd := partFilterHeaderSize + segmentsCount*partFilterSegmentEntrySize
	if dirEnd+partFilterChecksumSize > int(readSize) {
		return nil, nil
	}
	if binary.LittleEndian.Uint64(buf[dirEnd:]) != xxhash.Sum64(buf[:dirEnd]) {
		return nil, nil
	}
	pf := &partFilter{
		itemsCount:    binary.LittleEndian.Uint64(buf[len(partFilterMagic)+4:]),
		segments:      make([]partFilterSegmentInfo, segmentsCount),
		fileSizeBytes: fileSize,
	}
	totalItemsCount := uint64(0)
	for i := range pf.segments {
		be := buf[partFilterHeaderSize+i*partFilterSegmentEntrySize:]
		si := &pf.segments[i]
		si.offset = binary.LittleEndian.Uint64(be)
		si.sizeBytes = binary.LittleEndian.Uint32(be[8:])
		si.itemsCount = binary.LittleEndian.Uint32(be[12:])
		si.checksum = binary.LittleEndian.Uint64(be[16:])
		// A legit segment is never bigger than partFilterMaxBytes. The check keeps a
		// corrupted directory from triggering a too big allocation on segment read.
		if si.sizeBytes < partFilterMinSegmentBytes || si.sizeBytes%8 != 0 || uint64(si.sizeBytes) > partFilterMaxBytes {
			return nil, nil
		}
		if si.offset < uint64(dirEnd)+partFilterChecksumSize || si.offset+uint64(si.sizeBytes) > fileSize {
			return nil, nil
		}
		totalItemsCount += uint64(si.itemsCount)
	}
	if totalItemsCount != pf.itemsCount {
		return nil, nil
	}
	return pf, nil
}

// partFilterHashes returns the hashes for the given filter key.
//
// hSeg is used for selecting the filter segment, while h1 and h2 are used for
// selecting bits inside the segment.
//
// hSeg is derived from the bits which are not used by h1, since otherwise the
// first probe of every key from the same segment would be confined to a
// (bitsInSegment/segmentsCount) fraction of the segment bits whenever the
// segments count has a common factor with the segment bits count - for example,
// both being powers of two. Such a confinement doesn't reduce the fraction of
// the set bits, but it increases the false positive rate, since the first probe
// is checked against the confined subset of the bits.
func partFilterHashes(key []byte) (hSeg, h1, h2 uint64) {
	h := xxhash.Sum64(key)
	// Use the two halves of the hash for double hashing.
	hSeg = h >> 32
	h1 = uint64(uint32(h))
	h2 = uint64(uint32(h>>32)) | 1
	return hSeg, h1, h2
}
