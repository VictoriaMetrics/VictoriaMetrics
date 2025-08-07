//go:build !cgo

package zstd

import (
	"io"
	"sync"

	"github.com/klauspost/compress/zstd"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// Reader is zstd reader
type Reader struct {
	d *zstd.Decoder
}

// NewReader returns zstd reader for the given r.
func NewReader(r io.Reader) *Reader {
	d, err := zstd.NewReader(r, zstd.WithDecoderConcurrency(1))
	if err != nil {
		logger.Panicf("BUG: unexpected error returned when creating ZSTD reader: %s", err)
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

// GetReader returns Reader for reading zstd-uncompressed data from r.
//
// When the reader is no longer needed, return back it to the pool via PutReader().
func GetReader(r io.Reader) *Reader {
	v := readerPool.Get()
	if v == nil {
		return NewReader(r)
	}
	zr := v.(*Reader)
	if err := zr.d.Reset(r); err != nil {
		logger.Panicf("BUG: unexpected error when resetting ZSTD reader: %s", err)
	}
	return zr
}

// PutReader returns zr to the pool, so it could be reused via GetReader.
func PutReader(zr *Reader) {
	if err := zr.d.Reset(nil); err != nil {
		logger.Panicf("BUG: unexpected error when resetting ZSTD reader: %s", err)
	}
	readerPool.Put(zr)
}

var readerPool sync.Pool

// Writer is zstd writer
type Writer struct {
	e     *zstd.Encoder
	level int
}

// NewWriterLevel returns zstd writer for the given w and level.
func NewWriterLevel(w io.Writer, level int) *Writer {
	l := zstd.EncoderLevelFromZstd(level)
	e, err := zstd.NewWriter(w, zstd.WithEncoderLevel(l))
	if err != nil {
		logger.Panicf("BUG: failed to create ZSTD writer: %s", err)
	}
	return &Writer{
		e:     e,
		level: level,
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

// Close flushes the pending data to the underlying writer and finishes the compressed stream.
func (w *Writer) Close() error {
	return w.e.Close()
}

// Release releases w.
func (w *Writer) Release() {
	w.e.Reset(nil)
	w.e = nil
}

// GetWriter returns Writer for writing zstd-compressed data to w.
//
// When the writer is no longer needed, return back it to the pool via PutWriter.
func GetWriter(w io.Writer, level int) *Writer {
	p := getWriterPool(level)

	v := p.Get()
	if v == nil {
		return NewWriterLevel(w, level)
	}
	zw := v.(*Writer)
	zw.e.Reset(w)
	return zw
}

// PutWriter returns zw to the pool, so it could be reused via GetWriter.
func PutWriter(zw *Writer) {
	zw.e.Reset(nil)

	p := getWriterPool(zw.level)
	p.Put(zw)
}

func getWriterPool(level int) *sync.Pool {
	l := zstd.EncoderLevelFromZstd(level)

	writersPoolLock.Lock()
	p := writersPool[l]
	if p == nil {
		p = &sync.Pool{}
		writersPool[l] = p
	}
	writersPoolLock.Unlock()

	return p
}

var (
	writersPoolLock sync.Mutex
	writersPool     = make(map[zstd.EncoderLevel]*sync.Pool)
)
