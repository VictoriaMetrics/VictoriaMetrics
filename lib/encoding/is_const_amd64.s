#include "textflag.h"

TEXT ·IsConst(SB), NOSPLIT | NOFRAME, $0-25
    // frame length:  0
    // param length: 24 bytes
    // return value length: 1 bytes
    MOVQ inPtr+0(FP), R8  // current
    MOVQ inLen+8(FP), R9  // int64 count
    // check param
    CMPQ R9, $0
    JE not_equal
    // variables
    MOVQ R9, R10
    ANDQ $-4, R10  // align_len = count & -4
    LEAQ (R8)(R10*8), R11  // align_4_end = in + align_len*8
    MOVQ (R8), R12  // first_value = *current
    CMPQ R9, $4
    JLT align_4_end
    VPBROADCASTQ R12, Y1  // Y1 = [first_value,first_value,first_value,first_value]
align_4:
    CMPQ R8, R11  // if current==align_4_end then goto label_align_4_end
    JE align_4_end
    VMOVDQU (R8), Y0  // load_u, 256 bit, 4 x int64
    ADDQ  $32, R8  // current += 32
    VPCMPEQQ Y0, Y1, Y2  // y2 = y0==y1
    VMOVMSKPD Y2, R13  // move mask
    CMPQ R13, $15  // if mask==15 then goto align_4
    JE align_4
    // not equal
    MOVB $0, ret+24(FP)  // return 0
    VZEROUPPER
    RET
align_4_end:
    MOVQ R9, R10
    ANDQ $3, R10  // left_count = count & 3
    LEAQ (R8)(R10*8), R11  // end = current + left_count*8
align_1:
    CMPQ R8, R11
    JEQ end  // if current==end then goto end
    MOVQ (R8), R13 // r13 = *current
    ADDQ $8, R8  // current += 8
    CMPQ R12,R13  // if *current == first_value then goto align_1
    JEQ align_1
    // not equal
not_equal:
    MOVB $0, ret+24(FP)  // return 0
    VZEROUPPER
    RET
end:
    MOVB $1, ret+24(FP)  // return 1
    VZEROUPPER
    RET
