package logstorage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

// markerEntry describes a single RLE blob for a block inside rowmarker.dat.
// Offset and Size point to the blob location inside the data file. Data is a
// slice alias containing the blob bytes once the marker index has been loaded
// into RAM.
//
// Size can take a special value 0xFFFF_FFFF meaning the entire block is
// deleted and therefore no RLE blob is needed.
type markerEntry struct {
	Offset uint64
	Size   uint32
	Data   []byte
}

// markerIndex keeps an in-RAM copy of presence bitmap + vector entries for a
// single marker type (e.g. Deleted) belonging to a part.
//
// It is optimized for fast binary search (log₂N with N = marked blocks).
type markerIndex struct {
	blockIDs []uint32      // sorted block sequence numbers that have marker data
	entries  []markerEntry // same length and order as blockIDs
}

// EntryFor returns markerEntry for the given block sequence number. The second
// returned value reports whether the entry exists.
func (mi *markerIndex) EntryFor(blockSeq uint32) (markerEntry, bool) {
	idx := binarySearchUint32(mi.blockIDs, blockSeq)
	if idx < 0 {
		return markerEntry{}, false
	}
	return mi.entries[idx], true
}

// binarySearchUint32 searches x in a sorted slice and returns its index or -1
// if not found.
func binarySearchUint32(a []uint32, x uint32) int {
	lo := 0
	hi := len(a) - 1
	for lo <= hi {
		mid := (lo + hi) >> 1
		v := a[mid]
		if v == x {
			return mid
		}
		if v < x {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return -1
}

// openMarkerIndex loads marker index and data for the requested marker type
// from <partPath>/<rowmarkerIdxFilename> and <partPath>/<rowmarkerDatFilename>.
func openMarkerIndex(partPath string, wantedType uint8) (*markerIndex, error) {
	idxPath := filepath.Join(partPath, rowmarkerIdxFilename)
	datPath := filepath.Join(partPath, rowmarkerDatFilename)

	// Read idx file.
	idxBytes, err := os.ReadFile(idxPath)
	if err != nil {
		return nil, err
	}
	if len(idxBytes) < 16 {
		return nil, fmt.Errorf("rowmarker.idx too small: %d bytes", len(idxBytes))
	}

	// Parse header.
	typeCount := encoding.UnmarshalUint32(idxBytes)
	pos := 4
	var sectionOffset uint64
	found := false
	for i := uint32(0); i < typeCount; i++ {
		if pos+12 > len(idxBytes) {
			return nil, fmt.Errorf("corrupted header, unexpected end")
		}
		markerID := idxBytes[pos]
		pos += 4 // 1 byte id + 3 pad
		off := encoding.UnmarshalUint64(idxBytes[pos:])
		pos += 8
		if markerID == wantedType {
			sectionOffset = off
			found = true
			break
		}
	}
	if !found {
		return &markerIndex{}, nil // no markers of wanted type
	}
	if int(sectionOffset) >= len(idxBytes) {
		return nil, fmt.Errorf("sectionOffset beyond file size")
	}

	// Read part header to get blocksCount.
	var ph partHeader
	ph.mustReadMetadata(partPath)
	blocksCount := ph.BlocksCount
	presenceLen := (blocksCount + 7) / 8

	if int(sectionOffset+presenceLen) > len(idxBytes) {
		return nil, fmt.Errorf("presence bitmap beyond file size")
	}

	presBits := idxBytes[sectionOffset : sectionOffset+presenceLen]
	vectorPos := sectionOffset + presenceLen

	// Load entire data file once into memory for slicing.
	datBytes, err := os.ReadFile(datPath)
	if err != nil {
		return nil, err
	}

	mi := &markerIndex{}

	// Iterate over blocks to build entries.
	for blk := uint32(0); blk < uint32(blocksCount); blk++ {
		byteIdx := blk / 8
		bitMask := byte(1 << (blk % 8))
		if presBits[byteIdx]&bitMask == 0 {
			continue
		}
		if int(vectorPos+12) > len(idxBytes) {
			return nil, fmt.Errorf("vector entry beyond file size")
		}
		off := encoding.UnmarshalUint64(idxBytes[vectorPos:])
		size := encoding.UnmarshalUint32(idxBytes[vectorPos+8:])
		vectorPos += 12

		var dataSlice []byte
		if size != 0xFFFF_FFFF {
			if int(off)+int(size) > len(datBytes) {
				return nil, fmt.Errorf("RLE blob beyond dat file")
			}
			dataSlice = datBytes[off : off+uint64(size)]
		}

		mi.blockIDs = append(mi.blockIDs, blk)
		mi.entries = append(mi.entries, markerEntry{
			Offset: off,
			Size:   size,
			Data:   dataSlice,
		})
	}

	return mi, nil
}
