//go:build !cgo

package zstd

import (
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/klauspost/compress/zstd"
)

var (
	decodersMu sync.Mutex
	decoders   atomic.Value

	mu sync.Mutex

	// do not use atomic.Pointer, since the stored map there is already a pointer type.
	av atomic.Value
)

func init() {
	r := make(map[zstd.EncoderLevel]*zstd.Encoder)
	av.Store(r)

	var err error
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		logger.Panicf("BUG: failed to create ZSTD reader: %s", err)
	}
	d := make(map[int]*zstd.Decoder)
	d[0] = decoder
	decoders.Store(d)
}

// Decompress appends decompressed src to dst and returns the result.
//
// This function must be called only for the trusted src.
func Decompress(dst, src []byte) ([]byte, error) {
	d := getDecoder(0)
	return d.DecodeAll(src, dst)
}

// Decompress appends decompressed src to dst and returns the result.
func DecompressLimited(dst, src []byte, maxDataSizeBytes int) ([]byte, error) {
	d := getDecoder(maxDataSizeBytes)
	return d.DecodeAll(src, dst)
}

// CompressLevel appends compressed src to dst and returns the result.
//
// The given compressionLevel is used for the compression.
func CompressLevel(dst, src []byte, compressionLevel int) []byte {
	// Convert the compressionLevel to the real compression level supported by github.com/klauspost/compress/zstd
	// This allows saving memory on caching zstd.Encoder instances per each level,
	// since the number of real compression levels at github.com/klauspost/compress/zstd
	// is smaller than the number of zstd compression levels.
	// See https://github.com/klauspost/compress/discussions/1025
	realCompressionLevel := zstd.EncoderLevelFromZstd(compressionLevel)

	e := getEncoder(realCompressionLevel)
	return e.EncodeAll(src, dst)
}

func getDecoder(maxMemory int) *zstd.Decoder {
	r := decoders.Load().(map[int]*zstd.Decoder)
	d := r[maxMemory]
	if d != nil {
		return d
	}
	decodersMu.Lock()
	// Create the decoder under lock in order to prevent from wasted work
	// when concurrent goroutines create decoder for the same compressionLevel.
	r1 := decoders.Load().(map[int]*zstd.Decoder)
	if d = r1[maxMemory]; d == nil {
		var err error
		d, err = zstd.NewReader(nil, zstd.WithDecoderMaxMemory(uint64(maxMemory)))
		if err != nil {
			logger.Panicf("BUG: failed to create ZSTD reader: %s", err)
		}
		r2 := make(map[int]*zstd.Decoder)
		for k, v := range r1 {
			r2[k] = v
		}
		r2[maxMemory] = d
		decoders.Store(r2)
	}
	decodersMu.Unlock()

	return d
}

func getEncoder(compressionLevel zstd.EncoderLevel) *zstd.Encoder {
	r := av.Load().(map[zstd.EncoderLevel]*zstd.Encoder)
	e := r[compressionLevel]
	if e != nil {
		return e
	}

	mu.Lock()
	// Create the encoder under lock in order to prevent from wasted work
	// when concurrent goroutines create encoder for the same compressionLevel.
	r1 := av.Load().(map[zstd.EncoderLevel]*zstd.Encoder)
	if e = r1[compressionLevel]; e == nil {
		e = newEncoder(compressionLevel)
		r2 := make(map[zstd.EncoderLevel]*zstd.Encoder)
		for k, v := range r1 {
			r2[k] = v
		}
		r2[compressionLevel] = e
		av.Store(r2)
	}
	mu.Unlock()

	return e
}

func newEncoder(compressionLevel zstd.EncoderLevel) *zstd.Encoder {
	e, err := zstd.NewWriter(nil,
		zstd.WithEncoderCRC(false), // Disable CRC for performance reasons.
		zstd.WithEncoderLevel(compressionLevel))
	if err != nil {
		logger.Panicf("BUG: failed to create ZSTD writer: %s", err)
	}
	return e
}
