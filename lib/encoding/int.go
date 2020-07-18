package encoding

import (
	"encoding/binary"
	"fmt"
	"sync"
)

// MarshalUint16 appends marshaled v to dst and returns the result.
func MarshalUint16(dst []byte, u uint16) []byte {
	return append(dst, byte(u>>8), byte(u))
}

// UnmarshalUint16 returns unmarshaled uint32 from src.
func UnmarshalUint16(src []byte) uint16 {
	// This is faster than the manual conversion.
	return binary.BigEndian.Uint16(src)
}

// MarshalUint32 appends marshaled v to dst and returns the result.
func MarshalUint32(dst []byte, u uint32) []byte {
	return append(dst, byte(u>>24), byte(u>>16), byte(u>>8), byte(u))
}

// UnmarshalUint32 returns unmarshaled uint32 from src.
func UnmarshalUint32(src []byte) uint32 {
	// This is faster than the manual conversion.
	return binary.BigEndian.Uint32(src)
}

// MarshalUint64 appends marshaled v to dst and returns the result.
func MarshalUint64(dst []byte, u uint64) []byte {
	return append(dst, byte(u>>56), byte(u>>48), byte(u>>40), byte(u>>32), byte(u>>24), byte(u>>16), byte(u>>8), byte(u))
}

// UnmarshalUint64 returns unmarshaled uint64 from src.
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
func UnmarshalInt64(src []byte) int64 {
	// This is faster than the manual conversion.
	u := binary.BigEndian.Uint64(src)
	v := int64(u>>1) ^ (int64(u<<63) >> 63) // zig-zag decoding without branching.
	return v
}

// MarshalVarInt64 appends marshalsed v to dst and returns the result.
func MarshalVarInt64(dst []byte, v int64) []byte {
	var tmp [1]int64
	tmp[0] = v
	return MarshalVarInt64s(dst, tmp[:])
}

// MarshalVarInt64s appends marshaled vs to dst and returns the result.
func MarshalVarInt64s(dst []byte, vs []int64) []byte {
	for _, v := range vs {
		if v < 0x40 && v > -0x40 {
			// Fast path
			c := int8(v)
			v := (c << 1) ^ (c >> 7) // zig-zag encoding without branching.
			dst = append(dst, byte(v))
			continue
		}

		v = (v << 1) ^ (v >> 63) // zig-zag encoding without branching.
		u := uint64(v)
		for u > 0x7f {
			dst = append(dst, 0x80|byte(u))
			u >>= 7
		}
		dst = append(dst, byte(u))
	}
	return dst
}

// UnmarshalVarInt64 returns unmarshaled int64 from src and returns
// the remaining tail from src.
func UnmarshalVarInt64(src []byte) ([]byte, int64, error) {
	var tmp [1]int64
	tail, err := UnmarshalVarInt64s(tmp[:], src)
	return tail, tmp[0], err
}

// UnmarshalVarInt64s unmarshals len(dst) int64 values from src to dst
// and returns the remaining tail from src.
func UnmarshalVarInt64s(dst []int64, src []byte) ([]byte, error) {
	idx := uint(0)
	for i := range dst {
		if idx >= uint(len(src)) {
			return nil, fmt.Errorf("cannot unmarshal varint from empty data")
		}
		c := src[idx]
		idx++
		if c < 0x80 {
			// Fast path
			v := int8(c>>1) ^ (int8(c<<7) >> 7) // zig-zag decoding without branching.
			dst[i] = int64(v)
			continue
		}

		// Slow path
		u := uint64(c & 0x7f)
		startIdx := idx - 1
		shift := uint8(0)
		for c >= 0x80 {
			if idx >= uint(len(src)) {
				return nil, fmt.Errorf("unexpected end of encoded varint at byte %d; src=%x", idx-startIdx, src[startIdx:])
			}
			if idx-startIdx > 9 {
				return src[idx:], fmt.Errorf("too long encoded varint; the maximum allowed length is 10 bytes; got %d bytes; src=%x",
					(idx-startIdx)+1, src[startIdx:])
			}
			c = src[idx]
			idx++
			shift += 7
			u |= uint64(c&0x7f) << shift
		}
		v := int64(u>>1) ^ (int64(u<<63) >> 63) // zig-zag decoding without branching.
		dst[i] = v
	}
	return src[idx:], nil
}

// MarshalVarUint64 appends marshaled u to dst and returns the result.
func MarshalVarUint64(dst []byte, u uint64) []byte {
	var tmp [1]uint64
	tmp[0] = u
	return MarshalVarUint64s(dst, tmp[:])
}

// MarshalVarUint64s appends marshaled us to dst and returns the result.
func MarshalVarUint64s(dst []byte, us []uint64) []byte {
	for _, u := range us {
		if u < 0x80 {
			// Fast path
			dst = append(dst, byte(u))
			continue
		}
		for u > 0x7f {
			dst = append(dst, 0x80|byte(u))
			u >>= 7
		}
		dst = append(dst, byte(u))
	}
	return dst
}

// UnmarshalVarUint64 returns unmarshaled uint64 from src and returns
// the remaining tail from src.
func UnmarshalVarUint64(src []byte) ([]byte, uint64, error) {
	var tmp [1]uint64
	tail, err := UnmarshalVarUint64s(tmp[:], src)
	return tail, tmp[0], err
}

// UnmarshalVarUint64s unmarshals len(dst) uint64 values from src to dst
// and returns the remaining tail from src.
func UnmarshalVarUint64s(dst []uint64, src []byte) ([]byte, error) {
	idx := uint(0)
	for i := range dst {
		if idx >= uint(len(src)) {
			return nil, fmt.Errorf("cannot unmarshal varuint from empty data")
		}
		c := src[idx]
		idx++
		if c < 0x80 {
			// Fast path
			dst[i] = uint64(c)
			continue
		}

		// Slow path
		u := uint64(c & 0x7f)
		startIdx := idx - 1
		shift := uint8(0)
		for c >= 0x80 {
			if idx >= uint(len(src)) {
				return nil, fmt.Errorf("unexpected end of encoded varint at byte %d; src=%x", idx-startIdx, src[startIdx:])
			}
			if idx-startIdx > 9 {
				return src[idx:], fmt.Errorf("too long encoded varint; the maximum allowed length is 10 bytes; got %d bytes; src=%x",
					(idx-startIdx)+1, src[startIdx:])
			}
			c = src[idx]
			idx++
			shift += 7
			u |= uint64(c&0x7f) << shift
		}
		dst[i] = u
	}
	return src[idx:], nil
}

// MarshalBytes appends marshaled b to dst and returns the result.
func MarshalBytes(dst, b []byte) []byte {
	dst = MarshalVarUint64(dst, uint64(len(b)))
	dst = append(dst, b...)
	return dst
}

// UnmarshalBytes returns unmarshaled bytes from src.
func UnmarshalBytes(src []byte) ([]byte, []byte, error) {
	tail, n, err := UnmarshalVarUint64(src)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot unmarshal string size: %w", err)
	}
	src = tail
	if uint64(len(src)) < n {
		return nil, nil, fmt.Errorf("src is too short for reading string with size %d; len(src)=%d", n, len(src))
	}
	return src[n:], src[:n], nil
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
	if n := size - cap(is.A); n > 0 {
		is.A = append(is.A[:cap(is.A)], make([]int64, n)...)
	}
	is.A = is.A[:size]
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
	if n := size - cap(is.A); n > 0 {
		is.A = append(is.A[:cap(is.A)], make([]uint64, n)...)
	}
	is.A = is.A[:size]
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
