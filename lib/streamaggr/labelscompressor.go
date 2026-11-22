package streamaggr

import (
	"flag"
	"sync"
	"sync/atomic"
	"time"
)

var labelsCompressorCleanupInterval = flag.Duration("streamAggr.labelsCompressorCleanupInterval", time.Minute, "How often to free labels that stream aggregation no longer uses. "+
	"Zero disables it. See https://docs.victoriametrics.com/victoriametrics/stream-aggregation/#high-resource-usage")

var (
	lcUsersLock sync.Mutex

	// lcUsers holds keysGoneBeforeMsec of every running aggregator and deduplicator.
	// It is the time before which no key in its state was compressed.
	lcUsers = map[*atomic.Int64]struct{}{}

	lcNextCleanupMsec atomic.Int64

	// lcCleanupRunning is set while a cleanup runs in the background.
	lcCleanupRunning atomic.Bool

	// lcCleanupWG tracks the cleanups running in the background.
	lcCleanupWG sync.WaitGroup

	// lcClockMsec is the latest time seen by Push calls and flushes. Labels moved to standby
	// never get an earlier time, so they aren't freed too early if the clock goes back.
	lcClockMsec atomic.Int64
)

func registerLabelsCompressorUser(keysGoneBeforeMsec *atomic.Int64) {
	lcUsersLock.Lock()
	lcUsers[keysGoneBeforeMsec] = struct{}{}
	lcUsersLock.Unlock()
}

func unregisterLabelsCompressorUser(keysGoneBeforeMsec *atomic.Int64) {
	lcUsersLock.Lock()
	delete(lcUsers, keysGoneBeforeMsec)
	lcUsersLock.Unlock()
}

// cleanupLabelsCompressorIfDue starts cleanupLabelsCompressor at most once per -streamAggr.labelsCompressorCleanupInterval.
// It runs in the background, so it doesn't delay the flush.
func cleanupLabelsCompressorIfDue() {
	interval := *labelsCompressorCleanupInterval
	if interval <= 0 {
		return
	}
	nowMsec := time.Now().UnixMilli()
	next := lcNextCleanupMsec.Load()
	// next is more than one interval ahead only if the clock went back.
	if nowMsec < next && next-nowMsec <= interval.Milliseconds() {
		return
	}
	if !lcNextCleanupMsec.CompareAndSwap(next, nowMsec+interval.Milliseconds()) {
		return
	}
	// Skip this run if the previous cleanup is still running.
	if !lcCleanupRunning.CompareAndSwap(false, true) {
		return
	}
	lcCleanupWG.Go(func() {
		cleanupLabelsCompressor()
		lcCleanupRunning.Store(false)
	})
}

// cleanupLabelsCompressor frees labels that no running aggregator or deduplicator can still use.
func cleanupLabelsCompressor() int {
	lcUsersLock.Lock()
	keysGoneBeforeMsec := time.Now().UnixMilli()
	for u := range lcUsers {
		keysGoneBeforeMsec = min(keysGoneBeforeMsec, u.Load())
	}
	lcUsersLock.Unlock()
	return lc.Cleanup(time.UnixMilli(keysGoneBeforeMsec), labelsCompressorNow)
}

func labelsCompressorNow() time.Time {
	return time.UnixMilli(max(time.Now().UnixMilli(), lcClockMsec.Load()))
}

func advanceLabelsCompressorClock(msec int64) {
	for {
		clock := lcClockMsec.Load()
		if msec <= clock || lcClockMsec.CompareAndSwap(clock, msec) {
			return
		}
	}
}

// pushesInFlight tracks the start times of the Push calls in flight, without locks.
// A Push counts itself in the current generation. The flusher starts a new generation
// when no Push of the previous one is in flight.
type pushesInFlight struct {
	gen    atomic.Uint64
	counts [2]atomic.Int64

	// startMsec holds the start time of each generation. It is used only by the flusher.
	startMsec [2]int64
}

// start registers a Push and returns its start time and the count to pass to done.
func (p *pushesInFlight) start() (time.Time, *atomic.Int64) {
	for {
		gen := p.gen.Load()
		count := &p.counts[gen%2]
		count.Add(1)
		if p.gen.Load() == gen {
			now := time.Now()
			advanceLabelsCompressorClock(now.UnixMilli())
			return now, count
		}
		count.Add(-1)
	}
}

func (p *pushesInFlight) done(count *atomic.Int64) {
	count.Add(-1)
}

// oldestMsec returns a time no later than the start of any Push in flight.
//
// It must be called by one goroutine at a time.
func (p *pushesInFlight) oldestMsec() int64 {
	nowMsec := time.Now().UnixMilli()
	advanceLabelsCompressorClock(nowMsec)
	gen := p.gen.Load()
	if p.counts[(gen-1)%2].Load() > 0 {
		return p.startMsec[(gen-1)%2]
	}
	// No Push of the previous generation is in flight, so start a new generation.
	p.startMsec[(gen+1)%2] = nowMsec
	p.gen.Store(gen + 1)
	if p.counts[gen%2].Load() > 0 {
		return p.startMsec[gen%2]
	}
	return nowMsec
}
