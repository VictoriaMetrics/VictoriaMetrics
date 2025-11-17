//go:build cgo

package zstd

import (
	"github.com/valyala/gozstd"
)

// Decompress appends decompressed src to dst and returns the result.
//
// This function must be called only for the trusted src.
func Decompress(dst, src []byte) ([]byte, error) {
	return gozstd.Decompress(dst, src)
}

// Decompress appends decompressed src to dst and returns the result.
func DecompressLimited(dst, src []byte, maxDataSizeBytes int) ([]byte, error) {
	return gozstd.DecompressLimited(dst, src, maxDataSizeBytes)
}

// CompressLevel appends compressed src to dst and returns the result.
//
// The given compressionLevel is used for the compression.
func CompressLevel(dst, src []byte, compressionLevel int) []byte {
	return gozstd.CompressLevel(dst, src, compressionLevel)
}
