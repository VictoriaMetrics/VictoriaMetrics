// +build amd64,!appengine

//go:build amd64 && !appengine

#include "textflag.h"

// AVX2 population-count routines for amd64. They count the set bits across a
// []uint64 (the backing storage of a bitmap container), optionally combining
// each pair of words with a boolean op first: And, Or, Xor, and Mask (s &^ m).
//
// Algorithm (Mula/Lemire VPSHUFB nibble lookup)
// ---------------------------------------------
// AVX2 has no single "popcount a whole vector" instruction, so each byte's
// popcount is taken from a 16-entry lookup table indexed by a 4-bit nibble:
// a byte is split into its low and high nibble, each nibble is looked up (one
// VPSHUFB performs all 32 lookups in a 256-bit register at once), and VPSADBW
// then combines the two results and sums each group of 8 byte-counts into a
// 64-bit lane total that is accumulated (COUNTBLOCK).
// After the loop the four lane totals are summed (HSUM) into a scalar register.
// Each iteration handles 256 bits (4 uint64); a scalar POPCNTQ tail handles
// the trailing len%4 words, so any slice length is counted correctly.
//
// Go assembler conventions used below
// -----------------------------------
//   - Operands are written source(s) first, destination LAST. So
//     "VPAND Ymask, Ydata, Ylo" means Ylo = Ydata AND Ymask.
//   - Yn are the 256-bit AVX registers; Xn aliases the low 128 bits of Yn.
//   - Arguments/results are read from the frame pointer (FP). A Go slice is a
//     3-word header {ptr,len,cap}: s_base+0(FP), s_len+8(FP); a second slice
//     argument starts at +24(FP). The uint64 result slot follows the args
//     (e.g. ret+24(FP) for one slice arg, ret+48(FP) for two).
//   - Every routine is a leaf (makes no calls): NOSPLIT with a $0 local frame.
//   - Loads/stores use VMOVDQU (unaligned): container slices are only 8-byte
//     aligned, not 32. Generic (VEX-encoded) AVX instructions impose no
//     alignment requirement on a memory source either, so the second input of
//     the two-operand loops is read straight out of memory by VPAND/VPOR/
//     VPXOR/VPANDN instead of being loaded into a register first.
//     VZEROUPPER precedes every RET to avoid the AVX<->SSE transition penalty
//     in any non-VEX SSE code that runs afterwards.

// lutmask is a 17-byte read-only blob (the linker pads it out to whatever its
// alignment requires) holding the two constants used by every routine:
//   bytes  0..15 - the nibble popcount table, i.e. table[i] = number of set
//                  bits in the 4-bit value i. Read low-byte-first, the first
//                  qword 0x0302020102010100 is the bytes {0,1,1,2,1,2,2,3} for
//                  nibbles 0..7, and 0x0403030203020201 is {1,2,2,3,2,3,3,4}
//                  for nibbles 8..15. VPSHUFB indexes within each 128-bit lane
//                  independently and so needs the table in both lanes, but
//                  VBROADCASTI128 duplicates the 16 bytes at load time; only
//                  one copy has to be stored.
//   byte     16  - 0x0F, the mask that isolates the low nibble of each byte,
//                  splatted to all 32 bytes by VPBROADCASTB.
// RODATA|NOPTR marks it read-only and pointer-free (so the GC ignores it).
DATA lutmask<>+0(SB)/8, $0x0302020102010100
DATA lutmask<>+8(SB)/8, $0x0403030203020201
DATA lutmask<>+16(SB)/1, $0x0f
GLOBL lutmask<>(SB), RODATA|NOPTR, $17

// Register aliases. Ylut1/Ylut2/Ymask are constants set up once per call (see
// SETUP); Yacc is the running accumulator of lane totals; Ydata holds the
// current input vector; Ylo/Yhi are scratch used by COUNTBLOCK. Ydata is dead
// once its nibbles have been extracted, so Yhi shares its register: only five
// architectural registers are needed.
#define Ylut1 Y0
#define Ylut2 Y1
#define Ymask Y2
#define Yacc Y3
#define Ydata Y4
#define Yhi Y4
#define Ylo Y5

// Low 128-bit halves of Yacc and Ylo, used as scratch by HSUM.
#define Xacc X3
#define Xtmp X5

// COUNTBLOCK folds the popcount of the 32 bytes currently in Ydata into the
// accumulator Yacc. Line by line:
//   VPAND  Ymask,Ydata,Ylo   : Ylo = low nibble of every byte
//   VPSRLW $4,Ydata,Yhi      : shift each 16-bit lane right by 4...
//   VPAND  Ymask,Yhi,Yhi     : ...then mask, leaving the high nibble of each byte
//   VPSHUFB Ylo,Ylut1,Ylo    : Ylo[b] = B + popcount(low nibble of byte b)
//   VPSHUFB Yhi,Ylut2,Yhi    : Yhi[b] = B - popcount(high nibble of byte b)
//   VPSADBW Ylo,Yhi,Ylo      : sum each group of 8 bytes -> 4 lane totals
//   VPADDQ  Ylo,Yacc,Yacc    : add the 4 lane totals into the accumulator
//
// VPSADBW computes |a-b| per byte and sums each group of 8, so it can do the
// work of the per-byte add as well: feeding it the two nibble counts directly
// yields (B + lo) - (B - hi) = lo + hi, and the separate VPADDB the naive
// version needs (with a zero second VPSADBW operand) disappears along with its
// latency. That is why SETUP builds two tables, one biased up by B and one
// subtracted from B. The bias must satisfy 4 <= B <= 251 so that neither
// table's entries (max nibble popcount is 4) wrap around as unsigned bytes and
// so that a >= b always holds, making the absolute value a no-op; B = 15 is
// used simply because Ymask already holds 15 in every byte.
//
// Per-byte counts max at 8 and lane totals at 512, so accumulating across the
// whole loop never overflows the 64-bit lanes.
#define COUNTBLOCK \
	VPAND Ymask, Ydata, Ylo \
	VPSRLW $4, Ydata, Yhi \
	VPAND Ymask, Yhi, Yhi \
	VPSHUFB Ylo, Ylut1, Ylo \
	VPSHUFB Yhi, Ylut2, Yhi \
	VPSADBW Ylo, Yhi, Ylo \
	VPADDQ Ylo, Yacc, Yacc

// SETUP builds the two biased lookup tables and the nibble mask, and zeroes
// Yacc (the accumulator). Run once per routine, after the check that the
// vector loop runs at least one iteration. Ylut1 first holds the raw table, so
// the VPSUBB must come before the VPADDB that overwrites it.
#define SETUP \
	VBROADCASTI128 lutmask<>+0(SB), Ylut1 \
	VPBROADCASTB lutmask<>+16(SB), Ymask \
	VPXOR Yacc, Yacc, Yacc \
	VPSUBB Ylut1, Ymask, Ylut2 \
	VPADDB Ylut1, Ymask, Ylut1

// HSUM reduces Yacc's four 64-bit lane totals to a single sum in AX.
// VEXTRACTI128 pulls Yacc's high 128 bits into Xtmp and the two halves are
// added, leaving two qwords in Xacc; VPSHUFD $0x4e then swaps those two qwords
// so a second VPADDQ puts their total in the low qword, which one MOVQ moves
// out. Finishing the reduction in SIMD avoids VPEXTRQ, which is 2 uops on both
// AMD Zen and Intel, against 1 each for VPSHUFD and VPADDQ.
#define HSUM \
	VEXTRACTI128 $1, Yacc, Xtmp \
	VPADDQ Xtmp, Xacc, Xacc \
	VPSHUFD $0x4e, Xacc, Xtmp \
	VPADDQ Xtmp, Xacc, Xacc \
	MOVQ Xacc, DX \
	ADDQ DX, AX

// func _popcntSliceAVX2(s []uint64) uint64
// Returns the total number of set bits in s. This is the canonical routine;
// the And/Or/Xor/Mask variants below share its structure and differ only by
// the boolean op applied before counting.
TEXT ·_popcntSliceAVX2(SB), NOSPLIT, $0-32
	MOVQ s_base+0(FP), SI   // SI = &s[0]
	MOVQ s_len+8(FP), CX    // CX = len(s), in 64-bit words
	XORL AX, AX             // AX = running result
	MOVQ CX, R8
	SHRQ $2, R8             // R8 = len/4 = number of full 256-bit blocks
	JZ slicetail            // fewer than 4 words: skip the vector loop
	SETUP                   // load tables/mask; zero Yacc
sliceloop:
	VMOVDQU (SI), Ydata     // load 4 words (32 bytes)
	COUNTBLOCK              // Yacc += popcount(those 32 bytes)
	ADDQ $32, SI            // advance to the next block
	DECQ R8
	JNZ sliceloop
	HSUM                    // AX += sum of Yacc's lane totals
slicetail:
	ANDL $3, CX             // CX = len % 4 = leftover words (0..3)
	JZ slicedone
slicetailloop:
	POPCNTQ (SI), DX        // scalar popcount of one word
	ADDQ DX, AX
	ADDQ $8, SI
	DECL CX
	JNZ slicetailloop
slicedone:
	VZEROUPPER              // clear upper YMM state before returning
	MOVQ AX, ret+24(FP)     // return AX
	RET

// func _popcntAndSliceAVX2(s, m []uint64) uint64
// Returns the sum of popcount(s[i] & m[i]). Mirrors _popcntSliceAVX2 but ANDs
// a vector of s with the matching bytes of m before counting. s and m are
// assumed to have equal length.
TEXT ·_popcntAndSliceAVX2(SB), NOSPLIT, $0-56
	MOVQ s_base+0(FP), SI    // SI = &s[0]
	MOVQ m_base+24(FP), DI   // DI = &m[0]
	MOVQ s_len+8(FP), CX     // CX = len
	XORL AX, AX
	MOVQ CX, R8
	SHRQ $2, R8
	JZ andtail
	SETUP
andloop:
	VMOVDQU (SI), Ydata
	VPAND (DI), Ydata, Ydata // Ydata = s & m
	COUNTBLOCK
	ADDQ $32, SI
	ADDQ $32, DI
	DECQ R8
	JNZ andloop
	HSUM
andtail:
	ANDL $3, CX
	JZ anddone
andtailloop:
	MOVQ (SI), DX
	ANDQ (DI), DX            // s & m, one word
	POPCNTQ DX, DX
	ADDQ DX, AX
	ADDQ $8, SI
	ADDQ $8, DI
	DECL CX
	JNZ andtailloop
anddone:
	VZEROUPPER
	MOVQ AX, ret+48(FP)      // +48: result follows two 24-byte slice headers
	RET

// func _popcntOrSliceAVX2(s, m []uint64) uint64
// Returns the sum of popcount(s[i] | m[i]); see _popcntAndSliceAVX2 for the
// shared structure.
TEXT ·_popcntOrSliceAVX2(SB), NOSPLIT, $0-56
	MOVQ s_base+0(FP), SI
	MOVQ m_base+24(FP), DI
	MOVQ s_len+8(FP), CX
	XORL AX, AX
	MOVQ CX, R8
	SHRQ $2, R8
	JZ ortail
	SETUP
orloop:
	VMOVDQU (SI), Ydata
	VPOR (DI), Ydata, Ydata  // Ydata = s | m
	COUNTBLOCK
	ADDQ $32, SI
	ADDQ $32, DI
	DECQ R8
	JNZ orloop
	HSUM
ortail:
	ANDL $3, CX
	JZ ordone
ortailloop:
	MOVQ (SI), DX
	ORQ (DI), DX             // s | m, one word
	POPCNTQ DX, DX
	ADDQ DX, AX
	ADDQ $8, SI
	ADDQ $8, DI
	DECL CX
	JNZ ortailloop
ordone:
	VZEROUPPER
	MOVQ AX, ret+48(FP)
	RET

// func _popcntXorSliceAVX2(s, m []uint64) uint64
// Returns the sum of popcount(s[i] ^ m[i]); see _popcntAndSliceAVX2 for the
// shared structure.
TEXT ·_popcntXorSliceAVX2(SB), NOSPLIT, $0-56
	MOVQ s_base+0(FP), SI
	MOVQ m_base+24(FP), DI
	MOVQ s_len+8(FP), CX
	XORL AX, AX
	MOVQ CX, R8
	SHRQ $2, R8
	JZ xortail
	SETUP
xorloop:
	VMOVDQU (SI), Ydata
	VPXOR (DI), Ydata, Ydata // Ydata = s ^ m
	COUNTBLOCK
	ADDQ $32, SI
	ADDQ $32, DI
	DECQ R8
	JNZ xorloop
	HSUM
xortail:
	ANDL $3, CX
	JZ xordone
xortailloop:
	MOVQ (SI), DX
	XORQ (DI), DX            // s ^ m, one word
	POPCNTQ DX, DX
	ADDQ DX, AX
	ADDQ $8, SI
	ADDQ $8, DI
	DECL CX
	JNZ xortailloop
xordone:
	VZEROUPPER
	MOVQ AX, ret+48(FP)
	RET

// func _popcntMaskSliceAVX2(s, m []uint64) uint64
// Returns the sum of popcount(s[i] &^ m[i]) == popcount(s & ~m). Same structure
// as _popcntAndSliceAVX2; the combine is VPANDN, which computes (NOT first) AND
// second. Only VPANDN's second source may come from memory, and it is the
// operand that is *not* negated, so here it is m that is loaded into a register
// and s that is read straight out of memory:
// "VPANDN (SI), Ydata, Ydata" with Ydata = m gives (NOT m) AND s = s &^ m.
TEXT ·_popcntMaskSliceAVX2(SB), NOSPLIT, $0-56
	MOVQ s_base+0(FP), SI
	MOVQ m_base+24(FP), DI
	MOVQ s_len+8(FP), CX
	XORL AX, AX
	MOVQ CX, R8
	SHRQ $2, R8
	JZ masktail
	SETUP
maskloop:
	VMOVDQU (DI), Ydata        // Ydata = m
	VPANDN (SI), Ydata, Ydata  // Ydata = s &^ m  (= (NOT m) AND s)
	COUNTBLOCK
	ADDQ $32, SI
	ADDQ $32, DI
	DECQ R8
	JNZ maskloop
	HSUM
masktail:
	ANDL $3, CX
	JZ maskdone
masktailloop:
	MOVQ (DI), R10
	NOTQ R10                 // ~m
	MOVQ (SI), DX
	ANDQ R10, DX             // s &^ m = s & ~m, one word
	POPCNTQ DX, DX
	ADDQ DX, AX
	ADDQ $8, SI
	ADDQ $8, DI
	DECL CX
	JNZ masktailloop
maskdone:
	VZEROUPPER
	MOVQ AX, ret+48(FP)
	RET

// func _hasAVX2() bool
// Reports whether the CPU supports AVX2 and the OS has enabled the wide (YMM)
// register state. All three checks must pass; otherwise the Go wrappers fall
// back to the scalar implementation. Note CPUID clobbers AX/BX/CX/DX.
//
// Each check complements the feature word and then TESTs the bits of interest:
// ZF is set exactly when every required bit was set in the original value. That
// needs only one large immediate instead of the two an AND/CMP pair would
// encode, and it lets all three checks converge on a single SETEQ, which stores
// the final ZF straight into the bool result. Since this runs once per process,
// code size matters more here than the (negligible) speed difference.
TEXT ·_hasAVX2(SB), NOSPLIT, $0-1
	// CPUID leaf 1: require OSXSAVE (ECX bit 27) and AVX (ECX bit 28).
	MOVL $1, AX
	XORL CX, CX
	CPUID
	NOTL CX
	TESTL $0x18000000, CX
	JNE noavx2
	// XGETBV(0): the OS must have enabled saving of SSE and AVX/YMM state, i.e.
	// XCR0 bits 1 and 2. Without this the YMM registers would be corrupted
	// across a context switch even though the CPU supports the instructions.
	XORL CX, CX
	XGETBV
	NOTL AX
	TESTL $0x6, AX
	JNE noavx2
	// CPUID leaf 7, sub-leaf 0: require AVX2 itself (EBX bit 5). The sub-leaf is
	// selected via ECX, which must be 0.
	MOVL $7, AX
	XORL CX, CX
	CPUID
	NOTL BX
	TESTL $0x20, BX
noavx2:
	SETEQ ret+0(FP)          // ZF is still set by whichever TESTL ran last
	RET
