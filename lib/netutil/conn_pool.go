package netutil

import (
	"fmt"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
)

// ConnPool is a connection pool with ZSTD-compressed connections.
type ConnPool struct {
	mu sync.Mutex
	d  *TCPDialer

	name             string
	handshakeFunc    handshake.Func
	compressionLevel int

	conns []*handshake.BufferedConn
}

// NewConnPool creates a new connection pool for the given addr.
//
// Name is used in exported metrics.
// handshakeFunc is used for handshaking after the connection establishing.
// The compression is disabled if compressionLevel <= 0.
func NewConnPool(name, addr string, handshakeFunc handshake.Func, compressionLevel int) *ConnPool {
	return &ConnPool{
		d: NewTCPDialer(name, addr),

		name:             name,
		handshakeFunc:    handshakeFunc,
		compressionLevel: compressionLevel,
	}
}

// Addr returns the address where connections are established.
func (cp *ConnPool) Addr() string {
	return cp.d.addr
}

// Get returns free connection from the pool.
func (cp *ConnPool) Get() (*handshake.BufferedConn, error) {
	var bc *handshake.BufferedConn
	cp.mu.Lock()
	if len(cp.conns) > 0 {
		bc = cp.conns[len(cp.conns)-1]
		cp.conns[len(cp.conns)-1] = nil
		cp.conns = cp.conns[:len(cp.conns)-1]
	}
	cp.mu.Unlock()
	if bc != nil {
		return bc, nil
	}

	// Pool is empty. Create new connection.
	c, err := cp.d.Dial()
	if err != nil {
		return nil, fmt.Errorf("cannot dial %s: %w", cp.d.Addr(), err)
	}
	if bc, err = cp.handshakeFunc(c, cp.compressionLevel); err != nil {
		err = fmt.Errorf("cannot perform %q handshake with server %q: %w", cp.name, cp.d.Addr(), err)
		_ = c.Close()
		return nil, err
	}
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
	cp.conns = append(cp.conns, bc)
	cp.mu.Unlock()
}
