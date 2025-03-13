//go:build cgo

package zstd

import (
	"io"

	"github.com/valyala/gozstd"
)

// Reader is zstd reader
type Reader struct {
	d *gozstd.Reader
}

// NewReader returns zstd reader for the given r.
func NewReader(r io.Reader) *Reader {
	return &Reader{
		d: gozstd.NewReader(r),
	}
}

// Close implements io.ReadCloser interface
func (r *Reader) Close() error {
	r.d.Reset(nil, nil)
	return nil
}

// Reset updates supplied stream r.
func (r *Reader) Reset(reader io.Reader) {
	r.d.Reset(reader, nil)
}

// Read reads up to len(p) bytes to p from r.
func (r *Reader) Read(p []byte) (int, error) {
	return r.d.Read(p)
}

// Release releases r.
//
// r cannot be used after the release.
func (r *Reader) Release() {
	r.d.Release()
	r.d = nil
}

// Writer is zstd writer
type Writer = gozstd.Writer

// NewWriterLevel returns zstd writer for the given w and level.
func NewWriterLevel(w io.Writer, level int) *Writer {
	return gozstd.NewWriterLevel(w, level)
}
