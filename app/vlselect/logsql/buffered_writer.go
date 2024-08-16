package logsql

import (
	"bufio"
	"io"
	"sync"
)

func getBufferedWriter(w io.Writer) *bufferedWriter {
	v := bufferedWriterPool.Get()
	if v == nil {
		return &bufferedWriter{
			bw: bufio.NewWriter(w),
		}
	}
	bw := v.(*bufferedWriter)
	bw.bw.Reset(w)
	return bw
}

func putBufferedWriter(bw *bufferedWriter) {
	bw.reset()
	bufferedWriterPool.Put(bw)
}

var bufferedWriterPool sync.Pool

type bufferedWriter struct {
	mu sync.Mutex
	bw *bufio.Writer
}

func (bw *bufferedWriter) reset() {
	// nothing to do
}

func (bw *bufferedWriter) WriteIgnoreErrors(p []byte) {
	bw.mu.Lock()
	_, _ = bw.bw.Write(p)
	bw.mu.Unlock()
}

func (bw *bufferedWriter) FlushIgnoreErrors() {
	bw.mu.Lock()
	_ = bw.bw.Flush()
	bw.mu.Unlock()
}
