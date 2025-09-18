package chunkedbuffer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

const chunkSize = 4 * 1024

// Get returns Buffer from the pool.
//
// Return back the Buffer to the pool via Put() call when it is no longer needed.
func Get() *Buffer {
	v := cbPool.Get()
	if v == nil {
		return &Buffer{}
	}
	return v.(*Buffer)
}

// Put returns cb to the pool, so it could be reused via Get() call.
//
// The cb cannot be used after Put() call.
func Put(cb *Buffer) {
	cb.Reset()
	cbPool.Put(cb)
}

var cbPool sync.Pool

// Buffer provides in-memory buffer optimized for storing big bytes volumes.
//
// It stores the data in chunks of fixed size. This reduces memory fragmentation
// and memory waste comparing to the contiguous slices of bytes.
type Buffer struct {
	chunks []*[chunkSize]byte

	// offset is the offset in the last chunk to write data to.
	offset int
}

// Reset resets the cb, so it can be reused for writing new data into it.
//
// Reset frees up memory chunks allocated for cb, so they could be reused by other Buffer instances.
func (cb *Buffer) Reset() {
	for _, chunk := range cb.chunks {
		putChunk(chunk)
	}
	clear(cb.chunks)
	cb.chunks = cb.chunks[:0]

	cb.offset = 0
}

// SizeBytes returns the number of bytes occupied by the cb.
func (cb *Buffer) SizeBytes() int {
	return len(cb.chunks) * chunkSize
}

// Len returns the length of the data stored at cb.
func (cb *Buffer) Len() int {
	if len(cb.chunks) == 0 {
		return 0
	}
	return (len(cb.chunks)-1)*chunkSize + cb.offset
}

// MustWrite writes p to cb.
func (cb *Buffer) MustWrite(p []byte) {
	for len(p) > 0 {
		if len(cb.chunks) == 0 || cb.offset == chunkSize {
			chunk := getChunk()
			cb.chunks = append(cb.chunks, chunk)
			cb.offset = 0
		}
		dst := cb.chunks[len(cb.chunks)-1]
		n := copy(dst[cb.offset:], p)
		cb.offset += n
		p = p[n:]
	}
}

// Write implements io.Writer interface for cb.
func (cb *Buffer) Write(p []byte) (int, error) {
	cb.MustWrite(p)
	return len(p), nil
}

// MustReadAt reads len(p) bytes from cb at the offset off.
func (cb *Buffer) MustReadAt(p []byte, off int64) {
	if len(p) == 0 {
		return
	}

	chunkIdx := off / chunkSize
	offset := off % chunkSize

	chunk := cb.chunks[chunkIdx]
	n := copy(p, chunk[offset:])
	p = p[n:]

	for len(p) > 0 {
		chunkIdx++
		chunk := cb.chunks[chunkIdx]
		n := copy(p, chunk[:])
		p = p[n:]
	}
}

// ReadFrom reads all the data from r and appends it to cb.
func (cb *Buffer) ReadFrom(r io.Reader) (int64, error) {
	v := copyBufPool.Get()
	if v == nil {
		v = new([16 * 1024]byte)
	}
	b := (v.(*[16 * 1024]byte))[:]

	bytesRead := int64(0)
	for {
		n, err := r.Read(b)
		cb.MustWrite(b[:n])
		bytesRead += int64(n)
		if err != nil {
			copyBufPool.Put(v)
			if errors.Is(err, io.EOF) {
				return bytesRead, nil
			}
			return bytesRead, err
		}
	}
}

var copyBufPool sync.Pool

// WriteTo writes cb data to w.
func (cb *Buffer) WriteTo(w io.Writer) (int64, error) {
	bLen := cb.Len()
	if bLen == 0 {
		return 0, nil
	}

	switch t := w.(type) {
	case *bytesutil.ByteBuffer:
		t.Grow(bLen)
	case *bytes.Buffer:
		t.Grow(bLen)
	}

	nTotal := 0

	// Write all the chunks except the last one, which may be incomplete.
	for _, chunk := range cb.chunks[:len(cb.chunks)-1] {
		n, err := w.Write(chunk[:])
		nTotal += n
		if err != nil {
			return int64(nTotal), err
		}
		if n != chunkSize {
			return int64(nTotal), fmt.Errorf("unexpected number of bytes written; got %d; want %d", n, chunkSize)
		}
	}

	// Write the last chunk
	chunk := cb.chunks[len(cb.chunks)-1]
	n, err := w.Write(chunk[:cb.offset])
	nTotal += n
	if err != nil {
		return int64(nTotal), err
	}
	if n != cb.offset {
		return int64(nTotal), fmt.Errorf("unexpected number of bytes written; got %d; want %d", n, cb.offset)
	}

	return int64(nTotal), nil
}

// MustWriteTo writes cb contents w.
//
// Use this function only if w cannot return errors. For example, if w is bytes.Buffer of bytesutil.ByteBuffer.
// If w can return errors, then use WriteTo function instead.
func (cb *Buffer) MustWriteTo(w io.Writer) {
	if _, err := cb.WriteTo(w); err != nil {
		logger.Panicf("BUG: unexpected error writing Buffer data to the provided writer: %s", err)
	}
}

// Path returns cb path.
func (cb *Buffer) Path() string {
	return fmt.Sprintf("Buffer/%p/mem", cb)
}

// MustClose closes cb for subsequent reuse.
func (cb *Buffer) MustClose() {
	// Do nothing, since certain code rely on cb reading after MustClose call.
}

// NewReader returns a reader for reading the data stored in cb.
func (cb *Buffer) NewReader() filestream.ReadCloser {
	return &reader{
		cb: cb,
	}
}

type reader struct {
	cb *Buffer

	// offset is the offset at cb to read the next data at Read call.
	offset int
}

func (r *reader) Path() string {
	return r.cb.Path()
}

func (r *reader) Read(p []byte) (int, error) {
	chunkIdx := r.offset / chunkSize
	offset := r.offset % chunkSize

	if chunkIdx == len(r.cb.chunks) {
		if offset != 0 {
			panic(fmt.Errorf("BUG: offset must be 0; got %d", offset))
		}
		return 0, io.EOF
	}

	chunk := r.cb.chunks[chunkIdx]
	if chunkIdx == len(r.cb.chunks)-1 {
		// read the last chunk
		n := copy(p, chunk[offset:r.cb.offset])
		r.offset += n
		if offset+n == r.cb.offset {
			return n, io.EOF
		}
		return n, nil
	}
	n := copy(p, chunk[offset:])
	r.offset += n
	return n, nil
}

func (r *reader) MustClose() {
	r.cb = nil
	r.offset = 0
}

func getChunk() *[chunkSize]byte {
	v := chunkPool.Get()
	if v == nil {
		return new([chunkSize]byte)
	}
	return v.(*[chunkSize]byte)
}

func putChunk(chunk *[chunkSize]byte) {
	chunkPool.Put(chunk)
}

var chunkPool sync.Pool
