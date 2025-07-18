package logstorage

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

type cache struct {
	curr atomic.Pointer[sync.Map]
	prev atomic.Pointer[sync.Map]

	stopCh chan struct{}
	wg     sync.WaitGroup
}

func newCache() *cache {
	var c cache
	c.curr.Store(&sync.Map{})
	c.prev.Store(&sync.Map{})

	c.stopCh = make(chan struct{})
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.runCleaner()
	}()
	return &c
}

func (c *cache) MustStop() {
	close(c.stopCh)
	c.wg.Wait()
}

func (c *cache) runCleaner() {
	d := timeutil.AddJitterToDuration(3 * time.Minute)
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			c.clean()
		case <-c.stopCh:
			return
		}
	}
}

func (c *cache) clean() {
	curr := c.curr.Load()
	c.prev.Store(curr)
	c.curr.Store(&sync.Map{})
}

func (c *cache) Get(k []byte) (any, bool) {
	kStr := bytesutil.ToUnsafeString(k)

	curr := c.curr.Load()
	v, ok := curr.Load(kStr)
	if ok {
		return v, true
	}

	prev := c.prev.Load()
	v, ok = prev.Load(kStr)
	if ok {
		kStr = strings.Clone(kStr)
		curr.Store(kStr, v)
		return v, true
	}
	return nil, false
}

func (c *cache) Set(k []byte, v any) {
	kStr := string(k)
	curr := c.curr.Load()
	curr.Store(kStr, v)
}
