package logstorage

import (
	"math/bits"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

// MarshalBoolRLE encodes the given bitmap into run-length encoding and appends the
// resulting bytes to dst. The format starts with the length of the initial run
// of zero bits and then alternates zero-runs and one-runs until the end of the
// bitmap is reached. All run lengths are encoded with variable-length unsigned
// integers from the encoding package.
//
// Examples:
//
//	000111 -> 3,3      (encoded as VarUInt64 3,3)
//	1100   -> 0,2,2    (encoded 0,2,2)
func (bm *bitmap) MarshalBoolRLE(dst []byte) boolRLE {
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

// IsStateful returns true if the bitmap encoded in rle
// contains two or more 1-bits.
//
// It walks the RLE stream once and exits as soon as the second
// one-bit is found, so it is O(#runs) with early termination.
// IsStateful reports whether the RLE slice contains
// at least two encoded run-lengths.
func (rle boolRLE) IsStateful() bool {
	if len(rle) == 0 {
		return false
	}

	_, n := encoding.UnmarshalVarUint64(rle)
	if n <= 0 || n >= len(rle) {
		return false
	}
	return true
}

func (rle boolRLE) IsSubsetOf(other boolRLE) bool {
	// Fast paths --------------------------------------------------------
	if len(rle) == 0 { // empty bitmap is subset of anything
		return true
	}
	if len(other) == 0 { // other is all-zero ⇒ rle must have no 1s
		return !rle.containsOne()
	}

	// Decoder for a single RLE stream ----------------------------------
	type decoder struct {
		src  boolRLE
		idx  int    // read offset in src
		rem  uint64 // rows left in current run
		ones bool   // type of current run (false = zeros)
	}
	load := func(d *decoder) {
		for d.rem == 0 && d.idx < len(d.src) {
			run, n := encoding.UnmarshalVarUint64(d.src[d.idx:])
			d.idx += n
			if run == 0 {
				// Zero-length run: flip run type and continue reading.
				d.ones = !d.ones
				continue
			}
			d.rem = run
			// NOTE: do *not* flip d.ones here; it already describes this run.
		}
	}

	var a, b decoder
	a.src = rle
	b.src = other

	for {
		load(&a)
		load(&b)

		// a finished → every 1-bit matched ⇒ subset
		if a.rem == 0 && a.idx >= len(a.src) {
			return true
		}

		// b finished → remaining bits are zeros
		if b.rem == 0 && b.idx >= len(b.src) {
			// If a still has any 1s, not a subset
			return !(a.ones && a.rem > 0)
		}

		// Determine how many rows we can consume in this step.
		var span uint64
		switch {
		case a.rem == 0:
			span = b.rem
		case b.rem == 0:
			span = a.rem
		case a.rem < b.rem:
			span = a.rem
		default:
			span = b.rem
		}

		// If this span has 1s in a and 0s in b → violation.
		if a.ones && !b.ones && span > 0 {
			return false
		}

		// Consume span from both streams.
		if a.rem >= span {
			a.rem -= span
			if a.rem == 0 {
				a.ones = !a.ones
			}
		}
		if b.rem >= span {
			b.rem -= span
			if b.rem == 0 {
				b.ones = !b.ones
			}
		}
	}
}

// containsOne is an O(#runs) helper: true if bitmap has any 1-bit.
func (rle boolRLE) containsOne() bool {
	idx := 0
	ones := false
	for idx < len(rle) {
		run, n := encoding.UnmarshalVarUint64(rle[idx:])
		idx += n
		if ones && run > 0 {
			return true
		}
		ones = !ones
	}
	return false
}

// Union returns an RLE-encoded bitmap equal to the bit-wise OR of rle and other.
// It walks the two RLE streams in lock-step, so it never allocates an
// intermediate bitmap.  The first run in every RLE stream is zeros.
func (rle boolRLE) Union(other boolRLE) boolRLE {
	// Fast paths.
	if len(rle) == 0 {
		return append(boolRLE(nil), other...)
	}
	if len(other) == 0 {
		return append(boolRLE(nil), rle...)
	}

	// Decoder state for one stream.
	type decoder struct {
		src   boolRLE
		idx   int    // byte offset
		pos   int    // absolute bit position at start of current run
		n     uint64 // rows left in current run
		isOne bool   // current run type (false = zeros)
	}

	// Advance d to the next non-empty run.
	step := func(d *decoder) {
		for d.idx < len(d.src) && d.n == 0 {
			run, n := encoding.UnmarshalVarUint64(d.src[d.idx:])
			d.idx += n
			d.pos += int(run)
			d.n = run
			if d.n == 0 {
				d.isOne = !d.isOne // zero-length run, just flip type
				continue
			}
			return
		}
	}

	var a, b decoder
	a.src = rle
	b.src = other
	step(&a)
	step(&b)

	// Output builder.
	outRuns := make([]uint64, 0, 8)
	outIsOne := false // first run type (zeros)
	curLen := uint64(0)

	flush := func() {
		outRuns = append(outRuns, curLen)
		curLen = 0
	}

	const inf = ^uint64(0) >> 1

	for {
		// Finished?
		if (a.n == 0 && a.idx >= len(a.src)) &&
			(b.n == 0 && b.idx >= len(b.src)) {
			break
		}

		// Remaining run lengths (∞ once a stream is exhausted).
		nA := a.n
		if nA == 0 && a.idx >= len(a.src) {
			nA = inf
		}
		nB := b.n
		if nB == 0 && b.idx >= len(b.src) {
			nB = inf
		}
		span := nA
		if nB < span {
			span = nB
		}
		if span == inf { // both exhausted
			break
		}

		spanOnes := (a.isOne && a.n > 0) || (b.isOne && b.n > 0)

		if spanOnes == outIsOne {
			// Same run type – extend.
			curLen += span
		} else {
			// Run type flips.
			if curLen > 0 || len(outRuns) > 0 {
				flush()
			} else if curLen == 0 && len(outRuns) == 0 {
				// First span is a 1-run: emit leading zero-run of length 0.
				outRuns = append(outRuns, 0)
			}
			outIsOne = spanOnes
			curLen = span
		}

		// Consume span from stream A.
		if a.n >= span {
			a.n -= span
			if a.n == 0 {
				a.isOne = !a.isOne
				step(&a)
			}
		}
		// Consume span from stream B.
		if b.n >= span {
			b.n -= span
			if b.n == 0 {
				b.isOne = !b.isOne
				step(&b)
			}
		}
	}

	flush()

	// Drop trailing zero-run if present (length-saving convention).
	if len(outRuns) > 0 && !outIsOne && outRuns[len(outRuns)-1] == 0 {
		outRuns = outRuns[:len(outRuns)-1]
	}

	// Varint-encode result.
	var dst boolRLE
	for _, rl := range outRuns {
		dst = encoding.MarshalVarUint64(dst, rl)
	}
	return dst
}

// CountOnes returns the total number of 1‑bits encoded in the RLE stream.
func (rle boolRLE) CountOnes() uint64 {
	if len(rle) == 0 {
		return 0
	}

	var (
		idx   int    // read offset in rle
		ones  bool   // current run type; false = zeros, true = ones
		total uint64 // accumulated 1‑bits
	)

	for idx < len(rle) {
		run, n := encoding.UnmarshalVarUint64(rle[idx:])
		idx += n
		if run == 0 { // explicit run‑type flip, no bits to count
			ones = !ones
			continue
		}
		if ones {
			total += run
		}
		ones = !ones
	}

	return total
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
