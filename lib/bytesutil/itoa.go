package bytesutil

import (
	"strconv"
)

// Itoa returns string representation of n.
//
// This function doesn't allocate memory on repeated calls for the same n.
func Itoa(n int) string {
	bb := bbPool.Get()
	b := bb.B[:0]
	b = strconv.AppendInt(b, int64(n), 10)
	s := InternBytes(b)
	bb.B = b
	bbPool.Put(bb)
	return s
}

var bbPool ByteBufferPool
