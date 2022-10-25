package netutil

import (
	"fmt"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

// ConnPool is a connection pool with ZSTD-compressed connections.
type ConnPool struct {
	mu sync.Mutex
	d  *TCPDialer

	// concurrentDialsCh limits the number of concurrent dials the ConnPool can make.
	// This should prevent from creating an excees number of connections during temporary
	// spikes in workload at vmselect and vmstorage nodes.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2552
	concurrentDialsCh chan struct{}

	name             string
	handshakeFunc    handshake.Func
	compressionLevel int

	conns []connWithTimestamp

	isStopped bool

	// lastDialError contains the last error seen when dialing remote addr.
	// When it is non-nil and conns is empty, then ConnPool.Get() return this error.
	// This reduces the time needed for dialing unavailable remote storage systems.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/711#issuecomment-1160363187
	lastDialError error
}

type connWithTimestamp struct {
	bc             *handshake.BufferedConn
	lastActiveTime uint64
}

// NewConnPool creates a new connection pool for the given addr.
//
// Name is used in metrics registered at ms.
// handshakeFunc is used for handshaking after the connection establishing.
// The compression is disabled if compressionLevel <= 0.
//
// Call ConnPool.MustStop when the returned ConnPool is no longer needed.
func NewConnPool(ms *metrics.Set, name, addr string, handshakeFunc handshake.Func, compressionLevel int, dialTimeout time.Duration) *ConnPool {
	cp := &ConnPool{
		d:                 NewTCPDialer(ms, name, addr, dialTimeout),
		concurrentDialsCh: make(chan struct{}, 8),

		name:             name,
		handshakeFunc:    handshakeFunc,
		compressionLevel: compressionLevel,
	}
	cp.checkAvailability(true)
	_ = ms.NewGauge(fmt.Sprintf(`vm_tcpdialer_conns_idle{name=%q, addr=%q}`, name, addr), func() float64 {
		cp.mu.Lock()
		n := len(cp.conns)
		cp.mu.Unlock()
		return float64(n)
	})
	_ = ms.NewGauge(fmt.Sprintf(`vm_tcpdialer_addr_available{name=%q, addr=%q}`, name, addr), func() float64 {
		cp.mu.Lock()
		isAvailable := len(cp.conns) > 0 || cp.lastDialError == nil
		cp.mu.Unlock()
		if isAvailable {
			return 1
		}
		return 0
	})
	connPoolsMu.Lock()
	connPools = append(connPools, cp)
	connPoolsMu.Unlock()
	return cp
}

// MustStop frees up resources occupied by cp.
//
// ConnPool.Get() immediately returns an error after MustStop call.
// ConnPool.Put() immediately closes the connection returned to the pool.
func (cp *ConnPool) MustStop() {
	cp.mu.Lock()
	isStopped := cp.isStopped
	cp.isStopped = true
	for _, c := range cp.conns {
		_ = c.bc.Close()
	}
	cp.conns = nil
	cp.mu.Unlock()
	if isStopped {
		logger.Panicf("BUG: MustStop is called multiple times")
	}

	connPoolsMu.Lock()
	cpDeleted := false
	for i, cpTmp := range connPools {
		if cpTmp == cp {
			connPoolsNew := append(connPools[:i], connPools[i+1:]...)
			connPools[len(connPools)-1] = nil
			connPools = connPoolsNew
			cpDeleted = true
			break
		}
	}
	connPoolsMu.Unlock()
	if !cpDeleted {
		logger.Panicf("BUG: couldn't find the ConnPool in connPools")
	}
}

// Addr returns the address where connections are established.
func (cp *ConnPool) Addr() string {
	return cp.d.addr
}

// Get returns free connection from the pool.
func (cp *ConnPool) Get() (*handshake.BufferedConn, error) {
	bc, err := cp.tryGetConn()
	if err != nil {
		return nil, err
	}
	if bc != nil {
		// Fast path - obtained the connection from pool.
		return bc, nil
	}
	return cp.getConnSlow()
}

func (cp *ConnPool) getConnSlow() (*handshake.BufferedConn, error) {
	// Limit the number of concurrent dials.
	// This should help https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2552
	cp.concurrentDialsCh <- struct{}{}
	defer func() {
		<-cp.concurrentDialsCh
	}()
	// Make an attempt to get already established connections from the pool.
	// It may appear there while waiting for cp.concurrentDialsCh.
	bc, err := cp.tryGetConn()
	if err != nil {
		return nil, err
	}
	if bc != nil {
		return bc, nil
	}
	// Pool is empty. Create new connection.
	return cp.dialAndHandshake()
}

func (cp *ConnPool) dialAndHandshake() (*handshake.BufferedConn, error) {
	c, err := cp.d.Dial()
	if err != nil {
		err = fmt.Errorf("cannot dial %s: %w", cp.d.Addr(), err)
	}
	cp.mu.Lock()
	cp.lastDialError = err
	cp.mu.Unlock()
	if err != nil {
		return nil, err
	}
	bc, err := cp.handshakeFunc(c, cp.compressionLevel)
	if err != nil {
		// Do not put handshake error to cp.lastDialError, because handshake
		// is perfomed on an already established connection.
		err = fmt.Errorf("cannot perform %q handshake with server %q: %w", cp.name, cp.d.Addr(), err)
		_ = c.Close()
		return nil, err
	}
	return bc, err
}

func (cp *ConnPool) tryGetConn() (*handshake.BufferedConn, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.isStopped {
		return nil, fmt.Errorf("conn pool to %s cannot be used, since it is stopped", cp.d.addr)
	}
	if len(cp.conns) == 0 {
		return nil, cp.lastDialError
	}
	c := cp.conns[len(cp.conns)-1]
	bc := c.bc
	c.bc = nil
	cp.conns = cp.conns[:len(cp.conns)-1]
	return bc, nil
}

// Put puts bc back to the pool.
//
// Do not put broken and closed connections to the pool!
func (cp *ConnPool) Put(bc *handshake.BufferedConn) {
	if err := bc.SetDeadline(time.Time{}); err != nil {
		// Close the connection instead of returning it to the pool,
		// since it may be broken.
		_ = bc.Close()
		return
	}
	cp.mu.Lock()
	if cp.isStopped {
		_ = bc.Close()
	} else {
		cp.conns = append(cp.conns, connWithTimestamp{
			bc:             bc,
			lastActiveTime: fasttime.UnixTimestamp(),
		})
	}
	cp.mu.Unlock()
}

func (cp *ConnPool) closeIdleConns() {
	// Close connections, which were idle for more than 30 seconds.
	// This should reduce the number of connections after sudden spikes in query rate.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2508
	deadline := fasttime.UnixTimestamp() - 30
	var activeConns []connWithTimestamp
	cp.mu.Lock()
	conns := cp.conns
	for _, c := range conns {
		if c.lastActiveTime > deadline {
			activeConns = append(activeConns, c)
		} else {
			_ = c.bc.Close()
			c.bc = nil
		}
	}
	cp.conns = activeConns
	cp.mu.Unlock()
}

func (cp *ConnPool) checkAvailability(force bool) {
	cp.mu.Lock()
	isStopped := cp.isStopped
	hasDialError := cp.lastDialError != nil
	cp.mu.Unlock()
	if isStopped {
		return
	}
	if hasDialError || force {
		bc, _ := cp.dialAndHandshake()
		if bc != nil {
			cp.Put(bc)
		}
	}
}

func init() {
	go func() {
		for {
			time.Sleep(17 * time.Second)
			forEachConnPool(func(cp *ConnPool) {
				cp.closeIdleConns()
			})
		}
	}()
	go func() {
		for {
			time.Sleep(time.Second)
			forEachConnPool(func(cp *ConnPool) {
				cp.checkAvailability(false)
			})
		}
	}()
}

var connPoolsMu sync.Mutex
var connPools []*ConnPool

func forEachConnPool(f func(cp *ConnPool)) {
	connPoolsMu.Lock()
	var wg sync.WaitGroup
	for _, cp := range connPools {
		wg.Add(1)
		go func(cp *ConnPool) {
			defer wg.Done()
			f(cp)
		}(cp)
	}
	wg.Wait()
	connPoolsMu.Unlock()
}
