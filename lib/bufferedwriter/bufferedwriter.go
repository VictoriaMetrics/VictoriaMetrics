package bufferedwriter

import (
	"bufio"
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
)

// Get returns buffered writer for the given w.
//
// The writer must be returned to the pool after use by calling Put().
func Get(w io.Writer) *Writer {
	v := writerPool.Get()
	if v == nil {
		v = &Writer{
			// By default net/http.Server uses 4KB buffers, which are flushed to client with chunked responses.
			// These buffers may result in visible overhead for responses exceeding a few megabytes.
			// So allocate 64Kb buffers.
			bw: bufio.NewWriterSize(w, 64*1024),
		}
	}
	bw := v.(*Writer)
	bw.bw.Reset(w)
	return bw
}

// Put returns back bw to the pool.
//
// bw cannot be used after returning to the pool.
func Put(bw *Writer) {
	bw.reset()
	writerPool.Put(bw)
}

var writerPool sync.Pool

// Writer is buffered writer, which may be used in order to reduce overhead
// when sending moderately big responses to http server.
//
// Writer methods can be called from concurrently running goroutines.
// The writer remembers the first occurred error, which can be inspected with Error method.
type Writer struct {
	lock sync.Mutex
	bw   *bufio.Writer
	err  error
}

func (bw *Writer) reset() {
	bw.bw.Reset(nil)
	bw.err = nil
}

// Write writes p to bw.
func (bw *Writer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	bw.lock.Lock()
	defer bw.lock.Unlock()
	if bw.err != nil {
		return 0, bw.err
	}
	n, err := bw.bw.Write(p)
	if err != nil {
		bw.err = fmt.Errorf("cannot send %d bytes to client: %w", len(p), err)
	}
	return n, bw.err
}

// Flush flushes bw to the underlying writer.
//
// Connection close errors are ignored to not trigger on them and to not write to logs, but Write method doesn't ignore
// them since it may lead to an unexpected behaviour (see https://github.com/VictoriaMetrics/VictoriaMetrics/pull/8157)
func (bw *Writer) Flush() error {
	bw.lock.Lock()
	defer bw.lock.Unlock()
	if bw.err != nil {
		if netutil.IsTrivialNetworkError(bw.err) {
			return nil
		}
		return bw.err
	}
	if err := bw.bw.Flush(); err != nil {
		bw.err = fmt.Errorf("cannot flush data to client: %w", err)
		if netutil.IsTrivialNetworkError(err) {
			return nil
		}
	}
	return bw.err
}

// Error returns the first occurred error in bw.
func (bw *Writer) Error() error {
	bw.lock.Lock()
	defer bw.lock.Unlock()
	return bw.err
}
