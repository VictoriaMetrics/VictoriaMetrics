package syncwg

import (
	"sync"
)

// WaitGroup wraps sync.WaitGroup and makes safe to call Add/Wait
// from concurrent goroutines.
//
// An additional limitation is that call to Wait prohibits further calls to Add
// until return.
type WaitGroup struct {
	sync.WaitGroup
	mu sync.Mutex
}

// Add registers n additional workers. Add may be called from concurrent goroutines.
func (wg *WaitGroup) Add(n int) {
	wg.mu.Lock()
	wg.WaitGroup.Add(n)
	wg.mu.Unlock()
}

// Wait waits until all the goroutines call Done.
//
// Wait may be called from concurrent goroutines.
//
// Further calls to Add are blocked until return from Wait.
func (wg *WaitGroup) Wait() {
	wg.mu.Lock()
	wg.WaitGroup.Wait()
	wg.mu.Unlock()
}

// WaitAndBlock waits until all the goroutines call Done and then prevents
// from new goroutines calling Add.
//
// Further calls to Add are always blocked. This is useful for graceful shutdown
// when other goroutines calling Add must be stopped.
//
// wg cannot be used after this call.
func (wg *WaitGroup) WaitAndBlock() {
	wg.mu.Lock()
	wg.WaitGroup.Wait()

	// Do not unlock wg.mu, so other goroutines calling Add are blocked.
}

// There is no need in wrapping WaitGroup.Done, since it is already goroutine-safe.
