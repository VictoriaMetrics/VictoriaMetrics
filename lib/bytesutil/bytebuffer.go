package bytesutil

import (
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var (
	// Verify ByteBuffer implements the given interfaces.
	_ io.Writer           = &ByteBuffer{}
	_ fs.MustReadAtCloser = &ByteBuffer{}
	_ io.ReaderFrom       = &ByteBuffer{}

	// Verify reader implement filestream.ReadCloser interface.
	_ filestream.ReadCloser = &reader{}
)

// ByteBuffer implements a simple byte buffer.
type ByteBuffer struct {
	// B is the underlying byte slice.
	B []byte
}

// Reset resets bb.
func (bb *ByteBuffer) Reset() {
	bb.B = bb.B[:0]
}

// Write appends p to bb.
func (bb *ByteBuffer) Write(p []byte) (int, error) {
	bb.B = append(bb.B, p...)
	return len(p), nil
}

// MustReadAt reads len(p) bytes starting from the given offset.
func (bb *ByteBuffer) MustReadAt(p []byte, offset int64) {
	if offset < 0 {
		logger.Panicf("BUG: cannot read at negative offset=%d", offset)
	}
	if offset > int64(len(bb.B)) {
		logger.Panicf("BUG: too big offset=%d; cannot exceed len(bb.B)=%d", offset, len(bb.B))
	}
	if n := copy(p, bb.B[offset:]); n < len(p) {
		logger.Panicf("BUG: EOF occurred after reading %d bytes out of %d bytes at offset %d", n, len(p), offset)
	}
}

// ReadFrom reads all the data from r to bb until EOF.
func (bb *ByteBuffer) ReadFrom(r io.Reader) (int64, error) {
	b := bb.B
	bLen := len(b)
	b = Resize(b, 4*1024)
	b = b[:cap(b)]
	offset := bLen
	for {
		if free := len(b) - offset; free < offset {
			n := len(b)
			b = append(b, make([]byte, n)...)
		}
		n, err := r.Read(b[offset:])
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

// MustClose closes bb for subsequent re-use.
func (bb *ByteBuffer) MustClose() {
	// Do nothing, since certain code rely on bb reading after MustClose call.
}

// NewReader returns new reader for the given bb.
func (bb *ByteBuffer) NewReader() filestream.ReadCloser {
	return &reader{
		bb: bb,
	}
}

type reader struct {
	bb *ByteBuffer

	// readOffset is the offset in bb.B for read.
	readOffset int
}

// Read reads up to len(p) bytes from bb.
func (r *reader) Read(p []byte) (int, error) {
	var err error
	n := copy(p, r.bb.B[r.readOffset:])
	if n < len(p) {
		err = io.EOF
	}
	r.readOffset += n
	return n, err
}

// MustClose closes bb for subsequent re-use.
func (r *reader) MustClose() {
	r.bb = nil
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
