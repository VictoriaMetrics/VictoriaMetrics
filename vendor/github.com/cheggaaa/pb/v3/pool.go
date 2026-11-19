// +build linux darwin freebsd netbsd openbsd solaris dragonfly windows plan9 aix

package pb

import (
	"io"
	"sync"
	"time"

	"github.com/cheggaaa/pb/v3/termutil"
)

// Create and start new pool with given bars
// You need call pool.Stop() after work
func StartPool(pbs ...*ProgressBar) (pool *Pool, err error) {
	pool = new(Pool)
	if err = pool.Start(); err != nil {
		return
	}
	pool.Add(pbs...)
	return
}

// NewPool initialises a pool with progress bars, but
// doesn't start it. You need to call Start manually
func NewPool(pbs ...*ProgressBar) (pool *Pool) {
	pool = new(Pool)
	pool.Add(pbs...)
	return
}

type Pool struct {
	Output        io.Writer
	RefreshRate   time.Duration
	bars          []*ProgressBar
	lastBarsCount int
	shutdownCh    chan struct{}
	workerCh      chan struct{}
	m             sync.Mutex
	finishOnce    sync.Once
}

// Add progress bars.
// A bar that is already started stops writing on its own and is no longer
// IsStarted; the pool renders it from now on.
func (p *Pool) Add(pbs ...*ProgressBar) {
	for _, bar := range pbs {
		bar.Set(Static, true)
		if !bar.stopWriter() {
			bar.Start()
		}
	}
	p.m.Lock()
	defer p.m.Unlock()
	p.bars = append(p.bars, pbs...)
}

// visibleBars returns the bars to draw, the last rows of them when rows > 0.
// Finished CleanOnFinish bars are hidden; they stay in the pool so that a
// restarted bar shows up again. finished reports whether every bar is finished.
func (p *Pool) visibleBars(rows int) (bars []*ProgressBar, finished bool) {
	finished = true
	bars = make([]*ProgressBar, 0, len(p.bars))
	for _, bar := range p.bars {
		if bar.IsFinished() {
			if bar.GetBool(CleanOnFinish) {
				continue
			}
		} else {
			finished = false
		}
		bars = append(bars, bar)
	}
	if rows > 0 && len(bars) > rows {
		// we need to hide bars that overflow terminal height
		bars = bars[len(bars)-rows:]
	}
	return
}

func (p *Pool) Start() (err error) {
	p.RefreshRate = defaultRefreshRate
	p.shutdownCh, err = termutil.RawModeOn()
	if err != nil {
		return
	}
	p.workerCh = make(chan struct{})
	go p.writer()
	return
}

func (p *Pool) writer() {
	var first = true
	defer func() {
		if first == false {
			p.print(false)
		} else {
			p.print(true)
			p.print(false)
		}
		close(p.workerCh)
	}()

	for {
		select {
		case <-time.After(p.RefreshRate):
			if p.print(first) {
				p.print(false)
				return
			}
			first = false
		case <-p.shutdownCh:
			return
		}
	}
}

// Restore terminal state and close pool
func (p *Pool) Stop() error {
	p.finishOnce.Do(func() {
		if p.shutdownCh != nil {
			close(p.shutdownCh)
		}
	})

	// Wait for the worker to complete
	select {
	case <-p.workerCh:
	}

	return termutil.RawModeOff()
}
