//go:build cgo
// +build cgo

package zstd

import (
	"io"

	"github.com/valyala/gozstd"
)

// Reader is zstd reader
type Reader = gozstd.Reader

// NewReader returns zstd reader for the given r.
func NewReader(r io.Reader) *Reader {
	return gozstd.NewReader(r)
}

// Writer is zstd writer
type Writer = gozstd.Writer

// NewWriterLevel returns zstd writer for the given w and level.
func NewWriterLevel(w io.Writer, level int) *Writer {
	return gozstd.NewWriterLevel(w, level)
}
