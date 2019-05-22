package encoding

import (
	"github.com/valyala/gozstd"
)

// CompressZSTD compresses src, appends the result to dst and returns
// the appended dst.
//
// src must be non-empty.
func CompressZSTD(dst, src []byte) []byte {
	return gozstd.CompressLevel(dst, src, 5)
}

// CompressZSTDLevel appends compressed src to dst and returns
// the appended dst.
//
// The given compressLevel is used for the compression.
func CompressZSTDLevel(dst, src []byte, compressLevel int) []byte {
	return gozstd.CompressLevel(dst, src, compressLevel)
}

// DecompressZSTD decompresses src, appends the result to dst and returns
// the appended dst.
func DecompressZSTD(dst, src []byte) ([]byte, error) {
	return gozstd.Decompress(dst, src)
}
