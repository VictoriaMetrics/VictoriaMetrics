// +build !cgo

package zstd

import (
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/klauspost/compress/zstd"
)

var (
	decoder *zstd.Decoder

	mu sync.Mutex
	av atomic.Value
)

type registry map[int]*zstd.Encoder

func init() {
	r := make(registry)
	av.Store(r)

	var err error
	decoder, err = zstd.NewReader(nil)
	if err != nil {
		logger.Panicf("BUG: failed to create ZSTD reader: %s", err)
	}
}

// Decompress appends decompressed src to dst and returns the result.
func Decompress(dst, src []byte) ([]byte, error) {
	return decoder.DecodeAll(src, dst)
}

// CompressLevel appends compressed src to dst and returns the result.
//
// The given compressionLevel is used for the compression.
func CompressLevel(dst, src []byte, compressionLevel int) []byte {
	e := getEncoder(compressionLevel)
	return e.EncodeAll(src, dst)
}

func getEncoder(compressionLevel int) *zstd.Encoder {
	r := av.Load().(registry)
	if e, ok := r[compressionLevel]; ok {
		return e
	}

	level := zstd.EncoderLevelFromZstd(compressionLevel)
	e, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(level))
	if err != nil {
		logger.Panicf("BUG: failed to create ZSTD writer: %s", err)
	}

	mu.Lock()
	r1 := av.Load().(registry)
	r2 := make(registry)
	for k, v := range r1 {
		r2[k] = v
	}
	r2[compressionLevel] = e
	av.Store(r2)
	mu.Unlock()

	return e
}
