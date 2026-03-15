package ioutil

import (
	"io"
	"sync"
)

// GetFirstByteReader returns FirstByteReader wrapper for r.
//
// The returned FirstByteReader must be returned back to the pool when no longer needed, by calling PutFirstByteReader().
func GetFirstByteReader(r io.Reader) *FirstByteReader {
	v := fbrPool.Get()
	if v == nil {
		v = &FirstByteReader{}
	}
	fbr := v.(*FirstByteReader)
	fbr.r = r
	return fbr
}

// PutFirstByteReader returns fbr to the pool.
//
// The fbr cannot be used after returning to the pool.
func PutFirstByteReader(fbr *FirstByteReader) {
	fbr.reset()
	fbrPool.Put(fbr)
}

var fbrPool sync.Pool

// FirstByteReader is an io.Reader, which provides WaitForData() function for waiting for the next data from the wrapped reader.
//
// This reader is useful for postponing resource allocations after the next chunk of data is read from the wrapped reader.
type FirstByteReader struct {
	r io.Reader

	firstChunk     [16]byte
	firstChunkLen  int
	firstChunkErr  error
	firstChunkRead bool
}

func (fbr *FirstByteReader) reset() {
	fbr.r = nil
	fbr.firstChunkLen = 0
	fbr.firstChunkErr = nil
	fbr.firstChunkRead = false
}

// WaitForData waits for the next chunk of data from the wrapped reader at fbr.
func (fbr *FirstByteReader) WaitForData() {
	if !fbr.firstChunkRead {
		n, err := fbr.r.Read(fbr.firstChunk[:])
		fbr.firstChunkLen = n
		fbr.firstChunkErr = err
		fbr.firstChunkRead = true
	}
}

// Read implements io.Reader for fbr.
func (fbr *FirstByteReader) Read(p []byte) (int, error) {
	n := 0
	if fbr.firstChunkRead {
		n = copy(p, fbr.firstChunk[:fbr.firstChunkLen])
		if n < fbr.firstChunkLen {
			copy(fbr.firstChunk[:], fbr.firstChunk[n:])
			fbr.firstChunkLen -= n
			return n, fbr.firstChunkErr
		}

		p = p[n:]
		fbr.firstChunkLen = 0
		fbr.firstChunkRead = false
		if fbr.firstChunkErr != nil {
			return n, fbr.firstChunkErr
		}
	}

	n1, err := fbr.r.Read(p)
	n += n1

	return n, err
}
