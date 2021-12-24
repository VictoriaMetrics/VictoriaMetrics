package limiter

import (
	"io"
)

// NewWriteLimiter creates a new WriteLimiter object
// for the give writer and Limiter.
func NewWriteLimiter(w io.Writer, limiter *Limiter) *WriteLimiter {
	return &WriteLimiter{
		writer:  w,
		limiter: limiter,
	}
}

// WriteLimiter limits the amount of bytes written
// per second via Write() method.
// Must be created via NewWriteLimiter.
type WriteLimiter struct {
	writer  io.Writer
	limiter *Limiter
}

// Close implements io.Closer
// also calls Close for wrapped io.WriteCloser
func (wl *WriteLimiter) Close() error {
	if c, ok := wl.writer.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// Write implements io.Writer
func (wl *WriteLimiter) Write(p []byte) (n int, err error) {
	wl.limiter.Register(len(p))
	return wl.writer.Write(p)
}
