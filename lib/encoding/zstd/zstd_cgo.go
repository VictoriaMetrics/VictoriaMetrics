//go:build cgo

package zstd

import (
	"github.com/valyala/gozstd"
)

// Decompress appends decompressed src to dst and returns the result.
func Decompress(dst, src []byte) ([]byte, error) {
	return gozstd.Decompress(dst, src)
}

// CompressLevel appends compressed src to dst and returns the result.
//
// The given compressionLevel is used for the compression.
func CompressLevel(dst, src []byte, compressionLevel int) []byte {
	return gozstd.CompressLevel(dst, src, compressionLevel)
}
