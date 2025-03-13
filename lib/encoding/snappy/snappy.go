package snappy

import (
	"github.com/golang/snappy"
	"io"
)

// NewReader returns snappy reader for the given r.
func NewReader(r io.Reader) *Reader {
	return &Reader{
		d: snappy.NewReader(r),
	}
}

// Reader is snappy reader
type Reader struct {
	d *snappy.Reader
}

// Close implements io.ReadCloser interface
func (r *Reader) Close() error {
	r.Reset(nil)
	return nil
}

// Reset updates
func (r *Reader) Reset(reader io.Reader) {
	r.d.Reset(reader)
}

// Read reads up to len(p) bytes to p from r.
func (r *Reader) Read(p []byte) (int, error) {
	return r.d.Read(p)
}
