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

// StoreUint64LE stores v as little-endian uint64 into b.
func StoreUint64LE(b []byte, v uint64) {
	_ = b[7] // bounds check
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
	b[4] = byte(v >> 32)
	b[5] = byte(v >> 40)
	b[6] = byte(v >> 48)
	b[7] = byte(v >> 56)
}

// LoadUint64LE loads a little-endian uint64 from b.
func LoadUint64LE(b []byte) uint64 {
	_ = b[7] // bounds check
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
}
