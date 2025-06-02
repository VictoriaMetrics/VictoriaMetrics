#include "textflag.h"

TEXT ·IsASCII(SB), NOSPLIT | NOFRAME, $0-17
    // frame length:  0
    // args size: 16 bytes
    // return value size: 1 byte
    MOVQ inPtr+0(FP), R8  // start
    MOVQ inLen+8(FP), R9  // string length
    // variables
    LEAQ (R8)(R9*1), R10  // end = in + len
    XORQ R11, R11  // offset = 0
    MOVQ R9, R12  // left_len = len
align_32:
    CMPQ R12, $31  // if left_len < 32 then goto align_32_end
    JLE align_32_end
    VMOVDQU (R8)(R11*1), Y0  // _mm256_loadu_si256, load 32 bytes to Y0
    VPMOVMSKB Y0, R13  // _mm256_movemask_epi8, move mask
    ADDQ $32, R11  // offset += 32
    ADDQ $-32, R12  // left_len -= 32
    TESTQ R13, R13  // if mask== 0 then goto align_32
    JE align_32
    MOVB $0, ret+16(FP)  // return 0
    VZEROUPPER  // clear registers
    RET
align_32_end:
    LEAQ (R8)(R11*1), R12  // current
next_char:
    CMPQ R12, R10  // if current==end then goto end
    JEQ end
    MOVQ $0, R13 //
    MOVB (R12), R13  // r13 = str[i]
    CMPQ R13,$127 // if r13 >= 127 then goto not_ascii_end
    JA not_ascii_end
    ADDQ $1, R12  // current += 1
    JMP next_char
end:
    MOVB $1, ret+16(FP)  // return 1
    VZEROUPPER
    RET
not_ascii_end:
    MOVB $0, ret+16(FP)  // return 0
    VZEROUPPER
    RET
