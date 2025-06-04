#include "textflag.h"

TEXT ·IsDeltaConst(SB), NOSPLIT | NOFRAME, $0-25
    // frame length:  0
    // param length: 24 bytes
    // return value: 1 bytes
    MOVQ inPtr+0(FP), R8  // current
    MOVQ inLen+8(FP), R9  // count
    // check params
    CMPQ R9, $2
    JLT not_equal  // if count<2 then goto not_equal
    // variables
    MOVQ R9, AX
    SHLQ $3, AX  // totalBytes = count<<3
    ADDQ R8, AX  // end = current + totalBytes
    MOVQ (R8), R12  // first_value = *current
    MOVQ R12, BX
    ADDQ $8, R8  // current += 8
    SUBQ $1, R9  // count -= 1
    MOVQ (R8), DX  // dx = second_value
    SUBQ R12, DX  // delta = second_value - first_value
    MOVQ R9, R10
    ANDQ $-4, R10  // align_count = count & -4
    LEAQ (R8)(R10*8), R11  // align_4_end = current + align_count*8
    VPBROADCASTQ DX, Y0  // y0 = [delta,delta,delta,delta]
align_4:
    CMPQ R8, R11  // if current==align_4_end then goto label_align_4_end
    JE align_4_end
    VMOVDQU -8(R8), Y1  // load_u, 256 bit, 4 x int64
    VMOVDQU (R8), Y2  // load_u, 256 bit, 4 x int64
    ADDQ  $32, R8  // current += 32
    VPSUBQ Y1, Y2, Y3  // y3 = y2 - y1
    VPCMPEQQ Y3, Y0, Y4  // y4 = y3==y0
    VMOVMSKPD Y4, R13  // move mask
    CMPQ R13, $15  // if mask==15 then goto align_4
    JE align_4
    // not equal
    MOVB $0, ret+24(FP)  // return 0
    VZEROUPPER
    RET
align_4_end:
    MOVQ -8(R8), BX  // bx = prev
align_1:
    CMPQ R8, AX
    JEQ end  // if current==end then goto end
    MOVQ (R8), R13 // r13 = *current
    MOVQ R13, CX  // cx = *current
    ADDQ $8, R8  // current += 8
    SUBQ BX, R13  // delta = *current - prev
    MOVQ CX, BX   // prev = *current
    CMPQ DX ,R13
    JEQ align_1
not_equal:
    MOVB $0, ret+24(FP)  // return 0
    VZEROUPPER
    RET
end:
    MOVB $1, ret+24(FP)  // return 1
    VZEROUPPER
    RET
