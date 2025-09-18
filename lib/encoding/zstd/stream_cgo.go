//go:build cgo

package zstd

import (
	"io"
	"sync"

	"github.com/valyala/gozstd"
)

// Reader is zstd reader
type Reader = gozstd.Reader

// NewReader returns zstd reader for the given r.
func NewReader(r io.Reader) *Reader {
	return gozstd.NewReader(r)
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
	zr.Reset(r, nil)
	return zr
}

// PutReader returns zr to the pool, so it could be reused via GetReader.
func PutReader(zr *Reader) {
	// Do not call zr.Reset() in order to avoid CGO call.
	// The zr.Reset() is automatically called when zr is destroyed by Go GC.

	readerPool.Put(zr)
}

var readerPool sync.Pool

// Writer is zstd writer
type Writer = gozstd.Writer

// NewWriterLevel returns zstd writer for the given w and level.
func NewWriterLevel(w io.Writer, level int) *Writer {
	return gozstd.NewWriterLevel(w, level)
}

// GetWriter returns Writer for writing zstd-compressed data to w.
//
// When the writer is no longer needed, return back it to the pool via PutWriter.
func GetWriter(w io.Writer, compressLevel int) *Writer {
	v := writerPool.Get()
	if v == nil {
		return NewWriterLevel(w, compressLevel)
	}
	zw := v.(*Writer)
	zw.Reset(w, nil, compressLevel)
	return zw
}

// PutWriter returns zw to the pool, so it could be reused via GetWriter.
func PutWriter(zw *Writer) {
	// Do not call zw.Reset() in order to avoid CGO call.
	// The zw.Reset() is automatically called when zw is destroyed by Go GC.

	writerPool.Put(zw)
}

var writerPool sync.Pool
