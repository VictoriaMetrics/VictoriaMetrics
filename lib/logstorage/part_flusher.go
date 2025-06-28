package logstorage

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

// partFlusher accumulates per-block marker updates and flushes them atomically
// to the on-disk <part>/rowmarker.idx and <part>/rowmarker.dat> files.
//
// It is not safe for concurrent use; external synchronization (markersWriteLock)
// is required.
type markerPartFlusher struct {
	partPath    string
	blocksCount uint64

	// Marker type we are writing (v1 fixed to 1 == Deleted).
	markerType uint8

	// Accumulated updates for the current flush window.
	pendingPresBits []byte        // presence bitmap (bitsPerBlock) – will be merged with existing on flush
	pendingBlockIDs []uint32      // block sequence numbers that were updated – ascending after sorting
	pendingEntries  []markerEntry // parallel to pendingBlockIDs

	datBuf []byte // RLE blobs written sequentially; offsets are relative to beginning of this buffer
}

// newPartFlusher returns initialized partFlusher for the given on-disk part.
func newMarkerPartFlusher(partPath string, markerType uint8) *markerPartFlusher {
	// Read part header to obtain BlocksCount (needed for presence bitmap sizing).
	var ph partHeader
	ph.mustReadMetadata(partPath)

	presBitsLen := int((ph.BlocksCount + 7) / 8)
	return &markerPartFlusher{
		partPath:        partPath,
		blocksCount:     ph.BlocksCount,
		markerType:      markerType,
		pendingPresBits: make([]byte, presBitsLen),
	}
}

// AddMarker registers marker RLE for the given block sequence number.
//
// If the entire block is deleted, pass size == 0xFFFF_FFFF and rle == nil.
func (pf *markerPartFlusher) AddMarker(blockSeq uint32, rle []byte) {
	if uint64(blockSeq) >= pf.blocksCount {
		logger.Panicf("BUG: blockSeq=%d exceeds BlocksCount=%d", blockSeq, pf.blocksCount)
	}

	// Remember presence bit
	byteIdx := blockSeq / 8
	bitMask := byte(1 << (blockSeq % 8))
	pf.pendingPresBits[byteIdx] |= bitMask

	// Determine offset in datBuf.
	offset := uint64(len(pf.datBuf))
	size := uint32(len(rle))
	if size == 0 && rle == nil {
		size = 0xFFFF_FFFF // full-block delete marker
	}

	var dataCopy []byte
	if size != 0xFFFF_FFFF {
		pf.datBuf = append(pf.datBuf, rle...)
		dataCopy = rle
	}

	pf.pendingBlockIDs = append(pf.pendingBlockIDs, blockSeq)
	pf.pendingEntries = append(pf.pendingEntries, markerEntry{
		Offset: offset,
		Size:   size,
		Data:   dataCopy,
	})
}

// FlushMarkers writes rowmarker.idx/dat files (via atomic rename) and resets buffers.
func (pf *markerPartFlusher) FlushMarkers() {
	if len(pf.pendingBlockIDs) == 0 {
		return // nothing to flush
	}

	flushStart := time.Now()

	// Paths.
	idxPath := filepath.Join(pf.partPath, rowmarkerIdxFilename)
	datPath := filepath.Join(pf.partPath, rowmarkerDatFilename)

	// mergedPresBits will contain final OR-ed presence bits (existing | pending)
	// Start with zeroed slice; fill with existing bits first, then add pending bits later.
	mergedPresBits := make([]byte, len(pf.pendingPresBits))

	mergedMap := make(map[uint32]markerEntry)

	// Load existing index if exists.
	if fs.IsPathExist(idxPath) {
		idxBytes, err := os.ReadFile(idxPath)
		if err == nil && len(idxBytes) >= 16 {
			typeCnt := encoding.UnmarshalUint32(idxBytes)
			pos := 4
			var sectionOff uint64
			found := false
			for i := uint32(0); i < typeCnt; i++ {
				if pos+12 > len(idxBytes) {
					break
				}
				mtype := idxBytes[pos]
				pos += 4 // skip 1+3 pad
				off := encoding.UnmarshalUint64(idxBytes[pos:])
				pos += 8
				if mtype == pf.markerType {
					sectionOff = off
					found = true
				}
			}
			if found {
				if int(sectionOff) < len(idxBytes) {
					section := idxBytes[sectionOff:]
					if len(section) >= len(mergedPresBits) {
						// Presence bitmap from existing index
						existingPresBits := section[:len(mergedPresBits)]

						// Copy existing presence bits into mergedPresBits for now.
						copy(mergedPresBits, existingPresBits)

						// Vector entries
						vec := section[len(mergedPresBits):]
						readerPos := 0
						// Determine blockIDs by scanning bits.
						var blockID uint32
						for bIdx := 0; bIdx < len(mergedPresBits); bIdx++ {
							b := existingPresBits[bIdx]
							for bit := 0; bit < 8 && int(blockID) < len(mergedPresBits)*8; bit++ {
								if b&(1<<bit) != 0 {
									if readerPos+12 > len(vec) {
										break
									}
									off := encoding.UnmarshalUint64(vec[readerPos:])
									size := encoding.UnmarshalUint32(vec[readerPos+8:])
									readerPos += 12

									// Load data bytes if not full delete
									var data []byte
									if size != 0xFFFF_FFFF {
										// read from dat file
										f, err := os.Open(datPath)
										if err == nil {
											data = make([]byte, size)
											_, _ = f.ReadAt(data, int64(off))
											f.Close()
										}
									}
									mergedMap[blockID] = markerEntry{Offset: off, Size: size, Data: data}
								}
								blockID++
							}
						}
					}
				}
			}
		}
	}

	// Override / add pending entries and OR pending presence bits.
	for i, id := range pf.pendingBlockIDs {
		ent := pf.pendingEntries[i]
		mergedMap[id] = ent
		// set presence bit
		byteIdx := id / 8
		bitMask := byte(1 << (id % 8))
		mergedPresBits[byteIdx] |= bitMask
	}

	// Build merged slices in ascending order.
	mergedIDs := make([]uint32, 0, len(mergedMap))
	for id := range mergedMap {
		mergedIDs = append(mergedIDs, id)
	}
	sort.Slice(mergedIDs, func(i, j int) bool { return mergedIDs[i] < mergedIDs[j] })

	mergedEntries := make([]markerEntry, len(mergedIDs))
	for i, id := range mergedIDs {
		mergedEntries[i] = mergedMap[id]
	}

	// Rebuild dat buffer and update offsets.
	var datBuf []byte
	for i := range mergedEntries {
		if mergedEntries[i].Size == 0xFFFF_FFFF {
			mergedEntries[i].Offset = 0
			continue
		}
		mergedEntries[i].Offset = uint64(len(datBuf))
		datBuf = append(datBuf, mergedEntries[i].Data...)
	}

	// 1. Write dat file atomically
	fs.MustWriteAtomic(datPath, datBuf, true /*overwrite*/)

	// 2. Build idx buffer
	idxBuf := make([]byte, 0, 16+len(mergedPresBits)+len(mergedEntries)*12)

	idxBuf = encoding.MarshalUint32(idxBuf, 1)  // typeCount
	idxBuf = append(idxBuf, pf.markerType)      // markerTypeID
	idxBuf = append(idxBuf, 0, 0, 0)            // pad
	idxBuf = encoding.MarshalUint64(idxBuf, 16) // section offset

	idxBuf = append(idxBuf, mergedPresBits...)
	for _, m := range mergedEntries {
		idxBuf = encoding.MarshalUint64(idxBuf, m.Offset)
		idxBuf = encoding.MarshalUint32(idxBuf, m.Size)
	}

	fs.MustWriteAtomic(idxPath, idxBuf, true)

	fs.MustSyncPath(pf.partPath)

	// Reset buffers
	pf.pendingPresBits = make([]byte, len(pf.pendingPresBits))
	pf.pendingBlockIDs = pf.pendingBlockIDs[:0]
	pf.pendingEntries = pf.pendingEntries[:0]
	pf.datBuf = pf.datBuf[:0]

	metrics.GetOrCreateHistogram(`vm_marker_flush_seconds`).UpdateDuration(flushStart)
}
