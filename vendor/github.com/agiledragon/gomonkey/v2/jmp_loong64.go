//go:build loong64
// +build loong64

package gomonkey

import "unsafe"

const (
	REG_R0  uint32 = 0
	REG_R29        = 29
	REG_R30        = 30
)

const (
	OP_ORI    uint32 = 0x00E << 22
	OP_LU12IW        = 0x00A << 25
	OP_LU32ID        = 0x00B << 25
	OP_LU52ID        = 0x00C << 22
	OP_LDD           = 0x0A3 << 22
	OP_JIRL          = 0x013 << 26
)

func buildJmpDirective(double uintptr) []byte {
	res := make([]byte, 0, 24)

	bit11_0 := (double >> 0) & 0xFFF
	bit31_12 := (double >> 12) & 0xFFFFF
	bit51_32 := (double >> 32) & 0xFFFFF
	bit63_52 := (double >> 52) & 0xFFF

	// lu12i.w r29, bit31_12
	// ori     r29, r29, bit11_0
	// lu32i.d r29, bit51_32
	// lu52i.d r29, bit63_52
	// ld.d,   r30, r29, 0
	// jirl    r0,  r30, 0
	res = append(res, wireup_opc(OP_LU12IW, REG_R29, 0, bit31_12)...)
	res = append(res, wireup_opc(OP_ORI, REG_R29, REG_R29, bit11_0)...)
	res = append(res, wireup_opc(OP_LU32ID, REG_R29, 0, bit51_32)...)
	res = append(res, wireup_opc(OP_LU52ID, REG_R29, REG_R29, bit63_52)...)
	res = append(res, wireup_opc(OP_LDD, REG_R30, REG_R29, 0)...)
	res = append(res, wireup_opc(OP_JIRL, REG_R0, REG_R30, 0)...)

	return res
}

func wireup_opc(opc uint32, rd, rj uint32, val uintptr) []byte {
	var m uint32 = 0

	switch opc {
	case OP_ORI, OP_LU52ID, OP_LDD:
		m |= opc
		m |= (rd & 0x1F) << 0            // rd
		m |= (rj & 0x1F) << 5            // rj
		m |= (uint32(val) & 0xFFF) << 10 // si12

	case OP_LU12IW, OP_LU32ID:
		m |= opc
		m |= (rd & 0x1F) << 0             // rd
		m |= (uint32(val) & 0xFFFFF) << 5 // si20

	case OP_JIRL:
		m |= opc
		m |= (rd & 0x1F) << 0             // rd
		m |= (rj & 0x1F) << 5             // rj
		m |= (uint32(val) & 0xFFFF) << 10 // si16
	}

	res := make([]byte, 4)
	*(*uint32)(unsafe.Pointer(&res[0])) = m

	return res
}
