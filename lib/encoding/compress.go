package encoding

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/zstd"
	"github.com/VictoriaMetrics/metrics"
)

// CompressZSTDLevel appends compressed src to dst and returns the appended dst.
//
// The given compressLevel is used for the compression.
func CompressZSTDLevel(dst, src []byte, compressLevel int) []byte {
	compressCalls.Inc()
	originalBytes.Add(len(src))
	dstLen := len(dst)
	dst = zstd.CompressLevel(dst, src, compressLevel)
	compressedBytes.Add(len(dst) - dstLen)
	return dst
}

// DecompressZSTD decompresses src, appends the result to dst and returns the appended dst.
//
// This function must be called only for the trusted src.
// Use DecompressZSTDLimited for untrusted src.
func DecompressZSTD(dst, src []byte) ([]byte, error) {
	decompressCalls.Inc()
	b, err := zstd.Decompress(dst, src)
	if err != nil {
		return b, fmt.Errorf("cannot decompress zstd block with len=%d: %w; block data (hex): %X", len(src), err, src)
	}
	return b, nil
}

// DecompressZSTDLimited decompresses src, appends the result to dst and returns the appended dst.
//
// If the decompressed result exceeds maxDataSizeBytes, then error is returned.
func DecompressZSTDLimited(dst, src []byte, maxDataSizeBytes int) ([]byte, error) {
	decompressCalls.Inc()
	b, err := zstd.DecompressLimited(dst, src, maxDataSizeBytes)
	if err != nil {
		return b, fmt.Errorf("cannot decompress zstd block with len=%d and maxDataSizeBytes=%d: %w", len(src), maxDataSizeBytes, err)
	}
	return b, nil
}

var (
	compressCalls   = metrics.NewCounter(`vm_zstd_block_compress_calls_total`)
	decompressCalls = metrics.NewCounter(`vm_zstd_block_decompress_calls_total`)

	originalBytes   = metrics.NewCounter(`vm_zstd_block_original_bytes_total`)
	compressedBytes = metrics.NewCounter(`vm_zstd_block_compressed_bytes_total`)
)
