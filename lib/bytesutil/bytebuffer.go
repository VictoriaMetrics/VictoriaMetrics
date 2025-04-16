package bytesutil

import (
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// ByteBuffer implements a simple byte buffer.
type ByteBuffer struct {
	// B is the underlying byte slice.
	B []byte
}

// Path returns an unique id for bb.
func (bb *ByteBuffer) Path() string {
	return fmt.Sprintf("ByteBuffer/%p/mem", bb)
}

// Reset resets bb.
func (bb *ByteBuffer) Reset() {
	bb.B = bb.B[:0]
}

// Len returns the length of the data stored in bb.
func (bb *ByteBuffer) Len() int {
	return len(bb.B)
}

// MustWrite writes p to bb.
func (bb *ByteBuffer) MustWrite(p []byte) {
	bb.B = append(bb.B, p...)
}

// Grow grows bb capacity, so it can accept n bytes without additional allocations.
func (bb *ByteBuffer) Grow(n int) {
	bLen := len(bb.B)
	bb.B = slicesutil.SetLength(bb.B, bLen+n)
	bb.B = bb.B[:bLen]
}

// Write appends p to bb.
func (bb *ByteBuffer) Write(p []byte) (int, error) {
	bb.MustWrite(p)
	return len(p), nil
}

// WriteTo writes bb contents to w.
func (bb *ByteBuffer) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(bb.B)
	return int64(n), err
}

// ReadFrom reads all the data from r to bb until EOF.
func (bb *ByteBuffer) ReadFrom(r io.Reader) (int64, error) {
	b := bb.B
	bLen := len(b)
	if cap(b) < 4*1024 {
		// Pre-allocate at least 4KiB
		b = slicesutil.SetLength(b, 4*1024)
	}
	offset := bLen
	for {
		if free := cap(b) - offset; free < (cap(b) / 16) {
			// grow slice by 30% similar to how Go does this
			// https://go.googlesource.com/go/+/2dda92ff6f9f07eeb110ecbf0fc2d7a0ddd27f9d
			// higher growth rates could consume excessive memory when reading big amounts of data.
			n := int(1.3 * float64(cap(b)))
			b = slicesutil.SetLength(b, n)
		}
		n, err := r.Read(b[offset:cap(b)])
		offset += n
		if err != nil {
			bb.B = b[:offset]
			if err == io.EOF {
				err = nil
			}
			return int64(offset - bLen), err
		}
	}
}

// NewReader returns new reader for the given bb.
func (bb *ByteBuffer) NewReader() filestream.ReadCloser {
	return &reader{
		bb:   bb,
		data: ToUnsafeString(bb.B),
	}
}

type reader struct {
	// bb is used only for Path() call.
	bb *ByteBuffer

	// data to read.
	data string

	// readOffset is the offset in the data for read.
	readOffset int
}

// Path returns an unique id for the underlying ByteBuffer.
func (r *reader) Path() string {
	return r.bb.Path()
}

// Read reads up to len(p) bytes from bb.
func (r *reader) Read(p []byte) (int, error) {
	var err error
	n := copy(p, r.data[r.readOffset:])
	if n < len(p) {
		err = io.EOF
	}
	r.readOffset += n
	return n, err
}

// MustClose closes bb for subsequent reuse.
func (r *reader) MustClose() {
	r.bb = nil
	r.data = ""
	r.readOffset = 0
}

// ByteBufferPool is a pool of ByteBuffers.
type ByteBufferPool struct {
	p sync.Pool
}

// Get obtains a ByteBuffer from bbp.
func (bbp *ByteBufferPool) Get() *ByteBuffer {
	bbv := bbp.p.Get()
	if bbv == nil {
		return &ByteBuffer{}
	}
	return bbv.(*ByteBuffer)
}

// Put puts bb into bbp.
func (bbp *ByteBufferPool) Put(bb *ByteBuffer) {
	bb.Reset()
	bbp.p.Put(bb)
}
