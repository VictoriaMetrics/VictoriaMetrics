package logsql

import (
	"io"
	"sync"
)

func getBufferedWriter() *bufferedWriter {
	v := bufferedWriterPool.Get()
	if v == nil {
		return &bufferedWriter{}
	}
	return v.(*bufferedWriter)
}

func putBufferedWriter(bw *bufferedWriter) {
	bw.reset()
	bufferedWriterPool.Put(bw)
}

var bufferedWriterPool sync.Pool

type bufferedWriter struct {
	w   io.Writer
	buf []byte
}

func (bw *bufferedWriter) reset() {
	bw.w = nil
	bw.buf = bw.buf[:0]
}

func (bw *bufferedWriter) Init(w io.Writer, bufLen int) {
	bw.reset()
	bw.w = w

	buf := bw.buf
	if n := bufLen - cap(buf); n > 0 {
		buf = append(buf[:cap(buf)], make([]byte, n)...)
	}
	bw.buf = buf[:0]
}

func (bw *bufferedWriter) Write(p []byte) (int, error) {
	buf := bw.buf
	if len(buf)+len(p) <= cap(buf) {
		bw.buf = append(buf, p...)
		return len(p), nil
	}
	if len(buf) > 0 {
		if _, err := bw.w.Write(buf); err != nil {
			return 0, err
		}
		buf = buf[:0]
	}
	if len(p) <= cap(buf) {
		bw.buf = append(buf, p...)
		return len(p), nil
	}
	bw.buf = buf
	return bw.w.Write(p)
}

func (bw *bufferedWriter) FlushIgnoreErrors() {
	buf := bw.buf
	if len(buf) > 0 {
		_, _ = bw.w.Write(buf)
		bw.buf = buf[:0]
	}
}
