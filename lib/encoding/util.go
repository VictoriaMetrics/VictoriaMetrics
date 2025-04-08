package encoding

import "encoding/binary"

// IsZstd checks if the given data is compressed using the zstd format.
// It does this by verifying the presence of the zstd magic number (0xFD2FB528)
// at the beginning of the byte slice.
//
// See: https://github.com/facebook/zstd/blob/dev/doc/zstd_compression_format.md#zstandard-frames
func IsZstd(data []byte) bool {
	return len(data) >= 4 && binary.LittleEndian.Uint32(data) == 0xFD2FB528
}
