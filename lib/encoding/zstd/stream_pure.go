//go:build !cgo
// +build !cgo

package zstd

import (
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/klauspost/compress/zstd"
)

// Reader is zstd reader
type Reader struct {
	d *zstd.Decoder
}

// NewReader returns zstd reader for the given r.
func NewReader(r io.Reader) *Reader {
	d, err := zstd.NewReader(r)
	if err != nil {
		logger.Panicf("BUG: failed to create ZSTD reader: %s", err)
	}
	return &Reader{
		d: d,
	}
}

// Read reads up to len(p) bytes to p from r.
func (r *Reader) Read(p []byte) (int, error) {
	return r.d.Read(p)
}

// Release releases r.
func (r *Reader) Release() {
	r.d.Close()
	r.d = nil
}

// Writer is zstd writer
type Writer struct {
	e *zstd.Encoder
}

// NewWriterLevel returns zstd writer for the given w and level.
func NewWriterLevel(w io.Writer, level int) *Writer {
	l := zstd.EncoderLevelFromZstd(level)
	e, err := zstd.NewWriter(w, zstd.WithEncoderLevel(l))
	if err != nil {
		logger.Panicf("BUG: failed to create ZSTD writer: %s", err)
	}
	return &Writer{
		e: e,
	}
}

// Write writes p to w.
func (w *Writer) Write(p []byte) (int, error) {
	return w.e.Write(p)
}

// Flush flushes all the pending data from w to the underlying writer.
func (w *Writer) Flush() error {
	return w.e.Flush()
}

// Release releases w.
func (w *Writer) Release() {
	w.e.Reset(nil)
	w.e = nil
}
