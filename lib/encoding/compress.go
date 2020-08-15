package encoding

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/zstd"
	"github.com/VictoriaMetrics/metrics"
)

// CompressZSTDLevel appends compressed src to dst and returns
// the appended dst.
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

// DecompressZSTD decompresses src, appends the result to dst and returns
// the appended dst.
func DecompressZSTD(dst, src []byte) ([]byte, error) {
	decompressCalls.Inc()
	b, err := zstd.Decompress(dst, src)
	if err != nil {
		return b, fmt.Errorf("cannot decompress zstd block with len=%d to a buffer with len=%d: %w; block data (hex): %X", len(src), len(dst), err, src)
	}
	return b, nil
}

var (
	compressCalls   = metrics.NewCounter(`vm_zstd_block_compress_calls_total`)
	decompressCalls = metrics.NewCounter(`vm_zstd_block_decompress_calls_total`)

	originalBytes   = metrics.NewCounter(`vm_zstd_block_original_bytes_total`)
	compressedBytes = metrics.NewCounter(`vm_zstd_block_compressed_bytes_total`)
)
