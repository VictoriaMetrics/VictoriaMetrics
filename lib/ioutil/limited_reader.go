package ioutil

import (
	"io"
	"sync"
)

// GetLimitedReader returns LimitedReader for reading up to n bytes from r.
//
// Return the used LimitedReader to pool when no longer needed via PutLimitedReader().
func GetLimitedReader(r io.Reader, n int64) *io.LimitedReader {
	v := limitedReaderPool.Get()
	if v == nil {
		v = &io.LimitedReader{}
	}
	lr := v.(*io.LimitedReader)
	lr.R = r
	lr.N = n
	return lr
}

// PutLimitedReader returns lr to the pool, so it could be re-used via GetLimitedReader().
//
// lr cannot be used after returning to the pool.
func PutLimitedReader(lr *io.LimitedReader) {
	lr.R = nil
	lr.N = 0
	limitedReaderPool.Put(lr)
}

var limitedReaderPool sync.Pool
