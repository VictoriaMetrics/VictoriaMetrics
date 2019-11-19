package fslocal

import (
	"io"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type bandwidthLimiter struct {
	perSecondLimit int

	c *sync.Cond

	// quota for the current second
	quota int
}

func newBandwidthLimiter(perSecondLimit int) *bandwidthLimiter {
	if perSecondLimit <= 0 {
		logger.Panicf("BUG: perSecondLimit must be positive; got %d", perSecondLimit)
	}
	var bl bandwidthLimiter
	bl.perSecondLimit = perSecondLimit
	var mu sync.Mutex
	bl.c = sync.NewCond(&mu)
	go bl.perSecondUpdater()
	return &bl
}

func (bl *bandwidthLimiter) NewReadCloser(rc io.ReadCloser) *bandwidthLimitedReader {
	return &bandwidthLimitedReader{
		rc: rc,
		bl: bl,
	}
}

func (bl *bandwidthLimiter) NewWriteCloser(wc io.WriteCloser) *bandwidthLimitedWriter {
	return &bandwidthLimitedWriter{
		wc: wc,
		bl: bl,
	}
}

type bandwidthLimitedReader struct {
	rc io.ReadCloser
	bl *bandwidthLimiter
}

func (blr *bandwidthLimitedReader) Read(p []byte) (int, error) {
	quota := blr.bl.GetQuota(len(p))
	return blr.rc.Read(p[:quota])
}

func (blr *bandwidthLimitedReader) Close() error {
	return blr.rc.Close()
}

type bandwidthLimitedWriter struct {
	wc io.WriteCloser
	bl *bandwidthLimiter
}

func (blw *bandwidthLimitedWriter) Write(p []byte) (int, error) {
	nn := 0
	for len(p) > 0 {
		quota := blw.bl.GetQuota(len(p))
		n, err := blw.wc.Write(p[:quota])
		nn += n
		if err != nil {
			return nn, err
		}
		p = p[quota:]
	}
	return nn, nil
}

func (blw *bandwidthLimitedWriter) Close() error {
	return blw.wc.Close()
}

func (bl *bandwidthLimiter) perSecondUpdater() {
	tc := time.NewTicker(time.Second)
	c := bl.c
	for range tc.C {
		c.L.Lock()
		bl.quota = bl.perSecondLimit
		c.Signal()
		c.L.Unlock()
	}
}

// GetQuota returns the number in the range [1..n] - the allowed quota for now.
//
// The function blocks until at least 1 can be returned from it.
func (bl *bandwidthLimiter) GetQuota(n int) int {
	if n <= 0 {
		logger.Panicf("BUG: n must be positive; got %d", n)
	}
	c := bl.c
	c.L.Lock()
	for bl.quota <= 0 {
		c.Wait()
	}
	quota := bl.quota
	if quota > n {
		quota = n
	}
	bl.quota -= quota
	if bl.quota > 0 {
		c.Signal()
	}
	c.L.Unlock()
	return quota
}
