package netutil

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
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

var (
	concurrentDialLimit = 8
)

// InitConcurrentDialLimit initiates the concurrentDialLimit with value between 8 and 64 based on the given concurrentRequestLimit.
// It must be called before NewConnPool call.
func InitConcurrentDialLimit(concurrentRequestLimit int) {
	// It should be initialized with`sync.Once`. Since it's used in only one place,
	// extra code has been removed for simplicity.
	limit := max(concurrentRequestLimit/2, 8)
	concurrentDialLimit = min(limit, 64)
}

// NewConnPool creates a new connection pool for the given addr.
//
// Name is used in metrics registered at ms.
// handshakeFunc is used for handshaking after the connection establishing.
// The compression is disabled if compressionLevel <= 0.
//
// Call ConnPool.MustStop when the returned ConnPool is no longer needed.
func NewConnPool(ms *metrics.Set, name, addr string, handshakeFunc handshake.Func, compressionLevel int, dialTimeout, userTimeout time.Duration) *ConnPool {
	cp := &ConnPool{
		d:                 NewTCPDialer(ms, name, addr, dialTimeout, userTimeout),
		concurrentDialsCh: make(chan struct{}, concurrentDialLimit),

		name:             name,
		handshakeFunc:    handshakeFunc,
		compressionLevel: compressionLevel,
	}
	cp.checkAvailability(true)
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
	c, err := cp.tryGetConn()
	if err != nil {
		return nil, err
	}
	if c.bc != nil {
		// Fast path - obtained the connection from pool.
		return c.bc, nil
	}
	return cp.getConnSlow()
}

func (cp *ConnPool) healthCheck(bc *handshake.BufferedConn) bool {
	logger.Infof("jayice health check")
	time.Sleep(6 * time.Second)

	funcName := "healthCheck_v1"
	buf := []byte(funcName)
	sizeBuf := encoding.MarshalUint64(nil, uint64(len(buf)))
	if _, err := bc.Write(sizeBuf); err != nil {
		return false
	}
	_, err := bc.Write(buf)
	if err != nil {
		return false
	}
	var trace [1]byte
	_, err = bc.Write(trace[:])
	if err != nil {
		return false
	}
	timeout := encoding.MarshalUint32(nil, 5)
	_, err = bc.Write(timeout)
	if err != nil {
		return false
	}
	if err = bc.Flush(); err != nil {
		return false
	}

	var resp [8]byte
	if _, err = io.ReadFull(bc, resp[:]); err != nil {
		return false
	}
	n := encoding.UnmarshalUint64(resp[:])
	logger.Infof("jayice health check success:%d", n)

	if _, err = io.ReadFull(bc, resp[:]); err != nil {
		return false
	}
	n = encoding.UnmarshalUint64(resp[:])
	logger.Infof("jayice health check trace size:%d", n)

	return true
}

func (cp *ConnPool) GetV2() (*handshake.BufferedConn, error) {
	c, err := cp.tryGetConn()
	if err == nil && c.bc != nil {
		// fast path: fresh connection don't need health check
		if fasttime.UnixTimestamp()-c.lastActiveTime < 30000 {
			logger.Infof("jayice1")
			return c.bc, nil
		}
	} else {
		//
		return cp.getConnSlow()
	}

	// slow path
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	connChannel := make(chan *handshake.BufferedConn, 1)
	done := make(chan struct{})
	defer close(done)
	go func() {
		defer close(connChannel)
		if c.bc != nil {
			logger.Infof("jayice4")
			if cp.healthCheck(c.bc) {
				select {
				case connChannel <- c.bc:
				case <-done:
					logger.Infof("jayice iam timeout")
					cp.Put(c.bc)
				}
				return
			} else {
				logger.Infof("jayice health check failed")
				_ = c.bc.Close()
				c.bc = nil
			}
		}
		for {
			select {
			case <-done:
				if c.bc != nil {
					_ = c.bc.Close()
				}
				return
			default:
			}
			logger.Infof("jayice test1")

			c, err = cp.tryGetConn()
			if err != nil || c.bc == nil {
				select {
				case connChannel <- nil:
				case <-done:
				}
				return
			}
			if fasttime.UnixTimestamp()-c.lastActiveTime < 30000 || cp.healthCheck(c.bc) {
				select {
				case connChannel <- c.bc:
				case <-done:
					cp.Put(c.bc)
				}
				return
			} else {
				_ = c.bc.Close()
				c.bc = nil
				continue
			}
		}
	}()

	select {
	case conn := <-connChannel:
		logger.Infof("jayice2")
		if conn != nil {
			return conn, nil
		}
		return cp.getConnSlow()
	case <-timeout.C:
		logger.Infof("jayice3")
		return nil, fmt.Errorf("error")
	}
}

func (cp *ConnPool) getConnSlow() (*handshake.BufferedConn, error) {
	for {
		select {
		// Limit the number of concurrent dials.
		// This should help https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2552
		case cp.concurrentDialsCh <- struct{}{}:
			// Create new connection.
			conn, err := cp.dialAndHandshake()
			<-cp.concurrentDialsCh
			return conn, err
		default:
			// Make attempt to get already established connections from the pool.
			// It may appear there while waiting for cp.concurrentDialsCh.
			c, err := cp.tryGetConn()
			if err != nil {
				return nil, err
			}
			if c.bc == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return c.bc, nil
		}
	}
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

func (cp *ConnPool) tryGetConn() (connWithTimestamp, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.isStopped {
		return connWithTimestamp{}, fmt.Errorf("conn pool to %s cannot be used, since it is stopped", cp.d.addr)
	}
	if len(cp.conns) == 0 {
		return connWithTimestamp{}, cp.lastDialError
	}
	logger.Infof("jayice con len:%d", len(cp.conns))
	c := cp.conns[len(cp.conns)-1]
	cp.conns = cp.conns[:len(cp.conns)-1]
	return c, nil
}

// Put puts bc back to the pool.
//
// Do not put broken and closed connections to the pool!
func (cp *ConnPool) Put(bc *handshake.BufferedConn) {
	logger.Infof("jayice put")
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
	// Close connections, which were idle for more than 120 seconds.
	// This should reduce the number of connections after sudden spikes in query rate.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2508
	deadline := fasttime.UnixTimestamp() - 120
	var activeConns []connWithTimestamp
	var closeConns []connWithTimestamp
	cp.mu.Lock()

	// fast path, if there are less than 3 connections in the pool.
	if len(cp.conns) < 3 {
		cp.mu.Unlock()
		return
	}

	conns := cp.conns
	for _, c := range conns {
		if c.lastActiveTime > deadline {
			activeConns = append(activeConns, c)
		} else {
			closeConns = append(closeConns, c)
		}
	}
	for _, c := range closeConns {
		if len(activeConns) < 3 {
			activeConns = append(activeConns, c)
			continue
		}

		_ = c.bc.Close()
		c.bc = nil
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
