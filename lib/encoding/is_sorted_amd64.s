#include "textflag.h"

TEXT ·IsInt64ArraySorted(SB), NOSPLIT | NOFRAME, $0-25
    // frame length:  0
    // param length: 24 bytes
    // return value length: 1 bytes
    MOVQ inPtr+0(FP), R8  // current
    MOVQ inLen+8(FP), R9  // count
    // check params
    CMPQ R9, $2
    JLT sorted  // if count<2 then goto sorted
    // variables
    MOVQ R9, AX
    SHLQ $3, AX  // totalBytes = count<<3
    ADDQ R8, AX  // end = current + totalBytes
    ADDQ $8, R8  // current += 8
    SUBQ $1, R9  // count -= 1
    MOVQ R9, R10
    ANDQ $-4, R10  // align_count = count & -4
    LEAQ (R8)(R10*8), R11  // align_4_end = current + align_count*8
align_4:
    CMPQ R8, R11  // if current==align_4_end then goto label_align_4_end
    JE align_4_end
    VMOVDQU -8(R8), Y1  // load_u, 256 bit, 4 x int64
    VMOVDQU (R8), Y2  // load_u, 256 bit, 4 x int64
    ADDQ  $32, R8  // current += 32
    VPCMPGTQ Y2, Y1, Y3  // y3 = y1 > y2
    VMOVMSKPD Y3, R13  // move mask
    CMPQ R13, $0  // if mask==0 then goto align_4
    JE align_4
    // not sorted
    MOVB $0, ret+24(FP)  // return 0
    VZEROUPPER
    RET
align_4_end:
    MOVQ -8(R8), BX  // bx = prev
align_1:
    CMPQ R8, AX
    JEQ sorted  // if current==end then goto end
    MOVQ (R8), R13 // r13 = *current
    ADDQ $8, R8  // current += 8
    MOVQ BX, R10  // r10 = prev
    MOVQ R13, BX   // prev = *current
    CMPQ R13, R10  // if *current>=prev then goto align_1
    JGE align_1
not_sorted:
    MOVB $0, ret+24(FP)  // return 0
    VZEROUPPER
    RET
sorted:
    MOVB $1, ret+24(FP)  // return 1
    VZEROUPPER
    RET
