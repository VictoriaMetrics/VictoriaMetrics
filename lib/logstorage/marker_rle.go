package logstorage

import (
	"math/bits"
	"strconv"
	"strings"

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
//	1100   -> 0,2,2    (encoded 0,2,2)
func (bm *bitmap) MarshalRLE(dst []byte) []byte {
	if bm == nil || bm.bitsLen == 0 {
		return nil
	}

	zerosRun := true // we always start with a zeros run
	runLen := uint64(0)

	flush := func() {
		dst = encoding.MarshalVarUint64(dst, runLen)
		runLen = 0
		zerosRun = !zerosRun
	}

	words := bm.a
	fullWords := bm.bitsLen / 64
	tailBits := bm.bitsLen % 64

	for wi := range fullWords {
		w := words[wi]

		// fast-path whole-word runs
		if zerosRun && w == 0 {
			runLen += 64
			continue
		}
		if !zerosRun && w == ^uint64(0) {
			runLen += 64
			continue
		}

		// drill into the mixed word
		for wBits := 64; wBits > 0; {
			var step int
			if zerosRun {
				step = bits.TrailingZeros64(w)
			} else {
				step = trailingOnes64(w)
			}
			if step == 0 { // current bit flips
				flush()
				continue
			}
			if step > wBits { // when trailing* returns 64
				step = wBits
			}
			runLen += uint64(step)
			w >>= uint(step)
			wBits -= step
		}
	}

	// handle tail bits (if bitsLen not a multiple of 64)
	if tailBits > 0 {
		tail := words[fullWords] & ((uint64(1) << tailBits) - 1)
		for tb := tailBits; tb > 0; {
			var step int
			if zerosRun {
				step = bits.TrailingZeros64(tail)
			} else {
				step = trailingOnes64(tail)
			}
			if step == 0 {
				flush()
				continue
			}
			if step > tb {
				step = tb
			}
			runLen += uint64(step)
			tail >>= uint(step)
			tb -= step
		}
	}

	// flush the final run
	dst = encoding.MarshalVarUint64(dst, runLen)
	return dst
}

func trailingOnes64(x uint64) int {
	return bits.TrailingZeros64(^x)
}

// AndNotRLE performs dst &= ^rle, where rle is a bitmap encoded with MarshalRLE.
// In other words, it clears bits in dst that correspond to one-runs in the supplied RLE stream.
func (rle boolRLE) AndNotRLE(dst *bitmap) {
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

func (rle boolRLE) IsOnes(totalRows uint64) bool {
	zeros, n := encoding.UnmarshalVarUint64(rle)
	if zeros != 0 {
		return false
	}
	ones, _ := encoding.UnmarshalVarUint64(rle[n:])
	return ones >= totalRows
}

// clearBitsRange clears n bits starting from the given bit offset.
func (bm *bitmap) clearBitsRange(start, n int) {
	if n <= 0 || start >= bm.bitsLen {
		return
	}

	end := min(start+n, bm.bitsLen)

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

type boolRLE []byte

// String decodes the RLE stream (var-uint64-encoded run lengths of
// zero-runs and one-runs) and prints it as "[r0,r1,r2,…]".
// Debug only.
func (r boolRLE) String() string {
	if len(r) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')

	pos := 0
	first := true
	for pos < len(r) {
		run, n := encoding.UnmarshalVarUint64(r[pos:])
		if n == 0 {
			// malformed tail: give up on further decoding
			break
		}
		pos += n
		if !first {
			sb.WriteByte(',')
		}
		first = false
		sb.WriteString(strconv.FormatUint(run, 10))
	}
	sb.WriteByte(']')
	return sb.String()
}

func (rle boolRLE) SetAllOnes(count int) boolRLE {
	rle = make([]byte, 0, 16)
	rle = encoding.MarshalVarUint64(rle, 0)
	rle = encoding.MarshalVarUint64(rle, uint64(count))
	return rle
}

// func (rle boolRLE) UnionV1(other boolRLE) boolRLE {
// 	// Fast paths.
// 	if len(rle) == 0 {
// 		return append(boolRLE(nil), other...)
// 	}
// 	if len(other) == 0 {
// 		return append(boolRLE(nil), rle...)
// 	}

// 	totalRows := totalRowsInRLE(rle)
// 	if tr := totalRowsInRLE(other); tr > totalRows {
// 		totalRows = tr
// 	}

// 	// Initialize bitmap large enough to hold the union of both inputs.
// 	bm := getBitmap(totalRows)
// 	bm.init(totalRows)

// 	// Helper that sets bits in bm according to the supplied RLE.
// 	decodeIntoBitmap := func(src boolRLE) {
// 		idx := 0
// 		pos := 0
// 		isOnesRun := false // first run is zeros
// 		for idx < len(src) {
// 			run, n := encoding.UnmarshalVarUint64(src[idx:])
// 			idx += n
// 			if isOnesRun && run > 0 {
// 				// Set bits [pos, pos+run)
// 				for i := 0; i < int(run); i++ {
// 					bm.setBit(pos + i)
// 				}
// 			}
// 			pos += int(run)
// 			isOnesRun = !isOnesRun
// 		}
// 	}

// 	decodeIntoBitmap(rle)
// 	decodeIntoBitmap(other)

// 	// Encode back to RLE and release the bitmap.
// 	result := bm.MarshalRLE(nil)
// 	putBitmap(bm)
// 	return result
// }

// // totalRowsInRLE returns the total number of rows represented by the RLE stream.
// func totalRowsInRLE(rle boolRLE) int {
// 	idx := 0
// 	total := 0
// 	for idx < len(rle) {
// 		run, n := encoding.UnmarshalVarUint64(rle[idx:])
// 		idx += n
// 		total += int(run)
// 	}
// 	return total
// }

// Union returns an RLE-encoded bitmap equal to the bit-wise OR of rle and other.
// It walks the two RLE streams in lock-step, so it never allocates an
// intermediate bitmap.  The first run in every RLE stream is zeros.
func (rle boolRLE) Union(other boolRLE) boolRLE {
	// Fast paths – nothing to merge.
	if len(rle) == 0 {
		return append(boolRLE(nil), other...)
	}
	if len(other) == 0 {
		return append(boolRLE(nil), rle...)
	}

	// Decoder tracks the remaining rows in the current run and its type.
	type decoder struct {
		src    boolRLE
		idx    int    // offset in src
		rem    uint64 // rows left in current run
		isOnes bool   // current run type (false = zeros)
	}

	// Initialise decoders (first run is zeros).
	d1 := decoder{src: rle}
	d2 := decoder{src: other}

	// step pulls the next non-empty run into d.rem / d.isOnes.
	step := func(d *decoder) {
		for d.rem == 0 && d.idx < len(d.src) {
			run, n := encoding.UnmarshalVarUint64(d.src[d.idx:])
			d.idx += n
			d.rem = run
			if d.rem == 0 {
				// Empty run – just toggle type and continue.
				d.isOnes = !d.isOnes
				continue
			}
			// Non-empty run loaded; keep current type.
		}
	}

	// Output builder collects run lengths before varint-encoding.
	outRuns := make([]uint64, 0, 8)
	outIsOnes := false // first run is zeros
	outLen := uint64(0)

	flush := func() {
		outRuns = append(outRuns, outLen)
		outLen = 0
	}

	for {
		step(&d1)
		step(&d2)

		// Both streams exhausted?
		if d1.rem == 0 && d2.rem == 0 {
			break
		}

		// Span length = min(non-zero rems).
		var span uint64
		switch {
		case d1.rem == 0:
			span = d2.rem
		case d2.rem == 0:
			span = d1.rem
		case d1.rem < d2.rem:
			span = d1.rem
		default:
			span = d2.rem
		}

		// Union bit for this span.
		spanOnes := d1.isOnes || d2.isOnes

		// Emit or extend run in output.
		if spanOnes == outIsOnes {
			outLen += span
		} else {
			if outLen > 0 {
				flush()
			}
			outIsOnes = spanOnes
			outLen = span
		}

		// Advance decoders.
		if d1.rem >= span {
			d1.rem -= span
			if d1.rem == 0 {
				d1.isOnes = !d1.isOnes
			}
		}
		if d2.rem >= span {
			d2.rem -= span
			if d2.rem == 0 {
				d2.isOnes = !d2.isOnes
			}
		}
	}

	flush()

	// Drop trailing zero-run (saves bytes, matches existing style).
	if len(outRuns) > 0 && !outIsOnes && outRuns[len(outRuns)-1] == 0 {
		outRuns = outRuns[:len(outRuns)-1]
	}

	// Varint-encode result.
	var dst boolRLE
	for _, l := range outRuns {
		dst = encoding.MarshalVarUint64(dst, l)
	}
	return dst
}

// ForEachZeroBit calls f(idx) for every bit that is 0 (i.e. not set) in the RLE-encoded bitmap.
// It iterates in ascending order of idx and never allocates.
// totalRows must be the number of rows the bitmap applies to – this is needed
// to iterate possible trailing zeros when the RLE stream ends earlier.
func (rle boolRLE) ForEachZeroBit(totalRows int, f func(idx int)) {
	if totalRows <= 0 {
		return
	}

	pos := 0 // current row index
	idx := 0 // byte offset inside rle

	for pos < totalRows && idx < len(rle) {
		// zeros-run length
		zeros, n := encoding.UnmarshalVarUint64(rle[idx:])
		idx += n
		for i := 0; i < int(zeros) && pos < totalRows; i++ {
			f(pos)
			pos++
		}

		if pos >= totalRows || idx >= len(rle) {
			break
		}

		// ones-run length – skip these rows
		ones, n := encoding.UnmarshalVarUint64(rle[idx:])
		idx += n
		pos += int(ones)
	}

	// Handle tail zeros if any rows left after RLE stream.
	for pos < totalRows {
		f(pos)
		pos++
	}
}
