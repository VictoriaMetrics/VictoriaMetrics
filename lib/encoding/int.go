package encoding

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// MarshalUint16 appends marshaled v to dst and returns the result.
func MarshalUint16(dst []byte, u uint16) []byte {
	return append(dst, byte(u>>8), byte(u))
}

// UnmarshalUint16 returns unmarshaled uint16 from src.
//
// the caller must ensure that len(src) >= 2
func UnmarshalUint16(src []byte) uint16 {
	// This is faster than the manual conversion.
	return binary.BigEndian.Uint16(src)
}

// MarshalUint32 appends marshaled v to dst and returns the result.
func MarshalUint32(dst []byte, u uint32) []byte {
	return append(dst, byte(u>>24), byte(u>>16), byte(u>>8), byte(u))
}

// UnmarshalUint32 returns unmarshaled uint32 from src.
//
// The caller must ensure than len(src) >= 4
func UnmarshalUint32(src []byte) uint32 {
	// This is faster than the manual conversion.
	return binary.BigEndian.Uint32(src)
}

// MarshalUint64 appends marshaled v to dst and returns the result.
func MarshalUint64(dst []byte, u uint64) []byte {
	return append(dst, byte(u>>56), byte(u>>48), byte(u>>40), byte(u>>32), byte(u>>24), byte(u>>16), byte(u>>8), byte(u))
}

// UnmarshalUint64 returns unmarshaled uint64 from src.
//
// The caller must ensure that len(src) >= 8
func UnmarshalUint64(src []byte) uint64 {
	// This is faster than the manual conversion.
	return binary.BigEndian.Uint64(src)
}

// MarshalInt16 appends marshaled v to dst and returns the result.
func MarshalInt16(dst []byte, v int16) []byte {
	// Such encoding for negative v must improve compression.
	v = (v << 1) ^ (v >> 15) // zig-zag encoding without branching.
	u := uint16(v)
	return append(dst, byte(u>>8), byte(u))
}

// UnmarshalInt16 returns unmarshaled int16 from src.
//
// The caller must ensure that len(src) >= 2
func UnmarshalInt16(src []byte) int16 {
	// This is faster than the manual conversion.
	u := binary.BigEndian.Uint16(src)
	v := int16(u>>1) ^ (int16(u<<15) >> 15) // zig-zag decoding without branching.
	return v
}

// MarshalInt64 appends marshaled v to dst and returns the result.
func MarshalInt64(dst []byte, v int64) []byte {
	// Such encoding for negative v must improve compression.
	v = (v << 1) ^ (v >> 63) // zig-zag encoding without branching.
	u := uint64(v)
	return append(dst, byte(u>>56), byte(u>>48), byte(u>>40), byte(u>>32), byte(u>>24), byte(u>>16), byte(u>>8), byte(u))
}

// UnmarshalInt64 returns unmarshaled int64 from src.
//
// The caller must ensure that len(src) >= 8
func UnmarshalInt64(src []byte) int64 {
	// This is faster than the manual conversion.
	u := binary.BigEndian.Uint64(src)
	v := int64(u>>1) ^ (int64(u<<63) >> 63) // zig-zag decoding without branching.
	return v
}

// MarshalVarInt64 appends marshaled v to dst and returns the result.
func MarshalVarInt64(dst []byte, v int64) []byte {
	u := uint64((v << 1) ^ (v >> 63))

	if v < (1<<6) && v > (-1<<6) {
		return append(dst, byte(u))
	}
	if u < (1 << (2 * 7)) {
		return append(dst, byte(u|0x80), byte(u>>7))
	}
	if u < (1 << (3 * 7)) {
		return append(dst, byte(u|0x80), byte((u>>7)|0x80), byte(u>>(2*7)))
	}

	// Slow path for big integers
	var tmp [1]uint64
	tmp[0] = u
	return MarshalVarUint64s(dst, tmp[:])
}

// MarshalVarInt64s appends marshaled vs to dst and returns the result.
func MarshalVarInt64s(dst []byte, vs []int64) []byte {
	dstLen := len(dst)
	for _, v := range vs {
		if v >= (1<<6) || v <= (-1<<6) {
			return marshalVarInt64sSlow(dst[:dstLen], vs)
		}
		u := uint64((v << 1) ^ (v >> 63))
		dst = append(dst, byte(u))
	}
	return dst
}

func marshalVarInt64sSlow(dst []byte, vs []int64) []byte {
	for _, v := range vs {
		u := uint64((v << 1) ^ (v >> 63))

		// Cases below are sorted in the descending order of frequency on real data
		if u < (1 << 7) {
			dst = append(dst, byte(u))
			continue
		}
		if u < (1 << (2 * 7)) {
			dst = append(dst, byte(u|0x80), byte(u>>7))
			continue
		}
		if u < (1 << (3 * 7)) {
			dst = append(dst, byte(u|0x80), byte((u>>7)|0x80), byte(u>>(2*7)))
			continue
		}

		if u >= (1 << (8 * 7)) {
			if u < (1 << (9 * 7)) {
				dst = append(dst, byte(u|0x80), byte((u>>7)|0x80), byte((u>>(2*7))|0x80), byte((u>>(3*7))|0x80), byte((u>>(4*7))|0x80),
					byte((u>>(5*7))|0x80), byte((u>>(6*7))|0x80), byte((u>>(7*7))|0x80), byte(u>>(8*7)))
				continue
			}
			dst = append(dst, byte(u|0x80), byte((u>>7)|0x80), byte((u>>(2*7))|0x80), byte((u>>(3*7))|0x80), byte((u>>(4*7))|0x80),
				byte((u>>(5*7))|0x80), byte((u>>(6*7))|0x80), byte((u>>(7*7))|0x80), byte((u>>(8*7))|0x80), 1)
			continue
		}

		if u < (1 << (4 * 7)) {
			dst = append(dst, byte(u|0x80), byte((u>>7)|0x80), byte((u>>(2*7))|0x80), byte(u>>(3*7)))
			continue
		}
		if u < (1 << (5 * 7)) {
			dst = append(dst, byte(u|0x80), byte((u>>7)|0x80), byte((u>>(2*7))|0x80), byte((u>>(3*7))|0x80), byte(u>>(4*7)))
			continue
		}
		if u < (1 << (6 * 7)) {
			dst = append(dst, byte(u|0x80), byte((u>>7)|0x80), byte((u>>(2*7))|0x80), byte((u>>(3*7))|0x80), byte((u>>(4*7))|0x80), byte(u>>(5*7)))
			continue
		}
		if u < (1 << (7 * 7)) {
			dst = append(dst, byte(u|0x80), byte((u>>7)|0x80), byte((u>>(2*7))|0x80), byte((u>>(3*7))|0x80), byte((u>>(4*7))|0x80), byte((u>>(5*7))|0x80), byte(u>>(6*7)))
			continue
		}
		dst = append(dst, byte(u|0x80), byte((u>>7)|0x80), byte((u>>(2*7))|0x80), byte((u>>(3*7))|0x80), byte((u>>(4*7))|0x80),
			byte((u>>(5*7))|0x80), byte((u>>(6*7))|0x80), byte(u>>(7*7)))
	}
	return dst
}

// UnmarshalVarInt64 returns unmarshaled int64 from src and its size in bytes.
//
// It returns 0 or negative value if it cannot unmarshal int64 from src.
func UnmarshalVarInt64(src []byte) (int64, int) {
	// TODO substitute binary.Uvarint with binary.Varint when benchmark results will show it is faster.
	// It is slower on amd64/linux Go1.22.
	u64, nSize := binary.Uvarint(src)
	i64 := int64(int64(u64>>1) ^ (int64(u64<<63) >> 63))
	return i64, nSize
}

// UnmarshalVarInt64s unmarshals len(dst) int64 values from src to dst and returns the remaining tail from src.
func UnmarshalVarInt64s(dst []int64, src []byte) ([]byte, error) {
	if len(src) < len(dst) {
		return src, fmt.Errorf("too small len(src)=%d; it must be bigger or equal to len(dst)=%d", len(src), len(dst))
	}
	for i := range dst {
		c := src[i]
		if c >= 0x80 {
			return unmarshalVarInt64sSlow(dst, src)
		}
		dst[i] = int64(int8(c>>1) ^ (int8(c<<7) >> 7))
	}
	return src[len(dst):], nil
}

func unmarshalVarInt64sSlow(dst []int64, src []byte) ([]byte, error) {
	idx := uint(0)
	for i := range dst {
		if idx >= uint(len(src)) {
			return nil, fmt.Errorf("cannot unmarshal varint from empty data")
		}
		c := src[idx]
		idx++
		if c < 0x80 {
			// Fast path for 1 byte
			dst[i] = int64(int8(c>>1) ^ (int8(c<<7) >> 7))
			continue
		}

		if idx >= uint(len(src)) {
			return nil, fmt.Errorf("unexpected end of encoded varint at byte 1; src=%x", src[idx-1:])
		}
		d := src[idx]
		idx++
		if d < 0x80 {
			// Fast path for 2 bytes
			u := uint64(c&0x7f) | (uint64(d) << 7)
			dst[i] = int64(u>>1) ^ (int64(u<<63) >> 63)
			continue
		}

		if idx >= uint(len(src)) {
			return nil, fmt.Errorf("unexpected end of encoded varint at byte 2; src=%x", src[idx-2:])
		}
		e := src[idx]
		idx++
		if e < 0x80 {
			// Fast path for 3 bytes
			u := uint64(c&0x7f) | (uint64(d&0x7f) << 7) | (uint64(e) << (2 * 7))
			dst[i] = int64(u>>1) ^ (int64(u<<63) >> 63)
			continue
		}

		u := uint64(c&0x7f) | (uint64(d&0x7f) << 7) | (uint64(e&0x7f) << (2 * 7))

		// Slow path
		j := idx
		for {
			if idx >= uint(len(src)) {
				return nil, fmt.Errorf("unexpected end of encoded varint; src=%x", src[j-3:])
			}
			c := src[idx]
			idx++
			if c < 0x80 {
				break
			}
		}

		// These are the most common cases
		switch idx - j {
		case 1:
			u |= (uint64(src[j]) << (3 * 7))
		case 2:
			b := src[j : j+2 : j+2]
			u |= (uint64(b[0]&0x7f) << (3 * 7)) | (uint64(b[1]) << (4 * 7))
		case 3:
			b := src[j : j+3 : j+3]
			u |= (uint64(b[0]&0x7f) << (3 * 7)) | (uint64(b[1]&0x7f) << (4 * 7)) | (uint64(b[2]) << (5 * 7))
		case 4:
			b := src[j : j+4 : j+4]
			u |= (uint64(b[0]&0x7f) << (3 * 7)) | (uint64(b[1]&0x7f) << (4 * 7)) | (uint64(b[2]&0x7f) << (5 * 7)) | (uint64(b[3]) << (6 * 7))
		case 5:
			b := src[j : j+5 : j+5]
			u |= (uint64(b[0]&0x7f) << (3 * 7)) | (uint64(b[1]&0x7f) << (4 * 7)) | (uint64(b[2]&0x7f) << (5 * 7)) | (uint64(b[3]&0x7f) << (6 * 7)) |
				(uint64(b[4]) << (7 * 7))
		case 6:
			b := src[j : j+6 : j+6]
			u |= (uint64(b[0]&0x7f) << (3 * 7)) | (uint64(b[1]&0x7f) << (4 * 7)) | (uint64(b[2]&0x7f) << (5 * 7)) | (uint64(b[3]&0x7f) << (6 * 7)) |
				(uint64(b[4]&0x7f) << (7 * 7)) | (uint64(b[5]) << (8 * 7))
		case 7:
			b := src[j : j+7 : j+7]
			if b[6] > 1 {
				return src[idx:], fmt.Errorf("too big encoded varint; src=%x", src[j-3:])
			}
			u |= (uint64(b[0]&0x7f) << (3 * 7)) | (uint64(b[1]&0x7f) << (4 * 7)) | (uint64(b[2]&0x7f) << (5 * 7)) | (uint64(b[3]&0x7f) << (6 * 7)) |
				(uint64(b[4]&0x7f) << (7 * 7)) | (uint64(b[5]&0x7f) << (8 * 7)) | (1 << (9 * 7))
		default:
			return src[idx:], fmt.Errorf("too long encoded varint; the maximum allowed length is 10 bytes; got %d bytes; src=%x", idx-j+3, src[j-3:])
		}

		dst[i] = int64(u>>1) ^ (int64(u<<63) >> 63)
	}
	return src[idx:], nil
}

// MarshalVarUint64 appends marshaled u to dst and returns the result.
func MarshalVarUint64(dst []byte, u uint64) []byte {
	if u < (1 << 7) {
		return append(dst, byte(u))
	}
	if u < (1 << (2 * 7)) {
		return append(dst, byte(u|0x80), byte(u>>7))
	}
	if u < (1 << (3 * 7)) {
		return append(dst, byte(u|0x80), byte((u>>7)|0x80), byte(u>>(2*7)))
	}

	// Slow path for big integers.
	var tmp [1]uint64
	tmp[0] = u
	return MarshalVarUint64s(dst, tmp[:])
}

// MarshalVarUint64s appends marshaled us to dst and returns the result.
func MarshalVarUint64s(dst []byte, us []uint64) []byte {
	dstLen := len(dst)
	for _, u := range us {
		if u >= (1 << 7) {
			return marshalVarUint64sSlow(dst[:dstLen], us)
		}
		dst = append(dst, byte(u))
	}
	return dst
}

func marshalVarUint64sSlow(dst []byte, us []uint64) []byte {
	for _, u := range us {
		// Cases below are sorted in the descending order of frequency on real data
		if u < (1 << 7) {
			dst = append(dst, byte(u))
			continue
		}
		if u < (1 << (2 * 7)) {
			dst = append(dst, byte(u|0x80), byte(u>>7))
			continue
		}
		if u < (1 << (3 * 7)) {
			dst = append(dst, byte(u|0x80), byte((u>>7)|0x80), byte(u>>(2*7)))
			continue
		}

		if u >= (1 << (8 * 7)) {
			if u < (1 << (9 * 7)) {
				dst = append(dst, byte(u|0x80), byte((u>>7)|0x80), byte((u>>(2*7))|0x80), byte((u>>(3*7))|0x80), byte((u>>(4*7))|0x80),
					byte((u>>(5*7))|0x80), byte((u>>(6*7))|0x80), byte((u>>(7*7))|0x80), byte(u>>(8*7)))
				continue
			}
			dst = append(dst, byte(u|0x80), byte((u>>7)|0x80), byte((u>>(2*7))|0x80), byte((u>>(3*7))|0x80), byte((u>>(4*7))|0x80),
				byte((u>>(5*7))|0x80), byte((u>>(6*7))|0x80), byte((u>>(7*7))|0x80), byte((u>>(8*7))|0x80), 1)
			continue
		}

		if u < (1 << (4 * 7)) {
			dst = append(dst, byte(u|0x80), byte((u>>7)|0x80), byte((u>>(2*7))|0x80), byte(u>>(3*7)))
			continue
		}
		if u < (1 << (5 * 7)) {
			dst = append(dst, byte(u|0x80), byte((u>>7)|0x80), byte((u>>(2*7))|0x80), byte((u>>(3*7))|0x80), byte(u>>(4*7)))
			continue
		}
		if u < (1 << (6 * 7)) {
			dst = append(dst, byte(u|0x80), byte((u>>7)|0x80), byte((u>>(2*7))|0x80), byte((u>>(3*7))|0x80), byte((u>>(4*7))|0x80), byte(u>>(5*7)))
			continue
		}
		if u < (1 << (7 * 7)) {
			dst = append(dst, byte(u|0x80), byte((u>>7)|0x80), byte((u>>(2*7))|0x80), byte((u>>(3*7))|0x80), byte((u>>(4*7))|0x80), byte((u>>(5*7))|0x80), byte(u>>(6*7)))
			continue
		}
		dst = append(dst, byte(u|0x80), byte((u>>7)|0x80), byte((u>>(2*7))|0x80), byte((u>>(3*7))|0x80), byte((u>>(4*7))|0x80),
			byte((u>>(5*7))|0x80), byte((u>>(6*7))|0x80), byte(u>>(7*7)))
	}
	return dst
}

// UnmarshalVarUint64 returns unmarshaled uint64 from src and its size in bytes.
//
// It returns 0 or negative value if it cannot unmarshal uint64 from src.
func UnmarshalVarUint64(src []byte) (uint64, int) {
	if len(src) == 0 {
		return 0, 0
	}
	if src[0] < 0x80 {
		// Fast path for a single byte
		return uint64(src[0]), 1
	}
	if len(src) == 1 {
		return 0, 0
	}
	if src[1] < 0x80 {
		// Fast path for two bytes
		return uint64(src[0]&0x7f) | uint64(src[1])<<7, 2
	}

	// Slow path for other number of bytes
	return binary.Uvarint(src)
}

// UnmarshalVarUint64s unmarshals len(dst) uint64 values from src to dst and returns the remaining tail from src.
func UnmarshalVarUint64s(dst []uint64, src []byte) ([]byte, error) {
	if len(src) < len(dst) {
		return src, fmt.Errorf("too small len(src)=%d; it must be bigger or equal to len(dst)=%d", len(src), len(dst))
	}
	for i := range dst {
		c := src[i]
		if c >= 0x80 {
			return unmarshalVarUint64sSlow(dst, src)
		}
		dst[i] = uint64(c)
	}
	return src[len(dst):], nil
}

func unmarshalVarUint64sSlow(dst []uint64, src []byte) ([]byte, error) {
	idx := uint(0)
	for i := range dst {
		if idx >= uint(len(src)) {
			return nil, fmt.Errorf("cannot unmarshal varuint from empty data")
		}
		c := src[idx]
		idx++
		if c < 0x80 {
			// Fast path for 1 byte
			dst[i] = uint64(c)
			continue
		}

		if idx >= uint(len(src)) {
			return nil, fmt.Errorf("unexpected end of encoded varuint at byte 1; src=%x", src[idx-1:])
		}
		d := src[idx]
		idx++
		if d < 0x80 {
			// Fast path for 2 bytes
			dst[i] = uint64(c&0x7f) | (uint64(d) << 7)
			continue
		}

		if idx >= uint(len(src)) {
			return nil, fmt.Errorf("unexpected end of encoded varuint at byte 2; src=%x", src[idx-2:])
		}
		e := src[idx]
		idx++
		if e < 0x80 {
			// Fast path for 3 bytes
			dst[i] = uint64(c&0x7f) | (uint64(d&0x7f) << 7) | (uint64(e) << (2 * 7))
			continue
		}

		u := uint64(c&0x7f) | (uint64(d&0x7f) << 7) | (uint64(e&0x7f) << (2 * 7))

		// Slow path
		j := idx
		for {
			if idx >= uint(len(src)) {
				return nil, fmt.Errorf("unexpected end of encoded varint; src=%x", src[j-3:])
			}
			c := src[idx]
			idx++
			if c < 0x80 {
				break
			}
		}

		// These are the most common cases
		switch idx - j {
		case 1:
			u |= (uint64(src[j]) << (3 * 7))
		case 2:
			b := src[j : j+2 : j+2]
			u |= (uint64(b[0]&0x7f) << (3 * 7)) | (uint64(b[1]) << (4 * 7))
		case 3:
			b := src[j : j+3 : j+3]
			u |= (uint64(b[0]&0x7f) << (3 * 7)) | (uint64(b[1]&0x7f) << (4 * 7)) | (uint64(b[2]) << (5 * 7))
		case 4:
			b := src[j : j+4 : j+4]
			u |= (uint64(b[0]&0x7f) << (3 * 7)) | (uint64(b[1]&0x7f) << (4 * 7)) | (uint64(b[2]&0x7f) << (5 * 7)) | (uint64(b[3]) << (6 * 7))
		case 5:
			b := src[j : j+5 : j+5]
			u |= (uint64(b[0]&0x7f) << (3 * 7)) | (uint64(b[1]&0x7f) << (4 * 7)) | (uint64(b[2]&0x7f) << (5 * 7)) | (uint64(b[3]&0x7f) << (6 * 7)) |
				(uint64(b[4]) << (7 * 7))
		case 6:
			b := src[j : j+6 : j+6]
			u |= (uint64(b[0]&0x7f) << (3 * 7)) | (uint64(b[1]&0x7f) << (4 * 7)) | (uint64(b[2]&0x7f) << (5 * 7)) | (uint64(b[3]&0x7f) << (6 * 7)) |
				(uint64(b[4]&0x7f) << (7 * 7)) | (uint64(b[5]) << (8 * 7))
		case 7:
			b := src[j : j+7 : j+7]
			if b[6] > 1 {
				return src[idx:], fmt.Errorf("too big encoded varuint; src=%x", src[j-3:])
			}
			u |= (uint64(b[0]&0x7f) << (3 * 7)) | (uint64(b[1]&0x7f) << (4 * 7)) | (uint64(b[2]&0x7f) << (5 * 7)) | (uint64(b[3]&0x7f) << (6 * 7)) |
				(uint64(b[4]&0x7f) << (7 * 7)) | (uint64(b[5]&0x7f) << (8 * 7)) | (1 << (9 * 7))
		default:
			return src[idx:], fmt.Errorf("too long encoded varuint; the maximum allowed length is 10 bytes; got %d bytes; src=%x", idx-j+3, src[j-3:])
		}

		dst[i] = u
	}
	return src[idx:], nil
}

// MarshalBool appends marshaled v to dst and returns the result.
func MarshalBool(dst []byte, v bool) []byte {
	x := byte(0)
	if v {
		x = 1
	}
	return append(dst, x)
}

// UnmarshalBool unmarshals bool from src.
func UnmarshalBool(src []byte) bool {
	return src[0] != 0
}

// MarshalBytes appends marshaled b to dst and returns the result.
func MarshalBytes(dst, b []byte) []byte {
	dst = MarshalVarUint64(dst, uint64(len(b)))
	dst = append(dst, b...)
	return dst
}

// UnmarshalBytes returns unmarshaled bytes from src and the size of the unmarshaled bytes.
//
// It returns 0 or negative value if it is impossible to unmarshal bytes from src.
func UnmarshalBytes(src []byte) ([]byte, int) {
	n, nSize := UnmarshalVarUint64(src)
	if nSize <= 0 {
		return nil, 0
	}
	if uint64(nSize)+n > uint64(len(src)) {
		return nil, 0
	}
	start := nSize
	nSize += int(n)
	return src[start:nSize], nSize
}

// GetInt64s returns an int64 slice with the given size.
// The slice contents isn't initialized - it may contain garbage.
func GetInt64s(size int) *Int64s {
	v := int64sPool.Get()
	if v == nil {
		return &Int64s{
			A: make([]int64, size),
		}
	}
	is := v.(*Int64s)
	is.A = slicesutil.SetLength(is.A, size)
	return is
}

// PutInt64s returns is to the pool.
func PutInt64s(is *Int64s) {
	int64sPool.Put(is)
}

// Int64s holds an int64 slice
type Int64s struct {
	A []int64
}

var int64sPool sync.Pool

// GetUint64s returns an uint64 slice with the given size.
// The slice contents isn't initialized - it may contain garbage.
func GetUint64s(size int) *Uint64s {
	v := uint64sPool.Get()
	if v == nil {
		return &Uint64s{
			A: make([]uint64, size),
		}
	}
	is := v.(*Uint64s)
	is.A = slicesutil.SetLength(is.A, size)
	return is
}

// PutUint64s returns is to the pool.
func PutUint64s(is *Uint64s) {
	uint64sPool.Put(is)
}

// Uint64s holds an uint64 slice
type Uint64s struct {
	A []uint64
}

var uint64sPool sync.Pool

// GetUint32s returns an uint32 slice with the given size.
// The slice contents isn't initialized - it may contain garbage.
func GetUint32s(size int) *Uint32s {
	v := uint32sPool.Get()
	if v == nil {
		return &Uint32s{
			A: make([]uint32, size),
		}
	}
	is := v.(*Uint32s)
	is.A = slicesutil.SetLength(is.A, size)
	return is
}

// PutUint32s returns is to the pool.
func PutUint32s(is *Uint32s) {
	uint32sPool.Put(is)
}

// Uint32s holds an uint32 slice
type Uint32s struct {
	A []uint32
}

var uint32sPool sync.Pool
