// Package zstdkp provides zstd compression through github.com/klauspost/compress/zstd.
package zstdkp

import (
	"io"
	"sync"

	"github.com/klauspost/compress/gzhttp/writer"
	"github.com/klauspost/compress/zstd"
)

// zstdWriterPools stores a sync.Pool for each compression level for reuse of
// zstd.Encoders. Use poolIndex to convert a compression level to an index.
var zstdWriterPools [zstd.SpeedBestCompression - zstd.SpeedFastest + 1]*sync.Pool

func init() {
	for i := zstd.SpeedFastest; i <= zstd.SpeedBestCompression; i++ {
		addLevelPool(i)
	}
}

func poolIndex(level int) int {
	if level > int(zstd.SpeedBestCompression) {
		level = int(zstd.SpeedBestCompression)
	}
	if level < int(zstd.SpeedFastest) {
		level = int(zstd.SpeedFastest)
	}
	return level - int(zstd.SpeedFastest)
}

func addLevelPool(level zstd.EncoderLevel) {
	zstdWriterPools[poolIndex(int(level))] = &sync.Pool{
		New: func() any {
			w, _ := zstd.NewWriter(nil,
				zstd.WithEncoderLevel(level),
				zstd.WithEncoderConcurrency(1),
				zstd.WithLowerEncoderMem(true),
				zstd.WithWindowSize(128<<10),
			)
			return w
		},
	}
}

type pooledWriter struct {
	*zstd.Encoder
	index int
}

func (pw *pooledWriter) Close() error {
	if pw.Encoder == nil {
		return nil
	}
	err := pw.Encoder.Close()
	pw.Encoder.Reset(nil)
	zstdWriterPools[pw.index].Put(pw.Encoder)
	pw.Encoder = nil
	return err
}

// NewWriter returns a pooled zstd writer. The writer is returned to the pool on Close.
func NewWriter(w io.Writer, level int) writer.ZstdWriter {
	index := poolIndex(level)
	enc := zstdWriterPools[index].Get().(*zstd.Encoder)
	enc.Reset(w)
	return &pooledWriter{
		Encoder: enc,
		index:   index,
	}
}

// Levels returns the supported compression level range.
func Levels() (min, max int) {
	return int(zstd.SpeedFastest), int(zstd.SpeedBestCompression)
}
