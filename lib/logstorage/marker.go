package logstorage

import (
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

const (
	markerTypeDelete = 1
)

// marker aggregates indexes for all marker types belonging to a single part.
// Currently only Delete markers (type=1) are supported.
type marker struct {
	blocksCount uint64 // total blocks in the part – needed for delete marker

	// delete holds row-delete bitmap per block. Accessed without extra locking, so it
	// must be treated as **immutable** after publication. Writers must build a fresh
	// *deleteMarker, then store it via atomic.Store (see flushDeleteMarker).
	delete atomic.Pointer[deleteMarker]
}

// Marshal encodes and returns the marker data buffer.
// The format starts with marker type, followed by type-specific data.
func (mi *marker) Marshal() []byte {
	if mi == nil {
		return nil
	}

	del := mi.delete.Load()
	if del == nil || len(del.blockIDs) == 0 {
		return nil
	}

	// Format: [marker_type:uint8][type_specific_data...]
	buf := make([]byte, 0, 1024)
	buf = append(buf, markerTypeDelete)
	buf = del.Marshal(buf)
	return buf
}

// Unmarshal parses marker contents from the supplied data bytes.
// It reads the marker type first, then dispatches to type-specific unmarshaling.
// Caller must set mi.blocksCount before calling (usually from partHeader.BlocksCount).
func (mi *marker) Unmarshal(data []byte) error {
	if mi == nil {
		return fmt.Errorf("marker is nil")
	}

	if mi.blocksCount == 0 {
		return fmt.Errorf("marker.blocksCount is 0; must be set before Unmarshal")
	}

	if len(data) == 0 {
		// No markers - this is valid
		return nil
	}

	pos := 0
	for pos < len(data) {
		if pos >= len(data) {
			return fmt.Errorf("data too small for marker type at position %d", pos)
		}

		markerType := data[pos]
		pos++

		switch markerType {
		case 1: // Delete
			if mi.delete.Load() != nil {
				return fmt.Errorf("duplicated delete marker")
			}

			var del deleteMarker
			bytesConsumed, err := del.Unmarshal(mi.blocksCount, data[pos:])
			if err != nil {
				return fmt.Errorf("deleteMarker.Unmarshal: %w", err)
			}
			if len(del.blockIDs) > 0 {
				mi.delete.Store(&del)
			}
			pos += bytesConsumed
		default:
			// Unknown marker type – skip it by returning an error for now
			// In the future, we could implement a way to skip unknown marker types
			return fmt.Errorf("unknown marker type %d at position %d", markerType, pos-1)
		}
	}
	return nil
}

// markedBlocks is the common part shared by all marker indexes – a sparse
// mapping from blockSeq -> RLE blob (or full-delete sentinel).
type markedBlocks struct {
	blockIDs []uint32  // sorted block sequence numbers that have marker data
	rows     []boolRLE // same length and order as blockIDs
}

func (mb *markedBlocks) String() string {
	s := strings.Builder{}
	for i, blockID := range mb.blockIDs {
		s.WriteString(fmt.Sprintf("[%d] blockID: %d, row: %v\n", i, blockID, mb.rows[i]))
	}
	return s.String()
}

// GetMarkedRows returns marked rows for the given block sequence number.
func (mb *markedBlocks) GetMarkedRows(blockSeq uint32) (boolRLE, bool) {
	idx, found := slices.BinarySearch(mb.blockIDs, blockSeq)
	if !found {
		return nil, false
	}

	return mb.rows[idx], true
}

// mustReadMarkerData reads marker data from the provided reader.
// Caller must set returned marker.blocksCount before calling marker.Unmarshal().
func mustReadMarkerData(datReader filestream.ReadCloser, blocksCount uint64) *marker {
	if datReader == nil {
		return nil
	}

	datBytes, err := io.ReadAll(datReader)
	if err != nil {
		logger.Panicf("FATAL: %s: cannot read marker data: %s", datReader.Path(), err)
	}

	mi := &marker{blocksCount: blocksCount}
	if err := mi.Unmarshal(datBytes); err != nil {
		logger.Panicf("FATAL: %s: cannot unmarshal marker data: %s", datReader.Path(), err)
	}

	return mi
}

// deleteMarker keeps per-block Delete markers (markerType = 1).
type deleteMarker struct {
	markedBlocks
}

// Marshal serializes delete marker data to the provided buffer.
// Format: [num_blocks:varuint64][block_id:uint32][rle_len:varuint64][rle_data:bytes]...
func (dm *deleteMarker) Marshal(dst []byte) []byte {
	// Number of blocks with markers
	dst = encoding.MarshalVarUint64(dst, uint64(len(dm.blockIDs)))

	// For each block, write: block_id + rle_length + rle_data
	for i, blockID := range dm.blockIDs {
		dst = encoding.MarshalUint32(dst, blockID)

		rleData := dm.rows[i]
		dst = encoding.MarshalVarUint64(dst, uint64(len(rleData)))
		dst = append(dst, rleData...)
	}

	return dst
}

// Unmarshal parses delete marker data from the provided bytes.
// Returns the number of bytes consumed and any error.
func (dm *deleteMarker) Unmarshal(blocksCount uint64, data []byte) (int, error) {
	*dm = deleteMarker{} // reset

	if len(data) == 0 {
		return 0, nil // No delete markers
	}

	pos := 0

	// Read number of blocks
	if pos >= len(data) {
		return 0, fmt.Errorf("truncated data: cannot read num_blocks")
	}
	numBlocks, n := encoding.UnmarshalVarUint64(data[pos:])
	pos += n

	// Read each block's data
	for i := range numBlocks {
		// Read block ID
		if pos+4 > len(data) {
			return 0, fmt.Errorf("truncated data: cannot read block_id %d", i)
		}
		blockID := encoding.UnmarshalUint32(data[pos:])
		pos += 4

		// Read RLE data length
		if pos >= len(data) {
			return 0, fmt.Errorf("truncated data: cannot read rle_len for block %d", i)
		}
		rleLen, n := encoding.UnmarshalVarUint64(data[pos:])
		pos += n

		// Read RLE data
		if pos+int(rleLen) > len(data) {
			return 0, fmt.Errorf("truncated data: cannot read rle_data for block %d", i)
		}
		rleData := make([]byte, rleLen)
		copy(rleData, data[pos:pos+int(rleLen)])
		pos += int(rleLen)

		dm.blockIDs = append(dm.blockIDs, blockID)
		dm.rows = append(dm.rows, boolRLE(rleData))
	}

	return pos, nil
}

// merge combines this deleteMarker with another deleteMarker.
// It merges block deletions using RLE union operations where both markers have the same blockID.
func (dm *deleteMarker) merge(other *deleteMarker) {
	// Nothing to merge?
	if other == nil || len(other.blockIDs) == 0 {
		return
	}
	if len(dm.blockIDs) == 0 {
		// dm is empty – just copy other's data.
		dm.blockIDs = append([]uint32(nil), other.blockIDs...)
		dm.rows = append([]boolRLE(nil), other.rows...)
		return
	}

	// Two‑pointer merge because both blockID slices are already sorted.
	mergedIDs := make([]uint32, 0, len(dm.blockIDs)+len(other.blockIDs))
	mergedRows := make([]boolRLE, 0, len(dm.rows)+len(other.rows))

	i, j := 0, 0
	for i < len(dm.blockIDs) && j < len(other.blockIDs) {
		idA, idB := dm.blockIDs[i], other.blockIDs[j]

		switch {
		case idA == idB:
			// Same block exists in both markers – union their RLEs.
			mergedIDs = append(mergedIDs, idA)
			mergedRows = append(mergedRows, dm.rows[i].Union(other.rows[j]))
			i++
			j++

		case idA < idB:
			mergedIDs = append(mergedIDs, idA)
			mergedRows = append(mergedRows, dm.rows[i])
			i++

		default: // idB < idA
			mergedIDs = append(mergedIDs, idB)
			mergedRows = append(mergedRows, other.rows[j])
			j++
		}
	}

	// Append leftovers from either slice.
	for ; i < len(dm.blockIDs); i++ {
		mergedIDs = append(mergedIDs, dm.blockIDs[i])
		mergedRows = append(mergedRows, dm.rows[i])
	}
	for ; j < len(other.blockIDs); j++ {
		mergedIDs = append(mergedIDs, other.blockIDs[j])
		mergedRows = append(mergedRows, other.rows[j])
	}

	// Install merged slices.
	dm.blockIDs = mergedIDs
	dm.rows = mergedRows
}

// AddBlock adds a block with its RLE data to the deleteMarker.
// If blockID already exists, it merges the RLE data using union operation.
func (dm *deleteMarker) AddBlock(blockID uint32, rle boolRLE) {
	// Find existing block or insertion point
	idx, found := slices.BinarySearch(dm.blockIDs, blockID)

	if found {
		// Block already exists - merge RLE data using union operation
		existingRLE := dm.rows[idx]
		combined := existingRLE.Union(rle)
		dm.rows[idx] = combined
	} else {
		// Block doesn't exist - insert at correct position
		dm.blockIDs = slices.Insert(dm.blockIDs, idx, blockID)
		dm.rows = slices.Insert(dm.rows, idx, rle)
	}
}

// flushDeleteMarker writes delMarker to disk and updates the in-memory index
// for the given part. Writers are serialized by ddb.partsLock; readers access
// the index lock-free via atomic.Load.
func flushDeleteMarker(pw *partWrapper, dm *deleteMarker, seq uint64) {
	if dm == nil || len(dm.blockIDs) == 0 {
		return // nothing to flush
	}

	if pw == nil {
		logger.Panicf("FATAL: flushDeleteMarker: pw is nil")
	}

	p := pw.p
	// In-memory part has no path yet – merge marker in memory and return.
	if p.path == "" {
		addInMemoryDeleteMarker(p, dm)
		return
	}

	ddb := p.pt.ddb

	// Serialize with other modifications to the same part and protect against
	// concurrent merges that may delete / rename the directory.
	ddb.partsLock.Lock()
	defer ddb.partsLock.Unlock()
	if pw.isInMerge || pw.mustDrop.Load() {
		return
	}

	// Make sure marker object exists.
	if p.marker == nil {
		p.marker = &marker{blocksCount: p.ph.BlocksCount}
	}

	current := p.marker.delete.Load()

	var merged *deleteMarker
	if current == nil {
		// First marker – just deep-copy delMarker to avoid sharing mutable slices.
		merged = &deleteMarker{
			markedBlocks: markedBlocks{
				blockIDs: append([]uint32(nil), dm.blockIDs...),
				rows:     append([]boolRLE(nil), dm.rows...),
			},
		}
	} else {
		// Copy-on-write: start from current snapshot and merge additions.
		merged = &deleteMarker{
			markedBlocks: markedBlocks{
				blockIDs: append([]uint32(nil), current.blockIDs...),
				rows:     append([]boolRLE(nil), current.rows...),
			},
		}
		merged.merge(dm)
	}

	// Publish the new snapshot for readers.
	p.marker.delete.Store(merged)

	// Persist to disk.
	datBuf := p.marker.Marshal()
	partPath := p.path
	datPath := filepath.Join(partPath, rowMarkerDatFilename)
	fs.MustWriteAtomic(datPath, datBuf, true /*overwrite*/)
	fs.MustSyncPath(partPath)
	p.setAppliedTSeq(seq)
	logger.Infof("DEBUG: flushDeleteMarker: part=%s, seq=%d, rle=%s", partPath, seq, merged.String())
}

// addInMemoryDeleteMarker merges dm into p.marker.delete without touching disk.
// It performs copy-on-write merge similar to flushDeleteMarker, but deliberately
// skips fs writes, since the part directory may be concurrently renamed or
// deleted by an ongoing merge. Readers are immediately able to observe the new
// bitmap via atomic pointer publication.
func addInMemoryDeleteMarker(p *part, dm *deleteMarker) {
	if dm == nil || len(dm.blockIDs) == 0 {
		return
	}

	// Ensure marker container exists.
	if p.marker == nil {
		p.marker = &marker{blocksCount: p.ph.BlocksCount}
	}

	current := p.marker.delete.Load()
	var merged *deleteMarker
	if current == nil {
		merged = &deleteMarker{
			markedBlocks: markedBlocks{
				blockIDs: append([]uint32(nil), dm.blockIDs...),
				rows:     append([]boolRLE(nil), dm.rows...),
			},
		}
	} else {
		merged = &deleteMarker{
			markedBlocks: markedBlocks{
				blockIDs: append([]uint32(nil), current.blockIDs...),
				rows:     append([]boolRLE(nil), current.rows...),
			},
		}
		merged.merge(dm)
	}

	// Publish for readers.
	p.marker.delete.Store(merged)
}
