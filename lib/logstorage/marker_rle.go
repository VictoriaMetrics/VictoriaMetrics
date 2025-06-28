package logstorage

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

// MarshalRLE encodes the given bitmap into run-length encoding and appends the
// resulting bytes to dst. The format starts with the length of the initial run
// of zero bits and then alternates zero-runs and one-runs until the end of the
// bitmap is reached. All run lengths are encoded with variable-length unsigned
// integers from the encoding package.
//
// Examples:
//
//	000111 -> 3,3      (encoded as VarUInt64 3,3)
//	1100   -> 0,2,2,0 (encoded 0,2,2,0)
func MarshalRLE(dst []byte, bm *bitmap) []byte {
	if bm == nil || bm.bitsLen == 0 {
		return encoding.MarshalVarUint64(dst, 0) // single zero-run
	}

	bitsLen := bm.bitsLen
	pos := 0
	// The encoding always starts with zeros-run.
	for pos < bitsLen {
		// Count consecutive zeros.
		zeros := 0
		for pos < bitsLen && !bm.isSetBit(pos) {
			zeros++
			pos++
		}
		dst = encoding.MarshalVarUint64(dst, uint64(zeros))

		// Count consecutive ones.
		ones := 0
		for pos < bitsLen && bm.isSetBit(pos) {
			ones++
			pos++
		}
		dst = encoding.MarshalVarUint64(dst, uint64(ones))
	}

	// If the bitmap ends with a zeros-run we already emitted it. If it ends with
	// a ones-run we must emit the trailing zeros-run of length 0 so that the
	// decoder knows the stream finished on a ones-run. The loop above always
	// emits pairs (zeros,ones), so when the bitmap ends with ones we already
	// emitted zeros-run (possibly 0) at the start of the next iteration that is
	// prevented by pos>=bitsLen. Therefore no action is required here.

	return dst
}

// AndNotRLE performs dst &= ^RLE_bitmap, where RLE_bitmap is a bitmap encoded
// with MarshalRLE. In other words, it clears bits in dst that correspond to
// one-runs in the supplied RLE stream.
func AndNotRLE(dst *bitmap, rle []byte) {
	if dst == nil || len(rle) == 0 {
		return
	}

	var (
		idx int // current byte position in rle slice
		pos int // current bit position inside dst
	)

	for idx < len(rle) && pos < dst.bitsLen {
		// Decode zeros-run.
		zeros, n := encoding.UnmarshalVarUint64(rle[idx:])
		idx += n
		pos += int(zeros)
		if pos >= dst.bitsLen {
			break
		}

		// Decode ones-run.
		ones, n := encoding.UnmarshalVarUint64(rle[idx:])
		idx += n
		if ones > 0 {
			dst.clearBitsRange(pos, int(ones))
		}
		pos += int(ones)
	}
}

// clearBitsRange clears n bits starting from the given bit offset.
func (bm *bitmap) clearBitsRange(start, n int) {
	if n <= 0 || start >= bm.bitsLen {
		return
	}

	end := start + n
	if end > bm.bitsLen {
		end = bm.bitsLen
	}

	startWord := start >> 6   // starting word index
	endWord := (end - 1) >> 6 // ending word index (inclusive)

	// handle the first word (may be the only one)
	if startWord == endWord {
		maskStart := uint(start & 63)
		maskEnd := uint((end - 1) & 63)
		mask := ((^uint64(0)) << maskStart) & ((uint64(1) << (maskEnd + 1)) - 1)
		bm.a[startWord] &^= mask
		return
	}

	// clear from start bit to end of start word
	if offset := uint(start & 63); offset != 0 {
		mask := ^uint64(0) << offset
		bm.a[startWord] &^= mask
		startWord++
	}

	// clear full words in the middle
	for i := startWord; i < endWord; i++ {
		bm.a[i] = 0
	}

	// clear beginning part of the last word up to end bit
	maskEnd := uint((end - 1) & 63)
	tailMask := (uint64(1) << (maskEnd + 1)) - 1
	bm.a[endWord] &^= tailMask
}
