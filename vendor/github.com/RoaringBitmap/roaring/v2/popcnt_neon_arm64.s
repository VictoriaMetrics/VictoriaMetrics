// +build arm64,!appengine

//go:build arm64 && !appengine

#include "textflag.h"

// NEON (Advanced SIMD) population-count routines for arm64. They count the set
// bits across a []uint64 (the backing storage of a bitmap container), optionally
// combining each pair of words with a boolean op first: And, Or, Xor, and Mask
// (s &^ m). NEON is mandatory in the ARMv8-A baseline that every arm64 CPU
// implements, so unlike the amd64 AVX2 code there is no runtime feature check:
// these routines are always used on arm64 (see popcnt_neon_arm64.go).
//
// Algorithm (VCNT byte popcount + widening accumulation)
// ------------------------------------------------------
// arm64 has a dedicated per-byte popcount instruction, VCNT, which replaces each
// byte of a 128-bit register with the popcount (0..8) of the input byte. Turning
// those per-byte counts into a running total means widening and accumulating,
// and the loop is shaped to keep the arithmetic units busy:
//   - each iteration loads four 16-byte vectors (64 bytes = 8 words) and VCNTs
//     them independently, then sums the four with byte-wise VADD. Four counts of
//     at most 8 sum to at most 32, so no byte lane overflows.
//   - the summed bytes are folded into 16-bit lanes with add-wide: VUADDW takes
//     the low 8 bytes into partial accumulator V16 and VUADDW2 the high 8 into
//     V18. Two separate accumulators keep those adds off each other's
//     dependency chain. The obvious-looking alternative, accumulating pairs of
//     byte lanes straight into the 16-bit accumulator with UADALP, would save
//     an instruction per iteration but is a pessimisation in practice: it has
//     no Go assembler mnemonic, and it issues at roughly 2.4 per cycle on an
//     Apple M4 where VADD/VCNT/VUADDW all issue at 4 per cycle. Measured, it
//     costs this routine about 20%.
//   - a 16-bit lane would eventually overflow, so every INNERMAX iterations the
//     partials are drained (widened again) into a 4x32-bit accumulator (V17)
//     that cannot realistically overflow, and the partials are re-zeroed.
//   - at the end VUADDLV sums the four 32-bit lanes into a scalar.
// A scalar-width NEON tail (VCNT + VUADDLV on one 64-bit word at a time) mops up
// the trailing len%8 words, so any slice length is counted correctly.
//
// Go assembler conventions used below
// -----------------------------------
//   - Operands are written source(s) first, destination LAST. So
//     "VAND V4.B16, V0.B16, V0.B16" means V0 = V0 AND V4.
//   - Vn.B16/H8/H4/S4/D1 name the arrangement (element size x count) an
//     instruction operates on: B16 = 16 bytes, H8/H4 = 8/4 halfwords, S4 = 4
//     words, D1 = 1 doubleword. The same physical register is viewed either way.
//   - VLD1.P post-increments the pointer register by the number of bytes loaded.
//   - Arguments/results are read from the frame pointer (FP). A Go slice is a
//     3-word header {ptr,len,cap}: s_base+0(FP), s_len+8(FP); a second slice
//     argument starts at +24(FP). The uint64 result slot follows the args
//     (ret+24(FP) for one slice arg, ret+48(FP) for two).
//   - Every routine is a leaf (makes no calls): NOSPLIT with a $0 local frame.

// INNERMAX bounds how many 64-byte iterations fold into the 16-bit partial
// accumulators before they are drained into the wider one. Each iteration adds
// at most 32 (four byte-popcounts of at most 8) to a 16-bit lane, and
// 1024*32 = 32768 stays well under the 65535 lane limit.
#define INNERMAX $1024

// FOLD4 assumes 64 bytes of input (post-combine) sit in V0..V3 and folds their
// popcount into the partial accumulators V16/V18. VADD sums the four VCNT
// results byte-wise (each lane 0..32); VUADDW/VUADDW2 then widen the low/high
// halves into the two 16-bit accumulators.
#define FOLD4 \
	VCNT V0.B16, V0.B16 \
	VCNT V1.B16, V1.B16 \
	VCNT V2.B16, V2.B16 \
	VCNT V3.B16, V3.B16 \
	VADD V1.B16, V0.B16, V0.B16 \
	VADD V3.B16, V2.B16, V2.B16 \
	VADD V2.B16, V0.B16, V0.B16 \
	VUADDW V0.B8, V16.H8, V16.H8 \
	VUADDW2 V0.B16, V18.H8, V18.H8

// ZEROPART re-zeroes the two 16-bit partial accumulators at the start of each
// INNERMAX batch.
#define ZEROPART \
	VEOR V16.B16, V16.B16, V16.B16 \
	VEOR V18.B16, V18.B16, V18.B16

// DRAIN widens the 16-bit partials V16/V18 into the 32-bit accumulator V17
// (VUADDW low four halfwords, VUADDW2 high four, for each) and re-zeroes them.
#define DRAIN \
	VUADDW V16.H4, V17.S4, V17.S4 \
	VUADDW2 V16.H8, V17.S4, V17.S4 \
	VUADDW V18.H4, V17.S4, V17.S4 \
	VUADDW2 V18.H8, V17.S4, V17.S4 \
	ZEROPART

// REDUCE sums the four 32-bit lanes of V17 into a scalar and adds it to R2 (the
// running result). VUADDLV over .S4 yields a 64-bit sum; VMOV lifts it to a GPR.
#define REDUCE \
	VUADDLV V17.S4, V0 \
	VMOV V0.D[0], R4 \
	ADD R4, R2, R2

// TAILWORD popcounts the single 64-bit word already loaded into V0's low lane
// and adds it to R2: VCNT counts each of the 8 bytes, VUADDLV sums them.
#define TAILWORD \
	VCNT V0.B8, V0.B8 \
	VUADDLV V0.B8, V0 \
	VMOV V0.S[0], R4 \
	ADD R4, R2, R2

// func _popcntSliceNEON(s []uint64) uint64
// Returns the total number of set bits in s. This is the canonical routine; the
// And/Or/Xor/Mask variants below share its structure and differ only by the
// boolean op applied to the two inputs before counting.
TEXT ·_popcntSliceNEON(SB), NOSPLIT, $0-32
	MOVD s_base+0(FP), R0            // R0 = &s[0]
	MOVD s_len+8(FP), R1            // R1 = len(s), in 64-bit words
	MOVD $0, R2                    // R2 = running result
	VEOR V17.B16, V17.B16, V17.B16 // zero the 32-bit accumulator
	LSR $3, R1, R3                 // R3 = len/8 = number of 64-byte blocks
	CBZ R3, sltail                 // fewer than 8 words: skip the vector loop
slblock:
	MOVD INNERMAX, R4              // R4 = min(remaining blocks, INNERMAX)
	CMP R4, R3
	BHS slinner
	MOVD R3, R4
slinner:
	SUB R4, R3, R3                 // R3 -= this batch's block count
	ZEROPART
slloop:
	VLD1.P 64(R0), [V0.B16, V1.B16, V2.B16, V3.B16] // load 8 words (64 bytes)
	FOLD4                          // partials += popcount(those 64 bytes)
	SUBS $1, R4, R4
	BNE slloop
	DRAIN                          // fold partials into V17, re-zero them
	CBNZ R3, slblock               // more blocks remain
	REDUCE                         // R2 += sum of V17's lanes
sltail:
	AND $7, R1, R1                 // leftover words (0..7)
	CBZ R1, sldone
sltailloop:
	VLD1.P 8(R0), [V0.D1]          // load one word, advance R0 by 8
	TAILWORD
	SUBS $1, R1, R1
	BNE sltailloop
sldone:
	MOVD R2, ret+24(FP)
	RET

// func _popcntAndSliceNEON(s, m []uint64) uint64
// Returns the sum of popcount(s[i] & m[i]). Mirrors _popcntSliceNEON but loads
// four vectors from each of s and m and ANDs them before counting. s and m are
// assumed to have equal length.
TEXT ·_popcntAndSliceNEON(SB), NOSPLIT, $0-56
	MOVD s_base+0(FP), R0           // R0 = &s[0]
	MOVD m_base+24(FP), R1          // R1 = &m[0]
	MOVD s_len+8(FP), R5           // R5 = len
	MOVD $0, R2
	VEOR V17.B16, V17.B16, V17.B16
	LSR $3, R5, R3
	CBZ R3, andtail
andblock:
	MOVD INNERMAX, R4
	CMP R4, R3
	BHS andinner
	MOVD R3, R4
andinner:
	SUB R4, R3, R3
	ZEROPART
andloop:
	VLD1.P 64(R0), [V0.B16, V1.B16, V2.B16, V3.B16]
	VLD1.P 64(R1), [V4.B16, V5.B16, V6.B16, V7.B16]
	VAND V4.B16, V0.B16, V0.B16    // V0 = s & m
	VAND V5.B16, V1.B16, V1.B16
	VAND V6.B16, V2.B16, V2.B16
	VAND V7.B16, V3.B16, V3.B16
	FOLD4
	SUBS $1, R4, R4
	BNE andloop
	DRAIN
	CBNZ R3, andblock
	REDUCE
andtail:
	AND $7, R5, R5
	CBZ R5, anddone
andtailloop:
	VLD1.P 8(R0), [V0.D1]
	VLD1.P 8(R1), [V1.D1]
	VAND V1.B8, V0.B8, V0.B8       // s & m, one word
	TAILWORD
	SUBS $1, R5, R5
	BNE andtailloop
anddone:
	MOVD R2, ret+48(FP)            // +48: result follows two 24-byte slice headers
	RET

// func _popcntOrSliceNEON(s, m []uint64) uint64
// Returns the sum of popcount(s[i] | m[i]); see _popcntAndSliceNEON for the
// shared structure.
TEXT ·_popcntOrSliceNEON(SB), NOSPLIT, $0-56
	MOVD s_base+0(FP), R0
	MOVD m_base+24(FP), R1
	MOVD s_len+8(FP), R5
	MOVD $0, R2
	VEOR V17.B16, V17.B16, V17.B16
	LSR $3, R5, R3
	CBZ R3, ortail
orblock:
	MOVD INNERMAX, R4
	CMP R4, R3
	BHS orinner
	MOVD R3, R4
orinner:
	SUB R4, R3, R3
	ZEROPART
orloop:
	VLD1.P 64(R0), [V0.B16, V1.B16, V2.B16, V3.B16]
	VLD1.P 64(R1), [V4.B16, V5.B16, V6.B16, V7.B16]
	VORR V4.B16, V0.B16, V0.B16    // V0 = s | m
	VORR V5.B16, V1.B16, V1.B16
	VORR V6.B16, V2.B16, V2.B16
	VORR V7.B16, V3.B16, V3.B16
	FOLD4
	SUBS $1, R4, R4
	BNE orloop
	DRAIN
	CBNZ R3, orblock
	REDUCE
ortail:
	AND $7, R5, R5
	CBZ R5, ordone
ortailloop:
	VLD1.P 8(R0), [V0.D1]
	VLD1.P 8(R1), [V1.D1]
	VORR V1.B8, V0.B8, V0.B8       // s | m, one word
	TAILWORD
	SUBS $1, R5, R5
	BNE ortailloop
ordone:
	MOVD R2, ret+48(FP)
	RET

// func _popcntXorSliceNEON(s, m []uint64) uint64
// Returns the sum of popcount(s[i] ^ m[i]); see _popcntAndSliceNEON for the
// shared structure.
TEXT ·_popcntXorSliceNEON(SB), NOSPLIT, $0-56
	MOVD s_base+0(FP), R0
	MOVD m_base+24(FP), R1
	MOVD s_len+8(FP), R5
	MOVD $0, R2
	VEOR V17.B16, V17.B16, V17.B16
	LSR $3, R5, R3
	CBZ R3, xortail
xorblock:
	MOVD INNERMAX, R4
	CMP R4, R3
	BHS xorinner
	MOVD R3, R4
xorinner:
	SUB R4, R3, R3
	ZEROPART
xorloop:
	VLD1.P 64(R0), [V0.B16, V1.B16, V2.B16, V3.B16]
	VLD1.P 64(R1), [V4.B16, V5.B16, V6.B16, V7.B16]
	VEOR V4.B16, V0.B16, V0.B16    // V0 = s ^ m
	VEOR V5.B16, V1.B16, V1.B16
	VEOR V6.B16, V2.B16, V2.B16
	VEOR V7.B16, V3.B16, V3.B16
	FOLD4
	SUBS $1, R4, R4
	BNE xorloop
	DRAIN
	CBNZ R3, xorblock
	REDUCE
xortail:
	AND $7, R5, R5
	CBZ R5, xordone
xortailloop:
	VLD1.P 8(R0), [V0.D1]
	VLD1.P 8(R1), [V1.D1]
	VEOR V1.B8, V0.B8, V0.B8       // s ^ m, one word
	TAILWORD
	SUBS $1, R5, R5
	BNE xortailloop
xordone:
	MOVD R2, ret+48(FP)
	RET

// func _popcntMaskSliceNEON(s, m []uint64) uint64
// Returns the sum of popcount(s[i] &^ m[i]) == popcount(s & ~m). Same structure
// as _popcntAndSliceNEON, except that s &^ m is formed with a single instruction
// per vector instead of an invert-then-AND pair.
//
// arm64 does have a vector and-not, BIC, but the Go assembler exposes no
// mnemonic for it. VBIT fits just as well and is spelled. BIT ("bitwise insert
// if true") selects bit by bit between the destination and one source, under
// the control of the other source:
//
//	Vd<i> = Vm<i> ? Vn<i> : Vd<i>     i.e.  Vd = (Vm & Vn) | (^Vm & Vd)
//
// Holding Vn at zero reduces that to "clear every bit of Vd that is set in Vm",
// which is exactly Vd &^= Vm. So with Vd = s and Vm = m the mask is applied in
// place in one op. V15 -- which used to hold all-ones so that m could be
// inverted with VEOR -- is now simply kept at zero to serve as that Vn.
TEXT ·_popcntMaskSliceNEON(SB), NOSPLIT, $0-56
	MOVD s_base+0(FP), R0
	MOVD m_base+24(FP), R1
	MOVD s_len+8(FP), R5
	MOVD $0, R2
	VEOR V15.B16, V15.B16, V15.B16 // V15 = 0, the "insert" source for VBIT
	VEOR V17.B16, V17.B16, V17.B16
	LSR $3, R5, R3
	CBZ R3, masktail
maskblock:
	MOVD INNERMAX, R4
	CMP R4, R3
	BHS maskinner
	MOVD R3, R4
maskinner:
	SUB R4, R3, R3
	ZEROPART
maskloop:
	VLD1.P 64(R0), [V0.B16, V1.B16, V2.B16, V3.B16]
	VLD1.P 64(R1), [V4.B16, V5.B16, V6.B16, V7.B16]
	VBIT V4.B16, V15.B16, V0.B16   // V0 = s &^ m
	VBIT V5.B16, V15.B16, V1.B16
	VBIT V6.B16, V15.B16, V2.B16
	VBIT V7.B16, V15.B16, V3.B16
	FOLD4
	SUBS $1, R4, R4
	BNE maskloop
	DRAIN
	CBNZ R3, maskblock
	REDUCE
masktail:
	AND $7, R5, R5
	CBZ R5, maskdone
masktailloop:
	VLD1.P 8(R0), [V0.D1]
	VLD1.P 8(R1), [V1.D1]
	VBIT V1.B8, V15.B8, V0.B8      // s &^ m, one word
	TAILWORD
	SUBS $1, R5, R5
	BNE masktailloop
maskdone:
	MOVD R2, ret+48(FP)
	RET
