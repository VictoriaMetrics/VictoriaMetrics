package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

// u128 is 128-bit uint number.
//
// It is used as an unique id of stream.
type u128 struct {
	hi uint64
	lo uint64
}

// String returns human-readable representation of u.
func (u *u128) String() string {
	return fmt.Sprintf("{hi=%d,lo=%d}", u.hi, u.lo)
}

// less returns true if u is less than a.
func (u *u128) less(a *u128) bool {
	if u.hi != a.hi {
		return u.hi < a.hi
	}
	return u.lo < a.lo
}

// equal returns true if u equals to a.
func (u *u128) equal(a *u128) bool {
	return u.hi == a.hi && u.lo == a.lo
}

func (u *u128) marshalString(dst []byte) []byte {
	dst = marshalUint64Hex(dst, u.hi)
	dst = marshalUint64Hex(dst, u.lo)
	return dst
}

func marshalUint64Hex(dst []byte, n uint64) []byte {
	dst = marshalByteHex(dst, byte(n>>56))
	dst = marshalByteHex(dst, byte(n>>48))
	dst = marshalByteHex(dst, byte(n>>40))
	dst = marshalByteHex(dst, byte(n>>32))
	dst = marshalByteHex(dst, byte(n>>24))
	dst = marshalByteHex(dst, byte(n>>16))
	dst = marshalByteHex(dst, byte(n>>8))
	dst = marshalByteHex(dst, byte(n))
	return dst
}

func marshalByteHex(dst []byte, x byte) []byte {
	return append(dst, hexByteMap[(x>>4)&15], hexByteMap[x&15])
}

var hexByteMap = [16]byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f'}

// marshal appends the marshaled u to dst and returns the result.
func (u *u128) marshal(dst []byte) []byte {
	dst = encoding.MarshalUint64(dst, u.hi)
	dst = encoding.MarshalUint64(dst, u.lo)
	return dst
}

// unmarshal unmarshals u from src and returns the tail.
func (u *u128) unmarshal(src []byte) ([]byte, error) {
	if len(src) < 16 {
		return src, fmt.Errorf("cannot unmarshal u128 from %d bytes; need at least 16 bytes", len(src))
	}
	u.hi = encoding.UnmarshalUint64(src[:8])
	u.lo = encoding.UnmarshalUint64(src[8:])
	return src[16:], nil
}
