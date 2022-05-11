package netutil

import (
	"fmt"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
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
}

type connWithTimestamp struct {
	bc             *handshake.BufferedConn
	lastActiveTime uint64
}

// NewConnPool creates a new connection pool for the given addr.
//
// Name is used in exported metrics.
// handshakeFunc is used for handshaking after the connection establishing.
// The compression is disabled if compressionLevel <= 0.
func NewConnPool(name, addr string, handshakeFunc handshake.Func, compressionLevel int) *ConnPool {
	cp := &ConnPool{
		d:                 NewTCPDialer(name, addr),
		concurrentDialsCh: make(chan struct{}, 8),

		name:             name,
		handshakeFunc:    handshakeFunc,
		compressionLevel: compressionLevel,
	}
	_ = metrics.NewGauge(fmt.Sprintf(`vm_tcpdialer_conns_idle{name=%q, addr=%q}`, name, addr), func() float64 {
		cp.mu.Lock()
		n := len(cp.conns)
		cp.mu.Unlock()
		return float64(n)
	})
	connPoolsMu.Lock()
	connPools = append(connPools, cp)
	connPoolsMu.Unlock()
	return cp
}

// Addr returns the address where connections are established.
func (cp *ConnPool) Addr() string {
	return cp.d.addr
}

// Get returns free connection from the pool.
func (cp *ConnPool) Get() (*handshake.BufferedConn, error) {
	if bc := cp.tryGetConn(); bc != nil {
		return bc, nil
	}

	// Limit the number of concurrent dials.
	// This should help https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2552
	cp.concurrentDialsCh <- struct{}{}
	defer func() {
		<-cp.concurrentDialsCh
	}()
	// Make an attempt to get already established connections from the pool.
	// It may appear there while waiting for cp.concurrentDialsCh.
	if bc := cp.tryGetConn(); bc != nil {
		return bc, nil
	}
	// Pool is empty. Create new connection.
	c, err := cp.d.Dial()
	if err != nil {
		return nil, fmt.Errorf("cannot dial %s: %w", cp.d.Addr(), err)
	}
	bc, err := cp.handshakeFunc(c, cp.compressionLevel)
	if err != nil {
		err = fmt.Errorf("cannot perform %q handshake with server %q: %w", cp.name, cp.d.Addr(), err)
		_ = c.Close()
		return nil, err
	}
	return bc, nil
}

func (cp *ConnPool) tryGetConn() *handshake.BufferedConn {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if len(cp.conns) == 0 {
		return nil
	}
	c := cp.conns[len(cp.conns)-1]
	bc := c.bc
	c.bc = nil
	cp.conns = cp.conns[:len(cp.conns)-1]
	return bc
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
	cp.conns = append(cp.conns, connWithTimestamp{
		bc:             bc,
		lastActiveTime: fasttime.UnixTimestamp(),
	})
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

func init() {
	go func() {
		for {
			time.Sleep(17 * time.Second)
			connPoolsMu.Lock()
			for _, cp := range connPools {
				cp.closeIdleConns()
			}
			connPoolsMu.Unlock()
		}
	}()
}

var connPoolsMu sync.Mutex
var connPools []*ConnPool
